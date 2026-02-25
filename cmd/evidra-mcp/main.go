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

	bundlePath := config.ResolveBundlePath(*bundleFlag)
	environment := strings.TrimSpace(*envFlag)
	policyPath := strings.TrimSpace(*policyFlag)
	dataPath := strings.TrimSpace(*dataFlag)

	// If no bundle is configured and loose-file mode is not explicitly
	// requested, fall back to the built-in embedded ops-v0.1 bundle.
	looseMode := policyPath != "" || dataPath != "" ||
		strings.TrimSpace(os.Getenv("EVIDRA_POLICY_PATH")) != "" ||
		strings.TrimSpace(os.Getenv("EVIDRA_DATA_PATH")) != ""
	if bundlePath == "" && !looseMode {
		tmpDir, err := extractEmbeddedBundle(evidra.OpsV01BundleFS)
		if err != nil {
			fmt.Fprintf(stderr, "extract embedded bundle: %v\n", err)
			return 1
		}
		fmt.Fprintln(stderr, "using built-in ops-v0.1 bundle")
		bundlePath = tmpDir
	}

	// Resolve policy source: bundle takes precedence over loose files.
	var policyRef string
	if bundlePath == "" {
		resolvedPolicy, resolvedData, err := config.ResolvePolicyData(policyPath, dataPath)
		if err != nil {
			fmt.Fprintf(stderr, "resolve policy paths: %v\n", err)
			return 1
		}
		policyPath = resolvedPolicy
		dataPath = resolvedData
		ps := policysource.NewLocalFileSource(policyPath, dataPath)
		policyRef = policyRefOrEmpty(ps)
	} else {
		bs, err := bundlesource.NewBundleSource(bundlePath)
		if err != nil {
			fmt.Fprintf(stderr, "load bundle: %v\n", err)
			return 1
		}
		ref, _ := bs.PolicyRef()
		policyRef = ref
	}

	mode, err := resolveMode(*observeFlag)
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
		Mode:                     mode,
		PolicyRef:                policyRef,
		PolicyPath:               policyPath,
		DataPath:                 dataPath,
		BundlePath:               bundlePath,
		Environment:              environment,
		EvidencePath:             evidencePath,
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

// extractEmbeddedBundle copies the embedded ops-v0.1 bundle FS into a new
// temporary directory and returns its path. The caller owns the directory;
// it will be cleaned up by the OS on reboot or can be left for the process
// lifetime (acceptable for a short-lived CLI/MCP server invocation).
//
// Generated by AI | 2026-02-25 | Zero-config embedded bundle extraction | zero-config-embedded-bundle
func extractEmbeddedBundle(fsys fs.ReadDirFS) (string, error) {
	const bundleRoot = "policy/bundles/ops-v0.1"
	dir, err := os.MkdirTemp("", "evidra-bundle-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	err = fs.WalkDir(fsys, bundleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, bundleRoot+"/")
		if path == bundleRoot {
			return nil
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
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
		os.RemoveAll(dir)
		return "", fmt.Errorf("extract embedded bundle: %w", err)
	}
	return dir, nil
}

func printHelp(w io.Writer) {
	defaultEvidence := config.DefaultEvidencePathDescription()

	fmt.Fprintln(w, "evidra-mcp — Local MCP server that enforces deterministic policy on AI-generated infra changes.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "USAGE:")
	fmt.Fprintln(w, "  evidra-mcp --bundle <path> [flags]")
	fmt.Fprintln(w, "  evidra-mcp --policy <path> --data <path> [flags]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "POLICY SOURCE (one of):")
	fmt.Fprintln(w, "  --bundle <path>         Path to OPA bundle directory (e.g. policy/bundles/ops-v0.1)")
	fmt.Fprintln(w, "  --policy <path>         Path to rego entrypoint (legacy loose-file mode)")
	fmt.Fprintln(w, "  --data <path>           Path to policy data.json (legacy loose-file mode)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "FLAGS:")
	fmt.Fprintln(w, "  --environment <env>     Environment label for policy evaluation (e.g. prod, staging)")
	fmt.Fprintf(w, "  --evidence-dir <dir>    Where to store evidence chain (default: %s; override via EVIDRA_EVIDENCE_DIR/EVIDRA_EVIDENCE_PATH)\n", defaultEvidence)
	fmt.Fprintln(w, "  --evidence-store <dir>  Alias for --evidence-dir")
	fmt.Fprintln(w, "  --observe               Observe-only: do not block, only report (default: enforce)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "EXAMPLES:")
	fmt.Fprintln(w, "  evidra-mcp --bundle policy/bundles/ops-v0.1 \\")
	fmt.Fprintf(w, "             --evidence-dir %s\n", defaultEvidence)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NOTES:")
	fmt.Fprintln(w, "  - Use `evidra` for offline tools (policy sim, evidence inspect/report).")
}
