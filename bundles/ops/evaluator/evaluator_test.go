package evaluator

import (
	"path/filepath"
	"testing"
	"time"

	"samebits.com/evidra-mcp/bundles/ops/schema"
	"samebits.com/evidra-mcp/pkg/runtime"
)

func TestPolicyRuleBlocksK8sApplyKubeSystemWithoutBreakglass(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("k8s.apply", map[string]any{"namespace": "kube-system"}, map[string]any{}, "apply core system manifests", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if res.Pass {
		t.Fatalf("expected fail")
	}
	if !contains(res.PolicyHits, "k8s_apply_kube_system_blocked") {
		t.Fatalf("expected kube-system policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleBlocksTerraformPublicExposureWithoutApproval(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("terraform.plan", map[string]any{"dir": "./infra"}, map[string]any{"publicly_exposed": true}, "plan network change for service", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if res.Pass {
		t.Fatalf("expected fail")
	}
	if !contains(res.PolicyHits, "terraform_public_exposure_blocked") {
		t.Fatalf("expected terraform policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleAllowsTerraformPublicExposureWithApproval(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("terraform.plan", map[string]any{"dir": "./infra"}, map[string]any{"publicly_exposed": true}, "plan public endpoint after approval", []string{"approved_public"})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got reasons %v", res.Reasons)
	}
}

func TestPolicyRuleBlocksShortIntent(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("argocd.sync", map[string]any{"app": "payments"}, map[string]any{}, "short", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if res.Pass {
		t.Fatalf("expected fail")
	}
	if !contains(res.PolicyHits, "intent_too_short") {
		t.Fatalf("expected intent policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleAllowsIntentWhenLengthIsSufficient(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("argocd.sync", map[string]any{"app": "payments"}, map[string]any{}, "synchronize application to desired revision", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got reasons %v", res.Reasons)
	}
}

func TestPolicyRuleBlocksTerraformDestroyCountOverLimit(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("terraform.plan", map[string]any{"dir": "./infra"}, map[string]any{"destroy_count": 9}, "plan teardown for drifted resources", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if res.Pass {
		t.Fatalf("expected fail")
	}
	if !contains(res.PolicyHits, "terraform_destroy_count_exceeds_limit") {
		t.Fatalf("expected destroy_count policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleBlocksProdNamespaceWithoutApproval(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("helm.upgrade", map[string]any{"namespace": "prod"}, map[string]any{}, "upgrade production release after review", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if res.Pass {
		t.Fatalf("expected fail")
	}
	if !contains(res.PolicyHits, "prod_namespace_requires_change_approval") {
		t.Fatalf("expected prod namespace approval policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleAllowsProdNamespaceWithApprovalTag(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("helm.upgrade", map[string]any{"namespace": "prod"}, map[string]any{}, "upgrade production release after review", []string{"change-approved"})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got reasons %v", res.Reasons)
	}
}

func TestPolicyRuleFlagsAutonomousExecution(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("argocd.sync", map[string]any{"app": "payments"}, map[string]any{}, "synchronize application to approved revision", []string{})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got reasons %v", res.Reasons)
	}
	if res.RiskLevel != "high" {
		t.Fatalf("expected high risk for autonomous execution, got %q", res.RiskLevel)
	}
	if !contains(res.PolicyHits, "autonomous-execution") {
		t.Fatalf("expected autonomous-execution policy hit, got %v", res.PolicyHits)
	}
}

func TestPolicyRuleAllowsWithRequiredTagOverrides(t *testing.T) {
	e := newTestEvaluator(t)
	sc := baseScenario("k8s.apply", map[string]any{"namespace": "kube-system"}, map[string]any{}, "apply emergency kube-system patch", []string{"breakglass"})
	res, err := e.EvaluateScenario(sc)
	if err != nil {
		t.Fatalf("EvaluateScenario() error = %v", err)
	}
	if !res.Pass {
		t.Fatalf("expected pass, got reasons %v", res.Reasons)
	}
	if res.RiskLevel != "high" {
		t.Fatalf("expected high risk for breakglass, got %q", res.RiskLevel)
	}
}

func newTestEvaluator(t *testing.T) *Evaluator {
	t.Helper()
	eval, err := runtime.NewEvaluator(filepath.Join("..", "policies", "policy.rego"), "")
	if err != nil {
		t.Fatalf("NewEvaluator() error = %v", err)
	}
	return New(eval)
}

func baseScenario(kind string, target map[string]any, payload map[string]any, intent string, tags []string) schema.Scenario {
	return schema.Scenario{
		ScenarioID: "sc-test",
		Actor: schema.Actor{
			Type: "agent",
			ID:   "agent-1",
		},
		Source:    "mcp",
		Timestamp: time.Date(2026, 2, 21, 0, 0, 0, 0, time.UTC),
		Actions: []schema.Action{
			{
				Kind:     kind,
				Target:   target,
				Intent:   intent,
				Payload:  payload,
				RiskTags: tags,
			},
		},
	}
}

func contains(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
