package evidence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra/pkg/invocation"
)

func TestSegmentedStoreRotationAndValidateAcrossSegments(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	root := filepath.Join(t.TempDir(), "evidence")
	store := NewStoreWithPath(root)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for i := 0; i < 6; i++ {
		rec := segmentedTestRecord("evt-seg-"+strconv.Itoa(i), strings.Repeat("x", 280))
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	segments, err := SegmentFiles(root)
	if err != nil {
		t.Fatalf("SegmentFiles() error = %v", err)
	}
	if len(segments) < 2 {
		t.Fatalf("expected rotation to create at least 2 segments, got %d", len(segments))
	}

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(manifest.SealedSegments) == 0 {
		t.Fatalf("expected sealed_segments to be populated after rotation")
	}
	if containsSegment(manifest.SealedSegments, manifest.CurrentSegment) {
		t.Fatalf("current_segment must not be in sealed_segments")
	}

	if err := ValidateChainAtPath(root); err != nil {
		t.Fatalf("ValidateChainAtPath() error = %v", err)
	}
}

func TestSegmentedValidateFailsOnMiddleSegmentTamper(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	root := filepath.Join(t.TempDir(), "evidence")
	store := NewStoreWithPath(root)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for i := 0; i < 6; i++ {
		rec := segmentedTestRecord("evt-seg-tamper-"+strconv.Itoa(i), strings.Repeat("y", 280))
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	segmentPath := filepath.Join(root, "segments", "evidence-000002.jsonl")
	raw, err := os.ReadFile(segmentPath)
	if err != nil {
		t.Fatalf("ReadFile(segment) error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected at least one line in middle segment")
	}
	var rec EvidenceRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("Unmarshal(segment line) error = %v", err)
	}
	rec.ExecutionResult.Status = "tampered"
	b, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("Marshal(tampered) error = %v", err)
	}
	lines[0] = string(b)
	if err := os.WriteFile(segmentPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(segment) error = %v", err)
	}

	if err := ValidateChainAtPath(root); err == nil {
		t.Fatalf("expected ValidateChainAtPath() to fail on tampered middle segment")
	}
}

func TestSegmentedValidateFailsWhenSealedSegmentMissing(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_SEGMENT_MAX_BYTES", "400")
	root := filepath.Join(t.TempDir(), "evidence")
	store := NewStoreWithPath(root)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	for i := 0; i < 6; i++ {
		rec := segmentedTestRecord("evt-seg-missing-"+strconv.Itoa(i), strings.Repeat("m", 280))
		if err := store.Append(rec); err != nil {
			t.Fatalf("Append() error = %v", err)
		}
	}

	manifest, err := LoadManifest(root)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}
	if len(manifest.SealedSegments) == 0 {
		t.Fatalf("expected at least one sealed segment")
	}
	missing := manifest.SealedSegments[0]
	if err := os.Remove(filepath.Join(root, "segments", missing)); err != nil {
		t.Fatalf("Remove(sealed segment) error = %v", err)
	}

	if err := ValidateChainAtPath(root); err == nil {
		t.Fatalf("expected ValidateChainAtPath() to fail when sealed segment is missing")
	}
}

func segmentedTestRecord(eventID, text string) EvidenceRecord {
	return EvidenceRecord{
		EventID:   eventID,
		Timestamp: time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC),
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "seg-test",
			Origin: "cli",
		},
		Tool:      "echo",
		Operation: "run",
		Params:    map[string]interface{}{"text": text},
		PolicyDecision: PolicyDecision{
			Allow:     true,
			RiskLevel: "low",
			Reason:    "allowed_by_rule",
		},
		ExecutionResult: ExecutionResult{
			Status:   "success",
			ExitCode: intPtr(0),
		},
	}
}
