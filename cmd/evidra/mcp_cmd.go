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

type Profile string

const (
	ProfileOps Profile = "ops"
	ProfileDev Profile = "dev"

	defaultOpsPolicyPath = "./policy/profiles/ops-v0.1/policy.rego"
	defaultOpsDataPath   = "./policy/profiles/ops-v0.1/data.json"
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
	profile, err := loadProfileFromEnv()
	if err != nil {
		fmt.Fprintf(stderr, "load profile: %v\n", err)
		return 1
	}
	logger.Printf("Evidra profile: %s", profile)
	logger.Printf("Evidra mode: %s", mode)
	if *guardedFlag {
		logger.Printf("Running in GUARDED MODE (strict enforcement)")
	}
	if mode == mcpserver.ModeObserve {
		logger.Printf("Evidra running in OBSERVE mode. Policy violations will NOT block execution.")
	}

	policyPath, dataPath := resolvePolicyPaths(profile, strings.TrimSpace(*policyFlag), strings.TrimSpace(*dataFlag))

	ps := policysource.NewLocalFileSource(policyPath, dataPath)
	policyBytes, err := ps.LoadPolicy()
	if err != nil {
		fmt.Fprintf(stderr, "load policy source: %v\n", err)
		return 1
	}

	dataBytes, err := ps.LoadData()
	if err != nil {
		fmt.Fprintf(stderr, "load policy data: %v\n", err)
		return 1
	}

	policyEngine, err := policy.NewOPAEngine(policyBytes, dataBytes)
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

	toolRegistry, err := buildRegistryForProfile(profile)
	if err != nil {
		fmt.Fprintf(stderr, "build registry: %v\n", err)
		return 1
	}
	packsDir := strings.TrimSpace(os.Getenv("EVIDRA_PACKS_DIR"))
	if packsDir == "" {
		packsDir = defaultPacksDirForProfile(profile)
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

func loadProfileFromEnv() (Profile, error) {
	raw := strings.TrimSpace(os.Getenv("EVIDRA_PROFILE"))
	if raw == "" {
		return ProfileOps, nil
	}
	switch strings.ToLower(raw) {
	case string(ProfileOps):
		return ProfileOps, nil
	case string(ProfileDev):
		return ProfileDev, nil
	default:
		return "", fmt.Errorf("invalid EVIDRA_PROFILE %q (allowed: ops, dev)", raw)
	}
}

func defaultPacksDirForProfile(profile Profile) string {
	switch profile {
	case ProfileDev:
		return "./packs/_core"
	case ProfileOps:
		return "./packs/_core/ops"
	default:
		return ""
	}
}

func buildRegistryForProfile(profile Profile) (*registry.InMemoryRegistry, error) {
	toolRegistry := registry.NewInMemoryRegistry(nil)
	if profile == ProfileDev {
		if err := registry.RegisterDevTools(toolRegistry); err != nil {
			return nil, err
		}
	}
	return toolRegistry, nil
}

func envOrDefault(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func resolvePolicyPaths(profile Profile, policyFlag, dataFlag string) (string, string) {
	if policyFlag != "" {
		return policyFlag, dataFlag
	}

	policyEnv := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH"))
	dataEnv := strings.TrimSpace(os.Getenv("EVIDRA_POLICY_DATA_PATH"))
	if policyEnv != "" {
		return policyEnv, dataEnv
	}

	switch profile {
	case ProfileOps:
		return defaultOpsPolicyPath, defaultOpsDataPath
	case ProfileDev:
		return defaultOpsPolicyPath, defaultOpsDataPath
	default:
		return defaultOpsPolicyPath, defaultOpsDataPath
	}
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
