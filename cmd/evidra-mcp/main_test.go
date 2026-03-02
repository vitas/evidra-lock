package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	evidra "samebits.com/evidra"
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

func TestRunUsesEmbeddedBundleWhenNoPolicyConfigured(t *testing.T) {
	old := newServerFunc
	defer func() { newServerFunc = old }()

	stub := &fakeServer{}
	var capturedOpts mcpserver.Options
	newServerFunc = func(opts mcpserver.Options) serverRunner {
		capturedOpts = opts
		return stub
	}

	t.Setenv("EVIDRA_BUNDLE_PATH", "")
	t.Setenv("EVIDRA_POLICY_PATH", "")
	t.Setenv("EVIDRA_DATA_PATH", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", t.TempDir())

	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}
	if !stub.called {
		t.Fatalf("expected server to start with embedded bundle")
	}
	if !strings.Contains(errOut.String(), "using built-in ops-v0.1 bundle") {
		t.Fatalf("expected embedded bundle notice in stderr, got: %s", errOut.String())
	}
	if capturedOpts.BundlePath == "" {
		t.Fatalf("expected BundlePath to be set (extracted temp dir)")
	}
	if capturedOpts.PolicyRef == "" {
		t.Fatalf("expected PolicyRef from embedded bundle")
	}
}

func TestEmbeddedBundleHashIsDeterministic(t *testing.T) {
	t.Parallel()
	h1, err := embeddedBundleHash(evidra.OpsV01BundleFS)
	if err != nil {
		t.Fatalf("embeddedBundleHash: %v", err)
	}
	h2, err := embeddedBundleHash(evidra.OpsV01BundleFS)
	if err != nil {
		t.Fatalf("embeddedBundleHash second call: %v", err)
	}
	if h1 != h2 {
		t.Fatalf("hash is not deterministic: %q != %q", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("expected 64-char hex SHA-256, got %d chars: %q", len(h1), h1)
	}
}

func TestExtractEmbeddedBundleCached_InvalidatesOnChange(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	shaFile := filepath.Join(dir, bundleSHAFile)

	// Seed cache with a wrong hash — simulates a stale cache from an older binary.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(shaFile, []byte("stale-hash\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write a sentinel file that should be removed when the cache is busted.
	sentinel := filepath.Join(dir, "stale-file.txt")
	if err := os.WriteFile(sentinel, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	want, err := embeddedBundleHash(evidra.OpsV01BundleFS)
	if err != nil {
		t.Fatal(err)
	}

	// Verify test setup: stale hash must not equal the real hash.
	got, _ := os.ReadFile(shaFile)
	if strings.TrimSpace(string(got)) == want {
		t.Fatal("test setup error: stale hash unexpectedly matched real hash")
	}

	// Simulate cache bust: remove old dir, extract fresh, write correct SHA.
	os.RemoveAll(dir)
	if _, err := extractEmbeddedBundle(evidra.OpsV01BundleFS, dir); err != nil {
		t.Fatalf("extractEmbeddedBundle: %v", err)
	}
	if err := os.WriteFile(shaFile, []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Sentinel must be gone — directory was wiped.
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("expected sentinel to be removed after cache bust, stat err=%v", err)
	}

	// SHA file must contain the correct hash now.
	written, err := os.ReadFile(shaFile)
	if err != nil {
		t.Fatalf("read SHA file: %v", err)
	}
	if strings.TrimSpace(string(written)) != want {
		t.Fatalf("SHA file contains %q, want %q", string(written), want)
	}

	// .manifest must exist (bundle was extracted).
	if _, err := os.Stat(filepath.Join(dir, ".manifest")); err != nil {
		t.Fatalf("expected .manifest after re-extraction: %v", err)
	}
}

func TestExtractEmbeddedBundleCached_NoOpOnCacheHit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	want, err := embeddedBundleHash(evidra.OpsV01BundleFS)
	if err != nil {
		t.Fatal(err)
	}

	// Prime the cache with the correct hash and a sentinel file.
	if _, err := extractEmbeddedBundle(evidra.OpsV01BundleFS, dir); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(dir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("keep me"), 0o644); err != nil {
		t.Fatal(err)
	}
	shaFile := filepath.Join(dir, bundleSHAFile)
	if err := os.WriteFile(shaFile, []byte(want+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Verify: hash matches → a cache-hit path would leave sentinel untouched.
	got, _ := os.ReadFile(shaFile)
	if strings.TrimSpace(string(got)) != want {
		t.Fatal("test setup error: hash mismatch")
	}
	if _, err := os.Stat(sentinel); err != nil {
		t.Fatalf("sentinel should still exist on cache hit: %v", err)
	}
}

type fakeServer struct {
	called bool
}

func (f *fakeServer) Run(ctx context.Context, _ mcp.Transport) error {
	f.called = true
	return nil
}
