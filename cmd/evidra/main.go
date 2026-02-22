package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"samebits.com/evidra-mcp/bundles/ops"
	"samebits.com/evidra-mcp/internal/version"
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
	default:
		printUsage(stderr)
		return 2
	}
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil || fs.NArg() != 1 {
		fmt.Fprintln(stderr, "usage: evidra validate <file>")
		return 2
	}

	path := fs.Arg(0)
	result, err := ops.ValidateFile(path)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}

	decision := "FAIL"
	if result.Pass {
		decision = "PASS"
	}
	fmt.Fprintf(stdout, "Decision: %s\n", decision)
	fmt.Fprintf(stdout, "Risk level: %s\n", result.RiskLevel)
	fmt.Fprintf(stdout, "Evidence: %s\n", result.EvidenceID)
	for _, reason := range result.Reasons {
		fmt.Fprintf(stdout, "Reason: %s\n", reason)
	}
	if result.Pass {
		return 0
	}
	return 2
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: evidra <validate|version>")
	fmt.Fprintln(w, "  evidra validate <file>")
	fmt.Fprintln(w, "  evidra version")
}
