package mcpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// SemanticIntent captures the policy-relevant identity of a validate request.
// Two requests with identical SemanticIntent will get the same policy outcome.
type SemanticIntent struct {
	Tool      string          `json:"tool"`
	Operation string          `json:"operation"`
	Namespace string          `json:"namespace,omitempty"`
	Kind      string          `json:"kind,omitempty"`
	Name      string          `json:"name,omitempty"`
	Images    []string        `json:"images,omitempty"`
	Posture   SecurityPosture `json:"posture,omitempty"`
	// Terraform fields
	ResourceTypes      []string `json:"resource_types,omitempty"`
	DestroyCount       *int     `json:"destroy_count,omitempty"`
	SecurityGroupCIDRs []string `json:"security_group_cidrs,omitempty"`
	IAMActions         []string `json:"iam_actions,omitempty"`
	// Helm fields
	HelmNamespace string `json:"helm_namespace,omitempty"`
	// ArgoCD fields
	AppName string `json:"app_name,omitempty"`
}

// SecurityPosture captures per-container and pod-level security fields
// that directly trigger deny rules.
type SecurityPosture struct {
	Privileged   []bool   `json:"privileged,omitempty"`
	RunAsUser    []int64  `json:"run_as_user,omitempty"`
	HostPID      bool     `json:"host_pid,omitempty"`
	HostIPC      bool     `json:"host_ipc,omitempty"`
	HostNetwork  bool     `json:"host_network,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// ExtractSemanticIntent builds the intent from a validate invocation.
// Handles both native K8s and flat payload formats.
func ExtractSemanticIntent(tool, operation string, payload map[string]any) SemanticIntent {
	intent := SemanticIntent{
		Tool:      strings.ToLower(strings.TrimSpace(tool)),
		Operation: strings.ToLower(strings.TrimSpace(operation)),
	}

	if payload == nil {
		return intent
	}

	switch intent.Tool {
	case "kubectl", "oc":
		extractK8sIntent(&intent, payload)
	case "terraform":
		extractTerraformIntent(&intent, payload)
	case "helm":
		intent.HelmNamespace = strings.ToLower(firstNonEmpty(
			extractString(payload, "namespace"),
			extractNestedString(payload, "metadata", "namespace"),
		))
	case "argocd":
		intent.AppName = strings.ToLower(extractString(payload, "app_name"))
	}

	return intent
}

func extractK8sIntent(intent *SemanticIntent, payload map[string]any) {
	intent.Namespace = strings.ToLower(firstNonEmpty(
		extractString(payload, "namespace"),
		extractNestedString(payload, "metadata", "namespace"),
	))

	intent.Kind = strings.ToLower(firstNonEmpty(
		extractString(payload, "resource"),
		extractString(payload, "kind"),
	))

	intent.Name = strings.ToLower(firstNonEmpty(
		extractString(payload, "name"),
		extractNestedString(payload, "metadata", "name"),
	))

	// Collect containers from flat or native paths
	containers := extractContainerList(payload, "containers")
	initContainers := extractContainerList(payload, "init_containers")

	// Also check native K8s path: spec.template.spec.containers
	if len(containers) == 0 {
		containers = extractNestedContainerList(payload, "spec", "template", "spec", "containers")
	}
	if len(initContainers) == 0 {
		initContainers = extractNestedContainerList(payload, "spec", "template", "spec", "initContainers")
		if len(initContainers) == 0 {
			initContainers = extractNestedContainerList(payload, "spec", "template", "spec", "init_containers")
		}
	}

	// Extract images
	allContainers := append(containers, initContainers...)
	images := extractImages(allContainers)
	intent.Images = images

	// Extract security posture from all containers
	intent.Posture = extractContainerPosture(allContainers)

	// Pod-level security fields
	intent.Posture.HostPID = extractBoolAny(payload, "host_pid", "hostPID")
	intent.Posture.HostIPC = extractBoolAny(payload, "host_ipc", "hostIPC")
	intent.Posture.HostNetwork = extractBoolAny(payload, "host_network", "hostNetwork")

	// Also check native path for pod-level fields
	if spec := extractNestedMap(payload, "spec", "template", "spec"); spec != nil {
		if !intent.Posture.HostPID {
			intent.Posture.HostPID = extractBoolAny(spec, "host_pid", "hostPID", "hostPid")
		}
		if !intent.Posture.HostIPC {
			intent.Posture.HostIPC = extractBoolAny(spec, "host_ipc", "hostIPC", "hostIpc")
		}
		if !intent.Posture.HostNetwork {
			intent.Posture.HostNetwork = extractBoolAny(spec, "host_network", "hostNetwork")
		}
	}
}

func extractTerraformIntent(intent *SemanticIntent, payload map[string]any) {
	if rt, ok := extractStringSlice(payload, "resource_types"); ok {
		sorted := make([]string, len(rt))
		copy(sorted, rt)
		sort.Strings(sorted)
		intent.ResourceTypes = sorted
	}

	if dc, ok := payload["destroy_count"]; ok {
		if v, ok := toInt(dc); ok {
			intent.DestroyCount = &v
		}
	}

	// Security group CIDRs
	if sgRules, ok := payload["security_group_rules"].([]any); ok {
		var cidrs []string
		for _, rule := range sgRules {
			if rm, ok := rule.(map[string]any); ok {
				if cidr := extractString(rm, "cidr"); cidr != "" {
					cidrs = append(cidrs, cidr)
				}
				if cidr := extractString(rm, "cidr_blocks"); cidr != "" {
					cidrs = append(cidrs, cidr)
				}
			}
		}
		sort.Strings(cidrs)
		intent.SecurityGroupCIDRs = cidrs
	}

	// IAM policy statements
	if stmts, ok := payload["iam_policy_statements"].([]any); ok {
		var actions []string
		for _, stmt := range stmts {
			if sm, ok := stmt.(map[string]any); ok {
				if a, ok := extractStringSlice(sm, "actions"); ok {
					actions = append(actions, a...)
				}
				// Also check "Action" (AWS style)
				if a, ok := extractStringSlice(sm, "Action"); ok {
					actions = append(actions, a...)
				}
			}
		}
		sort.Strings(actions)
		intent.IAMActions = actions
	}
}

func extractContainerPosture(containers []any) SecurityPosture {
	var posture SecurityPosture
	for _, c := range containers {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}

		sc := extractMap(cm, "security_context")
		if sc == nil {
			sc = extractMap(cm, "securityContext")
		}

		posture.Privileged = append(posture.Privileged, extractBool(sc, "privileged"))
		posture.RunAsUser = append(posture.RunAsUser, extractInt64Any(sc, "run_as_user", "runAsUser"))

		caps := extractMap(sc, "capabilities")
		if adds, ok := extractStringSlice(caps, "add"); ok {
			posture.Capabilities = append(posture.Capabilities, adds...)
		}
	}
	sort.Strings(posture.Capabilities)
	return posture
}

// IntentKey returns a truncated SHA-256 hash of a SemanticIntent.
func IntentKey(intent SemanticIntent) string {
	b, _ := json.Marshal(intent)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:12])
}

// --- helpers ---

func extractString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func extractNestedString(m map[string]any, keys ...string) string {
	if len(keys) == 0 || m == nil {
		return ""
	}
	current := m
	for _, k := range keys[:len(keys)-1] {
		next, ok := current[k].(map[string]any)
		if !ok {
			return ""
		}
		current = next
	}
	return extractString(current, keys[len(keys)-1])
}

func extractNestedMap(m map[string]any, keys ...string) map[string]any {
	if m == nil {
		return nil
	}
	current := m
	for _, k := range keys {
		next, ok := current[k].(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return current
}

func extractMap(m map[string]any, key string) map[string]any {
	if m == nil {
		return nil
	}
	v, ok := m[key].(map[string]any)
	if !ok {
		return nil
	}
	return v
}

func extractBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, ok := m[key]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	if !ok {
		return false
	}
	return b
}

func extractBoolAny(m map[string]any, keys ...string) bool {
	for _, k := range keys {
		if extractBool(m, k) {
			return true
		}
	}
	return false
}

func extractInt64Any(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := extractInt64(m, k); ok {
			return v
		}
	}
	return -1
}

func extractInt64(m map[string]any, key string) (int64, bool) {
	if m == nil {
		return 0, false
	}
	v, ok := m[key]
	if !ok {
		return 0, false
	}
	return toInt64(v)
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	}
	return 0, false
}

func toInt(v any) (int, bool) {
	i, ok := toInt64(v)
	return int(i), ok
}

func extractStringSlice(m map[string]any, key string) ([]string, bool) {
	if m == nil {
		return nil, false
	}
	v, ok := m[key]
	if !ok {
		return nil, false
	}
	switch s := v.(type) {
	case []string:
		return s, len(s) > 0
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out, len(out) > 0
	}
	return nil, false
}

func extractContainerList(payload map[string]any, key string) []any {
	v, ok := payload[key]
	if !ok {
		return nil
	}
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	return list
}

func extractNestedContainerList(m map[string]any, keys ...string) []any {
	if len(keys) == 0 || m == nil {
		return nil
	}
	current := m
	for _, k := range keys[:len(keys)-1] {
		next, ok := current[k].(map[string]any)
		if !ok {
			return nil
		}
		current = next
	}
	return extractContainerList(current, keys[len(keys)-1])
}

func extractImages(containers []any) []string {
	seen := make(map[string]bool)
	var images []string
	for _, c := range containers {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		img := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", cm["image"])))
		if img == "" || img == "<nil>" {
			continue
		}
		if !seen[img] {
			seen[img] = true
			images = append(images, img)
		}
	}
	sort.Strings(images)
	return images
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
