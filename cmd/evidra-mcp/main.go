package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/registry"
	kubectlplugin "samebits.com/evidra-mcp/plugins/kubectl"
)

func main() {
	mode, err := loadModeFromEnv()
	if err != nil {
		log.Fatalf("load mode: %v", err)
	}
	log.Printf("Evidra mode: %s", mode)
	if mode == mcpserver.ModeObserve {
		log.Printf("Evidra running in OBSERVE mode. Policy violations will NOT block execution.")
	}

	policyPath := envOrDefault("EVIDRA_POLICY_PATH", "./policy/policy.rego")
	dataPath := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_DATA_PATH"))

	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	policyBytes, err := ps.LoadPolicy()
	if err != nil {
		log.Fatalf("load policy source: %v", err)
	}

	dataBytes, err := ps.LoadData()
	if err != nil {
		log.Fatalf("load policy data: %v", err)
	}

	policyEngine, err := policy.NewOPAEngine(policyBytes, dataBytes)
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	evidencePath := envOrDefault("EVIDRA_EVIDENCE_PATH", "./data/evidence")
	evidenceStore := evidence.NewStoreWithPath(evidencePath)
	if err := evidenceStore.Init(); err != nil {
		log.Fatalf("init evidence store: %v", err)
	}

	toolRegistry := registry.NewDefaultRegistry()
	if err := kubectlplugin.New().Register(toolRegistry); err != nil {
		log.Fatalf("register kubectl plugin: %v", err)
	}
	server := mcpserver.NewServer(
		mcpserver.Options{
			Name:         "evidra-mcp",
			Version:      "v0.1.0",
			Mode:         mode,
			PolicyRef:    mustPolicyRef(ps),
			EvidencePath: evidencePath,
		},
		toolRegistry,
		policyEngine,
		evidenceStore,
	)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run mcp server: %v", err)
	}
}

func mustPolicyRef(ps *policysource.LocalFileSource) string {
	ref, err := ps.PolicyRef()
	if err != nil {
		log.Fatalf("compute policy ref: %v", err)
	}
	return ref
}

func loadModeFromEnv() (mcpserver.Mode, error) {
	raw := strings.TrimSpace(os.Getenv("EVIDRA_MODE"))
	if raw == "" {
		return mcpserver.ModeEnforce, nil
	}
	switch strings.ToLower(raw) {
	case string(mcpserver.ModeEnforce):
		return mcpserver.ModeEnforce, nil
	case string(mcpserver.ModeObserve):
		return mcpserver.ModeObserve, nil
	default:
		return "", fmt.Errorf("invalid EVIDRA_MODE %q (allowed: enforce, observe)", raw)
	}
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}
