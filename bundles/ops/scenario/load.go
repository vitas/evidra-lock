package scenario

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

var ErrUnsupportedInputFormat = errors.New("unsupported input format")

func LoadFile(path string) (schema.Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return schema.Scenario{}, fmt.Errorf("open scenario file: %w", err)
	}

	if looksLikeScenarioJSON(raw) {
		return decodeScenario(raw)
	}

	if sc, err := scenarioFromTerraformPlan(raw, path); err == nil {
		return sc, nil
	} else if !errors.Is(err, ErrUnsupportedInputFormat) {
		return schema.Scenario{}, err
	}

	if sc, err := scenarioFromKubernetesManifest(raw, path); err == nil {
		return sc, nil
	} else if !errors.Is(err, ErrUnsupportedInputFormat) {
		return schema.Scenario{}, err
	}

	return schema.Scenario{}, ErrUnsupportedInputFormat
}

func decodeScenario(raw []byte) (schema.Scenario, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var sc schema.Scenario
	if err := dec.Decode(&sc); err != nil {
		return schema.Scenario{}, fmt.Errorf("decode scenario JSON: %w", err)
	}
	if dec.More() {
		return schema.Scenario{}, fmt.Errorf("scenario JSON contains multiple values")
	}

	if err := validate(sc); err != nil {
		return schema.Scenario{}, err
	}
	return sc, nil
}

func looksLikeScenarioJSON(raw []byte) bool {
	var probe map[string]interface{}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return false
	}
	_, hasActions := probe["actions"]
	return hasActions
}

func scenarioFromTerraformPlan(raw []byte, path string) (schema.Scenario, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return schema.Scenario{}, ErrUnsupportedInputFormat
	}
	if !looksLikeTerraformPlan(payload) {
		return schema.Scenario{}, ErrUnsupportedInputFormat
	}
	resourceCount, destroyCount := summarizeTerraformPlan(payload)
	sc := schema.Scenario{
		ScenarioID: terraformScenarioID(path, raw),
		Actor:      schema.Actor{Type: "human", ID: "cli"},
		Source:     "cli",
		Timestamp:  time.Now().UTC(),
		Actions: []schema.Action{
			{
				Kind: "terraform.plan",
				Target: map[string]interface{}{
					"file": filepath.Base(path),
				},
				Intent: fmt.Sprintf("plan detected from %s", filepath.Base(path)),
				Payload: map[string]interface{}{
					"resource_count":   resourceCount,
					"destroy_count":    destroyCount,
					"publicly_exposed": false,
					"plan_json":        filepath.Base(path),
				},
			},
		},
	}
	return sc, nil
}

func scenarioFromKubernetesManifest(raw []byte, path string) (schema.Scenario, error) {
	docs, err := parseYAMLDocs(raw)
	if err != nil {
		return schema.Scenario{}, err
	}
	if len(docs) == 0 {
		return schema.Scenario{}, ErrUnsupportedInputFormat
	}
	manifest := docs[0]
	if _, ok := manifest["apiVersion"]; !ok {
		return schema.Scenario{}, ErrUnsupportedInputFormat
	}
	if _, ok := manifest["kind"]; !ok {
		return schema.Scenario{}, ErrUnsupportedInputFormat
	}
	namespace := manifestNamespace(manifest)
	sc := schema.Scenario{
		ScenarioID: kubernetesScenarioID(path, raw),
		Actor:      schema.Actor{Type: "human", ID: "cli"},
		Source:     "cli",
		Timestamp:  time.Now().UTC(),
		Actions: []schema.Action{
			{
				Kind: "kubectl.apply",
				Target: map[string]interface{}{
					"namespace": namespace,
				},
				Intent: "apply manifest detected from CLI",
				Payload: map[string]interface{}{
					"manifests_ref": "inline",
					"inline_yaml":   string(raw),
					"namespace":     namespace,
				},
			},
		},
	}
	return sc, nil
}

func looksLikeTerraformPlan(payload map[string]interface{}) bool {
	if _, ok := payload["resource_changes"]; ok {
		return true
	}
	if _, ok := payload["planned_values"]; ok {
		return true
	}
	if _, ok := payload["configuration"]; ok {
		return true
	}
	return false
}

func summarizeTerraformPlan(payload map[string]interface{}) (resourceCount, destroyCount int) {
	list, _ := payload["resource_changes"].([]interface{})
	resourceCount = len(list)
	for _, entry := range list {
		obj, _ := entry.(map[string]interface{})
		change, _ := obj["change"].(map[string]interface{})
		actions, _ := change["actions"].([]interface{})
		for _, act := range actions {
			if s, _ := act.(string); strings.EqualFold(s, "delete") {
				destroyCount++
				break
			}
		}
	}
	return
}

func terraformScenarioID(path string, raw []byte) string {
	sum := sha256.Sum256(raw)
	base := filepath.Base(path)
	if base == "" {
		base = "terraform"
	}
	return fmt.Sprintf("tf-%s-%x", base, sum[:4])
}

func kubernetesScenarioID(path string, raw []byte) string {
	sum := sha256.Sum256(raw)
	base := filepath.Base(path)
	if base == "" {
		base = "k8s"
	}
	return fmt.Sprintf("k8s-%s-%x", base, sum[:4])
}

func parseYAMLDocs(raw []byte) ([]map[string]interface{}, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	docs := make([]map[string]interface{}, 0)
	for {
		var doc map[string]interface{}
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode yaml manifest: %w", err)
		}
		if len(doc) == 0 {
			continue
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func manifestNamespace(manifest map[string]interface{}) string {
	if meta, ok := manifest["metadata"].(map[string]interface{}); ok {
		if ns, ok := meta["namespace"].(string); ok && strings.TrimSpace(ns) != "" {
			return strings.ToLower(ns)
		}
	}
	return "default"
}

func validate(sc schema.Scenario) error {
	if strings.TrimSpace(sc.ScenarioID) == "" {
		return fmt.Errorf("scenario_id is required")
	}
	switch strings.TrimSpace(sc.Actor.Type) {
	case "human", "agent", "system":
	default:
		return fmt.Errorf("actor.type must be one of: human|agent|system")
	}
	switch strings.TrimSpace(sc.Source) {
	case "mcp", "cli", "ci":
	default:
		return fmt.Errorf("source must be one of: mcp|cli|ci")
	}
	if sc.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	if len(sc.Actions) == 0 {
		return fmt.Errorf("actions must contain at least one action")
	}
	for i, a := range sc.Actions {
		if strings.TrimSpace(a.Kind) == "" {
			return fmt.Errorf("actions[%d].kind is required", i)
		}
		if strings.TrimSpace(a.Intent) == "" {
			return fmt.Errorf("actions[%d].intent is required", i)
		}
		if a.Target == nil {
			return fmt.Errorf("actions[%d].target is required", i)
		}
		if a.Payload == nil {
			return fmt.Errorf("actions[%d].payload is required", i)
		}
	}

	_ = time.RFC3339
	return nil
}
