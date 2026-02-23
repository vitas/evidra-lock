package validators

type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

type Finding struct {
	Tool     string      `json:"tool"`
	Severity Severity    `json:"severity"`
	Title    string      `json:"title"`
	Message  string      `json:"message"`
	Resource string      `json:"resource,omitempty"`
	RuleID   string      `json:"rule_id,omitempty"`
	File     string      `json:"file,omitempty"`
	Line     int         `json:"line,omitempty"`
	Raw      interface{} `json:"raw,omitempty"`
}

type Report struct {
	Tool       string                 `json:"tool"`
	ExitCode   int                    `json:"exit_code"`
	DurationMS int64                  `json:"duration_ms"`
	Findings   []Finding              `json:"findings"`
	Summary    map[string]interface{} `json:"summary,omitempty"`
}

type RunResult struct {
	Reports   []Report `json:"reports"`
	Decision  string   `json:"decision"`
	RiskLevel string   `json:"risk_level"`
	Reasons   []string `json:"reasons"`
}
