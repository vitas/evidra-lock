package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestComputeHashExcludesHashField(t *testing.T) {
	ts := time.Date(2026, 2, 19, 12, 0, 0, 0, time.UTC)
	base := EvidenceRecord{
		ID:        "evt-1",
		Timestamp: ts,
		Actor:     "user-a",
		Action:    "create",
		Subject:   "resource-1",
		Details: map[string]interface{}{
			"reason": "initial",
		},
		PrevHash: "abc123",
	}

	h1, err := ComputeHash(base)
	if err != nil {
		t.Fatalf("ComputeHash(base) error = %v", err)
	}

	withHash := base
	withHash.Hash = "different-value-that-must-be-ignored"
	h2, err := ComputeHash(withHash)
	if err != nil {
		t.Fatalf("ComputeHash(withHash) error = %v", err)
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

	r1, err := Append(EvidenceRecord{
		ID:        "evt-1",
		Timestamp: time.Date(2026, 2, 19, 12, 1, 0, 0, time.UTC),
		Actor:     "user-a",
		Action:    "create",
		Subject:   "resource-1",
	})
	if err != nil {
		t.Fatalf("Append(r1) error = %v", err)
	}

	r2, err := Append(EvidenceRecord{
		ID:        "evt-2",
		Timestamp: time.Date(2026, 2, 19, 12, 2, 0, 0, time.UTC),
		Actor:     "user-b",
		Action:    "update",
		Subject:   "resource-1",
	})
	if err != nil {
		t.Fatalf("Append(r2) error = %v", err)
	}

	if r1.PrevHash != "" {
		t.Fatalf("expected head record PrevHash to be empty; got %q", r1.PrevHash)
	}
	if r2.PrevHash != r1.Hash {
		t.Fatalf("expected r2.PrevHash to match r1.Hash; got %q vs %q", r2.PrevHash, r1.Hash)
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

	if _, err := Append(EvidenceRecord{
		ID:        "evt-1",
		Timestamp: time.Date(2026, 2, 19, 12, 3, 0, 0, time.UTC),
		Actor:     "user-a",
		Action:    "create",
		Subject:   "resource-1",
	}); err != nil {
		t.Fatalf("Append(r1) error = %v", err)
	}
	if _, err := Append(EvidenceRecord{
		ID:        "evt-2",
		Timestamp: time.Date(2026, 2, 19, 12, 4, 0, 0, time.UTC),
		Actor:     "user-b",
		Action:    "update",
		Subject:   "resource-1",
	}); err != nil {
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
	first.Action = "tampered"
	tamperedLine, err := json.Marshal(first)
	if err != nil {
		t.Fatalf("Marshal(tampered first) error = %v", err)
	}
	lines[0] = string(tamperedLine)

	mutated := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatalf("WriteFile(mutated log) error = %v", err)
	}

	if err := ValidateChain(); err == nil {
		t.Fatalf("ValidateChain() expected tamper detection error, got nil")
	}
}
