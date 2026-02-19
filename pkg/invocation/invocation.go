package invocation

import "errors"

type Actor struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Origin string `json:"origin"`
}

type ToolInvocation struct {
	Actor     Actor                  `json:"actor"`
	Tool      string                 `json:"tool"`
	Operation string                 `json:"operation"`
	Params    map[string]interface{} `json:"params"`
	Context   map[string]interface{} `json:"context"`
}

func (ti *ToolInvocation) ValidateStructure() error {
	if ti.Actor.Type == "" {
		return errors.New("actor.type is required")
	}
	if ti.Actor.ID == "" {
		return errors.New("actor.id is required")
	}
	if ti.Actor.Origin == "" {
		return errors.New("actor.origin is required")
	}
	if ti.Tool == "" {
		return errors.New("tool is required")
	}
	if ti.Operation == "" {
		return errors.New("operation is required")
	}
	if ti.Params == nil {
		return errors.New("params is required")
	}
	return nil
}
