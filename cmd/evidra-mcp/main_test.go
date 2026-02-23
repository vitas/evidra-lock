package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/config"
	"samebits.com/evidra-mcp/pkg/mcpserver"
)

func TestRunLoadsPolicyAndStartsServer(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()

	var capturedOpts mcpserver.Options
	stub := &fakeServer{}
	newServerFunc = func(opts mcpserver.Options) serverRunner {
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
	newServerFunc = func(mcpserver.Options) serverRunner {
		t.Fatalf("server should not start when policy/data missing")
		return &fakeServer{}
	}

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--policy", "foo"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "missing policy/data") {
		t.Fatalf("expected missing policy/data error, got: %s", errOut.String())
	}
}

func TestRunHonorsEnvFallback(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()
	stub := &fakeServer{}
	newServerFunc = func(mcpserver.Options) serverRunner {
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

func TestResolveEvidencePathPrecedence(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_PATH", t.TempDir())
	t.Setenv("EVIDRA_EVIDENCE_DIR", filepath.Join(t.TempDir(), "dir"))

	if got := config.ResolveEvidenceDir(""); got != os.Getenv("EVIDRA_EVIDENCE_DIR") {
		t.Fatalf("expected EVIDRA_EVIDENCE_DIR to win, got %q", got)
	}

	const flagValue = "/tmp/flag-evidence"
	if got := config.ResolveEvidenceDir(flagValue); got != flagValue {
		t.Fatalf("expected flag to win, got %q", got)
	}

	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	if got := config.ResolveEvidenceDir(""); got != os.Getenv("EVIDRA_EVIDENCE_PATH") {
		t.Fatalf("expected fallback to EVIDRA_EVIDENCE_PATH, got %q", got)
	}
}

type fakeServer struct {
	called bool
}

func (f *fakeServer) Run(ctx context.Context, _ mcp.Transport) error {
	f.called = true
	return nil
}
