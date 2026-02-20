package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-mcp/pkg/invocation"
)

func TestComputeHashExcludesHashField(t *testing.T) {
	ts := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	record := EvidenceRecord{
		EventID:   "evt-1",
		Timestamp: ts,
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "cli",
		},
		Tool:      "echo",
		Operation: "run",
		Params:    map[string]interface{}{"text": "ok"},
		PolicyDecision: PolicyDecision{
			Allow:     true,
			RiskLevel: "low",
			Reason:    "allowed",
		},
		ExecutionResult: ExecutionResult{
			Status:   "success",
			ExitCode: intPtr(0),
		},
		PreviousHash: "abc123",
	}

	h1, err := ComputeHash(record)
	if err != nil {
		t.Fatalf("ComputeHash(record) error = %v", err)
	}

	recordWithHash := record
	recordWithHash.Hash = "ignored"
	h2, err := ComputeHash(recordWithHash)
	if err != nil {
		t.Fatalf("ComputeHash(recordWithHash) error = %v", err)
	}

	if h1 != h2 {
		t.Fatalf("hash should ignore record.Hash; got %q != %q", h1, h2)
	}
}

func TestAppendAndValidateChain(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	r1, err := Append(testRecord("evt-1", "echo", "run"))
	if err != nil {
		t.Fatalf("Append(r1) error = %v", err)
	}
	r2, err := Append(testRecord("evt-2", "git", "status"))
	if err != nil {
		t.Fatalf("Append(r2) error = %v", err)
	}

	if r1.PreviousHash != "" {
		t.Fatalf("expected first record previous_hash empty; got %q", r1.PreviousHash)
	}
	if r2.PreviousHash != r1.Hash {
		t.Fatalf("expected r2.PreviousHash to match r1.Hash; got %q vs %q", r2.PreviousHash, r1.Hash)
	}

	if err := ValidateChain(); err != nil {
		t.Fatalf("ValidateChain() error = %v", err)
	}
}

func TestValidateChainDetectsTamper(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	if _, err := Append(testRecord("evt-1", "echo", "run")); err != nil {
		t.Fatalf("Append(r1) error = %v", err)
	}
	if _, err := Append(testRecord("evt-2", "git", "status")); err != nil {
		t.Fatalf("Append(r2) error = %v", err)
	}

	path := filepath.Join("data", "evidence.log")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(log) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines; got %d", len(lines))
	}

	var first EvidenceRecord
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("Unmarshal(first) error = %v", err)
	}
	first.ExecutionResult.Status = "tampered"
	tampered, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal(tampered first) error = %v", err)
	}
	lines[0] = string(tampered)

	mutated := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile(mutated log) error = %v", err)
	}

	if err := ValidateChain(); err == nil {
		t.Fatalf("ValidateChain() expected tamper detection error, got nil")
	}
}

func TestEvidenceRecordContainsPolicyRef(t *testing.T) {
	temp := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	defer func() { _ = os.Chdir(oldWd) }()

	if err := os.Chdir(temp); err != nil {
		t.Fatalf("Chdir(temp) error = %v", err)
	}

	rec := testRecord("evt-policy-ref", "echo", "run")
	rec.PolicyRef = "abc123policyref"
	if _, err := Append(rec); err != nil {
		t.Fatalf("Append(rec) error = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join("data", "evidence.log"))
	if err != nil {
		t.Fatalf("ReadFile(evidence.log) error = %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	var persisted EvidenceRecord
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &persisted); err != nil {
		t.Fatalf("Unmarshal(last record) error = %v", err)
	}
	if persisted.PolicyRef != rec.PolicyRef {
		t.Fatalf("expected policy_ref %q, got %q", rec.PolicyRef, persisted.PolicyRef)
	}
}

func testRecord(eventID, tool, operation string) EvidenceRecord {
	return EvidenceRecord{
		EventID:   eventID,
		Timestamp: time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC),
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "cli",
		},
		Tool:      tool,
		Operation: operation,
		Params:    map[string]interface{}{"k": "v"},
		PolicyDecision: PolicyDecision{
			Allow:     true,
			RiskLevel: "low",
			Reason:    "allowed",
		},
		ExecutionResult: ExecutionResult{
			Status:   "success",
			ExitCode: intPtr(0),
		},
	}
}

func intPtr(v int) *int {
	return &v
}
