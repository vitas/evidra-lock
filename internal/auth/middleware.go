package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"
)

const (
	// staticTenantID is the synthetic tenant ID used for Phase 0 static key auth.
	staticTenantID = "static"

	// jitterMinMS and jitterMaxMS define the sleep range on auth failure
	// to prevent timing attacks.
	jitterMinMS = 50
	jitterMaxMS = 100
)

// KeyLookup is the interface satisfied by *store.KeyStore.
// Uses only primitive return types to avoid an import cycle between
// internal/auth and internal/store.
type KeyLookup interface {
	// LookupKey hashes the plaintext key and returns tenantID, keyID, and
	// log-safe prefix (e.g. "ev1_a8Fk3mQ9"), or ErrKeyNotFound.
	LookupKey(ctx context.Context, plaintext string) (tenantID, keyID, prefix string, err error)
	// TouchKey updates last_used_at for the given key ID (async, best-effort).
	TouchKey(keyID string)
}

// ErrKeyNotFound must be returned by KeyLookup.LookupKey when the key is absent or revoked.
var ErrKeyNotFound = errors.New("key not found")

// StaticKeyMiddleware returns HTTP middleware that authenticates requests
// using a constant-time comparison against a static API key.
//
// Phase 0 only: the key comes from EVIDRA_API_KEY env var.
// On success, the request context is populated with tenant_id "static".
// On failure, a random 50-100ms jitter sleep precedes the 401 response.
func StaticKeyMiddleware(apiKey string) func(http.Handler) http.Handler {
	keyBytes := []byte(apiKey)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				authFail(w, r)
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), keyBytes) != 1 {
				authFail(w, r)
				return
			}

			ctx := WithTenantID(r.Context(), staticTenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// KeyStoreMiddleware returns HTTP middleware that authenticates requests
// by looking up the Bearer token in the key store (Phase 1+).
//
// On success, the request context is populated with the stored tenant_id.
// On failure, a random 50-100ms jitter sleep precedes the 401 response.
func KeyStoreMiddleware(store KeyLookup) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractBearerToken(r)
			if token == "" {
				authFail(w, r)
				return
			}

			tenantID, keyID, prefix, err := store.LookupKey(r.Context(), token)
			if err != nil {
				if !errors.Is(err, ErrKeyNotFound) {
					slog.Error("auth: key lookup failed", "error", err)
				}
				authFail(w, r)
				return
			}

			store.TouchKey(keyID)

			slog.Debug("auth: key accepted",
				"prefix", prefix,
				"tenant_id", tenantID,
			)

			ctx := WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return h[len("Bearer "):]
}

const authHelpURL = "https://evidra.samebits.com/get-started"

func authFail(w http.ResponseWriter, r *http.Request) {
	jitterSleep()

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}
	slog.Warn("auth failure",
		"method", r.Method,
		"path", r.URL.Path,
		"client_ip", clientIP,
	)

	w.Header().Set("WWW-Authenticate", `Bearer realm="evidra"`)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	fmt.Fprintf(w, `{"error":"unauthorized","help":"generate a key at %s"}`, authHelpURL)
}

// jitterSleep sleeps for a random duration between jitterMinMS and jitterMaxMS
// using crypto/rand for the random value.
func jitterSleep() {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(jitterMaxMS-jitterMinMS+1)))
	if err != nil {
		// Fallback to fixed sleep if crypto/rand fails (should never happen).
		time.Sleep(time.Duration(jitterMinMS) * time.Millisecond)
		return
	}
	ms := jitterMinMS + int(n.Int64())
	time.Sleep(time.Duration(ms) * time.Millisecond)
}
