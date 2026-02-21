//go:build unix

package evlock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireBusyThenRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".evidra.lock")

	l1, err := Acquire(path, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire(first) error = %v", err)
	}

	start := time.Now()
	_, err = Acquire(path, 120*time.Millisecond)
	if err == nil {
		t.Fatalf("Acquire(second) expected busy error")
	}
	if err != ErrBusy {
		t.Fatalf("Acquire(second) expected ErrBusy, got %v", err)
	}
	if time.Since(start) < 100*time.Millisecond {
		t.Fatalf("Acquire(second) returned too quickly; timeout retry behavior not exercised")
	}

	if err := l1.Release(); err != nil {
		t.Fatalf("Release(first) error = %v", err)
	}

	l2, err := Acquire(path, 200*time.Millisecond)
	if err != nil {
		t.Fatalf("Acquire(after release) error = %v", err)
	}
	if err := l2.Release(); err != nil {
		t.Fatalf("Release(second) error = %v", err)
	}
}
