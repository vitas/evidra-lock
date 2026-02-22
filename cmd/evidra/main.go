package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"samebits.com/evidra-mcp/bundles/ops"
	opscli "samebits.com/evidra-mcp/bundles/ops/cli"
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
	case "mcp":
		return runMCPCommand(args[1:], stdout, stderr)
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
	case "ops":
		return runOpsCommand(args[1:], stdout, stderr)
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

	return printValidationResult(result, stdout)
}

func runOpsCommand(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printOpsUsage(stderr)
		return 2
	}
	switch args[0] {
	case "validate":
		// Legacy alias pointing to the same implementation as `evidra validate`.
		return runValidate(args[1:], stdout, stderr)
	case "init":
		return opscli.Init(args[1:], stdout, stderr)
	case "explain":
		return opscli.Explain(args[1:], stdout, stderr)
	default:
		printOpsUsage(stderr)
		return 2
	}
}

func printValidationResult(result ops.ValidationOutput, stdout io.Writer) int {
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
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Advanced commands are described in docs/advanced.md:")
	fmt.Fprintln(w, "  evidra mcp [--guarded] [--policy path] [--data path]")
	fmt.Fprintln(w, "  evidra evidence <verify|export|violations|cursor> ...")
	fmt.Fprintln(w, "  evidra policy sim --policy <path> --input <path> [--data <path>]")
	fmt.Fprintln(w, "  evidra ops <init|validate|explain> ...")
}

func printOpsUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: evidra ops <init|validate|explain> ... (legacy/advanced)")
	fmt.Fprintln(w, "  init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]")
	fmt.Fprintln(w, "  validate <file>")
	fmt.Fprintln(w, "  explain <schema|kinds|example|policies> [--verbose]")
}
