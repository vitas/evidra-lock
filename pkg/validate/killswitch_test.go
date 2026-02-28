// ══════════════════════════════════════════════════════════════
// Kill-switch E2E suite — MAX 35 TESTS.
//
// Add new tests ONLY for:
//   - new threat class (new tool, new bypass vector)
//   - regression (a real bug that slipped through)
//
// Do NOT add tests for:
//   - permutations of existing scenarios
//   - "what if both X and Y" combinations
//
// If you need more coverage, add unit tests in the rule's own
// _test.rego file, not here.
// ══════════════════════════════════════════════════════════════

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
					"block_public_acls":       false,
					"ignore_public_acls":      false,
					"block_public_policy":     false,
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
			name:   "e2e_privileged_container_deny",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
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
			name:   "e2e_workload_fake_empty_containers_deny",
			kind:   "kubectl.apply",
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
				"destroy_count":          0,
				"total_changes":          1,
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
		// ── Helm kill-switch tests ──
		{
			name:        "e2e_helm_upgrade_no_namespace_deny",
			kind:        "helm.upgrade",
			payload:     map[string]interface{}{"chart": "nginx"},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name:   "e2e_helm_upgrade_with_namespace_allow",
			kind:   "helm.upgrade",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"chart":     "nginx",
			},
			wantPass: true,
		},
		{
			name:   "e2e_helm_uninstall_with_namespace_allow",
			kind:   "helm.uninstall",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"release":   "my-release",
			},
			wantPass: true,
		},
		// ── ArgoCD kill-switch tests ──
		{
			name:        "e2e_argocd_sync_no_context_deny",
			kind:        "argocd.sync",
			payload:     map[string]interface{}{},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			name: "e2e_argocd_sync_with_app_name_allow",
			kind: "argocd.sync",
			payload: map[string]interface{}{
				"app_name": "staging-app",
			},
			wantPass: true,
		},
		// ── Domain rule + kill-switch interaction ──
		{
			// Verify domain rule fires THROUGH kill-switch (not blocked by it)
			name:   "e2e_kubectl_apply_host_namespace_deny",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "pod",
				"host_pid":  true,
				"containers": []interface{}{
					map[string]interface{}{"image": "nginx:1.25"},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"k8s.host_namespace_escape"},
		},
		{
			// Verify terraform domain rule fires (rule matches terraform.plan)
			name: "e2e_terraform_plan_sg_open_world_deny",
			kind: "terraform.plan",
			payload: map[string]interface{}{
				"security_group_rules": []interface{}{
					map[string]interface{}{
						"type":        "ingress",
						"from_port":   22,
						"to_port":     22,
						"protocol":    "tcp",
						"cidr_blocks": []interface{}{"0.0.0.0/0"},
					},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"terraform.sg_open_world"},
		},
		// ── Garbage input tests ──
		{
			// Empty resource_types array is NOT sufficient detail
			name: "e2e_terraform_empty_resource_types_deny",
			kind: "terraform.apply",
			payload: map[string]interface{}{
				"resource_types": []interface{}{},
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			// Container without image field is not "real"
			name:   "e2e_kubectl_apply_container_no_image_deny",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "deployment",
				"containers": []interface{}{
					map[string]interface{}{"name": "app"},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		{
			// argocd.sync with empty app_name is not sufficient
			name: "e2e_argocd_sync_empty_app_name_deny",
			kind: "argocd.sync",
			payload: map[string]interface{}{
				"app_name": "",
			},
			wantPass:    false,
			wantRuleIDs: []string{"ops.insufficient_context"},
		},
		// ── Additional domain rule interaction tests ──
		{
			// Verify run_as_root domain rule fires through kill-switch
			name:   "e2e_kubectl_apply_run_as_root_deny",
			kind:   "kubectl.apply",
			target: map[string]interface{}{"namespace": "default"},
			payload: map[string]interface{}{
				"namespace": "default",
				"resource":  "pod",
				"containers": []interface{}{
					map[string]interface{}{
						"image": "nginx:1.25",
						"security_context": map[string]interface{}{
							"run_as_user": 0,
						},
					},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"k8s.run_as_root"},
		},
		{
			// Verify argocd domain rule fires through kill-switch
			name: "e2e_argocd_wildcard_dest_deny",
			kind: "argocd.project",
			payload: map[string]interface{}{
				"destinations": []interface{}{
					map[string]interface{}{
						"namespace": "*",
						"server":    "*",
					},
				},
			},
			wantPass:    false,
			wantRuleIDs: []string{"argocd.wildcard_destination"},
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
		"destroy_count":              0,
		"total_changes":              1,
		"resource_types":             []interface{}{"aws_instance"},
		"resource_changes_truncated": true,
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

// TestKillswitch_BaselineProfile verifies that golden rules do not fire
// when ops.profile resolves to "baseline". Kill-switch rules remain active.
func TestKillswitch_BaselineProfile(t *testing.T) {
	t.Parallel()
	// kubectl.apply with privileged container + sufficient context
	sc := killswitchScenario("kubectl.apply",
		map[string]interface{}{"namespace": "default"},
		map[string]interface{}{
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
		nil,
	)
	opts := safeOpts(t)
	opts.SkipEvidence = true
	// Use by_env override: ops.profile resolves to "baseline" when
	// input.environment = "baseline-test".
	opts.Environment = "baseline-test"

	result, err := validate.EvaluateScenario(context.Background(), sc, opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With baseline profile, privileged container should PASS
	// (golden rule gated, kill-switch allows because context is sufficient)
	if !result.Pass {
		t.Errorf("Pass=false, want true in baseline; RuleIDs=%v", result.RuleIDs)
	}
}
