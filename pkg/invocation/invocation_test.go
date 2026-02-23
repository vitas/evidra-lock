package invocation

import "testing"

func TestValidateStructure_ValidInvocation(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{
			Type:   "human",
			ID:     "user-1",
			Origin: "cli",
		},
		Tool:      "git",
		Operation: "status",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}

	if err := ti.ValidateStructure(); err != nil {
		t.Fatalf("expected no validation error, got: %v", err)
	}
}

func TestValidateStructure_MissingActorFieldsFail(t *testing.T) {
	tests := []struct {
		name  string
		actor Actor
	}{
		{
			name: "missing type",
			actor: Actor{
				ID:     "user-1",
				Origin: "cli",
			},
		},
		{
			name: "missing id",
			actor: Actor{
				Type:   "human",
				Origin: "cli",
			},
		},
		{
			name: "missing origin",
			actor: Actor{
				Type: "human",
				ID:   "user-1",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ti := ToolInvocation{
				Actor:     tc.actor,
				Tool:      "git",
				Operation: "status",
				Params:    map[string]interface{}{},
				Context:   map[string]interface{}{},
			}

			if err := ti.ValidateStructure(); err == nil {
				t.Fatal("expected validation error, got nil")
			}
		})
	}
}

func TestValidateStructure_MissingToolFails(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{
			Type:   "human",
			ID:     "user-1",
			Origin: "cli",
		},
		Operation: "status",
		Params:    map[string]interface{}{},
		Context:   map[string]interface{}{},
	}

	if err := ti.ValidateStructure(); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestValidateStructure_MissingOperationFails(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{
			Type:   "human",
			ID:     "user-1",
			Origin: "cli",
		},
		Tool:    "git",
		Params:  map[string]interface{}{},
		Context: map[string]interface{}{},
	}

	if err := ti.ValidateStructure(); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestValidateStructure_InvalidTargetFails(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{Type: "human", ID: "1", Origin: "cli"},
		Tool: "test", Operation: "run",
		Params: map[string]interface{}{
			KeyTarget: "not-a-map",
		},
	}
	err := ti.ValidateStructure()
	if err == nil {
		t.Fatal("expected validation error for invalid target, got nil")
	}
	if err.Error() != "field 'target' must be a JSON object" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateStructure_InvalidPayloadFails(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{Type: "human", ID: "1", Origin: "cli"},
		Tool: "test", Operation: "run",
		Params: map[string]interface{}{
			KeyPayload: 123,
		},
	}
	err := ti.ValidateStructure()
	if err == nil {
		t.Fatal("expected validation error for invalid payload, got nil")
	}
	if err.Error() != "field 'payload' must be a JSON object" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestValidateStructure_InvalidRiskTagsFails(t *testing.T) {
	ti := ToolInvocation{
		Actor: Actor{Type: "human", ID: "1", Origin: "cli"},
		Tool: "test", Operation: "run",
		Params: map[string]interface{}{
			KeyRiskTags: []interface{}{"high", 123},
		},
	}
	err := ti.ValidateStructure()
	if err == nil {
		t.Fatal("expected validation error for invalid risk_tags, got nil")
	}
	if err.Error() != "field 'risk_tags' must be a list of strings" {
		t.Fatalf("unexpected error message: %v", err)
	}
}
