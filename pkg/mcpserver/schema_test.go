package mcpserver

import "testing"

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
}

func containsSchemaString(values []interface{}, want string) bool {
	for _, v := range values {
		if s, ok := v.(string); ok && s == want {
			return true
		}
	}
	return false
}
