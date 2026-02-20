package core

import (
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
)

type PolicySource interface {
	LoadPolicy() ([]byte, error)
	PolicyRef() string
}

type PolicyEngine interface {
	Evaluate(inv invocation.ToolInvocation) (policy.Decision, error)
}

type EvidenceStore interface {
	Append(rec evidence.Record) error
	ValidateChain() error
	LastHash() (string, error)
}
