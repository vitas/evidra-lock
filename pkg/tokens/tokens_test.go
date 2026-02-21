package tokens

import "testing"

func TestValidateTemplateUnknownToken(t *testing.T) {
	err := ValidateTemplate("{{x}}", map[string]bool{"y": true})
	if err == nil {
		t.Fatalf("expected unknown placeholder error")
	}
}

func TestExpandTemplateOptionalMissing(t *testing.T) {
	out, err := ExpandTemplate("{{x?}}", map[string]string{})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "" {
		t.Fatalf("expected empty output for missing optional placeholder, got %q", out)
	}
}

func TestExpandTemplateRequiredMissing(t *testing.T) {
	_, err := ExpandTemplate("{{x}}", map[string]string{})
	if err == nil {
		t.Fatalf("expected error for missing required placeholder")
	}
}

func TestExpandTemplateInline(t *testing.T) {
	out, err := ExpandTemplate("-chdir={{dir}}", map[string]string{"dir": "./infra"})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if out != "-chdir=./infra" {
		t.Fatalf("unexpected output: %q", out)
	}
}
