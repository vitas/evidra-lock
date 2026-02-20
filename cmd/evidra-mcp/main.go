package main

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/registry"
)

func main() {
	policyEngine, err := policy.LoadFromFile("policy/policy.rego")
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	evidenceStore := evidence.NewStore()
	if err := evidenceStore.Init(); err != nil {
		log.Fatalf("init evidence store: %v", err)
	}

	toolRegistry := registry.NewDefaultRegistry()
	server := mcpserver.NewServer(
		mcpserver.Options{Name: "evidra-mcp", Version: "v0.1.0"},
		toolRegistry,
		policyEngine,
		evidenceStore,
	)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run mcp server: %v", err)
	}
}
