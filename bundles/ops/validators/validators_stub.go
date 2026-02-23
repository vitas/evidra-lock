package validators

import (
	"context"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
	"samebits.com/evidra-mcp/bundles/ops/schema"
)

type RunOptions struct {
	Config opscfg.OpsConfig
}

type Report struct{}

type RunResult struct {
	Decision  string
	RiskLevel string
	Reasons   []string
	Hints     []string
	Reports   []Report
}

func RunForScenario(_ context.Context, _ schema.Scenario, _ string, _ RunOptions) (RunResult, map[string]interface{}, error) {
	return RunResult{Decision: "PASS", RiskLevel: "normal"}, map[string]interface{}{}, nil
}
