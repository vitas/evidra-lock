package engine

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"samebits.com/evidra/pkg/bundlesource"
	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/runtime"
)

var bundleDir = filepath.Join("..", "..", "policy", "bundles", "ops-v0.1")

func testAdapter(t *testing.T) *Adapter {
	t.Helper()
	bs, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource: %v", err)
	}
	a, err := NewAdapter(bs)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	return a
}

// denyAllSource is a PolicySource whose policy denies every evaluation.
type denyAllSource struct{}

func (d *denyAllSource) LoadPolicy() (map[string][]byte, error) {
	rego := `package evidra.policy
decision := {
	"allow": false,
	"risk_level": "high",
	"reason": "denied by test policy",
	"reasons": ["test.deny_all"],
	"hits": ["test.deny_all"],
	"hints": ["this is a test deny"],
}
`
	return map[string][]byte{"test_deny.rego": []byte(rego)}, nil
}

func (d *denyAllSource) LoadData() ([]byte, error)  { return []byte(`{}`), nil }
func (d *denyAllSource) PolicyRef() (string, error) { return "sha256:test-deny-all", nil }
func (d *denyAllSource) BundleRevision() string     { return "test-rev" }
func (d *denyAllSource) ProfileName() string        { return "test-profile" }

func testDenyAdapter(t *testing.T) *Adapter {
	t.Helper()
	a, err := NewAdapter(&denyAllSource{})
	if err != nil {
		t.Fatalf("NewAdapter deny: %v", err)
	}
	return a
}

func safeInvocation() invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:       invocation.Actor{Type: "agent", ID: "claude", Origin: "mcp"},
		Tool:        "kubectl",
		Operation:   "apply",
		Environment: "dev",
		Params: map[string]interface{}{
			"target": map[string]interface{}{
				"namespace": "default",
			},
		},
	}
}

func TestNewAdapter_Valid(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)
	if a == nil {
		t.Fatal("expected non-nil Adapter")
	}
}

func TestNewAdapter_InvalidSource(t *testing.T) {
	t.Parallel()
	_, err := bundlesource.NewBundleSource("/nonexistent/bundle")
	if err == nil {
		t.Fatal("expected error from NewBundleSource with nonexistent path")
	}
}

func TestEvaluate_Allow(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	dec, err := a.Evaluate(context.Background(), safeInvocation())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !dec.Allow {
		t.Errorf("expected allow=true, got false (reason: %s)", dec.Reason)
	}
	if dec.RiskLevel != "low" {
		t.Errorf("RiskLevel = %q, want low", dec.RiskLevel)
	}
	if dec.PolicyRef == "" {
		t.Error("expected non-empty PolicyRef")
	}
}

func TestEvaluate_Deny(t *testing.T) {
	t.Parallel()
	a := testDenyAdapter(t)

	dec, err := a.Evaluate(context.Background(), safeInvocation())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.Allow {
		t.Error("expected allow=false, got true")
	}
	if dec.RiskLevel != "high" {
		t.Errorf("RiskLevel = %q, want high", dec.RiskLevel)
	}
	if len(dec.Hits) == 0 {
		t.Error("expected non-empty Hits")
	}
	if dec.Reason != "denied by test policy" {
		t.Errorf("Reason = %q, want %q", dec.Reason, "denied by test policy")
	}
}

func TestEvaluate_DenyPolicyMetadata(t *testing.T) {
	t.Parallel()
	a := testDenyAdapter(t)

	dec, err := a.Evaluate(context.Background(), safeInvocation())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.PolicyRef != "sha256:test-deny-all" {
		t.Errorf("PolicyRef = %q, want sha256:test-deny-all", dec.PolicyRef)
	}
	if dec.BundleRevision != "test-rev" {
		t.Errorf("BundleRevision = %q, want test-rev", dec.BundleRevision)
	}
	if dec.ProfileName != "test-profile" {
		t.Errorf("ProfileName = %q, want test-profile", dec.ProfileName)
	}
}

func TestEvaluate_PolicyMetadata(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	dec, err := a.Evaluate(context.Background(), safeInvocation())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if dec.PolicyRef == "" {
		t.Error("PolicyRef should be set")
	}
	if dec.BundleRevision == "" {
		t.Error("BundleRevision should be set")
	}
	if dec.ProfileName == "" {
		t.Error("ProfileName should be set")
	}
}

func TestEvaluate_ValidationError_MissingTool(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Tool = ""

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for missing tool")
	}
	if !strings.Contains(err.Error(), "tool is required") {
		t.Errorf("error = %q, want to contain 'tool is required'", err.Error())
	}
}

func TestEvaluate_ValidationError_MissingActor(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Actor.Type = ""

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for missing actor.type")
	}
	if !strings.Contains(err.Error(), "actor.type is required") {
		t.Errorf("error = %q, want to contain 'actor.type is required'", err.Error())
	}
}

func TestEvaluate_ValidationError_MissingOperation(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Operation = ""

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for missing operation")
	}
}

func TestEvaluate_ValidationError_MissingParams(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Params = nil

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for nil params")
	}
}

func TestEvaluate_ValidationError_MissingActorID(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Actor.ID = ""

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for missing actor.id")
	}
}

func TestEvaluate_ValidationError_MissingActorOrigin(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	inv := safeInvocation()
	inv.Actor.Origin = ""

	_, err := a.Evaluate(context.Background(), inv)
	if err == nil {
		t.Fatal("expected validation error for missing actor.origin")
	}
}

func TestBundleRevision(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	rev := a.BundleRevision()
	if rev == "" {
		t.Error("expected non-empty BundleRevision")
	}
}

func TestProfileName(t *testing.T) {
	t.Parallel()
	a := testAdapter(t)

	name := a.ProfileName()
	if name == "" {
		t.Error("expected non-empty ProfileName")
	}
}

// Verify the interface: PolicySource is satisfied by BundleSource.
var _ runtime.PolicySource = (*bundlesource.BundleSource)(nil)
