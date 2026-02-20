package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/version"
)

const (
	exitInputInvalid = 2
	exitPolicyError  = 3
)

func main() {
	showVersion := flag.Bool("version", false, "Print version and exit")
	policyPath := flag.String("policy", "", "Path to Rego policy file")
	inputPath := flag.String("input", "", "Path to ToolInvocation JSON file")
	dataPath := flag.String("data", "", "Optional path to OPA data JSON file")
	flag.Parse()

	if *showVersion {
		fmt.Printf("evidra-policy-sim %s\n", version.Version)
		return
	}

	if *policyPath == "" || *inputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evidra-policy-sim --policy ./policy/kits/ops-v0.1/policy.rego --input ./examples/invocation.json [--data ./policy/kits/ops-v0.1/data.json]")
		os.Exit(exitInputInvalid)
	}

	inv, err := readInvocation(*inputPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitInputInvalid)
	}
	if err := inv.ValidateStructure(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitInputInvalid)
	}

	source := policysource.NewLocalFileSource(*policyPath, *dataPath)
	policyBytes, err := source.LoadPolicy()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitPolicyError)
	}
	dataBytes, err := source.LoadData()
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitPolicyError)
	}

	engine, err := policy.NewOPAEngine(policyBytes, dataBytes)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitPolicyError)
	}

	decision, err := engine.Evaluate(inv)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitPolicyError)
	}

	out, err := json.MarshalIndent(decision, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(exitPolicyError)
	}
	fmt.Println(string(out))
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
