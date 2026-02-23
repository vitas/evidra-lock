package config

import (
	"path/filepath"
	"testing"
)

func TestResolveEvidencePathExplicit(t *testing.T) {
	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", "")
	t.Setenv("EVIDRA_HOME", "")

	const explicit = "/tmp/evidra-custom"
	got, err := ResolveEvidencePath(explicit)
	if err != nil {
		t.Fatalf("ResolveEvidencePath(explicit) error = %v", err)
	}
	if got != explicit {
		t.Fatalf("ResolveEvidencePath(explicit) = %q, want %q", got, explicit)
	}
}

func TestResolveEvidencePathEnvPrecedence(t *testing.T) {
	t.Setenv("EVIDRA_HOME", filepath.Join(t.TempDir(), "home"))
	t.Setenv("EVIDRA_EVIDENCE_PATH", "/tmp/legacy")
	t.Setenv("EVIDRA_EVIDENCE_DIR", "/tmp/primary")

	got, err := ResolveEvidencePath("")
	if err != nil {
		t.Fatalf("ResolveEvidencePath() error = %v", err)
	}
	if got != "/tmp/primary" {
		t.Fatalf("ResolveEvidencePath() = %q, want /tmp/primary", got)
	}

	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	got, err = ResolveEvidencePath("")
	if err != nil {
		t.Fatalf("ResolveEvidencePath() fallback error = %v", err)
	}
	if got != "/tmp/legacy" {
		t.Fatalf("ResolveEvidencePath() fallback = %q, want /tmp/legacy", got)
	}
}

func TestResolveEvidencePathUsesEvidenceHomeOverride(t *testing.T) {
	home := t.TempDir()
	t.Setenv("EVIDRA_HOME", home)
	t.Setenv("EVIDRA_EVIDENCE_DIR", "")
	t.Setenv("EVIDRA_EVIDENCE_PATH", "")

	got, err := ResolveEvidencePath("")
	if err != nil {
		t.Fatalf("ResolveEvidencePath() error = %v", err)
	}
	want := filepath.Join(home, filepath.FromSlash(DefaultEvidenceRelativeDir))
	if got != want {
		t.Fatalf("ResolveEvidencePath() = %q, want %q", got, want)
	}
}
