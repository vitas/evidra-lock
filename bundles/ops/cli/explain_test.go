package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestExplainSchema(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Explain([]string{"schema"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, `"scenario_id"`) {
		t.Fatalf("expected schema to contain scenario_id, got: %s", s)
	}
	if !strings.Contains(s, `"actions"`) {
		t.Fatalf("expected schema to contain actions, got: %s", s)
	}
}

func TestExplainKinds(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Explain([]string{"kinds"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errOut.String())
	}
	s := out.String()
	for _, k := range []string{
		"terraform.plan",
		"kustomize.build",
		"kubectl.apply",
		"kubectl.delete",
		"helm.upgrade",
	} {
		if !strings.Contains(s, k) {
			t.Fatalf("expected output to include %s, got: %s", k, s)
		}
	}
}

func TestExplainExample(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Explain([]string{"example"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, `"scenario_id"`) || !strings.Contains(s, `"actions"`) {
		t.Fatalf("expected minimal scenario JSON, got: %s", s)
	}
}

func TestExplainPolicies(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := Explain([]string{"policies"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "Active guardrail rules:") {
		t.Fatalf("expected policy summary header, got: %s", s)
	}
	if !strings.Contains(s, "terraform.plan") {
		t.Fatalf("expected policy summary to mention terraform.plan, got: %s", s)
	}
}
