package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultEvidenceRelativeDir is the single source of truth for the evidence
// store location relative to the resolved home directory.
const DefaultEvidenceRelativeDir = ".evidra/evidence"

// ResolveEvidencePath resolves evidence path precedence:
// explicit flag > EVIDRA_EVIDENCE_DIR > EVIDRA_EVIDENCE_PATH > home default.
//
// EVIDRA_HOME is an optional home override used mainly for tests to avoid
// coupling to the real user home directory.
func ResolveEvidencePath(explicit string) (string, error) {
	if path := strings.TrimSpace(explicit); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_DIR")); path != "" {
		return path, nil
	}
	if path := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_PATH")); path != "" {
		return path, nil
	}

	home, err := resolveHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve default evidence path: %w", err)
	}
	return filepath.Join(home, filepath.FromSlash(DefaultEvidenceRelativeDir)), nil
}

// DefaultEvidencePathDescription renders the default path for help/docs output.
func DefaultEvidencePathDescription() string {
	path, err := ResolveEvidencePath("")
	if err != nil {
		return filepath.ToSlash(filepath.Join("$HOME", DefaultEvidenceRelativeDir))
	}
	return path
}

// ResolveEvidenceDir is kept for backward compatibility with existing callers.
func ResolveEvidenceDir(flagValue string) string {
	path, err := ResolveEvidencePath(flagValue)
	if err != nil {
		return ""
	}
	return path
}

func resolveHomeDir() (string, error) {
	if home := strings.TrimSpace(os.Getenv("EVIDRA_HOME")); home != "" {
		return home, nil
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return home, nil
	}
	if home := strings.TrimSpace(os.Getenv("HOME")); home != "" {
		return home, nil
	}
	if home := strings.TrimSpace(os.Getenv("USERPROFILE")); home != "" {
		return home, nil
	}
	return "", fmt.Errorf("home directory not found; set EVIDRA_HOME or EVIDRA_EVIDENCE_DIR")
}
