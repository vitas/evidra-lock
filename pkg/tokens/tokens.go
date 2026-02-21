package tokens

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	tokenNamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
)

type Placeholder struct {
	Name     string
	Optional bool
}

func ExtractTokens(template string) (required []string, optional []string, err error) {
	parts, err := parseTemplate(template)
	if err != nil {
		return nil, nil, err
	}
	reqSet := map[string]struct{}{}
	optSet := map[string]struct{}{}
	for _, p := range parts {
		if !p.isToken {
			continue
		}
		if p.optional {
			optSet[p.name] = struct{}{}
		} else {
			reqSet[p.name] = struct{}{}
		}
	}
	required = setToSortedSlice(reqSet)
	optional = setToSortedSlice(optSet)
	return required, optional, nil
}

func ValidateTemplate(template string, allowedKeys map[string]bool) error {
	required, optional, err := ExtractTokens(template)
	if err != nil {
		return err
	}
	for _, name := range required {
		if !allowedKeys[name] {
			return fmt.Errorf("placeholder %q is not declared in params", name)
		}
	}
	for _, name := range optional {
		if !allowedKeys[name] {
			return fmt.Errorf("placeholder %q is not declared in params", name)
		}
	}
	return nil
}

func Placeholders(template string) ([]Placeholder, error) {
	parts, err := parseTemplate(template)
	if err != nil {
		return nil, err
	}
	seen := map[string]Placeholder{}
	order := make([]string, 0)
	for _, p := range parts {
		if !p.isToken {
			continue
		}
		if _, exists := seen[p.name]; !exists {
			order = append(order, p.name)
			seen[p.name] = Placeholder{Name: p.name, Optional: p.optional}
			continue
		}
		// If the same token appears with required and optional forms, required wins.
		current := seen[p.name]
		if current.Optional && !p.optional {
			current.Optional = false
			seen[p.name] = current
		}
	}
	out := make([]Placeholder, 0, len(order))
	for _, name := range order {
		out = append(out, seen[name])
	}
	return out, nil
}

func ExpandTemplate(template string, values map[string]string) (string, error) {
	parts, err := parseTemplate(template)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, p := range parts {
		if !p.isToken {
			b.WriteString(p.text)
			continue
		}
		v, ok := values[p.name]
		if !ok || v == "" {
			if p.optional {
				continue
			}
			return "", fmt.Errorf("missing required param: %s", p.name)
		}
		b.WriteString(v)
	}
	return b.String(), nil
}

type templatePart struct {
	isToken  bool
	text     string
	name     string
	optional bool
}

func parseTemplate(template string) ([]templatePart, error) {
	parts := []templatePart{}
	i := 0
	for i < len(template) {
		start := strings.Index(template[i:], "{{")
		if start < 0 {
			parts = append(parts, templatePart{text: template[i:]})
			break
		}
		start += i
		if start > i {
			parts = append(parts, templatePart{text: template[i:start]})
		}
		endRel := strings.Index(template[start+2:], "}}")
		if endRel < 0 {
			return nil, fmt.Errorf("invalid template token: %s", template)
		}
		end := start + 2 + endRel
		raw := template[start+2 : end]
		if strings.Contains(raw, "{{") || strings.Contains(raw, "}}") {
			return nil, fmt.Errorf("invalid template token: %s", template)
		}
		name, optional, err := parseTokenBody(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid template token: %s", template)
		}
		parts = append(parts, templatePart{isToken: true, name: name, optional: optional})
		i = end + 2
	}
	return parts, nil
}

func parseTokenBody(body string) (name string, optional bool, err error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return "", false, fmt.Errorf("empty token")
	}
	if strings.HasSuffix(body, "?") {
		optional = true
		body = strings.TrimSuffix(body, "?")
	}
	if body == "" {
		return "", false, fmt.Errorf("invalid optional token")
	}
	if !tokenNamePattern.MatchString(body) {
		return "", false, fmt.Errorf("invalid token name")
	}
	return body, optional, nil
}

func setToSortedSlice(in map[string]struct{}) []string {
	out := make([]string, 0, len(in))
	for k := range in {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
