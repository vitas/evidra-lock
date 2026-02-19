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
