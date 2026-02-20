package outputlimit

import "testing"

func TestTruncateNoop(t *testing.T) {
	got, truncated := Truncate("hello", 10)
	if truncated {
		t.Fatalf("expected not truncated")
	}
	if got != "hello" {
		t.Fatalf("unexpected output: %q", got)
	}
}

func TestTruncateByBytesUTF8Safe(t *testing.T) {
	in := "ééééé"
	got, truncated := Truncate(in, 5)
	if !truncated {
		t.Fatalf("expected truncated")
	}
	if got == in {
		t.Fatalf("expected changed output")
	}
}
