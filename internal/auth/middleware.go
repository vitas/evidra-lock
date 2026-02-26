package auth

import (
	"crypto/rand"
	"crypto/subtle"
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

func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "Bearer ") {
		return ""
	}
	return h[len("Bearer "):]
}

func authFail(w http.ResponseWriter, r *http.Request) {
	jitterSleep()
	slog.Warn("auth failure",
		"method", r.Method,
		"path", r.URL.Path,
		"remote", r.RemoteAddr,
	)
	http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
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
