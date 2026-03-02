package mcpserver

import (
	_ "embed"
	"encoding/json"
	"fmt"
)

//go:embed schemas/validate.schema.json
var validateSchemaBytes []byte

func mustLoadInputSchema(raw []byte, name string) map[string]any {
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		panic(fmt.Sprintf("failed to parse embedded MCP schema %s: %v", name, err))
	}
	return schema
}
