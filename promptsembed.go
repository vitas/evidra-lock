package evidra

import "embed"

// MCPServerContentFS is the built-in guidance content used by evidra-mcp when
// no external content directory is provided.
//
//go:embed all:prompts/mcpserver
var MCPServerContentFS embed.FS
