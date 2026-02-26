package main

import (
	"context"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	evidra "samebits.com/evidra"
	"samebits.com/evidra/internal/api"
	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
	"samebits.com/evidra/pkg/bundlesource"
)

func main() {
	os.Exit(run())
}

func run() int {
	initLogger()

	// --- Config from env vars ---
	apiKey := os.Getenv("EVIDRA_API_KEY")
	if apiKey == "" {
		slog.Error("EVIDRA_API_KEY is required (minimum 32 characters)")
		return 1
	}
	if len(apiKey) < 32 {
		slog.Error("EVIDRA_API_KEY must be at least 32 characters")
		return 1
	}

	listenAddr := envOrDefault("LISTEN_ADDR", ":8080")
	serverID := envOrDefault("EVIDRA_SERVER_ID", hostname())
	devMode := os.Getenv("EVIDRA_ENV") == "development"

	// --- Signer ---
	signer, err := evidence.NewSigner(evidence.SignerConfig{
		KeyBase64: os.Getenv("EVIDRA_SIGNING_KEY"),
		KeyPath:   os.Getenv("EVIDRA_SIGNING_KEY_PATH"),
		DevMode:   devMode,
	})
	if err != nil {
		slog.Error("init signer", "error", err)
		return 1
	}

	// --- Engine (embedded bundle) ---
	bundlePath, err := extractEmbeddedBundle(evidra.OpsV01BundleFS)
	if err != nil {
		slog.Error("extract embedded bundle", "error", err)
		return 1
	}
	defer os.RemoveAll(bundlePath)

	bs, err := bundlesource.NewBundleSource(bundlePath)
	if err != nil {
		slog.Error("load bundle", "error", err)
		return 1
	}

	eng, err := engine.NewAdapter(bs)
	if err != nil {
		slog.Error("init engine", "error", err)
		return 1
	}

	policyRef, _ := bs.PolicyRef()

	// --- Router ---
	handler := api.NewRouter(api.RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   apiKey,
		ServerID: serverID,
	})

	// --- Server ---
	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// --- Startup log ---
	phase := "Phase 0 (stateless)"
	slog.Info("starting evidra-api",
		"addr", listenAddr,
		"phase", phase,
		"server_id", serverID,
		"policy_ref", policyRef,
		"bundle_revision", bs.BundleRevision(),
		"profile_name", bs.ProfileName(),
		"dev_mode", devMode,
	)

	// --- Graceful shutdown ---
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		slog.Info("shutting down", "signal", sig.String())
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			return 1
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "error", err)
		return 1
	}

	slog.Info("server stopped")
	return 0
}

func initLogger() {
	format := envOrDefault("LOG_FORMAT", "json")
	levelStr := envOrDefault("LOG_LEVEL", "info")

	var level slog.Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

// extractEmbeddedBundle copies the embedded ops-v0.1 bundle FS into a temp directory.
func extractEmbeddedBundle(fsys fs.ReadDirFS) (string, error) {
	const bundleRoot = "policy/bundles/ops-v0.1"
	dir, err := os.MkdirTemp("", "evidra-bundle-*")
	if err != nil {
		return "", err
	}
	err = fs.WalkDir(fsys, bundleRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == bundleRoot {
			return nil
		}
		rel := strings.TrimPrefix(path, bundleRoot+"/")
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return dir, nil
}
