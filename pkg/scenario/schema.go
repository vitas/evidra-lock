package scenario

import "time"

// Scenario is the stable ops scenario envelope for pre-execution validation.
type Scenario struct {
	ScenarioID string    `json:"scenario_id"`
	Actor      Actor     `json:"actor"`
	Source     string    `json:"source"`
	Timestamp  time.Time `json:"timestamp"`
	Actions    []Action  `json:"actions"`
}

type Actor struct {
	Type   string `json:"type"`
	ID     string `json:"id,omitempty"`
	Origin string `json:"origin,omitempty"`
}

type Action struct {
	Kind     string                 `json:"kind"`
	Target   map[string]interface{} `json:"target"`
	Intent   string                 `json:"intent"`
	Payload  map[string]interface{} `json:"payload"`
	RiskTags []string               `json:"risk_tags"`
}
