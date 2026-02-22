package main

import (
	"os"
	"path/filepath"
	"testing"

	"samebits.com/evidra-mcp/pkg/packs"
	"samebits.com/evidra-mcp/pkg/policysource"
)

func TestResolvePolicyPathsDefaults(t *testing.T) {
	t.Setenv("EVIDRA_POLICY_PATH", "")
	t.Setenv("EVIDRA_DATA_PATH", "")
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(origWd, "..", ".."))
	t.Cleanup(func() { _ = os.Chdir(origWd) })
	if err := os.Chdir(repoRoot); err != nil {
		t.Fatalf("Chdir(repo root) error = %v", err)
	}
	policyPath, dataPath, err := resolvePolicyPaths("", "")
	if err != nil {
		t.Fatalf("resolvePolicyPaths() error = %v", err)
	}
	if policyPath != defaultPolicyPath {
		t.Fatalf("expected default policy path %q, got %q", defaultPolicyPath, policyPath)
	}
	if dataPath != defaultDataPath {
		t.Fatalf("expected default data path %q, got %q", defaultDataPath, dataPath)
	}
}

func TestResolvePolicyPathsFlagsRequireData(t *testing.T) {
	_, _, err := resolvePolicyPaths("/tmp/policy.rego", "")
	if err == nil {
		t.Fatalf("expected error when --policy provided without --data")
	}
}

func TestResolvePolicyPathsEnvRequiresBoth(t *testing.T) {
	t.Setenv("EVIDRA_POLICY_PATH", "/tmp/policy.rego")
	t.Setenv("EVIDRA_DATA_PATH", "")
	_, _, err := resolvePolicyPaths("", "")
	if err == nil {
		t.Fatalf("expected error when only policy env is set")
	}
}

func TestDefaultPacksDirLoadsArgoCDPack(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	packDir := filepath.Join(root, defaultPacksDir)

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
	foundTerraform := false
	for _, def := range defs {
		switch def.Name {
		case "argocd":
			foundArgoCD = true
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("argocd ops pack must not include version operation")
				}
			}
		case "aws":
			foundAWS = true
			for _, op := range def.SupportedOperations {
				if op == "sts-whoami" {
					t.Fatalf("aws ops pack must not include sts-whoami operation")
				}
			}
		case "docker":
			foundDocker = true
		case "docker-compose":
			foundCompose = true
		case "helm":
			foundHelm = true
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("helm ops pack must not include version operation")
				}
			}
		case "kubectl":
			foundKubectl = true
		case "terraform":
			foundTerraform = true
			for _, op := range def.SupportedOperations {
				if op == "version" {
					t.Fatalf("terraform ops pack must not include version operation")
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
	if !foundHelm {
		t.Fatalf("expected helm tool from ops packs")
	}
	if !foundKubectl {
		t.Fatalf("expected kubectl tool from ops packs")
	}
	if !foundTerraform {
		t.Fatalf("expected terraform tool from ops packs")
	}
}

func TestOpsPolicyProfilePolicyRefIsComputable(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	root := filepath.Clean(filepath.Join(wd, "..", ".."))
	policyPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "policy.rego")
	dataPath := filepath.Join(root, "policy", "profiles", "ops-v0.1", "data.json")
	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	ref, err := ps.PolicyRef()
	if err != nil {
		t.Fatalf("PolicyRef() error = %v", err)
	}
	if ref == "" {
		t.Fatalf("expected non-empty policy ref")
	}
}
