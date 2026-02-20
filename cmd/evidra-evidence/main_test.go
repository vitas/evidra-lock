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
	if _, ok := files["policy/active.rego"]; !ok {
		t.Fatalf("expected policy/active.rego in tar")
	}
	if _, ok := files["policy/active-data.json"]; !ok {
		t.Fatalf("expected policy/active-data.json in tar")
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
	if policySHA != bytesSHA256Hex(files["policy/active.rego"]) {
		t.Fatalf("policy_file_sha256 does not match policy bytes in tar")
	}
	if dataSHA != bytesSHA256Hex(files["policy/active-data.json"]) {
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

func TestExportSegmentedIncludesManifestAndSegments(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
		newViolationRecord("evt-2", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
		newViolationRecord("evt-3", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
	}, strings.Repeat("z", 300))

	outPath := filepath.Join(t.TempDir(), "audit-pack-segmented.tar.gz")
	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"export", "--evidence", root, "--out", outPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	files := readTarGzFiles(t, outPath)
	if _, ok := files["evidence/manifest.json"]; !ok {
		t.Fatalf("expected evidence/manifest.json in tar")
	}
	segmentCount := 0
	for name := range files {
		if strings.HasPrefix(name, "evidence/segments/evidence-") {
			segmentCount++
		}
	}
	if segmentCount == 0 {
		t.Fatalf("expected segmented evidence files in tar")
	}

	var mf map[string]interface{}
	if err := json.Unmarshal(files["manifest.json"], &mf); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	if mf["evidence_store_format"] != "segmented" {
		t.Fatalf("expected evidence_store_format=segmented, got %v", mf["evidence_store_format"])
	}
}

func TestViolationsReportCountsDeniesAndHighRisk(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-deny", "policy-abc", "git", "push", "alice", false, "high", "policy_denied_default"),
		newViolationRecord("evt-high", "policy-abc", "git", "status", "bob", true, "high", "allowed_by_rule"),
		newViolationRecord("evt-low", "policy-abc", "echo", "run", "charlie", true, "low", "allowed_by_rule"),
	})

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"violations", "--evidence", logPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp struct {
		OK              bool `json:"ok"`
		ViolationsTotal int  `json:"violations_total"`
		ByReason        []struct {
			Reason string `json:"reason"`
			Count  int    `json:"count"`
		} `json:"by_reason"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if !resp.OK {
		t.Fatalf("expected ok=true")
	}
	if resp.ViolationsTotal != 2 {
		t.Fatalf("expected violations_total=2, got %d", resp.ViolationsTotal)
	}
	if len(resp.ByReason) == 0 || resp.ByReason[0].Reason != "allowed_by_rule" || resp.ByReason[0].Count != 1 {
		t.Fatalf("unexpected by_reason breakdown: %#v", resp.ByReason)
	}
}

func TestViolationsMinRiskFilter(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-medium", "policy-abc", "git", "status", "alice", true, "medium", "allowed_by_rule"),
		newViolationRecord("evt-critical", "policy-abc", "git", "push", "alice", true, "critical", "allowed_by_rule"),
	})

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"violations", "--evidence", logPath, "--min-risk", "critical"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp struct {
		ViolationsTotal int `json:"violations_total"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if resp.ViolationsTotal != 1 {
		t.Fatalf("expected violations_total=1, got %d", resp.ViolationsTotal)
	}
}

func TestViolationsSortingDeterministic(t *testing.T) {
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "git", "status", "beta", false, "high", "zzz_reason"),
		newViolationRecord("evt-2", "policy-abc", "echo", "run", "alpha", false, "high", "aaa_reason"),
	})

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"violations", "--evidence", logPath}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp struct {
		ByReason []struct {
			Reason string `json:"reason"`
			Count  int    `json:"count"`
		} `json:"by_reason"`
		ByTool []struct {
			Tool      string `json:"tool"`
			Operation string `json:"operation"`
			Count     int    `json:"count"`
		} `json:"by_tool"`
		TopActors []struct {
			ActorID string `json:"actor_id"`
			Count   int    `json:"count"`
		} `json:"top_actors"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}

	if len(resp.ByReason) < 2 || resp.ByReason[0].Reason != "aaa_reason" || resp.ByReason[1].Reason != "zzz_reason" {
		t.Fatalf("by_reason not deterministic: %#v", resp.ByReason)
	}
	if len(resp.ByTool) < 2 || resp.ByTool[0].Tool != "echo" || resp.ByTool[1].Tool != "git" {
		t.Fatalf("by_tool not deterministic: %#v", resp.ByTool)
	}
	if len(resp.TopActors) < 2 || resp.TopActors[0].ActorID != "alpha" || resp.TopActors[1].ActorID != "beta" {
		t.Fatalf("top_actors not deterministic: %#v", resp.TopActors)
	}
}

func TestViolationsWorksAcrossSegmentedStore(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-deny", "policy-abc", "git", "push", "alice", false, "high", "policy_denied_default"),
		newViolationRecord("evt-high", "policy-abc", "git", "status", "alice", true, "high", "allowed_by_rule"),
		newViolationRecord("evt-low", "policy-abc", "echo", "run", "bob", true, "low", "allowed_by_rule"),
	}, strings.Repeat("w", 280))

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"violations", "--evidence", root, "--min-risk", "high"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp struct {
		ViolationsTotal int `json:"violations_total"`
	}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if resp.ViolationsTotal != 2 {
		t.Fatalf("expected violations_total=2, got %d", resp.ViolationsTotal)
	}
}

func TestCursorShowReturnsNullWhenMissing(t *testing.T) {
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
	}, "small")

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"cursor", "show", "--evidence", root}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%s", code, errOut.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if _, ok := resp["cursor"]; !ok || resp["cursor"] != nil {
		t.Fatalf("expected cursor=null, got %v", resp["cursor"])
	}
}

func TestCursorAckAndShowRoundTrip(t *testing.T) {
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
		newViolationRecord("evt-2", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
	}, "small")

	var outAck strings.Builder
	var errAck strings.Builder
	code := run([]string{
		"cursor", "ack",
		"--evidence", root,
		"--segment", "evidence-000001.jsonl",
		"--line", "0",
	}, &outAck, &errAck)
	if code != 0 {
		t.Fatalf("expected ack exit code 0, got %d stderr=%s", code, errAck.String())
	}

	var ackResp map[string]interface{}
	if err := json.Unmarshal([]byte(outAck.String()), &ackResp); err != nil {
		t.Fatalf("unmarshal ack output: %v", err)
	}
	if ackResp["last_ack_hash"] == "" {
		t.Fatalf("expected last_ack_hash in ack response")
	}

	var outShow strings.Builder
	var errShow strings.Builder
	code = run([]string{"cursor", "show", "--evidence", root}, &outShow, &errShow)
	if code != 0 {
		t.Fatalf("expected show exit code 0, got %d stderr=%s", code, errShow.String())
	}

	var showResp struct {
		OK     bool `json:"ok"`
		Cursor struct {
			Segment string `json:"segment"`
			Line    int    `json:"line"`
		} `json:"cursor"`
		LastAckHash string `json:"last_ack_hash"`
	}
	if err := json.Unmarshal([]byte(outShow.String()), &showResp); err != nil {
		t.Fatalf("unmarshal show output: %v", err)
	}
	if !showResp.OK {
		t.Fatalf("expected ok=true")
	}
	if showResp.Cursor.Segment != "evidence-000001.jsonl" || showResp.Cursor.Line != 0 {
		t.Fatalf("unexpected cursor: %#v", showResp.Cursor)
	}
	if showResp.LastAckHash == "" {
		t.Fatalf("expected last_ack_hash from show")
	}
}

func TestCursorAckFailsOnTamperedChain(t *testing.T) {
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
		newViolationRecord("evt-2", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
	}, "small")
	tamperSegmentLine(t, filepath.Join(root, "segments", "evidence-000001.jsonl"), 0)

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{
		"cursor", "ack",
		"--evidence", root,
		"--segment", "evidence-000001.jsonl",
		"--line", "0",
	}, &out, &errOut)
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d stderr=%s", code, errOut.String())
	}
}

func TestCursorAckFailsOnMissingSegmentOrOutOfRange(t *testing.T) {
	root := writeSegmentedEvidenceRoot(t, []evidence.EvidenceRecord{
		newViolationRecord("evt-1", "policy-abc", "echo", "run", "alice", true, "low", "allowed_by_rule"),
	}, "small")

	var out1 strings.Builder
	var err1 strings.Builder
	code := run([]string{
		"cursor", "ack",
		"--evidence", root,
		"--segment", "evidence-000999.jsonl",
		"--line", "0",
	}, &out1, &err1)
	if code != 2 {
		t.Fatalf("expected exit code 2 for missing segment, got %d", code)
	}

	var out2 strings.Builder
	var err2 strings.Builder
	code = run([]string{
		"cursor", "ack",
		"--evidence", root,
		"--segment", "evidence-000001.jsonl",
		"--line", "99",
	}, &out2, &err2)
	if code != 2 {
		t.Fatalf("expected exit code 2 for line out of range, got %d", code)
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

func writeSegmentedEvidenceRoot(t *testing.T, records []evidence.EvidenceRecord, largeText string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "evidence")
	store := evidence.NewStoreWithPath(root)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	for _, rec := range records {
		rec.Params["text"] = largeText
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}
	return root
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

func newViolationRecord(eventID, policyRef, tool, operation, actorID string, allow bool, riskLevel, reason string) evidence.EvidenceRecord {
	rec := newRecord(eventID, policyRef)
	rec.Tool = tool
	rec.Operation = operation
	rec.Actor.ID = actorID
	rec.PolicyDecision.Allow = allow
	rec.PolicyDecision.RiskLevel = riskLevel
	rec.PolicyDecision.Reason = reason
	return rec
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

func tamperSegmentLine(t *testing.T, path string, lineIdx int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(segment) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if lineIdx >= len(lines) {
		t.Fatalf("line index out of range in tamper helper")
	}
	lines[lineIdx] = strings.Replace(lines[lineIdx], "\"success\"", "\"tampered\"", 1)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(segment) error = %v", err)
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
