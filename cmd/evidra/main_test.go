package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestOpsHelpMentionsExplain(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"ops", "--help"}, &out, &errOut)
	if code != 2 {
		t.Fatalf("expected exit code 2 for help/usage, got %d", code)
	}
	s := errOut.String()
	if !strings.Contains(s, "explain") {
		t.Fatalf("expected ops help to include explain, got: %s", s)
	}
	if !strings.Contains(s, "init") {
		t.Fatalf("expected ops help to include init, got: %s", s)
	}
}

func TestExplainKindsWiring(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"ops", "explain", "kinds"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d, stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "terraform.plan") {
		t.Fatalf("expected kinds output from explain command, got: %s", out.String())
	}
}

func TestInitPrintWiring(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"ops", "init", "--print"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected init --print code 0, got %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "enable_validators") {
		t.Fatalf("expected config output, got: %s", out.String())
	}
}

func TestVersionCommand(t *testing.T) {
	var out bytes.Buffer
	var errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("expected code 0, got %d stderr=%s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "version:") {
		t.Fatalf("expected version output, got: %s", out.String())
	}
}
