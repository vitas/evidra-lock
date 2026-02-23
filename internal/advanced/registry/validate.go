package registry

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

func ValidateParams(def ToolDefinition, operation string, params map[string]interface{}) error {
	if def.ValidateParams == nil {
		return fmt.Errorf("tool %q has no param validator", def.Name)
	}
	return def.ValidateParams(operation, params)
}

func convertParam(tool, operation, name string, value interface{}, typ string) (string, error) {
	switch typ {
	case "string":
		v, ok := value.(string)
		if !ok {
			return "", newInvalidParam(tool, operation, name, "must be string")
		}
		if strings.Contains(v, "\n") || strings.ContainsRune(v, rune(0)) {
			return "", newInvalidParam(tool, operation, name, "contains disallowed characters")
		}
		if name == "url" && strings.TrimSpace(v) != "" {
			if !strings.HasPrefix(v, "http://") && !strings.HasPrefix(v, "https://") {
				return "", newInvalidParam(tool, operation, name, "must start with http:// or https://")
			}
		}
		return v, nil
	case "int":
		switch x := value.(type) {
		case int:
			return strconv.Itoa(x), nil
		case int64:
			return strconv.FormatInt(x, 10), nil
		case float64:
			if math.Trunc(x) != x {
				return "", newInvalidParam(tool, operation, name, "must be int")
			}
			return strconv.FormatInt(int64(x), 10), nil
		default:
			return "", newInvalidParam(tool, operation, name, "must be int")
		}
	case "bool":
		v, ok := value.(bool)
		if !ok {
			return "", newInvalidParam(tool, operation, name, "must be bool")
		}
		return strconv.FormatBool(v), nil
	default:
		return "", newInvalidParam(tool, operation, name, fmt.Sprintf("unsupported param type %q", typ))
	}
}
