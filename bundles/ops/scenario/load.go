package scenario

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"samebits.com/evidra-mcp/bundles/ops/schema"
)

func LoadFile(path string) (schema.Scenario, error) {
	f, err := os.Open(path)
	if err != nil {
		return schema.Scenario{}, fmt.Errorf("open scenario file: %w", err)
	}
	defer f.Close()

	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()

	var sc schema.Scenario
	if err := dec.Decode(&sc); err != nil {
		return schema.Scenario{}, fmt.Errorf("decode scenario JSON: %w", err)
	}
	if dec.More() {
		return schema.Scenario{}, fmt.Errorf("scenario JSON contains multiple values")
	}

	if err := validate(sc); err != nil {
		return schema.Scenario{}, err
	}
	return sc, nil
}

func validate(sc schema.Scenario) error {
	if strings.TrimSpace(sc.ScenarioID) == "" {
		return fmt.Errorf("scenario_id is required")
	}
	switch strings.TrimSpace(sc.Actor.Type) {
	case "human", "agent", "system":
	default:
		return fmt.Errorf("actor.type must be one of: human|agent|system")
	}
	switch strings.TrimSpace(sc.Source) {
	case "mcp", "cli", "ci":
	default:
		return fmt.Errorf("source must be one of: mcp|cli|ci")
	}
	if sc.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is required")
	}
	if len(sc.Actions) == 0 {
		return fmt.Errorf("actions must contain at least one action")
	}
	for i, a := range sc.Actions {
		if strings.TrimSpace(a.Kind) == "" {
			return fmt.Errorf("actions[%d].kind is required", i)
		}
		if strings.TrimSpace(a.Intent) == "" {
			return fmt.Errorf("actions[%d].intent is required", i)
		}
		if a.Target == nil {
			return fmt.Errorf("actions[%d].target is required", i)
		}
		if a.Payload == nil {
			return fmt.Errorf("actions[%d].payload is required", i)
		}
	}

	_ = time.RFC3339 // explicit timestamp type expectation kept for readability
	return nil
}
