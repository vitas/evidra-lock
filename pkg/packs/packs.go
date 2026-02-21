package packs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.yaml.in/yaml/v3"

	"samebits.com/evidra-mcp/pkg/registry"
	"samebits.com/evidra-mcp/pkg/tokens"
)

type ParamSpec struct {
	Type     string `yaml:"type"`
	Required bool   `yaml:"required"`
}

type OperationSpec struct {
	Name   string               `yaml:"name"`
	Args   []string             `yaml:"args"`
	Params map[string]ParamSpec `yaml:"params"`
}

type ToolSpec struct {
	Name       string          `yaml:"name"`
	Kind       string          `yaml:"kind"`
	Binary     string          `yaml:"binary"`
	Operations []OperationSpec `yaml:"operations"`
}

type PackSpec struct {
	Pack    string     `yaml:"pack"`
	Version string     `yaml:"version"`
	Tools   []ToolSpec `yaml:"tools"`
}

func LoadToolDefinitions(dir string, existingTools []string) ([]registry.ToolDefinition, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	seenToolNames := make(map[string]struct{}, len(existingTools))
	for _, name := range existingTools {
		seenToolNames[name] = struct{}{}
	}

	packDirs := make([]string, 0)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		packFile := filepath.Join(dir, e.Name(), "pack.yaml")
		if _, err := os.Stat(packFile); err == nil {
			packDirs = append(packDirs, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(packDirs)

	defs := make([]registry.ToolDefinition, 0)
	for _, pdir := range packDirs {
		spec, err := loadPackFile(filepath.Join(pdir, "pack.yaml"))
		if err != nil {
			return nil, err
		}
		if spec.Version != "0.1" {
			return nil, fmt.Errorf("pack %q has unsupported version %q", spec.Pack, spec.Version)
		}

		for _, tool := range spec.Tools {
			name := strings.TrimSpace(tool.Name)
			if name == "" {
				return nil, fmt.Errorf("pack %q contains tool with empty name", spec.Pack)
			}
			if _, exists := seenToolNames[name]; exists {
				return nil, fmt.Errorf("duplicate tool name %q across built-ins/packs", name)
			}
			seenToolNames[name] = struct{}{}

			def, err := buildDefinition(name, tool)
			if err != nil {
				return nil, fmt.Errorf("pack %q tool %q: %w", spec.Pack, name, err)
			}
			defs = append(defs, def)
		}
	}

	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs, nil
}

func loadPackFile(path string) (PackSpec, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return PackSpec{}, err
	}
	var spec PackSpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		return PackSpec{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if strings.TrimSpace(spec.Pack) == "" {
		return PackSpec{}, fmt.Errorf("pack name is required")
	}
	return spec, nil
}

func buildDefinition(name string, tool ToolSpec) (registry.ToolDefinition, error) {
	if tool.Kind != "cli" {
		return registry.ToolDefinition{}, fmt.Errorf("unsupported kind %q", tool.Kind)
	}
	if strings.TrimSpace(tool.Binary) == "" {
		return registry.ToolDefinition{}, fmt.Errorf("binary is required")
	}

	opMap := make(map[string]registry.CLIOperationSpec, len(tool.Operations))
	seenOps := make(map[string]struct{}, len(tool.Operations))
	for _, op := range tool.Operations {
		opName := strings.TrimSpace(op.Name)
		if opName == "" {
			return registry.ToolDefinition{}, fmt.Errorf("operation name is required")
		}
		if _, exists := seenOps[opName]; exists {
			return registry.ToolDefinition{}, fmt.Errorf("duplicate operation %q", opName)
		}
		seenOps[opName] = struct{}{}

		if len(op.Args) == 0 {
			return registry.ToolDefinition{}, fmt.Errorf("operation %q args are required", opName)
		}
		paramRules, err := toParamRules(op.Params)
		if err != nil {
			return registry.ToolDefinition{}, fmt.Errorf("operation %q: %w", opName, err)
		}
		if err := validateArgsAndParams(op.Args, paramRules); err != nil {
			return registry.ToolDefinition{}, fmt.Errorf("operation %q: %w", opName, err)
		}

		opMap[opName] = registry.CLIOperationSpec{Args: op.Args, Params: paramRules}
	}

	return registry.NewDeclarativeCLIToolDefinition(
		name,
		"declarative cli tool pack",
		registry.CLIToolSpec{Binary: tool.Binary, Operations: opMap},
	)
}

func toParamRules(in map[string]ParamSpec) (map[string]registry.ParamRule, error) {
	out := make(map[string]registry.ParamRule, len(in))
	for name, spec := range in {
		t := strings.TrimSpace(spec.Type)
		switch t {
		case "string", "int", "bool":
		default:
			return nil, fmt.Errorf("param %q has unsupported type %q", name, t)
		}
		out[name] = registry.ParamRule{Type: t, Required: spec.Required}
	}
	return out, nil
}

func validateArgsAndParams(args []string, params map[string]registry.ParamRule) error {
	used := map[string]struct{}{}
	allowed := map[string]bool{}
	for name := range params {
		allowed[name] = true
	}
	for _, arg := range args {
		if err := tokens.ValidateTemplate(arg, allowed); err != nil {
			return err
		}
		required, optional, err := tokens.ExtractTokens(arg)
		if err != nil {
			return err
		}
		for _, name := range required {
			used[name] = struct{}{}
		}
		for _, name := range optional {
			used[name] = struct{}{}
		}
	}
	for name := range params {
		if _, ok := used[name]; !ok {
			return fmt.Errorf("param %q is declared but not used in args", name)
		}
	}
	return nil
}
