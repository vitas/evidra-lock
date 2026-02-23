package validators

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

const (
	payloadEnableValidators = "enable_validators"
	payloadPath             = "path"
	payloadOverlay          = "overlay"
	payloadManifestsRef     = "manifests_ref"
	payloadInlineYAML       = "inline_yaml"
	payloadSkipPlan         = "skip_plan"
	payloadManifestYAML     = "__manifest_yaml"
	payloadManifestFile     = "__manifest_file"
)

func shouldRunValidators(a schema.Action, globalEnable bool, enableOverride *bool) bool {
	if enableOverride != nil {
		globalEnable = *enableOverride
	}
	v, ok := a.Payload[payloadEnableValidators]
	if !ok {
		return globalEnable
	}
	b, ok := v.(bool)
	if !ok {
		return globalEnable
	}
	if b {
		return true
	}
	return globalEnable
}

func payloadString(payload map[string]interface{}, key string) string {
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	s, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func payloadBool(payload map[string]interface{}, key string) bool {
	raw, ok := payload[key]
	if !ok {
		return false
	}
	b, ok := raw.(bool)
	return ok && b
}

func payloadInt(payload map[string]interface{}, key string) int {
	raw, ok := payload[key]
	if !ok {
		return 0
	}
	switch n := raw.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func writeTempManifest(workdir, content string) (string, error) {
	dir := workdir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	f, err := os.CreateTemp(dir, ".evidra-manifest-*.yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return "", err
	}
	return f.Name(), nil
}

func readManifestForKubectl(action schema.Action, workdir string) (string, error) {
	ref := payloadString(action.Payload, payloadManifestsRef)
	switch ref {
	case "inline":
		s := payloadString(action.Payload, payloadInlineYAML)
		if s == "" {
			return "", fmt.Errorf("kubectl.apply payload.inline_yaml is required when manifests_ref=inline")
		}
		return s, nil
	case "":
		return "", fmt.Errorf("kubectl.apply payload.manifests_ref is required")
	default:
		p := ref
		if !filepath.IsAbs(p) && workdir != "" {
			p = filepath.Join(workdir, p)
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		return string(raw), nil
	}
}

func copyActionWithPayload(action schema.Action, payload map[string]interface{}) schema.Action {
	out := action
	out.Payload = payload
	return out
}

func clonePayload(payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		return map[string]interface{}{}
	}
	raw, _ := json.Marshal(payload)
	var out map[string]interface{}
	_ = json.Unmarshal(raw, &out)
	if out == nil {
		out = map[string]interface{}{}
	}
	return out
}
