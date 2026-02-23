package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/invocation"
)

type executeHandler struct {
	service *ValidateService
}

func registerExecuteTool(server *mcp.Server, svc *ValidateService) {
	executeTool := &executeHandler{service: svc}
	mcp.AddTool(server, &mcp.Tool{
		Name:        "execute",
		Title:       "Execute Tool Invocation",
		Description: "Run the validation pathway and emit hits/hints as execution metadata.",
		Annotations: &mcp.ToolAnnotations{
			Title:           "Execute Scenario",
			ReadOnlyHint:    false,
			IdempotentHint:  false,
			DestructiveHint: boolPtr(false),
			OpenWorldHint:   boolPtr(false),
		},
		InputSchema: map[string]any{
			"type":     "object",
			"required": []any{"actor", "tool", "operation", "params", "context"},
			"properties": map[string]any{
				"actor": map[string]any{
					"type":        "object",
					"description": "Invocation initiator identity.",
					"required":    []any{"type", "id", "origin"},
					"properties": map[string]any{
						"type":   map[string]any{"type": "string", "description": "Actor category (human|agent|system)."},
						"id":     map[string]any{"type": "string", "description": "Actor identifier."},
						"origin": map[string]any{"type": "string", "description": "Invocation source (mcp|cli|api)."},
					},
				},
				"tool":      map[string]any{"type": "string", "description": "Tool name (e.g. terraform)."},
				"operation": map[string]any{"type": "string", "description": "Operation (e.g. plan, apply)."},
				"params":    map[string]any{"type": "object", "description": "Operation parameters; include risk_tags/asserted data."},
				"context":   map[string]any{"type": "object", "description": "Optional context metadata."},
			},
		},
	}, executeTool.Handle)
}

func (h *executeHandler) Handle(
	ctx context.Context,
	_ *mcp.CallToolRequest,
	input invocation.ToolInvocation,
) (*mcp.CallToolResult, ValidateOutput, error) {
	return (&validateHandler{service: h.service}).Handle(ctx, nil, input)
}
