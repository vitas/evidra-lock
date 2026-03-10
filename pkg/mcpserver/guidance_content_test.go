package mcpserver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGuidanceContentAutoFallsBackToEmbedded(t *testing.T) {
	t.Setenv("EVIDRA_CONTENT_DIR", "")
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	tmpWD := t.TempDir()
	if err := os.Chdir(tmpWD); err != nil {
		t.Fatalf("chdir temp: %v", err)
	}
	defer func() {
		_ = os.Chdir(originalWD)
	}()

	content, err := loadGuidanceContentAuto("")
	if err != nil {
		t.Fatalf("loadGuidanceContentAuto fallback: %v", err)
	}
	if !strings.Contains(content.InitializeInstructions, "Always call `validate`") {
		t.Fatalf("unexpected initialize instructions: %q", content.InitializeInstructions)
	}
	if !strings.Contains(content.AgentContractV1Body, "Evidra-Lock Agent Contract v1") {
		t.Fatalf("unexpected agent contract body: %q", content.AgentContractV1Body)
	}
}

func TestLoadGuidanceContentAutoFailsForInvalidExplicitDir(t *testing.T) {
	_, err := loadGuidanceContentAuto(filepath.Join(t.TempDir(), "missing-content"))
	if err == nil {
		t.Fatal("expected error for invalid explicit content dir")
	}
	if !strings.Contains(err.Error(), "--content-dir") {
		t.Fatalf("unexpected error for explicit dir: %v", err)
	}
}

func TestLoadGuidanceContentAutoFailsForInvalidEnvDir(t *testing.T) {
	t.Setenv("EVIDRA_CONTENT_DIR", filepath.Join(t.TempDir(), "missing-content"))
	_, err := loadGuidanceContentAuto("")
	if err == nil {
		t.Fatal("expected error for invalid EVIDRA_CONTENT_DIR")
	}
	if !strings.Contains(err.Error(), "EVIDRA_CONTENT_DIR") {
		t.Fatalf("unexpected error for env dir: %v", err)
	}
}
