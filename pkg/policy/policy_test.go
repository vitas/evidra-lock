package policy

import (
	"reflect"
	"testing"
)

func TestBuildActionListSingleAction(t *testing.T) {
	params := map[string]interface{}{
		"action": map[string]interface{}{
			"kind": "kubectl.get",
		},
	}
	actions := buildActionList(params)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0]["kind"] != "kubectl.get" {
		t.Fatalf("unexpected action kind: %v", actions[0]["kind"])
	}
}

func TestBuildActionListFallbackActions(t *testing.T) {
	params := map[string]interface{}{
		"actions": []interface{}{
			map[string]interface{}{"kind": "terraform.plan"},
			map[string]interface{}{"kind": "kubectl.apply"},
		},
	}
	actions := buildActionList(params)
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
