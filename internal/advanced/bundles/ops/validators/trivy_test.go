package validators

import "testing"

func TestMapTrivySeverity(t *testing.T) {
	tests := []struct {
		in   string
		want Severity
	}{
		{"CRITICAL", SeverityCritical},
		{"HIGH", SeverityHigh},
		{"MEDIUM", SeverityMedium},
		{"LOW", SeverityLow},
		{"UNKNOWN", SeverityInfo},
		{"", SeverityInfo},
	}
	for _, tt := range tests {
		if got := mapTrivySeverity(tt.in); got != tt.want {
			t.Fatalf("mapTrivySeverity(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
