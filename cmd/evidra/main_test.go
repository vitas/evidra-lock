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
