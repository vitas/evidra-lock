package validate_test

import (
	"context"
	"testing"
	"time"

	"samebits.com/evidra/pkg/scenario"
	"samebits.com/evidra/pkg/validate"
)

// killswitchScenario builds a single-action scenario for kill-switch tests.
func killswitchScenario(kind string, target, payload map[string]interface{}, riskTags []string) scenario.Scenario {
	if target == nil {
		target = map[string]interface{}{}
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}
	if riskTags == nil {
		riskTags = []string{}
	}
	return scenario.Scenario{
		ScenarioID: "ks-test",
		Actor:      scenario.Actor{Type: "agent", ID: "test-agent", Origin: "test"},
		Source:     "test",
		Timestamp:  time.Now().UTC(),
		Actions: []scenario.Action{
			{
				Kind:     kind,
				Target:   target,
				Payload:  payload,
				RiskTags: riskTags,
			},
		},
	}
}

func TestKillswitch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		kind        string
		target      map[string]interface{}
		payload     map[string]interface{}
		riskTags    []string
		wantPass    bool
		wantRuleIDs []string // each must be present in result.RuleIDs
	}{
		{
			name:     "e2e_readonly_no_payload_allow",
			kind:     "kubectl.get",
			payload:  map[string]interface{}{},
			wantPass: true,
		},
		{
			name:     "e2e_terraform_plan_no_payload_allow",
			kind:     "terraform.plan",
			payload:  map[string]interface{}{},
			wantPass: true,
		},
		{
			name:        "e2e_destructive_empty_payload_deny",
			kind:        "terraform.apply",
			payload:     map[string]interface{}{},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name:        "e2e_kubectl_delete_no_namespace_deny",
			kind:        "kubectl.delete",
			payload:     map[string]interface{}{"resource": "pod"},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name:   "e2e_kubectl_delete_protected_ns_deny",
			kind:   "kubectl.delete",
			target: map[string]interface{}{"namespace": "kube-system"},
			payload: map[string]interface{}{
				"namespace": "kube-system",
				"resource":  "pod",
			},
			wantPass:    false,
			wantRuleIDs: []string{"k8s.protected_namespace"},
		},
		{
			name:   "e2e_kubectl_delete_safe_ns_allow",
			kind:   "kubectl.delete",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "pod",
			},
			wantPass: true,
		},
		{
			// Existing s3 rule checks terraform.plan — use that kind to
			// validate the domain rule fires on s3 config violations.
			name: "e2e_terraform_public_s3_deny",
			kind: "terraform.plan",
			payload: map[string]interface{}{
				"resource_type": "aws_s3_bucket",
				"s3_public_access_block": map[string]interface{}{
					"block_public_acls":     false,
					"ignore_public_acls":    false,
					"block_public_policy":   false,
					"restrict_public_buckets": false,
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"terraform.s3_public_access"},
		},
		{
			// Existing IAM rule checks terraform.plan — use that kind to
			// validate the domain rule fires on wildcard policies.
			name: "e2e_terraform_iam_wildcard_deny",
			kind: "terraform.plan",
			payload: map[string]interface{}{
				"iam_policy_statements": []interface{}{
					map[string]interface{}{
						"effect":   "Allow",
						"action":   "*",
						"resource": "*",
					},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"terraform.iam_wildcard_policy"},
		},
		{
			// Existing privileged container rule checks k8s.apply kind.
			// Using k8s.apply also triggers ops.unknown_destructive since
			// k8s is not a known tool prefix (kubectl is).
			name: "e2e_privileged_container_deny",
			kind: "k8s.apply",
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "deployment",
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.25",
						"security_context": map[string]interface{}{
							"privileged": true,
						},
					},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"k8s.privileged_container"},
		},
		{
			name:   "e2e_workload_apply_no_containers_deny",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "deployment",
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name:   "e2e_nonworkload_apply_no_containers_allow",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "configmap",
			},
			wantPass: true,
		},
		{
			name:        "e2e_unknown_tool_destructive_deny",
			kind:        "pulumi.up",
			payload:     map[string]interface{}{},
			wantPass:    false,
			wantRuleIDs: []string{"ops.unknown_destructive"},
		},
		{
			name: "e2e_valid_terraform_apply_allow",
			kind: "terraform.apply",
			payload: map[string]interface{}{
				"destroy_count":  0,
				"total_changes":  1,
				"resource_types": []interface{}{"aws_instance"},
			},
			wantPass: true,
		},
		{
			name: "e2e_terraform_counts_only_deny",
			kind: "terraform.apply",
			payload: map[string]interface{}{
				"destroy_count": 0,
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name:        "e2e_unknown_tool_safe_suffix_deny",
			kind:        "pulumi.plan",
			payload:     map[string]interface{}{},
			wantPass:    false,
			wantRuleIDs: []string{"ops.unknown_destructive"},
		},
		{
			name:     "e2e_unknown_tool_breakglass_allow",
			kind:     "crossplane.apply",
			payload:  map[string]interface{}{},
			riskTags: []string{"breakglass"},
			wantPass: true,
		},
		{
			name: "e2e_workload_fake_empty_containers_deny",
			kind: "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "deployment",
				"containers": []interface{}{
					map[string]interface{}{},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name: "e2e_terraform_detail_nonsense_deny",
			kind: "terraform.apply",
			payload: map[string]interface{}{
				"destroy_count":        0,
				"total_changes":        1,
				"security_group_rules": []interface{}{map[string]interface{}{"foo": "bar"}},
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name: "e2e_terraform_s3_block_empty_object_deny",
			kind: "terraform.apply",
			payload: map[string]interface{}{
				"destroy_count":         0,
				"total_changes":         1,
				"s3_public_access_block": map[string]interface{}{},
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			// crossplane.apply is NOT in destructive_operations, so
			// unknown_destructive fires (not insufficient_context).
			name: "e2e_added_tool_no_clause_deny",
			kind: "crossplane.apply",
			payload: map[string]interface{}{
				"anything": true,
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.unknown_destructive"},
		},
		{
			name:     "e2e_added_tool_read_op_auto_allow",
			kind:     "kubectl.version",
			payload:  map[string]interface{}{},
			wantPass: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sc := killswitchScenario(tc.kind, tc.target, tc.payload, tc.riskTags)
			opts := safeOpts(t)
			opts.SkipEvidence = true

			result, err := validate.EvaluateScenario(context.Background(), sc, opts)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Pass != tc.wantPass {
				t.Errorf("Pass=%v, want %v; RuleIDs=%v reasons=%v",
					result.Pass, tc.wantPass, result.RuleIDs, result.Reasons)
			}
			for _, wantRule := range tc.wantRuleIDs {
				if !containsString(result.RuleIDs, wantRule) {
					t.Errorf("RuleIDs=%v, want %q present", result.RuleIDs, wantRule)
				}
			}
			if !tc.wantPass && len(result.Hints) == 0 {
				t.Error("Hints empty on deny, want at least one actionable hint")
			}
		})
	}
}

// TestKillswitch_BreakglassWarning verifies that breakglass override
// produces a warning in the result even though the operation is allowed.
func TestKillswitch_BreakglassWarning(t *testing.T) {
	t.Parallel()
	sc := killswitchScenario("crossplane.apply", nil, map[string]interface{}{}, []string{"breakglass"})
	opts := safeOpts(t)
	opts.SkipEvidence = true

	result, err := validate.EvaluateScenario(context.Background(), sc, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Errorf("Pass=false, want true with breakglass; reasons=%v", result.Reasons)
	}
	if !containsString(result.RuleIDs, "ops.breakglass_used") {
		t.Errorf("RuleIDs=%v, want ops.breakglass_used warning", result.RuleIDs)
	}
}

// TestKillswitch_TruncatedContext verifies the truncation guard.
func TestKillswitch_TruncatedContext(t *testing.T) {
	t.Parallel()
	sc := killswitchScenario("terraform.apply", nil, map[string]interface{}{
		"destroy_count":                0,
		"total_changes":               1,
		"resource_types":              []interface{}{"aws_instance"},
		"resource_changes_truncated":  true,
	}, nil)
	opts := safeOpts(t)
	opts.SkipEvidence = true

	result, err := validate.EvaluateScenario(context.Background(), sc, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("Pass=true, want false for truncated context")
	}
	if !containsString(result.RuleIDs, "ops.truncated_context") {
		t.Errorf("RuleIDs=%v, want ops.truncated_context", result.RuleIDs)
	}
}
