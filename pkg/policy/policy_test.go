package policy

import (
	"reflect"
	"testing"
)

func TestBuildActionList_SingleAction(t *testing.T) {
	t.Parallel()
	params := map[string]interface{}{
		"action": map[string]interface{}{
			"kind": "kubectl.get",
		},
	}
	actions, err := buildActionList(params, "cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0]["kind"] != "kubectl.get" {
		t.Fatalf("unexpected action kind: %v", actions[0]["kind"])
	}
}

func TestBuildActionList_FallbackActions(t *testing.T) {
	t.Parallel()
	params := map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{"kind": "terraform.plan"},
			map[string]interface{}{"kind": "kubectl.apply"},
		},
	}
	actions, err := buildActionList(params, "cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	expected := []string{"terraform.plan", "kubectl.apply"}
	var kinds []string
	for _, act := range actions {
		kinds = append(kinds, act["kind"].(string))
	}
	if !reflect.DeepEqual(kinds, expected) {
		t.Fatalf("expected %v, got %v", expected, kinds)
	}
}

func TestBuildActionList_RejectActionsFromMCP(t *testing.T) {
	t.Parallel()
	params := map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{"kind": "kubectl.apply"},
			map[string]interface{}{"kind": "kubectl.delete"},
		},
	}
	_, err := buildActionList(params, "mcp")
	if err == nil {
		t.Fatal("expected error for actions from MCP origin, got nil")
	}
}

func TestBuildActionList_AllowActionsFromCLI(t *testing.T) {
	t.Parallel()
	params := map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{"kind": "kubectl.apply"},
			map[string]interface{}{"kind": "kubectl.delete"},
		},
	}
	actions, err := buildActionList(params, "cli")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
}

func TestBuildActionList_SingleActionFromMCP(t *testing.T) {
	t.Parallel()
	params := map[string]interface{}{
		"action": map[string]interface{}{
			"kind": "kubectl.apply",
		},
	}
	actions, err := buildActionList(params, "mcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
}

func TestIsMCPOrigin(t *testing.T) {
	t.Parallel()
	tests := []struct {
		origin string
		want   bool
	}{
		{"mcp", true},
		{"mcp-server", true},
		{"cli", false},
		{"api", false},
		{"test", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			t.Parallel()
			if got := isMCPOrigin(tt.origin); got != tt.want {
				t.Errorf("isMCPOrigin(%q) = %v, want %v", tt.origin, got, tt.want)
			}
		})
	}
}
