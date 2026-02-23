package ops

import (
	"context"

	"samebits.com/evidra-mcp/pkg/validate"
)

type ValidationOutput = validate.Result

type ActionFact = validate.ActionFact

type ValidateOptions = validate.Options

type AvailableValidators struct {
	Builtins    []string `json:"builtins"`
	ExecPlugins []string `json:"exec_plugins"`
}

func ValidateFile(path string) (ValidationOutput, error) {
	return ValidateFileWithOptions(path, ValidateOptions{})
}

func ValidateFileWithOptions(path string, opts ValidateOptions) (ValidationOutput, error) {
	return validate.EvaluateFile(context.Background(), path, opts)
}

func ListAvailableValidators(configPath string) (AvailableValidators, error) {
	return AvailableValidators{}, nil
}
