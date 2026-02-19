package main

import (
	"context"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/executor"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/policy"
)

func main() {
	policyEngine, err := policy.LoadFromFile("config/policy.yaml")
	if err != nil {
		log.Fatalf("load policy: %v", err)
	}

	evidenceStore := evidence.NewStore()
	if err := evidenceStore.Init(); err != nil {
		log.Fatalf("init evidence store: %v", err)
	}

	runner := executor.NewRunner(10 * time.Second)
	server := mcpserver.NewServer(
		mcpserver.Options{Name: "evidra-mcp", Version: "v0.1.0"},
		policyEngine,
		runner,
		evidenceStore,
	)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("run mcp server: %v", err)
	}
}
