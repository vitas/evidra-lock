package policy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	AllowedCommands []string `yaml:"allowed_commands"`
}

type Engine struct {
	allowed map[string]struct{}
}

func LoadFromFile(path string) (*Engine, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}

	allowed := make(map[string]struct{}, len(cfg.AllowedCommands))
	for _, cmd := range cfg.AllowedCommands {
		if cmd == "" {
			continue
		}
		allowed[cmd] = struct{}{}
	}

	return &Engine{allowed: allowed}, nil
}

func (e *Engine) IsAllowed(command string) bool {
	if e == nil {
		return false
	}
	_, ok := e.allowed[command]
	return ok
}
