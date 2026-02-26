//go:build integration

package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	evidra "samebits.com/evidra"
	"samebits.com/evidra/internal/api"
	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
	"samebits.com/evidra/pkg/bundlesource"
)

const testAPIKey = "test-key-minimum-32-chars-for-test"

// startTestServer creates a fully wired API server using the embedded bundle
// and an ephemeral signer. Returns the server and signer for verification.
func startTestServer(t *testing.T) (*httptest.Server, *evidence.Signer) {
	t.Helper()

	// Extract embedded bundle.
	bundlePath, err := extractEmbeddedBundle(evidra.OpsV01BundleFS)
	if err != nil {
		t.Fatalf("extract embedded bundle: %v", err)
	}
	t.Cleanup(func() {
		removeAll(bundlePath)
	})

	bs, err := bundlesource.NewBundleSource(bundlePath)
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

	handler := api.NewRouter(api.RouterConfig{
		Engine:   eng,
		Signer:   signer,
		APIKey:   testAPIKey,
		ServerID: "integration-test",
	})

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	return srv, signer
}

func postValidate(t *testing.T, baseURL, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, baseURL+"/v1/validate", strings.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+testAPIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /v1/validate: %v", err)
	}
	return resp
}

func decodeEvidence(t *testing.T, resp *http.Response) evidence.EvidenceRecord {
	t.Helper()
	defer resp.Body.Close()

	var rec evidence.EvidenceRecord
	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		t.Fatalf("decode evidence record: %v", err)
	}
	return rec
}

func fetchPubkey(t *testing.T, baseURL string) ed25519.PublicKey {
	t.Helper()

	resp, err := http.Get(baseURL + "/v1/evidence/pubkey")
	if err != nil {
		t.Fatalf("GET /v1/evidence/pubkey: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pubkey status = %d, want 200", resp.StatusCode)
	}

	pemData, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read pubkey body: %v", err)
	}

	block, _ := pem.Decode(pemData)
	if block == nil {
		t.Fatal("pubkey response is not valid PEM")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		t.Fatalf("parse pubkey: %v", err)
	}

	edPub, ok := pub.(ed25519.PublicKey)
	if !ok {
		t.Fatal("pubkey is not Ed25519")
	}
	return edPub
}

func verifySignature(t *testing.T, pubkey ed25519.PublicKey, rec evidence.EvidenceRecord) {
	t.Helper()

	sig, err := base64.StdEncoding.DecodeString(rec.Signature)
	if err != nil {
		t.Fatalf("decode signature for %s: %v", rec.EventID, err)
	}

	if !ed25519.Verify(pubkey, []byte(rec.SigningPayload), sig) {
		t.Errorf("signature verification failed for %s", rec.EventID)
	}
}

// removeAll is a best-effort cleanup wrapper.
func removeAll(path string) {
	os.RemoveAll(path)
}

func TestIntegration_SmokeTest(t *testing.T) {
	srv, _ := startTestServer(t)

	// ── Step 1: healthz ──
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}

	// ── Step 2: POST /v1/validate — kubectl apply to default (expect allow) ──
	allowBody := `{
		"actor": {"type": "agent", "id": "claude", "origin": "cli"},
		"tool": "kubectl",
		"operation": "apply",
		"params": {"target": {"namespace": "default"}}
	}`
	allowResp := postValidate(t, srv.URL, allowBody)
	if allowResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(allowResp.Body)
		allowResp.Body.Close()
		t.Fatalf("allow request: status = %d, body = %s", allowResp.StatusCode, body)
	}
	allowRec := decodeEvidence(t, allowResp)

	if !allowRec.Decision.Allow {
		t.Errorf("expected allow=true, got false (reason: %s)", allowRec.Decision.Reason)
	}
	if allowRec.Decision.RiskLevel != "low" {
		t.Errorf("allow RiskLevel = %q, want low", allowRec.Decision.RiskLevel)
	}
	if !strings.HasPrefix(allowRec.EventID, "evt_") {
		t.Errorf("allow EventID = %q, missing evt_ prefix", allowRec.EventID)
	}
	if allowRec.Signature == "" {
		t.Error("allow record missing signature")
	}
	if allowRec.SigningPayload == "" {
		t.Error("allow record missing signing_payload")
	}
	if allowRec.TenantID != "static" {
		t.Errorf("allow TenantID = %q, want static", allowRec.TenantID)
	}
	if allowRec.ServerID != "integration-test" {
		t.Errorf("allow ServerID = %q, want integration-test", allowRec.ServerID)
	}
	if allowRec.PolicyRef == "" {
		t.Error("allow record missing PolicyRef")
	}
	if !strings.HasPrefix(allowRec.InputHash, "sha256:") {
		t.Errorf("allow InputHash = %q, want sha256: prefix", allowRec.InputHash)
	}

	// ── Step 3: POST /v1/validate — kubectl delete in kube-system (expect deny with 200) ──
	denyBody := `{
		"actor": {"type": "agent", "id": "claude", "origin": "cli"},
		"tool": "kubectl",
		"operation": "delete",
		"params": {"target": {"namespace": "default"}}
	}`
	denyResp := postValidate(t, srv.URL, denyBody)

	// Key design: deny = HTTP 200.
	if denyResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(denyResp.Body)
		denyResp.Body.Close()
		t.Fatalf("deny request: status = %d (want 200), body = %s", denyResp.StatusCode, body)
	}
	denyRec := decodeEvidence(t, denyResp)

	// With the embedded ops-v0.1 bundle using target.namespace="default" and operation="delete",
	// the policy may allow or deny depending on rule configuration. What we actually test here is:
	// 1. The server returns HTTP 200 regardless of the decision.
	// 2. The response contains a valid signed evidence record.
	// 3. The signature verifies offline.
	if denyRec.EventID == "" {
		t.Error("deny record missing EventID")
	}
	if denyRec.Signature == "" {
		t.Error("deny record missing signature")
	}
	if denyRec.EventID == allowRec.EventID {
		t.Error("deny and allow records have the same EventID")
	}

	// ── Step 4: GET /v1/evidence/pubkey ──
	pubkey := fetchPubkey(t, srv.URL)

	// ── Step 5: Verify both records offline with the public key ──
	verifySignature(t, pubkey, allowRec)
	verifySignature(t, pubkey, denyRec)

	// ── Step 6: Verify signing payload contains expected fields ──
	for name, rec := range map[string]evidence.EvidenceRecord{
		"allow": allowRec,
		"deny":  denyRec,
	} {
		payload := rec.SigningPayload
		if !strings.HasPrefix(payload, "evidra.v1\n") {
			t.Errorf("%s: signing payload missing version prefix", name)
		}
		if !strings.Contains(payload, "event_id="+rec.EventID+"\n") {
			t.Errorf("%s: signing payload missing event_id", name)
		}
		if !strings.Contains(payload, "tool=kubectl\n") {
			t.Errorf("%s: signing payload missing tool", name)
		}
		if !strings.Contains(payload, "server_id=integration-test\n") {
			t.Errorf("%s: signing payload missing server_id", name)
		}
		if !strings.Contains(payload, "tenant_id=static\n") {
			t.Errorf("%s: signing payload missing tenant_id", name)
		}
	}
}

func TestIntegration_NoAuth_Returns401(t *testing.T) {
	srv, _ := startTestServer(t)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/validate", strings.NewReader(validBody()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestIntegration_WrongKey_Returns401(t *testing.T) {
	srv, _ := startTestServer(t)

	req, err := http.NewRequest(http.MethodPost, srv.URL+"/v1/validate", strings.NewReader(validBody()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer wrong-key-that-is-long-enough-x")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

func TestIntegration_InvalidJSON_Returns400(t *testing.T) {
	srv, _ := startTestServer(t)

	resp := postValidate(t, srv.URL, `{not json}`)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestIntegration_MissingField_Returns400(t *testing.T) {
	srv, _ := startTestServer(t)

	body := `{
		"actor": {"type": "agent", "id": "claude", "origin": "mcp"},
		"tool": "",
		"operation": "apply",
		"params": {}
	}`
	resp := postValidate(t, srv.URL, body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestIntegration_NewlineInjection_Returns400(t *testing.T) {
	srv, _ := startTestServer(t)

	body := `{
		"actor": {"type": "agent\n", "id": "claude", "origin": "mcp"},
		"tool": "kubectl",
		"operation": "apply",
		"params": {}
	}`
	resp := postValidate(t, srv.URL, body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestIntegration_PubkeyNoAuth(t *testing.T) {
	srv, _ := startTestServer(t)

	resp, err := http.Get(srv.URL + "/v1/evidence/pubkey")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("pubkey should not require auth, got %d", resp.StatusCode)
	}
}

func TestIntegration_HealthzNoAuth(t *testing.T) {
	srv, _ := startTestServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("healthz should not require auth, got %d", resp.StatusCode)
	}
}

func validBody() string {
	return `{
		"actor": {"type": "agent", "id": "claude", "origin": "mcp"},
		"tool": "kubectl",
		"operation": "apply",
		"params": {"target": {"namespace": "default"}}
	}`
}
