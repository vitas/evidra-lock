package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"samebits.com/evidra/pkg/client"
	"samebits.com/evidra/pkg/config"
	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/mode"
	"samebits.com/evidra/pkg/scenario"
	"samebits.com/evidra/pkg/validate"
	"samebits.com/evidra/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(stderr)
		return 4
	}

	switch args[0] {
	case "version":
		fmt.Fprintf(stdout, "Version: %s\nCommit: %s\nDate: %s\n", version.Version, version.Commit, version.Date)
		return 0
	case "validate":
		return runValidate(args[1:], stdout, stderr)
	case "evidence":
		return runEvidenceCommand(args[1:], stdout, stderr)
	case "policy":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "usage: evidra policy sim --policy <path> --input <path> [--data <path>]")
			return 4
		}
		switch args[1] {
		case "sim":
			return runPolicySimCommand(args[2:], stdout, stderr)
		default:
			fmt.Fprintln(stderr, "usage: evidra policy sim --policy <path> --input <path> [--data <path>]")
			return 4
		}
	default:
		printUsage(stderr)
		return 4
	}
}

type localOpts struct {
	PolicyPath  string
	DataPath    string
	BundlePath  string
	Environment string
	Verbose     bool
	Stderr      io.Writer
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		printValidateUsage(stderr)
		return 4
	}

	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	// Output flags
	jsonOut := fs.Bool("json", false, "output structured JSON")
	explain := fs.Bool("explain", false, "print a human-readable explanation for the decision")
	verbose := fs.Bool("verbose", false, "log request IDs, mode, timing to stderr")

	// Policy flags (offline/fallback only)
	policyFlag := fs.String("policy", "", "Path to policy rego file")
	dataFlag := fs.String("data", "", "Path to policy data JSON file")
	bundleFlag := fs.String("bundle", "", "Path to OPA bundle directory")
	envFlag := fs.String("environment", "", "Environment label for policy evaluation")

	// Connection flags
	offlineFlag := fs.Bool("offline", false, "Force offline mode")
	fallbackOffline := fs.Bool("fallback-offline", false, "Allow local eval when API unreachable")
	urlFlag := fs.String("url", "", "Evidra API URL (overrides EVIDRA_URL)")
	apiKeyFlag := fs.String("api-key", "", "API key (overrides EVIDRA_API_KEY)")
	timeoutFlag := fs.String("timeout", "", "HTTP timeout for API calls (default: 30s)")

	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		printValidateUsage(stderr)
		return 4
	}

	path := fs.Arg(0)

	// Parse timeout duration
	var timeout time.Duration
	if *timeoutFlag != "" {
		var err error
		timeout, err = time.ParseDuration(*timeoutFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: invalid --timeout value %q: %v\n", *timeoutFlag, err)
			return 4
		}
	}

	// Resolve fallback policy
	fallbackPolicy := os.Getenv("EVIDRA_FALLBACK")
	if *fallbackOffline {
		fallbackPolicy = "offline"
	}

	// Resolve environment with normalization
	environment := config.NormalizeEnvironment(coalesce(strings.TrimSpace(*envFlag), os.Getenv("EVIDRA_ENVIRONMENT")))

	// Resolve mode (instant, no I/O)
	resolved, err := mode.Resolve(mode.Config{
		URL:            coalesce(*urlFlag, os.Getenv("EVIDRA_URL")),
		APIKey:         coalesce(*apiKeyFlag, os.Getenv("EVIDRA_API_KEY")),
		FallbackPolicy: fallbackPolicy,
		ForceOffline:   *offlineFlag,
		Timeout:        timeout,
	})
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	modeLabel := "offline"
	if resolved.IsOnline {
		modeLabel = "online"
	}
	if *verbose {
		fmt.Fprintf(stderr, "mode: %s\n", modeLabel)
	}

	// Load scenario file locally (always — we need ToolInvocations for online path)
	sc, err := scenario.LoadFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	opts := localOpts{
		PolicyPath:  strings.TrimSpace(*policyFlag),
		DataPath:    strings.TrimSpace(*dataFlag),
		BundlePath:  strings.TrimSpace(*bundleFlag),
		Environment: environment,
		Verbose:     *verbose,
		Stderr:      stderr,
	}

	ctx := context.Background()
	var result validate.Result

	if resolved.IsOnline {
		result, err = evaluateOnline(ctx, resolved, sc, opts)
		if err != nil {
			if client.IsReachabilityError(err) && resolved.FallbackPolicy == "offline" {
				fmt.Fprintf(stderr, "API unreachable, falling back to local evaluation\n")
				result, err = evaluateLocal(ctx, sc, opts)
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				result.Source = "local-fallback"
			} else if client.IsReachabilityError(err) {
				// Fail closed (default)
				if *jsonOut {
					printJSONError(stdout, modeLabel, "API_UNREACHABLE",
						fmt.Sprintf("API unreachable at %s", resolved.Client.URL()),
						resolved.Client.URL())
				}
				fmt.Fprintf(stderr, "error: API unreachable at %s\n", resolved.Client.URL())
				fmt.Fprintf(stderr, "hint: set EVIDRA_FALLBACK=offline to allow local evaluation\n")
				return 3
			} else {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
		}
	} else {
		result, err = evaluateLocal(ctx, sc, opts)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
	}

	return printValidationResult(result, stdout, modeLabel, *jsonOut, *explain)
}

// evaluateOnline converts scenario actions to ToolInvocations and sends each to API.
func evaluateOnline(ctx context.Context, resolved mode.Resolved, sc scenario.Scenario, opts localOpts) (validate.Result, error) {
	aggregate := validate.Result{Pass: true, RiskLevel: "low", Source: "api"}

	for _, action := range sc.Actions {
		inv := actionToInvocation(sc, action, opts.Environment)
		result, reqID, err := resolved.Client.Validate(ctx, inv)
		if opts.Verbose {
			fmt.Fprintf(opts.Stderr, "  request_id=%s action=%s\n", reqID, action.Kind)
		}
		if err != nil {
			return validate.Result{}, err // caller handles fallback
		}
		if !result.Pass {
			aggregate.Pass = false
		}
		aggregate.RiskLevel = maxRiskLevel(aggregate.RiskLevel, result.RiskLevel)
		aggregate.Reasons = append(aggregate.Reasons, result.Reasons...)
		aggregate.RuleIDs = append(aggregate.RuleIDs, result.RuleIDs...)
		aggregate.Hints = append(aggregate.Hints, result.Hints...)
		aggregate.EvidenceIDs = append(aggregate.EvidenceIDs, result.EvidenceID)
		aggregate.RequestIDs = append(aggregate.RequestIDs, reqID)
		if aggregate.PolicyRef == "" {
			aggregate.PolicyRef = result.PolicyRef
		} else if result.PolicyRef != "" && result.PolicyRef != aggregate.PolicyRef {
			if opts.Verbose {
				fmt.Fprintf(opts.Stderr, "  warning: policy_ref changed mid-scenario: %s → %s\n",
					aggregate.PolicyRef, result.PolicyRef)
			}
			aggregate.PolicyRef = "mixed"
		}
	}

	// Single-action shortcut: promote to EvidenceID for backward compat
	if len(aggregate.EvidenceIDs) == 1 {
		aggregate.EvidenceID = aggregate.EvidenceIDs[0]
	}

	aggregate.RuleIDs = dedupeStrings(aggregate.RuleIDs)
	aggregate.Hints = dedupeStrings(aggregate.Hints)

	return aggregate, nil
}

// evaluateLocal uses existing pkg/validate path.
func evaluateLocal(_ context.Context, sc scenario.Scenario, opts localOpts) (validate.Result, error) {
	return validate.EvaluateScenario(context.Background(), sc, validate.Options{
		PolicyPath:  opts.PolicyPath,
		DataPath:    opts.DataPath,
		BundlePath:  opts.BundlePath,
		Environment: opts.Environment,
	})
}

// actionToInvocation converts a scenario.Action to invocation.ToolInvocation.
func actionToInvocation(sc scenario.Scenario, action scenario.Action, env string) invocation.ToolInvocation {
	tool, operation, _ := splitKind(action.Kind)
	return invocation.ToolInvocation{
		Actor: invocation.Actor{
			Type:   sc.Actor.Type,
			ID:     coalesce(sc.Actor.ID, sc.ScenarioID),
			Origin: sc.Source,
		},
		Tool:        tool,
		Operation:   operation,
		Environment: env,
		Params: map[string]interface{}{
			invocation.KeyTarget:   action.Target,
			invocation.KeyPayload:  action.Payload,
			invocation.KeyRiskTags: action.RiskTags,
		},
		Context: map[string]interface{}{
			invocation.KeyScenarioID: sc.ScenarioID,
			invocation.KeySource:     sc.Source,
		},
	}
}

func splitKind(kind string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(kind), ".", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	if strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// dedupeStrings removes duplicates preserving order.
func dedupeStrings(ss []string) []string {
	if len(ss) <= 1 {
		return ss
	}
	seen := make(map[string]struct{}, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, ok := seen[s]; !ok {
			seen[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

var riskLevelPriority = map[string]int{
	"low": 0, "medium": 1, "high": 2, "critical": 3,
}

func maxRiskLevel(a, b string) string {
	pa, oa := riskLevelPriority[a]
	pb, ob := riskLevelPriority[b]
	if !oa {
		pa = -1
	}
	if !ob {
		pb = -1
	}
	if pb > pa {
		return b
	}
	return a
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

type validationJSON struct {
	Status      string     `json:"status"`
	Mode        string     `json:"mode"`
	RiskLevel   string     `json:"risk_level"`
	Reason      string     `json:"reason"`
	Reasons     []string   `json:"reasons,omitempty"`
	RuleIDs     []string   `json:"rule_ids,omitempty"`
	Hints       []string   `json:"hints,omitempty"`
	EvidenceID  string     `json:"evidence_id,omitempty"`
	EvidenceIDs []string   `json:"evidence_ids,omitempty"`
	Source      string     `json:"source"`
	PolicyRef   string     `json:"policy_ref,omitempty"`
	RequestID   string     `json:"request_id,omitempty"`
	Timestamp   string     `json:"timestamp"`
	Error       *errorJSON `json:"error,omitempty"`
}

type errorJSON struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

func printValidationResult(result validate.Result, stdout io.Writer, modeLabel string, jsonOut bool, explain bool) int {
	status := "FAIL"
	if result.Pass {
		status = "PASS"
	}
	reason := "decision unavailable"
	if len(result.Reasons) > 0 {
		reason = result.Reasons[0]
	}
	if jsonOut {
		var requestID string
		if len(result.RequestIDs) == 1 {
			requestID = result.RequestIDs[0]
		}
		resp := validationJSON{
			Status:      status,
			Mode:        modeLabel,
			RiskLevel:   result.RiskLevel,
			Reason:      reason,
			Reasons:     result.Reasons,
			RuleIDs:     result.RuleIDs,
			Hints:       result.Hints,
			EvidenceID:  result.EvidenceID,
			EvidenceIDs: result.EvidenceIDs,
			Source:      result.Source,
			PolicyRef:   result.PolicyRef,
			RequestID:   requestID,
			Timestamp:   time.Now().UTC().Format(time.RFC3339),
		}
		b, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			fmt.Fprintf(stdout, "failed to render JSON: %v\n", err)
		} else {
			fmt.Fprintln(stdout, string(b))
		}
	} else {
		fmt.Fprintf(stdout, "Decision: %s\n", status)
		fmt.Fprintf(stdout, "Risk level: %s\n", result.RiskLevel)
		fmt.Fprintf(stdout, "Evidence: %s\n", result.EvidenceID)
		fmt.Fprintf(stdout, "Reason: %s\n", reason)
		if result.Pass {
			fmt.Fprintln(stdout, "No deny rules matched.")
		} else {
			printListWithCap("Rule IDs", result.RuleIDs, 10, stdout)
			printReasons(result.Reasons, reason, stdout)
			printHints(result.Hints, stdout)
		}
		if explain {
			printExplanation(result, stdout)
		}
	}
	if result.Pass {
		return 0
	}
	return 2
}

func printJSONError(stdout io.Writer, modeLabel, code, message, url string) {
	resp := validationJSON{
		Status: "ERROR",
		Mode:   modeLabel,
		Source: "none",
		Error: &errorJSON{
			Code:    code,
			Message: message,
			URL:     url,
		},
	}
	b, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Fprintln(stdout, string(b))
}

func printExplanation(result validate.Result, stdout io.Writer) {
	fmt.Fprintln(stdout, "Explanation:")
	if len(result.RuleIDs) > 0 {
		fmt.Fprintf(stdout, "- Rule IDs: %s\n", strings.Join(result.RuleIDs, ", "))
	}
	if len(result.Reasons) > 0 {
		fmt.Fprintf(stdout, "- Reasons: %s\n", strings.Join(result.Reasons, " | "))
	}
	if len(result.Hints) > 0 {
		fmt.Fprintln(stdout, "- Hints:")
		for _, hint := range result.Hints {
			fmt.Fprintf(stdout, "  - %s\n", hint)
		}
	}
}

func printListWithCap(title string, items []string, limit int, stdout io.Writer) {
	if len(items) == 0 {
		return
	}
	if limit <= 0 {
		limit = len(items)
	}
	fmt.Fprintf(stdout, "%s:\n", title)
	for i, entry := range items {
		if i >= limit {
			fmt.Fprintf(stdout, "- ... (%d more)\n", len(items)-limit)
			break
		}
		fmt.Fprintf(stdout, "- %s\n", entry)
	}
}

func printReasons(reasons []string, fallback string, stdout io.Writer) {
	fmt.Fprintln(stdout, "Reason:")
	if len(reasons) == 0 {
		fmt.Fprintf(stdout, "- %s\n", fallback)
		return
	}
	for _, reason := range firstN(reasons, 5) {
		fmt.Fprintf(stdout, "- %s\n", reason)
	}
}

func printHints(hints []string, stdout io.Writer) {
	fmt.Fprintln(stdout, "How to fix:")
	if len(hints) == 0 {
		fmt.Fprintln(stdout, "- Adjust the input (e.g., add approval tags) or update policy under policy/bundles/ops-v0.1.")
		return
	}
	for _, hint := range firstN(hints, 10) {
		fmt.Fprintf(stdout, "- %s\n", hint)
	}
}

func firstN(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}

func printUsage(w io.Writer) {
	defaultEvidence := config.DefaultEvidencePathDescription()

	fmt.Fprintln(w, "usage: evidra <validate|version>")
	fmt.Fprintln(w, "  evidra validate [flags] <file>")
	fmt.Fprintln(w, "  evidra version")
	fmt.Fprintln(w, "")
	fmt.Fprintf(w, "Default evidence store: %s\n", defaultEvidence)
	fmt.Fprintln(w, "Override via EVIDRA_EVIDENCE_DIR or legacy EVIDRA_EVIDENCE_PATH.")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Advanced commands are described in docs/advanced.md:")
	fmt.Fprintln(w, "  evidra evidence <verify|export|violations|cursor> ...")
	fmt.Fprintln(w, "  evidra policy sim --policy <path> --input <path> [--data <path>]")
}

func printValidateUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: evidra validate [flags] <file>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "CONNECTION FLAGS:")
	fmt.Fprintln(w, "  --url <url>             Evidra API URL (overrides EVIDRA_URL)")
	fmt.Fprintln(w, "  --api-key <key>         API key (overrides EVIDRA_API_KEY)")
	fmt.Fprintln(w, "  --timeout <duration>    HTTP timeout (default: 30s)")
	fmt.Fprintln(w, "  --offline               Force offline mode (skip API)")
	fmt.Fprintln(w, "  --fallback-offline      Allow local eval when API unreachable")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "POLICY FLAGS (offline/fallback only):")
	fmt.Fprintln(w, "  --bundle <path>         OPA bundle directory")
	fmt.Fprintln(w, "  --policy <path>         Policy rego file (loose mode)")
	fmt.Fprintln(w, "  --data <path>           Policy data JSON (loose mode)")
	fmt.Fprintln(w, "  --environment <env>     Environment label (normalized: prod→production, stg→staging)")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "OUTPUT FLAGS:")
	fmt.Fprintln(w, "  --json                  Structured JSON output")
	fmt.Fprintln(w, "  --explain               Human-readable explanation")
	fmt.Fprintln(w, "  --verbose               Log request IDs, mode, timing to stderr")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "EXIT CODES:")
	fmt.Fprintln(w, "  0  Policy allowed")
	fmt.Fprintln(w, "  1  Internal error (policy failure, auth error)")
	fmt.Fprintln(w, "  2  Policy denied")
	fmt.Fprintln(w, "  3  API unreachable (online mode, fallback=closed)")
	fmt.Fprintln(w, "  4  Usage error (bad flags, missing file, invalid input)")
}
