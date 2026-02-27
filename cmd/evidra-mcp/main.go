package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	evidra "samebits.com/evidra"
	"samebits.com/evidra/pkg/bundlesource"
	"samebits.com/evidra/pkg/config"
	"samebits.com/evidra/pkg/mcpserver"
	"samebits.com/evidra/pkg/mode"
	"samebits.com/evidra/pkg/policysource"
	"samebits.com/evidra/pkg/version"
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
	policyFlag := fs.String("policy", "", "Path to policy rego file")
	dataFlag := fs.String("data", "", "Path to policy data JSON file")
	bundleFlag := fs.String("bundle", "", "Path to OPA bundle directory")
	envFlag := fs.String("environment", "", "Environment label for policy evaluation")
	evidenceFlag := fs.String("evidence-dir", "", "Path to store evidence records")
	evidenceStoreFlag := fs.String("evidence-store", "", "Alias for --evidence-dir")
	observeFlag := fs.Bool("observe", false, "Enable observe mode (policy advises but execution proceeds)")
	offlineFlag := fs.Bool("offline", false, "Force offline mode (skip API)")
	fallbackOffline := fs.Bool("fallback-offline", false, "Allow local eval when API unreachable")
	helpFlag := fs.Bool("help", false, "Show help")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *showVersion {
		fmt.Fprintf(stdout, "evidra-mcp %s (commit: %s, built: %s)\n", version.Version, version.Commit, version.Date)
		return 0
	}
	if *helpFlag {
		printHelp(stdout)
		return 2
	}

	// Resolve fallback policy
	fallbackPolicy := os.Getenv("EVIDRA_FALLBACK")
	if *fallbackOffline {
		fallbackPolicy = "offline"
	}

	// Resolve mode (instant, no I/O)
	resolved, err := mode.Resolve(mode.Config{
		URL:            os.Getenv("EVIDRA_URL"),
		APIKey:         os.Getenv("EVIDRA_API_KEY"),
		FallbackPolicy: fallbackPolicy,
		ForceOffline:   *offlineFlag,
	})
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}

	bundlePath := config.ResolveBundlePath(*bundleFlag)
	environment := config.NormalizeEnvironment(coalesce(strings.TrimSpace(*envFlag), os.Getenv("EVIDRA_ENVIRONMENT")))
	policyPath := strings.TrimSpace(*policyFlag)
	dataPath := strings.TrimSpace(*dataFlag)

	// Determine if we need a local bundle.
	// Online + fallback=offline: need bundle ready for potential runtime fallback.
	// Offline: always need bundle.
	needLocalBundle := !resolved.IsOnline || resolved.FallbackPolicy == "offline"

	looseMode := policyPath != "" || dataPath != "" ||
		strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH")) != "" ||
		strings.TrimSpace(os.Getenv("EVIDRA_DATA_PATH")) != ""

	if needLocalBundle && bundlePath == "" && !looseMode {
		cachedPath, err := extractEmbeddedBundleCached(evidra.OpsV01BundleFS)
		if err != nil {
			fmt.Fprintf(stderr, "extract embedded bundle: %v\n", err)
			return 1
		}
		fmt.Fprintln(stderr, "using built-in ops-v0.1 bundle")
		bundlePath = cachedPath
	}

	// Resolve policy source: bundle takes precedence over loose files.
	var policyRef string
	if bundlePath == "" && needLocalBundle {
		if !looseMode {
			// Online-only mode with fallback=closed: no local bundle needed
			// This path shouldn't be reached if needLocalBundle is properly set
		} else {
			resolvedPolicy, resolvedData, err := config.ResolvePolicyData(policyPath, dataPath)
			if err != nil {
				fmt.Fprintf(stderr, "resolve policy paths: %v\n", err)
				return 1
			}
			policyPath = resolvedPolicy
			dataPath = resolvedData
			ps := policysource.NewLocalFileSource(policyPath, dataPath)
			policyRef = policyRefOrEmpty(ps)
		}
	} else if bundlePath != "" {
		bs, err := bundlesource.NewBundleSource(bundlePath)
		if err != nil {
			fmt.Fprintf(stderr, "load bundle: %v\n", err)
			return 1
		}
		ref, _ := bs.PolicyRef()
		policyRef = ref
	}

	mcpMode, err := resolveMode(*observeFlag)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	evidenceExplicit, err := resolveEvidenceFlagValue(*evidenceStoreFlag, *evidenceFlag)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 1
	}
	evidencePath, err := config.ResolveEvidencePath(evidenceExplicit)
	if err != nil {
		fmt.Fprintf(stderr, "resolve evidence path: %v\n", err)
		return 1
	}

	server := newServerFunc(mcpserver.Options{
		Name:                     "evidra-mcp",
		Version:                  version.Version,
		Mode:                     mcpMode,
		PolicyRef:                policyRef,
		PolicyPath:               policyPath,
		DataPath:                 dataPath,
		BundlePath:               bundlePath,
		Environment:              environment,
		EvidencePath:             evidencePath,
		IncludeFileResourceLinks: envBool("EVIDRA_INCLUDE_FILE_RESOURCE_LINKS", false),
		APIClient:                resolved.Client,
		FallbackPolicy:           resolved.FallbackPolicy,
		IsOnline:                 resolved.IsOnline,
	})

	logger := log.New(stderr, "", log.LstdFlags)
	modeLabel := "offline"
	if resolved.IsOnline {
		modeLabel = "online"
	}
	logger.Printf("evidra-mcp running in %s mode (%s)", mcpMode, modeLabel)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(stderr, "run mcp server: %v\n", err)
		return 1
	}
	return 0
}

func coalesce(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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

func resolveEvidenceFlagValue(evidenceStoreFlag, evidenceDirFlag string) (string, error) {
	store := strings.TrimSpace(evidenceStoreFlag)
	dir := strings.TrimSpace(evidenceDirFlag)
	if store != "" && dir != "" && store != dir {
		return "", fmt.Errorf("conflicting values for --evidence-store and --evidence-dir")
	}
	if store != "" {
		return store, nil
	}
	return dir, nil
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

// bundleCachePath returns the deterministic path for cached embedded bundles.
func bundleCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return filepath.Join(home, ".evidra", "bundles", "ops-v0.1")
}

// extractEmbeddedBundleCached copies the embedded ops-v0.1 bundle FS to a
// deterministic cache path (~/.evidra/bundles/ops-v0.1/). No-op if the
// directory already exists with a .manifest file.
func extractEmbeddedBundleCached(fsys fs.ReadDirFS) (string, error) {
	dir := bundleCachePath()

	// Check if already cached
	if _, err := os.Stat(filepath.Join(dir, ".manifest")); err == nil {
		return dir, nil
	}

	return extractEmbeddedBundle(fsys, dir)
}

// extractEmbeddedBundle copies the embedded ops-v0.1 bundle FS into the
// given target directory.
func extractEmbeddedBundle(fsys fs.ReadDirFS, targetDir string) (string, error) {
	const bundleRoot = "policy/bundles/ops-v0.1"

	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create bundle cache dir: %w", err)
	}

	err := fs.WalkDir(fsys, bundleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, bundleRoot+"/")
		if path == bundleRoot {
			return nil
		}
		dst := filepath.Join(targetDir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		os.RemoveAll(targetDir)
		return "", fmt.Errorf("extract embedded bundle: %w", err)
	}
	return targetDir, nil
}

func printHelp(w io.Writer) {
	defaultEvidence := config.DefaultEvidencePathDescription()

	fmt.Fprintln(w, "evidra-mcp — open-source utility by SameBits. MCP server for AI agent policy evaluation.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  evidra-mcp [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "CONNECTION FLAGS:")
	fmt.Fprintln(w, "  --offline               Force offline mode (skip API even if EVIDRA_URL set)")
	fmt.Fprintln(w, "  --fallback-offline      Allow local eval when API unreachable")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "POLICY SOURCE (offline/fallback only):")
	fmt.Fprintln(w, "  --bundle <path>         Path to OPA bundle directory (e.g. policy/bundles/ops-v0.1)")
	fmt.Fprintln(w, "  --policy <path>         Path to rego entrypoint (legacy loose-file mode)")
	fmt.Fprintln(w, "  --data <path>           Path to policy data.json (legacy loose-file mode)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FLAGS:")
	fmt.Fprintln(w, "  --environment <env>     Environment label for policy evaluation (e.g. prod, staging)")
	fmt.Fprintf(w, "  --evidence-dir <dir>    Where to store evidence chain (default: %s)\n", defaultEvidence)
	fmt.Fprintln(w, "  --evidence-store <dir>  Alias for --evidence-dir")
	fmt.Fprintln(w, "  --observe               Observe-only: do not block, only report (default: enforce)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ENVIRONMENT VARIABLES:")
	fmt.Fprintln(w, "  EVIDRA_URL              API endpoint (enables online mode, e.g. https://api.evidra.rest)")
	fmt.Fprintln(w, "  EVIDRA_API_KEY          Bearer token (required when EVIDRA_URL is set)")
	fmt.Fprintln(w, "  EVIDRA_FALLBACK         closed (default) or offline")
	fmt.Fprintln(w, "  EVIDRA_ENVIRONMENT      Environment label (normalized: prod→production, stg→staging)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "HOSTED SERVICE:")
	fmt.Fprintln(w, "  MCP endpoint:  https://evidra.samebits.com/mcp  (no local install needed)")
	fmt.Fprintln(w, "  REST API base: https://api.evidra.rest/v1")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NOTES:")
	fmt.Fprintln(w, "  - When EVIDRA_URL is set, evaluations are sent to the API server.")
	fmt.Fprintln(w, "  - Use --offline or unset EVIDRA_URL for local-only evaluation.")
	fmt.Fprintln(w, "  - Use `evidra` CLI for offline tools (policy sim, evidence inspect).")
}
