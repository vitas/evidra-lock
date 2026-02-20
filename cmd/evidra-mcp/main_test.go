package main

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/packs"
	"samebits.com/evidra-mcp/pkg/policysource"
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
	foundAWS := false
	foundDocker := false
	foundCompose := false
	foundHelm := false
	foundKubectl := false
	foundPodman := false
	foundTerraform := false
	for _, def := range defs {
		if def.Name == "argocd" {
			foundArgoCD = true
		}
		if def.Name == "aws" {
			foundAWS = true
		}
		if def.Name == "docker" {
			foundDocker = true
		}
		if def.Name == "docker-compose" {
			foundCompose = true
		}
		if def.Name == "podman" {
			foundPodman = true
		}
		if def.Name == "terraform" {
			foundTerraform = true
		}
		if def.Name == "helm" {
			foundHelm = true
		}
		if def.Name == "kubectl" {
			foundKubectl = true
		}
		if def.Name == "argocd" {
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("argocd ops pack must not include version operation")
				}
			}
		}
		if def.Name == "helm" {
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("helm ops pack must not include version operation")
				}
			}
		}
		if def.Name == "terraform" {
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("terraform ops pack must not include version operation")
				}
			}
		}
		if def.Name == "aws" {
			for _, op := range def.SupportedOperations {
				if op == "sts-whoami" {
					t.Fatalf("aws ops pack must not include sts-whoami operation")
				}
			}
		}
	}
	if !foundArgoCD {
		t.Fatalf("expected argocd tool from ops packs")
	}
	if !foundAWS {
		t.Fatalf("expected aws tool from ops packs")
	}
	if !foundDocker {
		t.Fatalf("expected docker tool from ops packs")
	}
	if !foundCompose {
		t.Fatalf("expected docker-compose tool from ops packs")
	}
	if !foundPodman {
		t.Fatalf("expected podman tool from ops packs")
	}
	if !foundTerraform {
		t.Fatalf("expected terraform tool from ops packs")
	}
	if !foundKubectl {
		t.Fatalf("expected kubectl tool from ops packs")
	}
	if !foundHelm {
		t.Fatalf("expected helm tool from ops packs")
	}
}

func TestResolvePolicyPathsDefaults(t *testing.T) {
	t.Setenv("EVIDRA_POLICY_PATH", "")
	t.Setenv("EVIDRA_POLICY_DATA_PATH", "")
	policyPath, dataPath := resolvePolicyPaths(ProfileOps, "", "")
	if policyPath != defaultOpsPolicyPath {
		t.Fatalf("expected ops default policy path %q, got %q", defaultOpsPolicyPath, policyPath)
	}
	if dataPath != defaultOpsDataPath {
		t.Fatalf("expected ops default data path %q, got %q", defaultOpsDataPath, dataPath)
	}

	devPolicyPath, devDataPath := resolvePolicyPaths(ProfileDev, "", "")
	if devPolicyPath != defaultOpsPolicyPath {
		t.Fatalf("expected dev default policy path %q, got %q", defaultOpsPolicyPath, devPolicyPath)
	}
	if devDataPath != defaultOpsDataPath {
		t.Fatalf("expected dev default data path %q, got %q", defaultOpsDataPath, devDataPath)
	}
}

func TestOpsPolicyKitPolicyRefIsComputable(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	policyPath := filepath.Join(root, "policy", "kits", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join(root, "policy", "kits", "ops-v0.1", "data.json")
	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	ref, err := ps.PolicyRef()
	if err != nil {
		t.Fatalf("PolicyRef() error = %v", err)
	}
	if ref == "" {
		t.Fatalf("expected non-empty policy ref")
	}
}
