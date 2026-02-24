//go:build unix

package evidence

import (
	"path/filepath"
	"testing"
	"time"

	"samebits.com/evidra/pkg/evlock"
	"samebits.com/evidra/pkg/invocation"
)

func TestAppendFailsWhenStoreBusy(t *testing.T) {
	t.Setenv(lockTimeoutEnv, "100")
	root := filepath.Join(t.TempDir(), "evidence")
	store := NewStoreWithPath(root)
	if err := store.Init(); err != nil {
		t.Fatalf("Init() error = %v", err)
	}

	lockPath := filepath.Join(root, lockFileName)
	lock, err := evlock.Acquire(lockPath, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire(lock) error = %v", err)
	}
	defer func() { _ = lock.Release() }()

	err = store.Append(EvidenceRecord{
		EventID:   "evt-busy",
		Timestamp: time.Now().UTC(),
		Actor: invocation.Actor{
			Type:   "human",
			ID:     "u1",
			Origin: "cli",
		},
		Tool:      "echo",
		Operation: "run",
		Params:    map[string]interface{}{"text": "busy"},
		PolicyDecision: PolicyDecision{
			Allow:     true,
			RiskLevel: "low",
			Reason:    "allowed_by_rule",
		},
		ExecutionResult: ExecutionResult{
			Status: "success",
		},
	})
	if err == nil {
		t.Fatalf("Append() expected busy error")
	}
	if !IsStoreBusyError(err) {
		t.Fatalf("Append() expected evidence_store_busy error, got %v", err)
	}
}
