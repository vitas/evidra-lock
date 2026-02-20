package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"samebits.com/evidra-mcp/pkg/invocation"
	"samebits.com/evidra-mcp/pkg/policy"
)

const (
	exitInputInvalid = 2
	exitPolicyError  = 3
)

func main() {
	policyPath := flag.String("policy", "", "Path to Rego policy file")
	inputPath := flag.String("input", "", "Path to ToolInvocation JSON file")
	dataPath := flag.String("data", "", "Optional path to OPA data JSON file")
	flag.Parse()

	if *policyPath == "" || *inputPath == "" {
		fmt.Fprintln(os.Stderr, "usage: evidra-policy-sim --policy ./policy/policy.rego --input ./examples/invocation.json [--data ./policy/data.json]")
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

	dataPaths := []string{}
	if *dataPath != "" {
		dataPaths = append(dataPaths, *dataPath)
	}

	engine, err := policy.LoadFromFiles(*policyPath, dataPaths)
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
