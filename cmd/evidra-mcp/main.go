package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
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
	contentDirFlag := fs.String("content-dir", "", "Optional override path to MCP guidance content directory")
	envFlag := fs.String("environment", "", "Environment label for policy evaluation")
	evidenceFlag := fs.String("evidence-dir", "", "Path to store evidence records")
	evidenceStoreFlag := fs.String("evidence-store", "", "Alias for --evidence-dir")
	offlineFlag := fs.Bool("offline", false, "Force offline mode (skip API)")
	fallbackOffline := fs.Bool("fallback-offline", false, "Allow local eval when API unreachable")
	denyCacheFlag := fs.Bool("deny-cache", false, "Enable deny-loop prevention cache (agent/CI only)")
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
	contentDir := coalesce(strings.TrimSpace(*contentDirFlag), strings.TrimSpace(os.Getenv("EVIDRA_CONTENT_DIR")))
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

	mcpMode := mcpserver.ModeEnforce
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
		ContentDir:               contentDir,
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
		DenyCacheEnabled:         *denyCacheFlag || envBool("EVIDRA_DENY_CACHE", false),
	})

	logger := log.New(stderr, "", log.LstdFlags)
	modeLabel := "offline"
	if resolved.IsOnline {
		modeLabel = "online"
	}
	logger.Printf("evidra-mcp running (%s)", modeLabel)

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

const bundleSHAFile = "BUNDLE.SHA256"

// embeddedBundleHash computes a deterministic SHA-256 over every file in the
// embedded bundle (paths sorted, content hashed in order). This changes
// whenever any .rego, data.json, or .manifest file changes, so the cache is
// always invalidated on binary update.
func embeddedBundleHash(fsys fs.ReadDirFS) (string, error) {
	const bundleRoot = "policy/bundles/ops-v0.1"
	h := sha256.New()

	var paths []string
	err := fs.WalkDir(fsys, bundleRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("embeddedBundleHash walk: %w", err)
	}
	sort.Strings(paths)

	for _, path := range paths {
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return "", fmt.Errorf("embeddedBundleHash read %s: %w", path, err)
		}
		fmt.Fprintf(h, "%s\n", path)
		h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// extractEmbeddedBundleCached copies the embedded ops-v0.1 bundle FS to a
// deterministic cache path (~/.evidra/bundles/ops-v0.1/). Re-extracts
// whenever the embedded bundle content changes (detected via SHA-256).
func extractEmbeddedBundleCached(fsys fs.ReadDirFS) (string, error) {
	dir := bundleCachePath()
	shaFile := filepath.Join(dir, bundleSHAFile)

	want, err := embeddedBundleHash(fsys)
	if err != nil {
		return "", err
	}

	// Cache hit: SHA matches — no extraction needed.
	if got, err := os.ReadFile(shaFile); err == nil && strings.TrimSpace(string(got)) == want {
		return dir, nil
	}

	// Cache miss or stale: remove and re-extract.
	os.RemoveAll(dir)
	if _, err := extractEmbeddedBundle(fsys, dir); err != nil {
		return "", err
	}

	if err := os.WriteFile(shaFile, []byte(want+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("write bundle sha: %w", err)
	}
	return dir, nil
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
	fmt.Fprintln(w, "  --content-dir <dir>     Optional override for MCP guidance content (initialize/tools/resources text files)")
	fmt.Fprintln(w, "  --deny-cache            Enable deny-loop prevention cache for agent/CI actors")
	fmt.Fprintln(w, "  --environment <env>     Environment label for policy evaluation (e.g. prod, staging)")
	fmt.Fprintf(w, "  --evidence-dir <dir>    Where to store evidence chain (default: %s)\n", defaultEvidence)
	fmt.Fprintln(w, "  --evidence-store <dir>  Alias for --evidence-dir")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "ENVIRONMENT VARIABLES:")
	fmt.Fprintln(w, "  EVIDRA_URL              API endpoint (enables online mode, e.g. https://api.evidra.rest)")
	fmt.Fprintln(w, "  EVIDRA_API_KEY          Bearer token (required when EVIDRA_URL is set)")
	fmt.Fprintln(w, "  EVIDRA_FALLBACK         closed (default) or offline")
	fmt.Fprintln(w, "  EVIDRA_DENY_CACHE       Enable deny-loop prevention (true/false, default: false)")
	fmt.Fprintln(w, "  EVIDRA_CONTENT_DIR      Override MCP guidance content directory (same as --content-dir)")
	fmt.Fprintln(w, "  EVIDRA_ENVIRONMENT      Environment label (normalized: prod→production, stg→staging)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "HOSTED SERVICE:")
	fmt.Fprintln(w, "  MCP endpoint:  https://evidra.samebits.com/mcp  (no local install needed)")
	fmt.Fprintln(w, "  REST API base: https://api.evidra.rest/v1")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "NOTES:")
	fmt.Fprintln(w, "  - When EVIDRA_URL is set, evaluations are sent to the API server.")
	fmt.Fprintln(w, "  - Use --offline or unset EVIDRA_URL for local-only evaluation.")
	fmt.Fprintln(w, "  - Guidance content defaults to embedded files; use --content-dir to override without rebuilding.")
	fmt.Fprintln(w, "  - Use `evidra` CLI for offline tools (policy sim, evidence inspect).")
}
