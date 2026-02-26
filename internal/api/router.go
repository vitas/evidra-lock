package api

import (
	"io/fs"
	"net/http"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
)

// RouterConfig holds the dependencies for building the API router.
type RouterConfig struct {
	Engine   *engine.Adapter
	Signer   *evidence.Signer
	APIKey   string
	ServerID string
	UIFS     fs.FS // Embedded UI filesystem; nil disables UI serving.
}

// NewRouter builds the HTTP handler with all routes and middleware.
func NewRouter(cfg RouterConfig) http.Handler {
	mux := http.NewServeMux()

	builderCfg := evidence.BuilderConfig{
		ServerID: cfg.ServerID,
		// TenantID set per-request from auth context.
	}

	authMW := auth.StaticKeyMiddleware(cfg.APIKey)

	// Public endpoints (no auth).
	mux.Handle("GET /healthz", handleHealthz())
	mux.Handle("GET /v1/evidence/pubkey", handlePubkey(cfg.Signer))

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
