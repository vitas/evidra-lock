package main

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/packs"
)

func TestDefaultProfileIsOps(t *testing.T) {
	t.Setenv("EVIDRA_PROFILE", "")
	p, err := loadProfileFromEnv()
	if err != nil {
		t.Fatalf("loadProfileFromEnv() error = %v", err)
	}
	if p != ProfileOps {
		t.Fatalf("expected default profile ops, got %q", p)
	}
}

func TestBuildRegistryForProfiles(t *testing.T) {
	opReg, err := buildRegistryForProfile(ProfileOps)
	if err != nil {
		t.Fatalf("buildRegistryForProfile(ops) error = %v", err)
	}
	if _, ok := opReg.Lookup("echo"); ok {
		t.Fatalf("ops profile must not register echo")
	}
	if _, ok := opReg.Lookup("git"); ok {
		t.Fatalf("ops profile must not register git")
	}
	if _, ok := opReg.Lookup("kubectl"); !ok {
		t.Fatalf("ops profile expected kubectl tool")
	}

	devReg, err := buildRegistryForProfile(ProfileDev)
	if err != nil {
		t.Fatalf("buildRegistryForProfile(dev) error = %v", err)
	}
	if _, ok := devReg.Lookup("echo"); !ok {
		t.Fatalf("dev profile expected echo")
	}
	if _, ok := devReg.Lookup("git"); !ok {
		t.Fatalf("dev profile expected git")
	}
}

func TestOpsDefaultPackDirLoadsArgoCDPack(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	packDir := filepath.Join(root, "packs", "_core", "ops")

	defs, err := packs.LoadToolDefinitions(packDir, nil)
	if err != nil {
		t.Fatalf("LoadToolDefinitions() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("expected at least one ops pack tool definition")
	}
	foundArgoCD := false
	foundTerraform := false
	for _, def := range defs {
		if def.Name == "argocd" {
			foundArgoCD = true
		}
		if def.Name == "terraform" {
			foundTerraform = true
		}
	}
	if !foundArgoCD {
		t.Fatalf("expected argocd tool from ops packs")
	}
	if !foundTerraform {
		t.Fatalf("expected terraform tool from ops packs")
	}
}
