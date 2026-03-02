package mcpserver

import (
	"testing"
	"time"
)

func TestDenyCache_FirstCall_NotBlocked(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(10 * time.Minute)
	if err := dc.CheckDenyLoop("some-key"); err != nil {
		t.Errorf("first call should not be blocked, got: %v", err)
	}
}

func TestDenyCache_AfterDeny_Blocked(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(10 * time.Minute)
	dc.RecordDeny("key-a", "privileged container", []string{"ops.privileged_pod"}, "evt-001")

	err := dc.CheckDenyLoop("key-a")
	if err == nil {
		t.Fatal("expected stop_after_deny error after recording deny")
	}
	if err.Code != ErrCodeStopAfterDeny {
		t.Errorf("expected code %q, got %q", ErrCodeStopAfterDeny, err.Code)
	}
}

func TestDenyCache_DifferentKey_NotBlocked(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(10 * time.Minute)
	dc.RecordDeny("key-a", "privileged container", []string{"ops.privileged_pod"}, "evt-001")

	if err := dc.CheckDenyLoop("key-b"); err != nil {
		t.Errorf("different key should not be blocked, got: %v", err)
	}
}

func TestDenyCache_TTLExpiry_Resumes(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(50 * time.Millisecond)
	dc.RecordDeny("key-a", "denied", nil, "evt-001")

	// Should be blocked immediately
	if err := dc.CheckDenyLoop("key-a"); err == nil {
		t.Fatal("expected blocked before TTL expires")
	}

	// Wait past TTL
	time.Sleep(60 * time.Millisecond)

	if err := dc.CheckDenyLoop("key-a"); err != nil {
		t.Errorf("should be unblocked after TTL, got: %v", err)
	}
}

func TestDenyCache_ClearOnAllow(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(10 * time.Minute)
	dc.RecordDeny("key-a", "denied", nil, "evt-001")

	// Blocked
	if err := dc.CheckDenyLoop("key-a"); err == nil {
		t.Fatal("expected blocked after deny")
	}

	// Clear on allow
	dc.ClearIntent("key-a")

	if err := dc.CheckDenyLoop("key-a"); err != nil {
		t.Errorf("should be unblocked after clear, got: %v", err)
	}
}

func TestDenyCache_DenyCount_Increments(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(10 * time.Minute)
	dc.RecordDeny("key-a", "denied", nil, "evt-001")
	dc.RecordDeny("key-a", "denied again", nil, "evt-002")

	dc.mu.Lock()
	entry := dc.entries["key-a"]
	dc.mu.Unlock()

	if entry == nil {
		t.Fatal("expected entry to exist")
	}
	if entry.DenyCount != 2 {
		t.Errorf("expected DenyCount=2, got %d", entry.DenyCount)
	}
	// Original reason preserved
	if entry.Reason != "denied" {
		t.Errorf("expected original reason preserved, got %q", entry.Reason)
	}
}

func TestDenyCache_Cleanup(t *testing.T) {
	t.Parallel()

	dc := NewDenyCache(50 * time.Millisecond)
	dc.RecordDeny("key-a", "denied", nil, "evt-001")
	dc.RecordDeny("key-b", "denied", nil, "evt-002")

	time.Sleep(60 * time.Millisecond)

	// Record a fresh one
	dc.RecordDeny("key-c", "denied", nil, "evt-003")

	dc.Cleanup()

	dc.mu.Lock()
	count := len(dc.entries)
	dc.mu.Unlock()

	if count != 1 {
		t.Errorf("expected 1 entry after cleanup, got %d", count)
	}
}
