package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	DefaultConfigPath = ".evidra/ops.yaml"
)

type OpsConfig struct {
	EnableValidators bool             `yaml:"enable_validators" json:"enable_validators"`
	Validators       ValidatorsConfig `yaml:"validators" json:"validators"`
	Decision         DecisionConfig   `yaml:"decision" json:"decision"`
}

type ValidatorsConfig struct {
	Builtins    []string           `yaml:"builtins" json:"builtins"`
	ExecPlugins []ExecPluginConfig `yaml:"exec_plugins" json:"exec_plugins"`
}

type DecisionConfig struct {
	FailOn []string `yaml:"fail_on" json:"fail_on"`
	WarnOn []string `yaml:"warn_on" json:"warn_on"`
}

type ExecPluginConfig struct {
	Name            string            `yaml:"name" json:"name"`
	Command         string            `yaml:"command" json:"command"`
	Args            []string          `yaml:"args" json:"args"`
	ApplicableKinds []string          `yaml:"applicable_kinds" json:"applicable_kinds"`
	Env             map[string]string `yaml:"env" json:"env"`
	TimeoutSeconds  int               `yaml:"timeout_seconds" json:"timeout_seconds"`
}

func Default() OpsConfig {
	return OpsConfig{
		EnableValidators: false,
		Validators: ValidatorsConfig{
			Builtins: []string{"terraform", "kubeconform", "trivy"},
		},
		Decision: DecisionConfig{
			FailOn: []string{"high", "critical"},
			WarnOn: []string{},
		},
	}
}

func Load(path string) (OpsConfig, error) {
	cfg := Default()
	if strings.TrimSpace(path) == "" {
		path = DefaultConfigPath
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return OpsConfig{}, fmt.Errorf("read ops config: %w", err)
	}

	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&cfg); err != nil {
		return OpsConfig{}, fmt.Errorf("parse ops config %s: %w", filepath.Clean(path), err)
	}

	normalizeConfig(&cfg)
	return cfg, nil
}

func normalizeConfig(cfg *OpsConfig) {
	if cfg == nil {
		return
	}
	if len(cfg.Validators.Builtins) == 0 {
		cfg.Validators.Builtins = []string{"terraform", "kubeconform", "trivy"}
	}
	if len(cfg.Decision.FailOn) == 0 {
		cfg.Decision.FailOn = []string{"high", "critical"}
	}
	for i := range cfg.Validators.ExecPlugins {
		if cfg.Validators.ExecPlugins[i].TimeoutSeconds <= 0 {
			cfg.Validators.ExecPlugins[i].TimeoutSeconds = 30
		}
	}
}
