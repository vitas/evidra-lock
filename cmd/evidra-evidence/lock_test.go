//go:build unix

package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/evlock"
)

func TestVerifyReturnsBusyCodeWhenLocked(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_LOCK_TIMEOUT_MS", "100")
	logPath := writeEvidenceLog(t, []evidence.EvidenceRecord{
		newRecord("evt-1", "policy-abc"),
	})

	lockPath := filepath.Join(filepath.Dir(logPath), ".evidra.lock")
	lock, err := evlock.Acquire(lockPath, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire(lock) error = %v", err)
	}
	defer func() { _ = lock.Release() }()

	var out strings.Builder
	var errOut strings.Builder
	code := run([]string{"verify", "--evidence", logPath}, &out, &errOut)
	if code != exitVerifyFailed {
		t.Fatalf("expected exit code %d, got %d stderr=%s", exitVerifyFailed, code, errOut.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out.String()), &resp); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if resp["code"] != evidence.ErrorCodeStoreBusy {
		t.Fatalf("expected code=%q got %v", evidence.ErrorCodeStoreBusy, resp["code"])
	}
}
