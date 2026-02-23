package registry

import "testing"

func TestMetadataFromOperationsInfersFlags(t *testing.T) {
	ops := map[string]CLIOperationSpec{
		"apply":  {Args: []string{"apply"}, Params: map[string]ParamRule{}},
		"delete": {Args: []string{"delete"}, Params: map[string]ParamRule{}},
	}
	meta := MetadataFromOperations(ops)
	if !meta.LongRunning {
		t.Fatalf("expected long running flag, got %+v", meta)
	}
	if !meta.Destructive {
		t.Fatalf("expected destructive flag, got %+v", meta)
	}
	if !containsString(meta.Labels, "destructive") {
		t.Fatalf("metadata missing destructive label: %v", meta.Labels)
	}
	if !containsString(meta.Labels, "long-running") {
		t.Fatalf("metadata missing long-running label: %v", meta.Labels)
	}
}

func TestMetadataFromOperationsEmpty(t *testing.T) {
	meta := MetadataFromOperations(map[string]CLIOperationSpec{})
	if meta.LongRunning || meta.Destructive {
		t.Fatalf("expected zero metadata for empty operations, got %+v", meta)
	}
	if len(meta.Labels) != 0 {
		t.Fatalf("expected no labels for empty operations, got %v", meta.Labels)
	}
}

func containsString(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
