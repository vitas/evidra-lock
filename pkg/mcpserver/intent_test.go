package mcpserver

import (
	"testing"
)

func TestIntentKey_SameInput_SameKey(t *testing.T) {
	t.Parallel()

	payload := map[string]any{
		"namespace": "production",
		"resource":  "deployment",
		"name":      "web-app",
		"containers": []any{
			map[string]any{"image": "nginx:1.25", "name": "web"},
		},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", payload))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", payload))

	if k1 != k2 {
		t.Errorf("same input produced different keys: %s vs %s", k1, k2)
	}
}

func TestIntentKey_DifferentImage_DifferentKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace":  "default",
		"resource":   "deployment",
		"containers": []any{map[string]any{"image": "nginx:1.25"}},
	}
	p2 := map[string]any{
		"namespace":  "default",
		"resource":   "deployment",
		"containers": []any{map[string]any{"image": "nginx:1.26"}},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("different images should produce different keys")
	}
}

func TestIntentKey_DifferentNamespace_DifferentKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{"namespace": "staging", "resource": "deployment"}
	p2 := map[string]any{"namespace": "production", "resource": "deployment"}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("different namespaces should produce different keys")
	}
}

func TestIntentKey_CaseInsensitive(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{"namespace": "Prod", "resource": "Deployment"}
	p2 := map[string]any{"namespace": "prod", "resource": "deployment"}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 != k2 {
		t.Errorf("case difference should not change key: %s vs %s", k1, k2)
	}
}

func TestIntentKey_IgnoresLabels(t *testing.T) {
	t.Parallel()

	base := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{"image": "nginx:1.25"},
		},
	}

	withLabels := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{"image": "nginx:1.25"},
		},
		"labels":      map[string]any{"app": "web", "version": "v2"},
		"annotations": map[string]any{"description": "test"},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", base))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", withLabels))

	if k1 != k2 {
		t.Errorf("labels/annotations should not change key: %s vs %s", k1, k2)
	}
}

func TestIntentKey_PrivilegedChange_ChangesKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image":            "nginx:1.25",
				"security_context": map[string]any{"privileged": true},
			},
		},
	}
	p2 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image":            "nginx:1.25",
				"security_context": map[string]any{"privileged": false},
			},
		},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("privileged change MUST change key (agent fix must get fresh eval)")
	}
}

func TestIntentKey_HostPIDChange_ChangesKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"host_pid":  true,
	}
	p2 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"host_pid":  false,
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("host_pid change should produce different key")
	}
}

func TestIntentKey_RunAsUserChange_ChangesKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image":            "nginx:1.25",
				"security_context": map[string]any{"run_as_user": float64(0)},
			},
		},
	}
	p2 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image":            "nginx:1.25",
				"security_context": map[string]any{"run_as_user": float64(1000)},
			},
		},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("run_as_user change should produce different key")
	}
}

func TestIntentKey_CapabilitiesChange_ChangesKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image": "nginx:1.25",
				"security_context": map[string]any{
					"capabilities": map[string]any{
						"add": []any{"SYS_ADMIN"},
					},
				},
			},
		},
	}
	p2 := map[string]any{
		"namespace": "default",
		"resource":  "deployment",
		"containers": []any{
			map[string]any{
				"image":            "nginx:1.25",
				"security_context": map[string]any{},
			},
		},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("capabilities change should produce different key")
	}
}

func TestIntentKey_NativeVsFlat_SameKey(t *testing.T) {
	t.Parallel()

	flat := map[string]any{
		"namespace": "production",
		"resource":  "deployment",
		"name":      "web-app",
		"containers": []any{
			map[string]any{"image": "nginx:1.25", "name": "web"},
		},
	}

	native := map[string]any{
		"metadata": map[string]any{
			"namespace": "production",
			"name":      "web-app",
		},
		"kind": "deployment",
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"image": "nginx:1.25", "name": "web"},
					},
				},
			},
		},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", flat))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", native))

	if k1 != k2 {
		t.Errorf("native and flat formats should produce same key: %s vs %s", k1, k2)
	}
}

func TestIntentKey_EmptyPayload(t *testing.T) {
	t.Parallel()

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", map[string]any{}))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", nil))

	// Both should produce a valid key (tool + operation only)
	if k1 == "" {
		t.Error("empty payload should still produce a key")
	}
	if k2 == "" {
		t.Error("nil payload should still produce a key")
	}
	if k1 != k2 {
		t.Errorf("empty and nil payload should produce same key: %s vs %s", k1, k2)
	}
}

func TestIntentKey_TerraformFields(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"resource_types": []any{"aws_instance", "aws_s3_bucket"},
		"destroy_count":  float64(2),
	}
	p2 := map[string]any{
		"resource_types": []any{"aws_instance", "aws_s3_bucket"},
		"destroy_count":  float64(3),
	}

	k1 := IntentKey(ExtractSemanticIntent("terraform", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("terraform", "apply", p2))

	if k1 == k2 {
		t.Error("different destroy_count should produce different key")
	}
}

func TestIntentKey_ArgocdFields(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{"app_name": "my-app"}
	p2 := map[string]any{"app_name": "other-app"}

	k1 := IntentKey(ExtractSemanticIntent("argocd", "sync", p1))
	k2 := IntentKey(ExtractSemanticIntent("argocd", "sync", p2))

	if k1 == k2 {
		t.Error("different app_name should produce different key")
	}
}

func TestIntentKey_ImageDigest_DifferentKey(t *testing.T) {
	t.Parallel()

	p1 := map[string]any{
		"namespace":  "default",
		"resource":   "deployment",
		"containers": []any{map[string]any{"image": "nginx:1.25"}},
	}
	p2 := map[string]any{
		"namespace":  "default",
		"resource":   "deployment",
		"containers": []any{map[string]any{"image": "nginx:1.25@sha256:abc123"}},
	}

	k1 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p1))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", p2))

	if k1 == k2 {
		t.Error("image with digest should produce different key than without")
	}
}

func TestIntentKey_ToolCaseNormalized(t *testing.T) {
	t.Parallel()

	payload := map[string]any{"namespace": "default", "resource": "deployment"}

	k1 := IntentKey(ExtractSemanticIntent("Kubectl", "Apply", payload))
	k2 := IntentKey(ExtractSemanticIntent("kubectl", "apply", payload))

	if k1 != k2 {
		t.Errorf("tool/operation case should be normalized: %s vs %s", k1, k2)
	}
}
