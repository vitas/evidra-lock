package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/policy"
	"samebits.com/evidra/pkg/policysource"
	"samebits.com/evidra/pkg/version"
)

const (
	policySimExitInputInvalid = 2
	policySimExitPolicyError  = 3
)

func runPolicySimCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("evidra policy sim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "Print version and exit")
	policyPath := fs.String("policy", "", "Path to Rego policy file")
	inputPath := fs.String("input", "", "Path to ToolInvocation JSON file")
	dataPath := fs.String("data", "", "Optional path to OPA data JSON file")
	if err := fs.Parse(args); err != nil {
		return policySimExitInputInvalid
	}

	if *showVersion {
		fmt.Fprintf(stdout, "evidra-policy-sim %s\n", version.Version)
		return 0
	}

	if *policyPath == "" || *inputPath == "" {
		fmt.Fprintln(stderr, "usage: evidra policy sim --policy ./policy/profiles/ops-v0.1/policy.rego --input ./examples/invocation.json [--data ./policy/profiles/ops-v0.1/data.json]")
		return policySimExitInputInvalid
	}

	inv, err := readInvocation(*inputPath)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitInputInvalid
	}
	if err := inv.ValidateStructure(); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitInputInvalid
	}

	source := policysource.NewLocalFileSource(*policyPath, *dataPath)
	policyModules, err := source.LoadPolicy()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitPolicyError
	}
	dataBytes, err := source.LoadData()
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitPolicyError
	}

	engine, err := policy.NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitPolicyError
	}

	decision, err := engine.Evaluate(inv)
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitPolicyError
	}

	out, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		fmt.Fprintln(stderr, err.Error())
		return policySimExitPolicyError
	}
	fmt.Fprintln(stdout, string(out))
	return 0
}

func readInvocation(path string) (invocation.ToolInvocation, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return invocation.ToolInvocation{}, fmt.Errorf("read input file: %w", err)
	}

	var inv invocation.ToolInvocation
	if err := json.Unmarshal(raw, &inv); err != nil {
		return invocation.ToolInvocation{}, fmt.Errorf("unmarshal input JSON: %w", err)
	}
	return inv, nil
}
