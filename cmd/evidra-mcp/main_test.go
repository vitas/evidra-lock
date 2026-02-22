package main

import (
	"bytes"
	"context"
	"path/filepath"
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

type fakeServer struct {
	called bool
}

func (f *fakeServer) Run(ctx context.Context, _ mcp.Transport) error {
	f.called = true
	return nil
}
