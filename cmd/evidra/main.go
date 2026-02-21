package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"samebits.com/evidra-mcp/bundles/ops"
	opscli "samebits.com/evidra-mcp/bundles/ops/cli"
	"samebits.com/evidra-mcp/bundles/regulated"
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
	if len(args) == 1 && args[0] == "version" {
		fmt.Fprintf(stdout, "Version: %s\nCommit: %s\nDate: %s\n", version.Version, version.Commit, version.Date)
		return 0
	}

	bundle := args[0]

	switch bundle {
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
		if len(args) < 2 {
			printUsage(stderr)
			return 2
		}
		verb := args[1]
		switch verb {
		case "init":
			return opscli.Init(args[2:], stdout, stderr)
		case "validate":
			fs := flag.NewFlagSet("ops validate", flag.ContinueOnError)
			fs.SetOutput(io.Discard)
			verbose := fs.Bool("verbose", false, "print validator report summaries")
			configPath := fs.String("config", "", "path to ops config file (default .evidra/ops.yaml)")
			enableValidators := fs.Bool("enable-validators", false, "enable external validators (override config)")
			validatorsArg := fs.String("validators", "", "validator selector: builtins=a,b exec=all|none|x,y")
			listValidators := fs.Bool("list-validators", false, "list available built-in and configured exec validators")
			if err := fs.Parse(args[2:]); err != nil {
				fmt.Fprintln(stderr, "usage: evidra ops validate [--verbose] [--config path] [--enable-validators] [--validators ...] [--list-validators] <file>")
				return 2
			}
			if *listValidators {
				list, err := ops.ListAvailableValidators(*configPath)
				if err != nil {
					fmt.Fprintln(stderr, err.Error())
					return 1
				}
				fmt.Fprintln(stdout, "Built-ins:")
				for _, b := range list.Builtins {
					fmt.Fprintf(stdout, "- %s\n", b)
				}
				fmt.Fprintln(stdout, "Exec plugins:")
				if len(list.ExecPlugins) == 0 {
					fmt.Fprintln(stdout, "- (none configured)")
				} else {
					for _, p := range list.ExecPlugins {
						fmt.Fprintf(stdout, "- %s\n", p)
					}
				}
				return 0
			}
			if fs.NArg() != 1 {
				fmt.Fprintln(stderr, "usage: evidra ops validate [--verbose] [--config path] [--enable-validators] [--validators ...] [--list-validators] <file>")
				return 2
			}
			file := fs.Arg(0)
			builtinFilter, execMode, execFilter := parseValidatorsFlag(*validatorsArg)
			enableOverride := (*bool)(nil)
			if *enableValidators {
				v := true
				enableOverride = &v
			}
			var opsOut ops.ValidationOutput
			opsOut, err := ops.ValidateFileWithOptions(file, ops.ValidateOptions{
				ConfigPath:       *configPath,
				EnableValidators: enableOverride,
				BuiltinFilter:    builtinFilter,
				ExecMode:         execMode,
				ExecFilter:       execFilter,
			})
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
			if *verbose {
				for _, rep := range opsOut.Reports {
					fmt.Fprintf(stdout, "Report: %s exit=%d findings=%d duration_ms=%d\n", rep.Tool, rep.ExitCode, len(rep.Findings), rep.DurationMS)
				}
			}
			if opsOut.Pass {
				return 0
			}
			return 2
		case "explain":
			return opscli.Explain(args[2:], stdout, stderr)
		case "--help", "-h":
			fmt.Fprintln(stderr, "usage: evidra ops <init|validate|explain> ...")
			fmt.Fprintln(stderr, "  init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]")
			fmt.Fprintln(stderr, "  validate [--verbose] [--config path] [--enable-validators] [--validators ...] [--list-validators] <file>")
			fmt.Fprintln(stderr, "  explain <schema|kinds|example|policies> [--verbose]")
			return 2
		default:
			fmt.Fprintln(stderr, "usage: evidra ops <init|validate|explain> ...")
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
	fmt.Fprintln(w, "usage: evidra <mcp|evidence|policy|ops|regulated> <command>")
	fmt.Fprintln(w, "  evidra version")
	fmt.Fprintln(w, "  evidra mcp [--guarded] [--policy path] [--data path]")
	fmt.Fprintln(w, "  evidra evidence <verify|export|violations|cursor> ...")
	fmt.Fprintln(w, "  evidra policy sim --policy <path> --input <path> [--data <path>]")
	fmt.Fprintln(w, "  evidra ops init [--path dir] [--force] [--enable-validators] [--with-plugins] [--minimal] [--print]")
	fmt.Fprintln(w, "  evidra ops validate [--verbose] [--config path] [--enable-validators] [--validators ...] [--list-validators] <file>")
	fmt.Fprintln(w, "  evidra ops explain <schema|kinds|example|policies> [--verbose]")
	fmt.Fprintln(w, "  evidra regulated validate <file>")
}

func parseValidatorsFlag(raw string) (map[string]bool, string, map[string]bool) {
	builtins := map[string]bool{}
	execMode := ""
	execFilter := map[string]bool{}
	parts := strings.Fields(raw)
	for _, p := range parts {
		if strings.HasPrefix(p, "builtins=") {
			list := strings.TrimPrefix(p, "builtins=")
			for _, name := range strings.Split(list, ",") {
				n := strings.ToLower(strings.TrimSpace(name))
				if n != "" {
					builtins[n] = true
				}
			}
		}
		if strings.HasPrefix(p, "exec=") {
			list := strings.TrimPrefix(p, "exec=")
			mode := strings.ToLower(strings.TrimSpace(list))
			if mode == "all" || mode == "none" {
				execMode = mode
				continue
			}
			execMode = "names"
			for _, name := range strings.Split(list, ",") {
				n := strings.ToLower(strings.TrimSpace(name))
				if n != "" {
					execFilter[n] = true
				}
			}
		}
	}
	if len(builtins) == 0 {
		builtins = nil
	}
	if len(execFilter) == 0 {
		execFilter = nil
	}
	return builtins, execMode, execFilter
}
