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
)

var ErrUnsupportedInputFormat = errors.New("unsupported input format")

func LoadFile(path string) (Scenario, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("open scenario file: %w", err)
	}

	if looksLikeScenarioJSON(raw) {
		return decodeScenario(raw)
	}

	if sc, err := scenarioFromTerraformPlan(raw, path); err == nil {
		return sc, nil
	} else if !errors.Is(err, ErrUnsupportedInputFormat) {
		return Scenario{}, err
	}

	if sc, err := scenarioFromKubernetesManifest(raw, path); err == nil {
		return sc, nil
	} else if !errors.Is(err, ErrUnsupportedInputFormat) {
		return Scenario{}, err
	}

	return Scenario{}, ErrUnsupportedInputFormat
}

func decodeScenario(raw []byte) (Scenario, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()

	var sc Scenario
	if err := dec.Decode(&sc); err != nil {
		return Scenario{}, fmt.Errorf("decode scenario JSON: %w", err)
	}
	if dec.More() {
		return Scenario{}, fmt.Errorf("scenario JSON contains multiple values")
	}

	if err := validate(sc); err != nil {
		return Scenario{}, err
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

func scenarioFromTerraformPlan(raw []byte, path string) (Scenario, error) {
	var payload map[string]interface{}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Scenario{}, ErrUnsupportedInputFormat
	}
	if !looksLikeTerraformPlan(payload) {
		return Scenario{}, ErrUnsupportedInputFormat
	}
	resourceCount, destroyCount, resourceAddresses := summarizeTerraformPlan(payload)
	sc := Scenario{
		ScenarioID: terraformScenarioID(path, raw),
		Actor:      Actor{Type: "human", ID: "cli"},
		Source:     "cli",
		Timestamp:  time.Now().UTC(),
		Actions: []Action{
			{
				Kind: "terraform.plan",
				Target: map[string]interface{}{
					"file": filepath.Base(path),
				},
				Intent: fmt.Sprintf("plan detected from %s", filepath.Base(path)),
				Payload: map[string]interface{}{
					"resource_count":     resourceCount,
					"destroy_count":      destroyCount,
					"publicly_exposed":   false,
					"plan_json":          filepath.Base(path),
					"resource_addresses": resourceAddresses,
				},
			},
		},
	}
	return sc, nil
}

func scenarioFromKubernetesManifest(raw []byte, path string) (Scenario, error) {
	docs, err := parseYAMLDocs(raw)
	if err != nil {
		return Scenario{}, err
	}
	if len(docs) == 0 {
		return Scenario{}, ErrUnsupportedInputFormat
	}
	manifest := docs[0]
	if _, ok := manifest["apiVersion"]; !ok {
		return Scenario{}, ErrUnsupportedInputFormat
	}
	if _, ok := manifest["kind"]; !ok {
		return Scenario{}, ErrUnsupportedInputFormat
	}
	namespace := manifestNamespace(manifest)
	sc := Scenario{
		ScenarioID: kubernetesScenarioID(path, raw),
		Actor:      Actor{Type: "human", ID: "cli"},
		Source:     "cli",
		Timestamp:  time.Now().UTC(),
		Actions: []Action{
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

func summarizeTerraformPlan(payload map[string]interface{}) (resourceCount, destroyCount int, addresses []string) {
	list, _ := payload["resource_changes"].([]interface{})
	resourceCount = len(list)
	seen := map[string]struct{}{}
	for _, entry := range list {
		obj, _ := entry.(map[string]interface{})
		if address, _ := obj["address"].(string); address != "" {
			if _, ok := seen[address]; !ok {
				addresses = append(addresses, address)
				seen[address] = struct{}{}
			}
		}
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
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func validate(sc Scenario) error {
	if strings.TrimSpace(sc.ScenarioID) == "" {
		return fmt.Errorf("scenario_id required")
	}
	if len(sc.Actions) == 0 {
		return fmt.Errorf("scenario must contain at least one action")
	}
	return nil
}

func manifestNamespace(manifest map[string]interface{}) string {
	metadata, _ := manifest["metadata"].(map[string]interface{})
	if metadata == nil {
		return ""
	}
	if ns, _ := metadata["namespace"].(string); ns != "" {
		return ns
	}
	return ""
}
