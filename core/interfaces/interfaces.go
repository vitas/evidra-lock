package interfaces

import (
	"samebits.com/evidra-mcp/pkg/invocation"
)

// TODO(monorepo-split): move core/interfaces to a standalone core module.

type ScenarioDecision struct {
	Allow     bool   `json:"allow"`
	RiskLevel string `json:"risk_level"`
	Reason    string `json:"reason"`
	PolicyRef string `json:"policy_ref,omitempty"`
}

// ScenarioEvaluator is the narrative-neutral core evaluation contract.
type ScenarioEvaluator interface {
	EvaluateInvocation(inv invocation.ToolInvocation) (ScenarioDecision, error)
}
