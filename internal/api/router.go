package api

import (
	"io/fs"
	"net/http"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
	"samebits.com/evidra/internal/store"
)

// RouterConfig holds the dependencies for building the API router.
type RouterConfig struct {
	Engine       *engine.Adapter
	Signer       *evidence.Signer
	APIKey       string         // Phase 0: static key; empty when Store is set.
	ServerID     string
	UIFS         fs.FS          // Embedded UI filesystem; nil disables UI serving.
	Store        *store.KeyStore // Phase 1: nil in Phase 0.
	DB           Pinger         // Phase 1: database pool for readyz; nil in Phase 0.
	InviteSecret string         // Phase 1: optional invite gate for POST /v1/keys.
}

// NewRouter builds the HTTP handler with all routes and middleware.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	builderCfg := evidence.BuilderConfig{
		ServerID: cfg.ServerID,
		// TenantID set per-request from auth context.
	}

	// Choose auth middleware: DB-backed (Phase 1) or static key (Phase 0).
	var authMW func(http.Handler) http.Handler
	if cfg.Store != nil {
		authMW = auth.KeyStoreMiddleware(cfg.Store)
	} else {
		authMW = auth.StaticKeyMiddleware(cfg.APIKey)
	}

	// Public endpoints (no auth).
	mux.Handle("GET /healthz", handleHealthz())
	mux.Handle("GET /v1/evidence/pubkey", handlePubkey(cfg.Signer))
	mux.Handle("POST /v1/keys", handleKeys(cfg.Store, cfg.InviteSecret))

	// Phase 1: readyz requires a database.
	if cfg.DB != nil {
		mux.Handle("GET /readyz", handleReadyz(cfg.DB))
	}

	// Authenticated endpoints.
	mux.Handle("POST /v1/validate", authMW(
		handleValidate(cfg.Engine, cfg.Signer, builderCfg),
	))

	// Embedded UI (SPA fallback).
	if cfg.UIFS != nil {
		mux.Handle("/", uiHandler(cfg.UIFS))
	}

	// Middleware stack: recovery → logging → body limit → router.
	var handler http.Handler = mux
	handler = bodyLimitMiddleware(handler)
	handler = requestLogMiddleware(handler)
	handler = recoveryMiddleware(handler)

	return handler
}
