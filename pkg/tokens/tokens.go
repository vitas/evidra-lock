package tokens

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	TokenPattern       = regexp.MustCompile(`^\{\{([a-zA-Z0-9_]+)(\?)?\}\}$`)
	InlineTokenPattern = regexp.MustCompile(`\{\{([a-zA-Z0-9_]+)(\?)?\}\}`)
)

type Placeholder struct {
	Name     string
	Optional bool
}

func Placeholders(template string) []Placeholder {
	matches := InlineTokenPattern.FindAllStringSubmatch(template, -1)
	out := make([]Placeholder, 0, len(matches))
	for _, m := range matches {
		out = append(out, Placeholder{
			Name:     m[1],
			Optional: m[2] == "?",
		})
	}
	return out
}

func ValidateTemplate(template string, allowedKeys map[string]bool) error {
	for _, ph := range Placeholders(template) {
		if !allowedKeys[ph.Name] {
			return fmt.Errorf("placeholder %q is not declared in params", ph.Name)
		}
	}
	return validateDelimiters(template)
}

func ExpandTemplate(template string, values map[string]string) (string, error) {
	full := TokenPattern.FindStringSubmatch(template)
	if len(full) > 0 {
		name := full[1]
		optional := full[2] == "?"
		v, ok := values[name]
		if !ok || v == "" {
			if optional {
				return "", nil
			}
			return "", fmt.Errorf("missing required param: %s", name)
		}
		return v, nil
	}

	expanded := template
	for _, ph := range Placeholders(template) {
		v, ok := values[ph.Name]
		if !ok || v == "" {
			if ph.Optional {
				return "", nil
			}
			return "", fmt.Errorf("missing required param: %s", ph.Name)
		}
		expanded = strings.ReplaceAll(expanded, "{{"+ph.Name+"}}", v)
		expanded = strings.ReplaceAll(expanded, "{{"+ph.Name+"?}}", v)
	}
	if err := validateDelimiters(expanded); err != nil {
		return "", err
	}
	return expanded, nil
}

func validateDelimiters(template string) error {
	s := strings.ReplaceAll(strings.ReplaceAll(template, "{{", ""), "}}", "")
	if strings.Contains(s, "{{") || strings.Contains(s, "}}") {
		return fmt.Errorf("invalid template token: %s", template)
	}
	if strings.Count(template, "{{") != strings.Count(template, "}}") {
		return fmt.Errorf("invalid template token: %s", template)
	}
	return nil
}
