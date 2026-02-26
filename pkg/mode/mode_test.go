package mode

import (
	"testing"
)

func TestResolve_NoURL(t *testing.T) {
	t.Parallel()
	r, err := Resolve(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsOnline {
		t.Error("expected offline mode when URL is empty")
	}
	if r.Client != nil {
		t.Error("expected nil client in offline mode")
	}
	if r.FallbackPolicy != "closed" {
		t.Errorf("expected default fallback=closed, got %s", r.FallbackPolicy)
	}
}

func TestResolve_ForceOffline(t *testing.T) {
	t.Parallel()
	r, err := Resolve(Config{
		URL:          "https://evidra.rest",
		APIKey:       "test-key",
		ForceOffline: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsOnline {
		t.Error("expected offline mode when --offline is set")
	}
	if r.Client != nil {
		t.Error("expected nil client when --offline is set")
	}
}

func TestResolve_NoAPIKey(t *testing.T) {
	t.Parallel()
	_, err := Resolve(Config{URL: "https://evidra.rest"})
	if err == nil {
		t.Fatal("expected error when URL is set but API key is missing")
	}
	if err.Error() != "EVIDRA_API_KEY is required when EVIDRA_URL is set" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestResolve_Online(t *testing.T) {
	t.Parallel()
	r, err := Resolve(Config{
		URL:    "https://evidra.rest",
		APIKey: "test-key",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r.IsOnline {
		t.Error("expected online mode")
	}
	if r.Client == nil {
		t.Error("expected non-nil client in online mode")
	}
	if r.Client.URL() != "https://evidra.rest" {
		t.Errorf("expected URL=https://evidra.rest, got %s", r.Client.URL())
	}
	if r.FallbackPolicy != "closed" {
		t.Errorf("expected fallback=closed, got %s", r.FallbackPolicy)
	}
}

func TestResolve_FallbackNormalization(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", "closed"},
		{"closed", "closed", "closed"},
		{"offline", "offline", "offline"},
		{"OFFLINE", "OFFLINE", "offline"},
		{"Offline", "Offline", "offline"},
		{"junk", "junk", "closed"},
		{"whitespace", "  offline  ", "offline"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r, err := Resolve(Config{FallbackPolicy: tt.input})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if r.FallbackPolicy != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, r.FallbackPolicy)
			}
		})
	}
}

func TestResolve_WhitespaceURL(t *testing.T) {
	t.Parallel()
	r, err := Resolve(Config{URL: "  ", APIKey: "key"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.IsOnline {
		t.Error("expected offline mode for whitespace-only URL")
	}
}

func TestResolve_WhitespaceAPIKey(t *testing.T) {
	t.Parallel()
	_, err := Resolve(Config{URL: "https://evidra.rest", APIKey: "  "})
	if err == nil {
		t.Fatal("expected error for whitespace-only API key")
	}
}
