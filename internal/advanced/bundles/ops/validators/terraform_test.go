package validators

import (
	"context"
	"errors"
	"testing"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

func TestTerraformValidatorToolMissingCreatesWarningFinding(t *testing.T) {
	orig := execRunner
	defer func() { execRunner = orig }()
	execRunner = func(ctx context.Context, workdir, name string, args []string, stdin []byte) ([]byte, []byte, int, int64, error) {
		return nil, nil, -1, 1, errToolMissing
	}

	v := TerraformValidator{}
	rep, err := v.Run(context.Background(), schema.Action{
		Kind: "terraform.plan",
		Payload: map[string]interface{}{
			"path": "./infra",
		},
	}, "")
	if err != nil {
		t.Fatalf("Run() unexpected error: %v", err)
	}
	if rep.Tool != "terraform" {
		t.Fatalf("expected terraform tool, got %s", rep.Tool)
	}
	if len(rep.Findings) != 1 || rep.Findings[0].Title != "tool-missing" {
		t.Fatalf("expected tool-missing warning finding, got %+v", rep.Findings)
	}
}

func TestExecCommandMissingBinary(t *testing.T) {
	_, _, _, _, err := execCommand(context.Background(), "", "definitely-not-installed-evidra-tool", nil, nil)
	if err == nil {
		t.Fatalf("expected missing tool error")
	}
	if !errors.Is(err, errToolMissing) {
		t.Fatalf("expected errToolMissing, got %v", err)
	}
}
