package mcpserver

import (
	"strings"
	"testing"
)

func TestEmbeddedValidateSchemaLoadable(t *testing.T) {
	t.Parallel()

	schema := mustLoadInputSchema(validateSchemaBytes, "schemas/validate.schema.json")
	if schema == nil {
		t.Fatal("expected non-nil schema")
	}
	if typ, ok := schema["type"].(string); !ok || typ != "object" {
		t.Fatalf("expected schema type object, got %T %v", schema["type"], schema["type"])
	}

	required, ok := schema["required"].([]interface{})
	if !ok || len(required) == 0 {
		t.Fatalf("expected non-empty required array, got %T", schema["required"])
	}
	for _, field := range []string{"actor", "tool", "operation", "params", "context"} {
		if !containsSchemaString(required, field) {
			t.Fatalf("expected required field %q in schema.required", field)
		}
	}

	properties, ok := schema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties object, got %T", schema["properties"])
	}
	actor, ok := properties["actor"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.actor object, got %T", properties["actor"])
	}
	actorProps, ok := actor["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.actor.properties object, got %T", actor["properties"])
	}
	actorType, ok := actorProps["type"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.actor.properties.type object, got %T", actorProps["type"])
	}
	typeEnum, ok := actorType["enum"].([]interface{})
	if !ok {
		t.Fatalf("expected actor.type enum array, got %T", actorType["enum"])
	}
	assertExactEnum(t, typeEnum, []string{"human", "agent", "ci"}, "actor.type")

	actorOrigin, ok := actorProps["origin"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.actor.properties.origin object, got %T", actorProps["origin"])
	}
	originEnum, ok := actorOrigin["enum"].([]interface{})
	if !ok {
		t.Fatalf("expected actor.origin enum array, got %T", actorOrigin["enum"])
	}
	assertExactEnum(t, originEnum, []string{"mcp", "cli", "api"}, "actor.origin")

	params, ok := properties["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.params object, got %T", properties["params"])
	}
	paramsProps, ok := params["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.params.properties object, got %T", params["properties"])
	}
	payload, ok := paramsProps["payload"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.params.properties.payload object, got %T", paramsProps["payload"])
	}
	if desc, ok := payload["description"].(string); !ok || desc == "" {
		t.Fatal("expected non-empty payload description")
	}

	contextObj, ok := properties["context"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.context object, got %T", properties["context"])
	}
	contextProps, ok := contextObj["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.context.properties object, got %T", contextObj["properties"])
	}
	sourceObj, ok := contextProps["source"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected properties.context.properties.source object, got %T", contextProps["source"])
	}
	if sourceType, ok := sourceObj["type"].(string); !ok || sourceType != "string" {
		t.Fatalf("expected context.source type string, got %T %v", sourceObj["type"], sourceObj["type"])
	}
	if sourceDesc, ok := sourceObj["description"].(string); !ok || sourceDesc == "" {
		t.Fatal("expected non-empty context.source description")
	}
	if !strings.Contains(strings.ToLower(sourceObj["description"].(string)), "not used for security classification") {
		t.Fatal("expected context.source description to state it is not used for security classification")
	}
}

func containsSchemaString(values []interface{}, want string) bool {
	for _, v := range values {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}

func assertExactEnum(t *testing.T, got []interface{}, want []string, field string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("expected %s enum length %d, got %d", field, len(want), len(got))
	}
	for i := range want {
		v, ok := got[i].(string)
		if !ok {
			t.Fatalf("expected %s enum[%d] string, got %T", field, i, got[i])
		}
		if v != want[i] {
			t.Fatalf("expected %s enum[%d]=%q, got %q", field, i, want[i], v)
		}
	}
}
