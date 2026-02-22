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
	"samebits.com/evidra-mcp/pkg/evidence"
	"samebits.com/evidra-mcp/pkg/mcpserver"
	"samebits.com/evidra-mcp/pkg/outputlimit"
	"samebits.com/evidra-mcp/pkg/packs"
	"samebits.com/evidra-mcp/pkg/policy"
	"samebits.com/evidra-mcp/pkg/policysource"
	"samebits.com/evidra-mcp/pkg/registry"
)

const (
	defaultPolicyPath = "./policy/profiles/ops-v0.1/policy.rego"
	defaultDataPath   = "./policy/profiles/ops-v0.1/data.json"
	defaultPacksDir   = "packs/_core/ops"
)

func runMCPCommand(args []string, stdout io.Writer, stderr io.Writer) int {
	fs := flag.NewFlagSet("evidra mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	showVersion := fs.Bool("version", false, "Print version and exit")
	policyFlag := fs.String("policy", "", "Path to policy rego file")
	dataFlag := fs.String("data", "", "Path to policy data JSON file")
	guardedFlag := fs.Bool("guarded", false, "Enable guarded mode strict enforcement")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *showVersion {
		fmt.Fprintf(stdout, "evidra-mcp %s\n", version.Version)
		return 0
	}

	logger := log.New(stderr, "", log.LstdFlags)

	mode, err := loadModeFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "load mode: %v\n", err)
		return 1
	}
	logger.Printf("Evidra mode: %s", mode)
	if *guardedFlag {
		logger.Printf("Running in GUARDED MODE (strict enforcement)")
	}
	if mode == mcpserver.ModeObserve {
		logger.Printf("Evidra running in OBSERVE mode. Policy violations will NOT block execution.")
	}

	policyPath, dataPath, err := resolvePolicyPaths(strings.TrimSpace(*policyFlag), strings.TrimSpace(*dataFlag))
	if err != nil {
		fmt.Fprintf(stderr, "resolve policy paths: %v\n", err)
		return 1
	}

	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	policyModules, err := ps.LoadPolicy()
	if err != nil {
		fmt.Fprintf(stderr, "load policy source: %v\n", err)
		return 1
	}

	dataBytes, err := ps.LoadData()
	if err != nil {
		fmt.Fprintf(stderr, "load policy data: %v\n", err)
		return 1
	}

	policyEngine, err := policy.NewOPAEngine(policyModules, dataBytes)
	if err != nil {
		fmt.Fprintf(stderr, "load policy: %v\n", err)
		return 1
	}

	evidencePath := envOrDefault("EVIDRA_EVIDENCE_PATH", "./data/evidence")
	evidenceStore := evidence.NewStoreWithPath(evidencePath)
	if err := evidenceStore.Init(); err != nil {
		fmt.Fprintf(stderr, "init evidence store: %v\n", err)
		return 1
	}

	toolRegistry := registry.NewInMemoryRegistry(nil)
	packsDir := strings.TrimSpace(os.Getenv("EVIDRA_PACKS_DIR"))
	if packsDir == "" && dirExists(defaultPacksDir) {
		packsDir = defaultPacksDir
	}
	if packsDir != "" {
		defs, err := packs.LoadToolDefinitions(packsDir, toolRegistry.ToolNames())
		if err != nil {
			fmt.Fprintf(stderr, "load tool packs: %v\n", err)
			return 1
		}
		for _, def := range defs {
			if err := toolRegistry.RegisterTool(def); err != nil {
				fmt.Fprintf(stderr, "register pack tool %q: %v\n", def.Name, err)
				return 1
			}
		}
	}
	server := mcpserver.NewServer(
		mcpserver.Options{
			Name:                     "evidra-mcp",
			Version:                  version.Version,
			Mode:                     mode,
			Guarded:                  *guardedFlag,
			PolicyRef:                policyRefOrEmpty(ps),
			EvidencePath:             evidencePath,
			IncludeFileResourceLinks: envBool("EVIDRA_INCLUDE_FILE_RESOURCE_LINKS", false),
			MaxOutputBytes:           outputlimit.MaxBytesFromEnv("EVIDRA_MAX_OUTPUT_BYTES", outputlimit.DefaultMaxBytes),
		},
		toolRegistry,
		policyEngine,
		evidenceStore,
	)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(stderr, "run mcp server: %v\n", err)
		return 1
	}
	return 0
}

func policyRefOrEmpty(ps *policysource.LocalFileSource) string {
	ref, err := ps.PolicyRef()
	if err != nil {
		return ""
	}
	return ref
}

func loadModeFromEnv() (mcpserver.Mode, error) {
	raw := strings.TrimSpace(os.Getenv("EVIDRA_MODE"))
	if raw == "" {
		return mcpserver.ModeEnforce, nil
	}
	switch strings.ToLower(raw) {
	case string(mcpserver.ModeEnforce):
		return mcpserver.ModeEnforce, nil
	case string(mcpserver.ModeObserve):
		return mcpserver.ModeObserve, nil
	default:
		return "", fmt.Errorf("invalid EVIDRA_MODE %q (allowed: enforce, observe)", raw)
	}
}

func resolvePolicyPaths(policyFlag, dataFlag string) (string, string, error) {
	if policyFlag != "" {
		if dataFlag == "" {
			return "", "", fmt.Errorf("--data is required when --policy is provided")
		}
		return policyFlag, dataFlag, nil
	}

	policyEnv := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH"))
	dataEnv := strings.TrimSpace(os.Getenv("EVIDRA_DATA_PATH"))
	if policyEnv != "" || dataEnv != "" {
		if policyEnv == "" || dataEnv == "" {
			return "", "", fmt.Errorf("both EVIDRA_POLICY_PATH and EVIDRA_DATA_PATH must be set together")
		}
		return policyEnv, dataEnv, nil
	}

	if fileExists(defaultPolicyPath) && fileExists(defaultDataPath) {
		return defaultPolicyPath, defaultDataPath, nil
	}
	return "", "", fmt.Errorf("policy/data paths not found; run inside repo root or pass --policy/--data")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
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
