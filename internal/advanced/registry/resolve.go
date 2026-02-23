package registry

func SupportsOperation(def ToolDefinition, operation string) bool {
	for _, op := range def.SupportedOperations {
		if op == operation {
			return true
		}
	}
	return false
}

func ResolveOperation(reg Registry, tool, operation string) (ToolDefinition, error) {
	def, ok := reg.Lookup(tool)
	if !ok {
		return ToolDefinition{}, ErrToolNotFound{Tool: tool}
	}
	if !SupportsOperation(def, operation) {
		return ToolDefinition{}, ErrOperationNotFound{Tool: tool, Operation: operation}
	}
	return def, nil
}
