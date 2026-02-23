package ops

import (
	"context"
	"strings"

	opscfg "samebits.com/evidra-mcp/bundles/ops/config"
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
	cfg, err := opscfg.Load(configPath)
	if err != nil {
		return AvailableValidators{}, err
	}
	builtins := []string{"terraform", "kubeconform", "trivy"}
	plugins := make([]string, 0, len(cfg.Validators.ExecPlugins))
	for _, p := range cfg.Validators.ExecPlugins {
		name := strings.TrimSpace(p.Name)
		if name == "" {
			continue
		}
		plugins = append(plugins, name)
	}
	return AvailableValidators{
		Builtins:    builtins,
		ExecPlugins: plugins,
	}, nil
}
