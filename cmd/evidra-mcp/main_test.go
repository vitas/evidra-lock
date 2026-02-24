package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra/pkg/config"
	"samebits.com/evidra/pkg/mcpserver"
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
	policyPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "policy", "policy.rego")
	dataPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "data", "params", "data.json")
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
	policyPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "policy", "policy.rego")
	dataPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "data", "params", "data.json")
	t.Setenv("EVIDRA_POLICY_PATH", policyPath)
	t.Setenv("EVIDRA_DATA_PATH", dataPath)
	t.Setenv("EVIDRA_EVIDENCE_PATH", t.TempDir())

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

func TestRunUsesResolvedDefaultEvidencePath(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()

	var capturedOpts mcpserver.Options
	stub := &fakeServer{}
	newServerFunc = func(opts mcpserver.Options) serverRunner {
		capturedOpts = opts
		return stub
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "policy", "policy.rego")
	dataPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "data", "params", "data.json")
	home := t.TempDir()
	t.Setenv("EVIDRA_HOME", home)
	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", "")

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"--policy", policyPath, "--data", dataPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !stub.called {
		t.Fatalf("expected server to start")
	}
	wantEvidence := filepath.Join(home, filepath.FromSlash(config.DefaultEvidenceRelativeDir))
	if capturedOpts.EvidencePath != wantEvidence {
		t.Fatalf("EvidencePath=%q, want %q", capturedOpts.EvidencePath, wantEvidence)
	}
}

func TestRunSupportsEvidenceStoreAlias(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()

	var capturedOpts mcpserver.Options
	stub := &fakeServer{}
	newServerFunc = func(opts mcpserver.Options) serverRunner {
		capturedOpts = opts
		return stub
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "policy", "policy.rego")
	dataPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "data", "params", "data.json")
	wantEvidence := t.TempDir()

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{
		"--policy", policyPath,
		"--data", dataPath,
		"--evidence-store", wantEvidence,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !stub.called {
		t.Fatalf("expected server to start")
	}
	if capturedOpts.EvidencePath != wantEvidence {
		t.Fatalf("EvidencePath=%q, want %q", capturedOpts.EvidencePath, wantEvidence)
	}
}

func TestRunRejectsConflictingEvidenceFlags(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()
	newServerFunc = func(mcpserver.Options) serverRunner {
		t.Fatalf("server should not start when evidence flags conflict")
		return &fakeServer{}
	}

	root := filepath.Join("..", "..")
	policyPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "policy", "policy.rego")
	dataPath := filepath.Join(root, "policy", "bundles", "ops-v0.1", "evidra", "data", "params", "data.json")

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{
		"--policy", policyPath,
		"--data", dataPath,
		"--evidence-dir", "/tmp/a",
		"--evidence-store", "/tmp/b",
	}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(errOut.String(), "conflicting values") {
		t.Fatalf("expected conflict error, got: %s", errOut.String())
	}
}

func TestResolveEvidencePathPrecedence(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_PATH", t.TempDir())
	t.Setenv("EVIDRA_EVIDENCE_DIR", filepath.Join(t.TempDir(), "dir"))

	got, err := config.ResolveEvidencePath("")
	if err != nil {
		t.Fatalf("ResolveEvidencePath() error = %v", err)
	}
	if got != os.Getenv("EVIDRA_EVIDENCE_DIR") {
		t.Fatalf("expected EVIDRA_EVIDENCE_DIR to win, got %q", got)
	}

	const flagValue = "/tmp/flag-evidence"
	got, err = config.ResolveEvidencePath(flagValue)
	if err != nil {
		t.Fatalf("ResolveEvidencePath(flag) error = %v", err)
	}
	if got != flagValue {
		t.Fatalf("expected flag to win, got %q", got)
	}

	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	got, err = config.ResolveEvidencePath("")
	if err != nil {
		t.Fatalf("ResolveEvidencePath(path fallback) error = %v", err)
	}
	if got != os.Getenv("EVIDRA_EVIDENCE_PATH") {
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
