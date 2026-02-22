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

	"samebits.com/evidra-mcp/internal/version"
	"samebits.com/evidra-mcp/pkg/core"
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/outputlimit"
	"samebits.com/evidra-mcp/pkg/packs"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/registry"
)

const (
	defaultEvidenceDir = "./data/evidence"
)

type serverRunner interface {
	Run(context.Context, mcp.Transport) error
}

var newServerFunc = func(opts mcpserver.Options, reg registry.Registry, policyEngine core.PolicyEngine, evidenceStore core.EvidenceStore) serverRunner {
	return mcpserver.NewServer(opts, reg, policyEngine, evidenceStore)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("evidra-mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "Print version information and exit")
	policyFlag := fs.String("policy", "", "Path to policy rego file (required)")
	dataFlag := fs.String("data", "", "Path to policy data JSON file (required)")
	evidenceFlag := fs.String("evidence-dir", "", "Path to store evidence records")
	packsFlag := fs.String("packs-dir", "", "Optional packs directory to load tool definitions")
	observeFlag := fs.Bool("observe", false, "Enable observe mode (policy advises but execution proceeds)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "evidra-mcp %s\n", version.Version)
		return 0
	}

	policyPath, dataPath, err := resolvePolicyPaths(strings.TrimSpace(*policyFlag), strings.TrimSpace(*dataFlag))
	if err != nil {
		fmt.Fprintf(stderr, "resolve policy paths: %v\n", err)
		return 1
	}

	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	policyModules, err := ps.LoadPolicy()
	if err != nil {
		fmt.Fprintf(stderr, "load policy: %v\n", err)
		return 1
	}
	dataBytes, err := ps.LoadData()
	if err != nil {
		fmt.Fprintf(stderr, "load policy data: %v\n", err)
		return 1
	}
	policyEngine, err := policy.NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		fmt.Fprintf(stderr, "compile policy: %v\n", err)
		return 1
	}

	evidencePath := resolveEvidencePath(strings.TrimSpace(*evidenceFlag))
	evidenceStore := evidence.NewStoreWithPath(evidencePath)
	if err := evidenceStore.Init(); err != nil {
		fmt.Fprintf(stderr, "init evidence store: %v\n", err)
		return 1
	}

	toolRegistry := registry.NewInMemoryRegistry(nil)
	packsDir := resolvePacksDir(strings.TrimSpace(*packsFlag))
	if packsDir != "" {
		defs, err := packs.LoadToolDefinitions(packsDir, toolRegistry.ToolNames())
		if err != nil {
			fmt.Fprintf(stderr, "load tool packs: %v\n", err)
			return 1
		}
		for _, def := range defs {
			if err := toolRegistry.RegisterTool(def); err != nil {
				fmt.Fprintf(stderr, "register tool %q: %v\n", def.Name, err)
				return 1
			}
		}
	}

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
		EvidencePath:             evidencePath,
		IncludeFileResourceLinks: envBool("EVIDRA_INCLUDE_FILE_RESOURCE_LINKS", false),
		MaxOutputBytes:           outputlimit.MaxBytesFromEnv("EVIDRA_MAX_OUTPUT_BYTES", outputlimit.DefaultMaxBytes),
	}, toolRegistry, policyEngine, evidenceStore)

	logger := log.New(stderr, "", log.LstdFlags)
	logger.Printf("evidra-mcp running in %s mode", mode)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(stderr, "run mcp server: %v\n", err)
		return 1
	}
	return 0
}

func resolvePolicyPaths(policyFlag, dataFlag string) (string, string, error) {
	if policyFlag != "" && dataFlag != "" {
		return policyFlag, dataFlag, nil
	}
	policyEnv := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH"))
	dataEnv := strings.TrimSpace(os.Getenv("EVIDRA_DATA_PATH"))
	if policyEnv != "" && dataEnv != "" {
		return policyEnv, dataEnv, nil
	}
	return "", "", fmt.Errorf("--policy/--data flags or EVIDRA_POLICY_PATH/EVIDRA_DATA_PATH must both be supplied")
}

func resolveEvidencePath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := strings.TrimSpace(os.Getenv("EVIDRA_EVIDENCE_PATH")); env != "" {
		return env
	}
	return defaultEvidenceDir
}

func resolvePacksDir(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return strings.TrimSpace(os.Getenv("EVIDRA_PACKS_DIR"))
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
