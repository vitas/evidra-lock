package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"samebits.com/evidra-mcp/bundles/ops"
	"samebits.com/evidra-mcp/bundles/regulated"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) != 3 {
		fmt.Fprintln(stderr, "usage: evidra <ops|regulated> validate <file>")
		return 2
	}

	bundle := args[0]
	verb := args[1]
	file := args[2]
	if verb != "validate" {
		fmt.Fprintln(stderr, "usage: evidra <ops|regulated> validate <file>")
		return 2
	}

	var (
		out interface{}
		err error
	)

	switch bundle {
	case "ops":
		out, err = ops.ValidateFile(file)
	case "regulated":
		out, err = regulated.ValidateFile(file)
	default:
		fmt.Fprintln(stderr, "usage: evidra <ops|regulated> validate <file>")
		return 2
	}

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
}
