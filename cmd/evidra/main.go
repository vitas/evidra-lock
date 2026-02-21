package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"samebits.com/evidra-mcp/bundles/ops"
	opscli "samebits.com/evidra-mcp/bundles/ops/cli"
	"samebits.com/evidra-mcp/bundles/regulated"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		printUsage(stderr)
		return 2
	}

	bundle := args[0]

	switch bundle {
	case "ops":
		if len(args) < 2 {
			printUsage(stderr)
			return 2
		}
		verb := args[1]
		switch verb {
		case "validate":
			if len(args) != 3 {
				fmt.Fprintln(stderr, "usage: evidra ops validate <file>")
				return 2
			}
			file := args[2]
			var opsOut ops.ValidationOutput
			opsOut, err := ops.ValidateFile(file)
			if err != nil {
				fmt.Fprintln(stderr, err.Error())
				return 1
			}
			decision := "FAIL"
			if opsOut.Pass {
				decision = "PASS"
			}
			fmt.Fprintf(stdout, "Decision: %s\n", decision)
			fmt.Fprintf(stdout, "Risk level: %s\n", opsOut.RiskLevel)
			fmt.Fprintf(stdout, "Evidence: %s\n", opsOut.EvidenceID)
			for _, reason := range opsOut.Reasons {
				fmt.Fprintf(stdout, "Reason: %s\n", reason)
			}
			if opsOut.Pass {
				return 0
			}
			return 1
		case "explain":
			return opscli.Explain(args[2:], stdout, stderr)
		case "--help", "-h":
			fmt.Fprintln(stderr, "usage: evidra ops <validate|explain> ...")
			fmt.Fprintln(stderr, "  validate <file>")
			fmt.Fprintln(stderr, "  explain <schema|kinds|example|policies> [--verbose]")
			return 2
		default:
			fmt.Fprintln(stderr, "usage: evidra ops <validate|explain> ...")
			return 2
		}
	case "regulated":
		if len(args) != 3 || args[1] != "validate" {
			fmt.Fprintln(stderr, "usage: evidra regulated validate <file>")
			return 2
		}
		out, err := regulated.ValidateFile(args[2])
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		b, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintln(stdout, string(b))
		return 0
	default:
		printUsage(stderr)
		return 2
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: evidra <ops|regulated> <command>")
	fmt.Fprintln(w, "  evidra ops validate <file>")
	fmt.Fprintln(w, "  evidra ops explain <schema|kinds|example|policies> [--verbose]")
	fmt.Fprintln(w, "  evidra regulated validate <file>")
}
