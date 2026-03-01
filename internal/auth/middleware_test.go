package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const testKey = "test-key-at-least-32-characters-long"

func testHandler() http.Handler {
	mw := StaticKeyMiddleware(testKey)
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tid := TenantID(r.Context())
		w.Header().Set("X-Tenant-ID", tid)
		w.WriteHeader(http.StatusOK)
	})
	return mw(inner)
}

func TestStaticKeyMiddleware_ValidKey(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if tid := rec.Header().Get("X-Tenant-ID"); tid != "static" {
		t.Errorf("tenant_id = %q, want %q", tid, "static")
	}
}

func TestStaticKeyMiddleware_WrongKey(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	req.Header.Set("Authorization", "Bearer wrong-key-definitely-not-correct")
	rec := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("jitter sleep too short: %v, want >= 50ms", elapsed)
	}
}

func TestStaticKeyMiddleware_MissingHeader(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStaticKeyMiddleware_EmptyBearer(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	req.Header.Set("Authorization", "Bearer ")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStaticKeyMiddleware_BasicAuthScheme(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	req.Header.Set("Authorization", "Basic "+testKey)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestStaticKeyMiddleware_NoTenantOnFailure(t *testing.T) {
	t.Parallel()

	mw := StaticKeyMiddleware(testKey)
	// This inner handler should never be called on auth failure.
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})
	handler := mw(inner)

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if called {
		t.Error("inner handler should not be called on auth failure")
	}
}

func TestStaticKeyMiddleware_JitterRange(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	// Run multiple auth failures and verify jitter is in range.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
		req.Header.Set("Authorization", "Bearer bad-key-xxxxxxxxxxxxxxxxxx")
		rec := httptest.NewRecorder()

		start := time.Now()
		handler.ServeHTTP(rec, req)
		elapsed := time.Since(start)

		if elapsed < 50*time.Millisecond {
			t.Errorf("iteration %d: jitter %v < 50ms", i, elapsed)
		}
		// Allow generous upper bound for CI scheduling delays.
		if elapsed > 500*time.Millisecond {
			t.Errorf("iteration %d: jitter %v > 500ms (expected ~50-100ms)", i, elapsed)
		}
	}
}

func TestExtractBearerToken_Valid(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer my-token-123")

	got := extractBearerToken(req)
	if got != "my-token-123" {
		t.Errorf("token = %q, want %q", got, "my-token-123")
	}
}

func TestExtractBearerToken_Missing(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("token = %q, want empty", got)
	}
}

func TestExtractBearerToken_WrongScheme(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Token my-token")

	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("token = %q, want empty for non-Bearer scheme", got)
	}
}

func TestExtractBearerToken_CaseSensitive(t *testing.T) {
	t.Parallel()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "bearer my-token")

	got := extractBearerToken(req)
	if got != "" {
		t.Errorf("token = %q, want empty for lowercase 'bearer'", got)
	}
}

func TestAuthFail_WWWAuthenticate(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}

	wwwAuth := rec.Header().Get("WWW-Authenticate")
	if wwwAuth != `Bearer realm="evidra"` {
		t.Errorf("WWW-Authenticate = %q, want Bearer realm=\"evidra\"", wwwAuth)
	}
}

func TestAuthFail_HelpURL(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "unauthorized" {
		t.Errorf("error = %q, want unauthorized", body["error"])
	}
	if body["help"] == "" {
		t.Error("expected help URL in response body")
	}
}

func TestAuthFail_ContentType(t *testing.T) {
	t.Parallel()
	handler := testHandler()

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
