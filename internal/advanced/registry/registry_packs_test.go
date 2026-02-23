package registry_test

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/packs"
	"samebits.com/evidra-mcp/internal/advanced/registry"
)

func TestInMemoryRegistryRegistersPackDefinitions(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", "..", ".."))
	packDir := filepath.Join(root, "packs", "_core", "ops")

	defs, err := packs.LoadToolDefinitions(packDir, nil)
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("expected at least one tool definition")
	}

	reg := registry.NewInMemoryRegistry(defs)
	names := reg.ToolNames()
	if !contains(names, "kubectl") {
		t.Fatalf("expected kubectl tool from ops packs, got %v", names)
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
