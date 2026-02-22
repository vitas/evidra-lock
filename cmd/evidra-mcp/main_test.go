package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/core"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/registry"
)

func TestRunLoadsPolicyAndStartsServer(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()

	var capturedOpts mcpserver.Options
	stub := &fakeServer{}
	newServerFunc = func(opts mcpserver.Options, reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) serverRunner {
		capturedOpts = opts
		return stub
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "data.json")
	t.Setenv("EVIDRA_EVIDENCE_PATH", t.TempDir())

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--policy", policyPath, "--data", dataPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !stub.called {
		t.Fatalf("expected server to start")
	}
	if capturedOpts.Mode != mcpserver.ModeEnforce {
		t.Fatalf("expected enforce mode, got %s", capturedOpts.Mode)
	}
	if capturedOpts.PolicyRef == "" {
		t.Fatalf("expected policy ref recorded")
	}
}

func TestRunRequiresPolicyAndData(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()
	newServerFunc = func(mcpserver.Options, registry.Registry, core.PolicyEngine, core.EvidenceStore) serverRunner {
		t.Fatalf("server should not start when policy/data missing")
		return &fakeServer{}
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--policy", "foo"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "missing --policy/--data") {
		t.Fatalf("expected missing policy/data error, got: %s", errOut.String())
	}
}

func TestRunHonorsEnvFallback(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()
	stub := &fakeServer{}
	newServerFunc = func(mcpserver.Options, registry.Registry, core.PolicyEngine, core.EvidenceStore) serverRunner {
		return stub
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "data.json")
	t.Setenv("EVIDRA_POLICY_PATH", policyPath)
	t.Setenv("EVIDRA_DATA_PATH", dataPath)
	t.Setenv("EVIDRA_EVIDENCE_PATH", filepath.Join(root, "data", "evidence"))

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !stub.called {
		t.Fatalf("expected server to start via env config")
	}
}

type fakeServer struct {
	called bool
}

func (f *fakeServer) Run(ctx context.Context, _ mcp.Transport) error {
	f.called = true
	return nil
}
