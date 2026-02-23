package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"samebits.com/evidra-mcp/pkg/config"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/version"
)

type serverRunner interface {
	Run(context.Context, mcp.Transport) error
}

var newServerFunc = func(opts mcpserver.Options) serverRunner {
	return mcpserver.NewServer(opts)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evidra-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printHelp(stderr) }
	showVersion := fs.Bool("version", false, "Print version information and exit")
	policyFlag := fs.String("policy", "", "Path to policy rego file (required)")
	dataFlag := fs.String("data", "", "Path to policy data JSON file (required)")
	evidenceFlag := fs.String("evidence-dir", "", "Path to store evidence records")
	observeFlag := fs.Bool("observe", false, "Enable observe mode (policy advises but execution proceeds)")
	helpFlag := fs.Bool("help", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "evidra-mcp %s\n", version.Version)
		return 0
	}
	if *helpFlag {
		printHelp(stdout)
		return 2
	}

	policyPath, dataPath, err := config.ResolvePolicyData(strings.TrimSpace(*policyFlag), strings.TrimSpace(*dataFlag))
	if err != nil {
		fmt.Fprintf(stderr, "resolve policy paths: %v\n", err)
		return 1
	}

	ps := policysource.NewLocalFileSource(policyPath, dataPath)

	mode, err := resolveMode(*observeFlag)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	server := newServerFunc(mcpserver.Options{
		Name:                     "evidra-mcp",
		Version:                  version.Version,
		Mode:                     mode,
		PolicyRef:                policyRefOrEmpty(ps),
		PolicyPath:               policyPath,
		DataPath:                 dataPath,
		EvidencePath:             config.ResolveEvidenceDir(strings.TrimSpace(*evidenceFlag)),
		IncludeFileResourceLinks: envBool("EVIDRA_INCLUDE_FILE_RESOURCE_LINKS", false),
	})

	logger := log.New(stderr, "", log.LstdFlags)
	logger.Printf("evidra-mcp running in %s mode", mode)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(stderr, "run mcp server: %v\n", err)
		return 1
	}
	return 0
}

func resolveMode(observeFlag bool) (mcpserver.Mode, error) {
	if observeFlag {
		return mcpserver.ModeObserve, nil
	}
	if raw := strings.TrimSpace(os.Getenv("EVIDRA_MODE")); raw != "" {
		switch strings.ToLower(raw) {
		case string(mcpserver.ModeEnforce):
			return mcpserver.ModeEnforce, nil
		case string(mcpserver.ModeObserve):
			return mcpserver.ModeObserve, nil
		default:
			return "", fmt.Errorf("invalid EVIDRA_MODE %q (allowed: enforce, observe)", raw)
		}
	}
	return mcpserver.ModeEnforce, nil
}

func policyRefOrEmpty(ps *policysource.LocalFileSource) string {
	ref, err := ps.PolicyRef()
	if err != nil {
		return ""
	}
	return ref
}

func envBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "evidra-mcp — Local MCP server that enforces deterministic policy on AI-generated infra changes.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  evidra-mcp --policy <path> --data <path> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "REQUIRED:")
	fmt.Fprintln(w, "  --policy <path>         Path to rego entrypoint (e.g. policy/profiles/ops-v0.1/policy.rego)")
	fmt.Fprintln(w, "  --data <path>           Path to policy data.json (e.g. policy/profiles/ops-v0.1/data.json)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FLAGS:")
	fmt.Fprintln(w, "  --evidence-dir <dir>    Where to store evidence chain (default: ~/.evidra/evidence; override via EVIDRA_EVIDENCE_DIR/EVIDRA_EVIDENCE_PATH)")
	fmt.Fprintln(w, "  --observe               Observe-only: do not block, only report (default: enforce)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  evidra-mcp --policy policy/profiles/ops-v0.1/policy.rego \\")
	fmt.Fprintln(w, "             --data   policy/profiles/ops-v0.1/data.json \\")
	fmt.Fprintln(w, "             --evidence-dir ~/.evidra/evidence")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NOTES:")
	fmt.Fprintln(w, "  - Use `evidra` for offline tools (policy sim, evidence inspect/report).")
}
