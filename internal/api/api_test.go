package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
	"samebits.com/evidra/pkg/bundlesource"
	"samebits.com/evidra/pkg/invocation"
)

const testAPIKey = "test-key-at-least-32-characters-long"

var bundleDir = "../../policy/bundles/ops-v0.1"

func testRouter(t *testing.T) http.Handler {
	t.Helper()
	bs, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource: %v", err)
	}
	eng, err := engine.NewAdapter(bs)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	signer, err := evidence.NewSigner(evidence.SignerConfig{DevMode: true})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return NewRouter(RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   testAPIKey,
		ServerID: "test-server",
	})
}

func testSigner(t *testing.T) *evidence.Signer {
	t.Helper()
	s, err := evidence.NewSigner(evidence.SignerConfig{DevMode: true})
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	return s
}

func authRequest(method, path string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, path, body)
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func validBody() string {
	return `{
		"actor": {"type": "agent", "id": "claude", "origin": "mcp"},
		"tool": "kubectl",
		"operation": "apply",
		"params": {"target": {"namespace": "default"}}
	}`
}

// --- Health handler ---

func TestHealthz(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status = %q, want ok", body["status"])
	}
}

func TestHealthz_NoAuth(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("healthz should not require auth, got status %d", rec.Code)
	}
}

// --- Pubkey handler ---

func TestPubkey(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/evidence/pubkey", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-pem-file" {
		t.Errorf("Content-Type = %q, want application/x-pem-file", ct)
	}

	block, _ := pem.Decode(rec.Body.Bytes())
	if block == nil {
		t.Fatal("response is not valid PEM")
	}
	if block.Type != "PUBLIC KEY" {
		t.Errorf("PEM type = %q, want PUBLIC KEY", block.Type)
	}
}

func TestPubkey_NoAuth(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/evidence/pubkey", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("pubkey should not require auth, got status %d", rec.Code)
	}
}

// --- Validate handler ---

func TestValidate_Allow(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(validBody()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var evRec evidence.EvidenceRecord
	if err := json.NewDecoder(rec.Body).Decode(&evRec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if !evRec.Decision.Allow {
		t.Error("expected allow=true")
	}
	if evRec.EventID == "" {
		t.Error("expected non-empty EventID")
	}
	if !strings.HasPrefix(evRec.EventID, "evt_") {
		t.Errorf("EventID = %q, want evt_ prefix", evRec.EventID)
	}
	if evRec.Signature == "" {
		t.Error("expected non-empty Signature")
	}
	if evRec.SigningPayload == "" {
		t.Error("expected non-empty SigningPayload")
	}
	if evRec.TenantID != "static" {
		t.Errorf("TenantID = %q, want static", evRec.TenantID)
	}
	if evRec.ServerID != "test-server" {
		t.Errorf("ServerID = %q, want test-server", evRec.ServerID)
	}
	if evRec.Tool != "kubectl" {
		t.Errorf("Tool = %q, want kubectl", evRec.Tool)
	}
}

func TestValidate_SignatureVerifies(t *testing.T) {
	t.Parallel()

	// Use a shared signer so we can verify the signature.
	signer := testSigner(t)
	bs, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource: %v", err)
	}
	eng, err := engine.NewAdapter(bs)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	handler := NewRouter(RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   testAPIKey,
		ServerID: "sig-test",
	})

	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(validBody()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	var evRec evidence.EvidenceRecord
	if err := json.NewDecoder(rec.Body).Decode(&evRec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	sig, err := base64.StdEncoding.DecodeString(evRec.Signature)
	if err != nil {
		t.Fatalf("decode signature: %v", err)
	}

	if !signer.Verify([]byte(evRec.SigningPayload), sig) {
		t.Error("signature does not verify")
	}
}

func TestValidate_DenyIsHTTP200(t *testing.T) {
	t.Parallel()

	// Use deny-all policy source.
	denyRego := `package evidra.policy
decision := {
	"allow": false,
	"risk_level": "high",
	"reason": "denied by test policy",
	"reasons": ["test.deny_all"],
	"hits": ["test.deny_all"],
	"hints": ["test deny hint"],
}
`
	src := &fakePolicySource{
		modules:  map[string][]byte{"deny.rego": []byte(denyRego)},
		data:     []byte(`{}`),
		ref:      "sha256:deny-test",
		revision: "test",
		profile:  "test",
	}
	eng, err := engine.NewAdapter(src)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	signer := testSigner(t)
	handler := NewRouter(RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   testAPIKey,
		ServerID: "deny-test",
	})

	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(validBody()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("deny should return 200, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var evRec evidence.EvidenceRecord
	if err := json.NewDecoder(rec.Body).Decode(&evRec); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if evRec.Decision.Allow {
		t.Error("expected allow=false")
	}
	if evRec.Decision.RiskLevel != "high" {
		t.Errorf("RiskLevel = %q, want high", evRec.Decision.RiskLevel)
	}
}

func TestValidate_NoAuth(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/validate", strings.NewReader(validBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestValidate_InvalidJSON(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(`{invalid`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestValidate_MissingRequiredFields(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	body := `{"actor": {"type": "agent", "id": "claude", "origin": "mcp"}, "tool": "", "operation": "apply", "params": {}}`
	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
}

func TestValidate_NewlineInField(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "newline in actor.type",
			body: `{"actor":{"type":"agent\n","id":"claude","origin":"mcp"},"tool":"kubectl","operation":"apply","params":{}}`,
		},
		{
			name: "carriage return in tool",
			body: `{"actor":{"type":"agent","id":"claude","origin":"mcp"},"tool":"kubectl\r","operation":"apply","params":{}}`,
		},
		{
			name: "newline in operation",
			body: `{"actor":{"type":"agent","id":"claude","origin":"mcp"},"tool":"kubectl","operation":"apply\n","params":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestValidate_WrongMethod(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := authRequest(http.MethodGet, "/v1/validate", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// ServeMux returns 405 for wrong method on a registered pattern.
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestValidate_ContentTypeJSON(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(validBody()))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// --- Middleware ---

func TestBodyLimit(t *testing.T) {
	t.Parallel()
	handler := testRouter(t)

	// Create a body larger than 1MB.
	large := strings.Repeat("x", 1<<20+1)
	req := authRequest(http.MethodPost, "/v1/validate", strings.NewReader(large))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should fail with 400 (bad request from body too large) or 413.
	if rec.Code == http.StatusOK {
		t.Error("expected non-200 for oversized body")
	}
}

func TestRecoveryMiddleware(t *testing.T) {
	t.Parallel()

	panicker := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	handler := recoveryMiddleware(panicker)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	// Should not panic.
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

// --- Response helpers ---

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeJSON(rec, http.StatusCreated, map[string]string{"key": "value"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["key"] != "value" {
		t.Errorf("body[key] = %q, want value", body["key"])
	}
}

func TestWriteError(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	writeError(rec, http.StatusBadRequest, "bad input")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error != "bad input" {
		t.Errorf("error = %q, want %q", body.Error, "bad input")
	}
}

// --- Validate payload field validation ---

func TestValidatePayloadFields_Clean(t *testing.T) {
	t.Parallel()

	inv := validInvocation()
	if err := validatePayloadFields(inv); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidatePayloadFields_Newlines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		value string
	}{
		{"actor.type with LF", "actor.type", "agent\n"},
		{"actor.id with CR", "actor.id", "claude\r"},
		{"actor.origin with CRLF", "actor.origin", "mcp\r\n"},
		{"tool with LF", "tool", "kubectl\n"},
		{"operation with LF", "operation", "apply\n"},
		{"environment with CR", "environment", "prod\r"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			inv := validInvocation()
			switch tt.field {
			case "actor.type":
				inv.Actor.Type = tt.value
			case "actor.id":
				inv.Actor.ID = tt.value
			case "actor.origin":
				inv.Actor.Origin = tt.value
			case "tool":
				inv.Tool = tt.value
			case "operation":
				inv.Operation = tt.value
			case "environment":
				inv.Environment = tt.value
			}
			if err := validatePayloadFields(inv); err == nil {
				t.Errorf("expected error for %s with newline", tt.field)
			}
		})
	}
}

// --- StatusWriter ---

func TestStatusWriter_CapturesCode(t *testing.T) {
	t.Parallel()

	inner := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: inner, status: http.StatusOK}
	sw.WriteHeader(http.StatusNotFound)

	if sw.status != http.StatusNotFound {
		t.Errorf("status = %d, want %d", sw.status, http.StatusNotFound)
	}
}

func TestStatusWriter_WriteDefaultsTo200(t *testing.T) {
	t.Parallel()

	inner := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: inner, status: http.StatusOK}
	sw.Write([]byte("hello"))

	if sw.status != http.StatusOK {
		t.Errorf("status = %d, want %d", sw.status, http.StatusOK)
	}
}

func TestStatusWriter_FirstWriteHeaderWins(t *testing.T) {
	t.Parallel()

	inner := httptest.NewRecorder()
	sw := &statusWriter{ResponseWriter: inner, status: http.StatusOK}
	sw.WriteHeader(http.StatusCreated)
	sw.WriteHeader(http.StatusNotFound) // should be ignored

	if sw.status != http.StatusCreated {
		t.Errorf("status = %d, want %d (first call)", sw.status, http.StatusCreated)
	}
}

// --- Request logging middleware ---

func TestRequestLogMiddleware(t *testing.T) {
	t.Parallel()

	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := requestLogMiddleware(inner)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// --- Body limit middleware ---

func TestBodyLimitMiddleware_UnderLimit(t *testing.T) {
	t.Parallel()

	var readBytes int
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		readBytes = len(data)
		w.WriteHeader(http.StatusOK)
	})
	handler := bodyLimitMiddleware(inner)

	body := bytes.NewReader([]byte("small body"))
	req := httptest.NewRequest(http.MethodPost, "/", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if readBytes != 10 {
		t.Errorf("read %d bytes, want 10", readBytes)
	}
}

// --- UI handler ---

func TestUIHandler_ServesIndexHTML(t *testing.T) {
	t.Parallel()

	handler := uiHandler(testUIFS())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "<html>") {
		t.Error("expected HTML response for /")
	}
}

func TestUIHandler_ServesStaticAsset(t *testing.T) {
	t.Parallel()

	handler := uiHandler(testUIFS())
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "console.log") {
		t.Error("expected JS content for /assets/app.js")
	}
}

func TestUIHandler_SPAFallback(t *testing.T) {
	t.Parallel()

	handler := uiHandler(testUIFS())
	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (SPA fallback)", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "<html>") {
		t.Error("expected index.html for unknown path (SPA fallback)")
	}
}

func TestRouter_UINotServedWhenNil(t *testing.T) {
	t.Parallel()

	handler := testRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Without UIFS, root path should 404.
	if rec.Code == http.StatusOK {
		t.Error("expected non-200 when UIFS is nil")
	}
}

func TestRouter_APITakesPrecedenceOverUI(t *testing.T) {
	t.Parallel()

	bs, err := bundlesource.NewBundleSource(bundleDir)
	if err != nil {
		t.Fatalf("NewBundleSource: %v", err)
	}
	eng, err := engine.NewAdapter(bs)
	if err != nil {
		t.Fatalf("NewAdapter: %v", err)
	}
	signer := testSigner(t)
	handler := NewRouter(RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   testAPIKey,
		ServerID: "ui-test",
		UIFS:     testUIFS(),
	})

	// /healthz should still return API response, not UI.
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("expected healthz API response, got %q", rec.Body.String())
	}
}

// testUIFS creates an in-memory filesystem for UI handler tests.
func testUIFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":    {Data: []byte("<html><body>Evidra UI</body></html>")},
		"assets/app.js": {Data: []byte("console.log('evidra');")},
		"favicon.svg":   {Data: []byte("<svg></svg>")},
	}
}

// --- Helpers ---

func validInvocation() invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "agent", ID: "claude", Origin: "mcp"},
		Tool:      "kubectl",
		Operation: "apply",
		Params:    map[string]interface{}{"target": map[string]interface{}{"namespace": "default"}},
	}
}

// fakePolicySource is a test double for runtime.PolicySource.
type fakePolicySource struct {
	modules  map[string][]byte
	data     []byte
	ref      string
	revision string
	profile  string
}

func (f *fakePolicySource) LoadPolicy() (map[string][]byte, error) { return f.modules, nil }
func (f *fakePolicySource) LoadData() ([]byte, error)              { return f.data, nil }
func (f *fakePolicySource) PolicyRef() (string, error)             { return f.ref, nil }
func (f *fakePolicySource) BundleRevision() string                 { return f.revision }
func (f *fakePolicySource) ProfileName() string                    { return f.profile }
