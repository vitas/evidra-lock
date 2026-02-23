package evidence

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

	path := filepath.Join("data", "evidence", "segments", "evidence-000001.jsonl")
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

	raw, err := os.ReadFile(filepath.Join("data", "evidence", "segments", "evidence-000001.jsonl"))
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

// TestConcurrentAppendsSameStore verifies that concurrent appends to the same
// Store produce a valid chain with no data races. The race detector enforces
// the correctness of s.mu serialization.
func TestConcurrentAppendsSameStore(t *testing.T) {
	store := NewStoreWithPath(filepath.Join(t.TempDir(), "evidence"))
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = store.Append(testRecord(fmt.Sprintf("evt-%d", i), "echo", "run"))
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: Append() error = %v", i, err)
		}
	}
	if err := store.ValidateChain(); err != nil {
		t.Fatalf("ValidateChain() after concurrent appends: %v", err)
	}
}

// TestConcurrentAppendsDifferentStores verifies that two stores at different
// paths operate independently. Before TD-05 was fixed, both would serialize on
// the global appendMu even though they write to unrelated directories.
func TestConcurrentAppendsDifferentStores(t *testing.T) {
	storeA := NewStoreWithPath(filepath.Join(t.TempDir(), "evidence-a"))
	storeB := NewStoreWithPath(filepath.Join(t.TempDir(), "evidence-b"))
	for _, s := range []*Store{storeA, storeB} {
		if err := s.Init(); err != nil {
			t.Fatalf("Init() error = %v", err)
		}
	}

	var wg sync.WaitGroup
	errA, errB := make([]error, 4), make([]error, 4)
	for i := range 4 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			errA[i] = storeA.Append(testRecord(fmt.Sprintf("a-%d", i), "echo", "run"))
		}(i)
		go func(i int) {
			defer wg.Done()
			errB[i] = storeB.Append(testRecord(fmt.Sprintf("b-%d", i), "git", "status"))
		}(i)
	}
	wg.Wait()

	for i, err := range errA {
		if err != nil {
			t.Errorf("storeA worker %d: Append() error = %v", i, err)
		}
	}
	for i, err := range errB {
		if err != nil {
			t.Errorf("storeB worker %d: Append() error = %v", i, err)
		}
	}
	if err := storeA.ValidateChain(); err != nil {
		t.Fatalf("storeA.ValidateChain() error = %v", err)
	}
	if err := storeB.ValidateChain(); err != nil {
		t.Fatalf("storeB.ValidateChain() error = %v", err)
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
