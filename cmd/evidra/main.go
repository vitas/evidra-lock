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

	"samebits.com/evidra-mcp/pkg/validate"
	"samebits.com/evidra-mcp/pkg/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(stderr)
		return 2
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
			return 2
		}
		switch args[1] {
		case "sim":
			return runPolicySimCommand(args[2:], stdout, stderr)
		default:
			fmt.Fprintln(stderr, "usage: evidra policy sim --policy <path> --input <path> [--data <path>]")
			return 2
		}
	default:
		printUsage(stderr)
		return 2
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	jsonOut := fs.Bool("json", false, "output structured JSON")
	explain := fs.Bool("explain", false, "print a human-readable explanation for the decision")
	policyFlag := fs.String("policy", "", "Path to policy rego file")
	dataFlag := fs.String("data", "", "Path to policy data JSON file")
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: evidra validate <file>")
		return 2
	}

	path := fs.Arg(0)
	result, err := validate.EvaluateFile(context.Background(), path, validate.Options{
		PolicyPath: strings.TrimSpace(*policyFlag),
		DataPath:   strings.TrimSpace(*dataFlag),
	})
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	return printValidationResult(result, stdout, *jsonOut, *explain)
}

type validationJSON struct {
	Status     string   `json:"status"`
	RiskLevel  string   `json:"risk_level"`
	Reason     string   `json:"reason"`
	Reasons    []string `json:"reasons,omitempty"`
	RuleIDs    []string `json:"rule_ids,omitempty"`
	Hints      []string `json:"hints,omitempty"`
	EvidenceID string   `json:"evidence_id"`
	Timestamp  string   `json:"timestamp"`
}

func printValidationResult(result validate.Result, stdout io.Writer, jsonOut bool, explain bool) int {
	status := "FAIL"
	if result.Pass {
		status = "PASS"
	}
	reason := "decision unavailable"
	if len(result.Reasons) > 0 {
		reason = result.Reasons[0]
	}
	if jsonOut {
		resp := validationJSON{
			Status:     status,
			RiskLevel:  result.RiskLevel,
			Reason:     reason,
			Reasons:    result.Reasons,
			RuleIDs:    result.RuleIDs,
			Hints:      result.Hints,
			EvidenceID: result.EvidenceID,
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
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
	printActionFacts(result.ActionFacts, stdout)
}

func printActionFacts(facts []validate.ActionFact, stdout io.Writer) {
	if len(facts) == 0 {
		return
	}
	fmt.Fprintln(stdout, "- Action facts:")
	for _, fact := range facts {
		switch {
		case strings.HasPrefix(fact.Kind, "terraform"):
			fmt.Fprintf(stdout, "  - Terraform plan: destroy_count=%d, public_exposure=%t", fact.DestroyCount, fact.PublicExposure)
			if len(fact.ResourceAddresses) > 0 {
				fmt.Fprintf(stdout, ", resources=%s", shortList(fact.ResourceAddresses, 3))
			}
			fmt.Fprintln(stdout)
		case strings.Contains(fact.Kind, "kubectl") || strings.Contains(fact.Kind, "kustomize") || strings.Contains(fact.Kind, "k8s"):
			ns := fact.Namespace
			if ns == "" {
				ns = "default"
			}
			fmt.Fprintf(stdout, "  - Kubernetes manifest: namespace=%s", ns)
			if len(fact.ManifestKinds) > 0 {
				fmt.Fprintf(stdout, ", kinds=%s", strings.Join(fact.ManifestKinds, ", "))
			}
			fmt.Fprintln(stdout)
		default:
			fmt.Fprintf(stdout, "  - %s action namespace=%s\n", fact.Kind, fact.Namespace)
		}
	}
}

func shortList(items []string, limit int) string {
	if len(items) <= limit {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:limit], ", ") + ", ..."
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
		fmt.Fprintln(stdout, "- Adjust the input (e.g., add approval tags) or update policy under policy/profiles/ops-v0.1.")
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
	fmt.Fprintln(w, "usage: evidra <validate|version>")
	fmt.Fprintln(w, "  evidra validate [--policy <path> --data <path>] <file>")
	fmt.Fprintln(w, "  evidra version")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Advanced commands are described in docs/advanced.md:")
	fmt.Fprintln(w, "  evidra evidence <verify|export|violations|cursor> ...")
	fmt.Fprintln(w, "  evidra policy sim --policy <path> --input <path> [--data <path>]")
}
