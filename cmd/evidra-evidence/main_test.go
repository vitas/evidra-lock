package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestVerifySuccess(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
		newRecord("evt-2", "policy-abc"),
	})

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"verify", "--evidence", logPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if ok, _ := resp["ok"].(bool); !ok {
		t.Fatalf("expected ok=true, got %v", resp["ok"])
	}
}

func TestVerifyFailsOnTamper(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
		newRecord("evt-2", "policy-abc"),
	})
	tamperEvidenceLine(t, logPath)

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"verify", "--evidence", logPath}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", code, errOut.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if ok, _ := resp["ok"].(bool); ok {
		t.Fatalf("expected ok=false")
	}
	if _, found := resp["failed_at"]; !found {
		t.Fatalf("expected failed_at in response")
	}
}

func TestExportCreatesAuditPack(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
	})
	policyPath := filepath.Join(t.TempDir(), "policy.rego")
	if err := os.WriteFile(policyPath, []byte("package evidra.policy"), 0o644); err != nil {
		t.Fatalf("WriteFile(policy.rego) error = %v", err)
	}

	dataPath := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(dataPath, []byte(`{"k":"v"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(data.json) error = %v", err)
	}

	outPath := filepath.Join(t.TempDir(), "audit-pack.tar.gz")
	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{
		"export",
		"--evidence", logPath,
		"--out", outPath,
		"--policy", policyPath,
		"--data", dataPath,
	}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	files := readTarGzFiles(t, outPath)
	if _, ok := files["evidence/evidence.log"]; !ok {
		t.Fatalf("expected evidence/evidence.log in tar")
	}
	manifestRaw, ok := files["manifest.json"]
	if !ok {
		t.Fatalf("expected manifest.json in tar")
	}
	if _, ok := files["policy/policy.rego"]; !ok {
		t.Fatalf("expected policy/policy.rego in tar")
	}
	if _, ok := files["policy/data.json"]; !ok {
		t.Fatalf("expected policy/data.json in tar")
	}

	var mf map[string]interface{}
	if err := json.Unmarshal(manifestRaw, &mf); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if mf["format"] != "evidra-audit-pack-v0.1" {
		t.Fatalf("unexpected manifest format: %v", mf["format"])
	}
	if mf["notes"] != "Evidra audit pack v0.1" {
		t.Fatalf("unexpected manifest notes: %v", mf["notes"])
	}
	policySHA, ok := mf["policy_file_sha256"].(string)
	if !ok || policySHA == "" {
		t.Fatalf("expected policy_file_sha256 in manifest")
	}
	dataSHA, ok := mf["data_file_sha256"].(string)
	if !ok || dataSHA == "" {
		t.Fatalf("expected data_file_sha256 in manifest")
	}
	if policySHA != bytesSHA256Hex(files["policy/policy.rego"]) {
		t.Fatalf("policy_file_sha256 does not match policy bytes in tar")
	}
	if dataSHA != bytesSHA256Hex(files["policy/data.json"]) {
		t.Fatalf("data_file_sha256 does not match data bytes in tar")
	}
}

func TestExportFailsOnInvalidChain(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
		newRecord("evt-2", "policy-abc"),
	})
	tamperEvidenceLine(t, logPath)

	outPath := filepath.Join(t.TempDir(), "audit-pack.tar.gz")
	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{
		"export",
		"--evidence", logPath,
		"--out", outPath,
	}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
}

func TestExportFailsOnMixedPolicyRef(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
		newRecord("evt-2", "policy-def"),
	})

	outPath := filepath.Join(t.TempDir(), "audit-pack.tar.gz")
	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{
		"export",
		"--evidence", logPath,
		"--out", outPath,
	}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1 for mixed policy_ref, got %d", code)
	}
}

func writeEvidenceLog(t *testing.T, records []evidence.EvidenceRecord) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "evidence.log")
	store := evidence.NewStoreWithPath(path)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	for _, rec := range records {
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}
	return path
}

func newRecord(eventID, policyRef string) evidence.EvidenceRecord {
	return evidence.EvidenceRecord{
		EventID:   eventID,
		Timestamp: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		PolicyRef: policyRef,
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "cli",
		},
		Tool:      "echo",
		Operation: "run",
		Params:    map[string]interface{}{"text": "hello"},
		PolicyDecision: evidence.PolicyDecision{
			Allow:     true,
			RiskLevel: "low",
			Reason:    "allowed_by_rule",
		},
		ExecutionResult: evidence.ExecutionResult{
			Status:   "success",
			ExitCode: intPtr(0),
		},
	}
}

func tamperEvidenceLine(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(evidence.log) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 {
		t.Fatalf("no evidence lines")
	}
	lines[0] = strings.Replace(lines[0], "\"success\"", "\"tampered\"", 1)
	mutated := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile(evidence.log) error = %v", err)
	}
}

func readTarGzFiles(t *testing.T, path string) map[string][]byte {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Open(tar.gz) error = %v", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatalf("gzip.NewReader() error = %v", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	files := make(map[string][]byte)
	for {
		hdr, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("tar.Next() error = %v", err)
		}
		content, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("read tar file %s: %v", hdr.Name, err)
		}
		files[hdr.Name] = content
	}
	return files
}

func intPtr(v int) *int {
	return &v
}

func bytesSHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
