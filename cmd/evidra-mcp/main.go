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

	ps := policysource.NewLocalFilePolicySource(policyPath)
	policyBytes, err := ps.LoadPolicy()
	if err != nil {
		log.Fatalf("load policy source: %v", err)
	}

	var dataBytes []byte
	if dataPath != "" {
		dataBytes, err = os.ReadFile(dataPath)
		if err != nil {
			log.Fatalf("load policy data: %v", err)
		}
	}

	policyEngine, err := policy.NewOPAEngine(policyBytes, dataBytes)
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	evidencePath := envOrDefault("EVIDRA_EVIDENCE_PATH", "./data/evidence.log")
	evidenceStore := evidence.NewStoreWithPath(evidencePath)
	if err := evidenceStore.Init(); err != nil {
		log.Fatalf("init evidence store: %v", err)
	}

	toolRegistry := registry.NewDefaultRegistry()
	server := mcpserver.NewServer(
		mcpserver.Options{
			Name:      "evidra-mcp",
			Version:   "v0.1.0",
			Mode:      mode,
			PolicyRef: ps.PolicyRef(),
		},
		toolRegistry,
		policyEngine,
		evidenceStore,
	)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run mcp server: %v", err)
	}
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
