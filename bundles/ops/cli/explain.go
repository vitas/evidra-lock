package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

const scenarioSchemaPath = "./bundles/ops/schema/scenario.schema.json"

type kindInfo struct {
	Kind        string
	PayloadHint string
	Description string
}

var supportedKinds = []kindInfo{
	{Kind: "terraform.plan", PayloadHint: `{"publicly_exposed":false,"destroy_count":0}`, Description: "Plan infrastructure changes and evaluate risk before apply."},
	{Kind: "kustomize.build", PayloadHint: `{"path":"./k8s"}`, Description: "Render Kubernetes manifests from kustomization."},
	{Kind: "kubectl.apply", PayloadHint: `{"manifest":"deploy.yaml"}`, Description: "Apply Kubernetes resources to a target namespace."},
	{Kind: "kubectl.delete", PayloadHint: `{"resource":"deployment/myapp"}`, Description: "Delete Kubernetes resources in a controlled path."},
	{Kind: "helm.upgrade", PayloadHint: `{"chart":"./chart","release":"app"}`, Description: "Upgrade or install a Helm release."},
}

func Explain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printExplainUsage(stderr)
		return 2
	}

	switch args[0] {
	case "schema":
		return explainSchema(stdout, stderr)
	case "kinds":
		return explainKinds(stdout)
	case "example":
		return explainExample(stdout)
	case "policies":
		return explainPolicies(args[1:], stdout, stderr)
	default:
		printExplainUsage(stderr)
		return 2
	}
}

func printExplainUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: evidra ops explain <schema|kinds|example|policies> [--verbose]")
}

func explainSchema(stdout, stderr io.Writer) int {
	if b, err := os.ReadFile(scenarioSchemaPath); err == nil {
		var v interface{}
		if err := json.Unmarshal(b, &v); err != nil {
			fmt.Fprintf(stderr, "invalid schema file %s: %v\n", scenarioSchemaPath, err)
			return 1
		}
		out, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "format schema: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(out))
		return 0
	}

	s := generatedScenarioSchema()
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "generate schema: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

func explainKinds(stdout io.Writer) int {
	for _, k := range supportedKinds {
		fmt.Fprintf(stdout, "- %s\n", k.Kind)
		fmt.Fprintf(stdout, "  payload: %s\n", k.PayloadHint)
		fmt.Fprintf(stdout, "  description: %s\n", k.Description)
	}
	return 0
}

func explainExample(stdout io.Writer) int {
	example := map[string]interface{}{
		"scenario_id": "sc-demo-001",
		"actor": map[string]string{
			"type": "agent",
			"id":   "agent-ops-1",
		},
		"source":    "mcp",
		"timestamp": "2026-02-21T00:00:00Z",
		"actions": []map[string]interface{}{
			{
				"kind":   "terraform.plan",
				"target": map[string]interface{}{"dir": "./infra"},
				"intent": "review infrastructure changes before deployment",
				"payload": map[string]interface{}{
					"publicly_exposed": false,
					"destroy_count":    0,
				},
				"risk_tags": []string{},
			},
		},
	}
	out, err := json.MarshalIndent(example, "", "  ")
	if err != nil {
		return 1
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

func explainPolicies(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("policies", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	verbose := fs.Bool("verbose", false, "Show raw policy file path and contents")
	if err := fs.Parse(args); err != nil {
		printExplainUsage(stderr)
		return 2
	}

	rules := []string{
		"1. Block k8s.apply to kube-system unless risk_tags includes breakglass.",
		"2. Block terraform.plan with payload.publicly_exposed=true unless risk_tags includes approved_public.",
		"3. Block terraform.plan when payload.destroy_count > 5.",
		"4. Block actions targeting namespace prod unless risk_tags includes change-approved.",
		"5. Block actions with missing or very short intent (<10 chars).",
		"6. Flag autonomous execution (actor.type=agent and source=mcp) as high-risk warning.",
	}

	fmt.Fprintln(stdout, "Active guardrail rules:")
	for _, r := range rules {
		fmt.Fprintln(stdout, r)
	}

	if *verbose {
		p := filepath.Clean("./bundles/ops/policies/policy.rego")
		fmt.Fprintf(stdout, "\nRaw policy (%s):\n", p)
		b, err := os.ReadFile(p)
		if err != nil {
			fmt.Fprintf(stderr, "read policy file: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, string(b))
	}
	return 0
}

func generatedScenarioSchema() map[string]interface{} {
	actionProps, actionRequired := structSchema(reflect.TypeOf(schema.Action{}))
	actorProps, actorRequired := structSchema(reflect.TypeOf(schema.Actor{}))
	scenarioProps, scenarioRequired := structSchema(reflect.TypeOf(schema.Scenario{}))

	scenarioProps["actor"] = map[string]interface{}{
		"type":       "object",
		"properties": actorProps,
		"required":   actorRequired,
	}
	scenarioProps["actions"] = map[string]interface{}{
		"type": "array",
		"items": map[string]interface{}{
			"type":       "object",
			"properties": actionProps,
			"required":   actionRequired,
		},
	}

	return map[string]interface{}{
		"$schema":    "https://json-schema.org/draft/2020-12/schema",
		"title":      "Evidra Ops Scenario",
		"type":       "object",
		"properties": scenarioProps,
		"required":   scenarioRequired,
	}
}

func structSchema(t reflect.Type) (map[string]interface{}, []string) {
	props := map[string]interface{}{}
	var required []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		jsonTag := f.Tag.Get("json")
		if jsonTag == "-" || jsonTag == "" {
			continue
		}
		name, omitEmpty := parseJSONTag(jsonTag)
		if name == "" {
			continue
		}
		props[name] = map[string]interface{}{"type": jsonType(f.Type)}
		if !omitEmpty {
			required = append(required, name)
		}
	}
	sort.Strings(required)
	return props, required
}

func parseJSONTag(tag string) (name string, omitEmpty bool) {
	parts := strings.Split(tag, ",")
	if len(parts) == 0 {
		return "", false
	}
	name = parts[0]
	for _, p := range parts[1:] {
		if strings.TrimSpace(p) == "omitempty" {
			omitEmpty = true
		}
	}
	return name, omitEmpty
}

func jsonType(t reflect.Type) string {
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	default:
		return "string"
	}
}

func NormalizeOutput(s string) string {
	var b bytes.Buffer
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		b.WriteString(strings.TrimRight(line, " \t"))
		b.WriteByte('\n')
	}
	return b.String()
}
