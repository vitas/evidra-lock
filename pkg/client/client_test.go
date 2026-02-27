package client

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"samebits.com/evidra/pkg/invocation"
)

func testInvocation() invocation.ToolInvocation {
	return invocation.ToolInvocation{
		Actor:     invocation.Actor{Type: "agent", ID: "test", Origin: "cli"},
		Tool:      "kubectl",
		Operation: "apply",
		Params: map[string]interface{}{
			"target": map[string]interface{}{"namespace": "default"},
		},
	}
}

func TestValidate_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/validate" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected Bearer test-key, got %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Request-ID") == "" {
			t.Error("expected X-Request-ID header")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":       true,
			"risk_level":  "low",
			"evidence_id": "evt-123",
			"policy_ref":  "ops-v0.1",
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	result, reqID, err := c.Validate(context.Background(), testInvocation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reqID == "" {
		t.Error("expected non-empty request ID")
	}
	if !result.Pass {
		t.Error("expected Pass=true")
	}
	if result.Source != "api" {
		t.Errorf("expected Source=api, got %s", result.Source)
	}
	if result.EvidenceID != "evt-123" {
		t.Errorf("expected evidence_id=evt-123, got %s", result.EvidenceID)
	}
	if result.PolicyRef != "ops-v0.1" {
		t.Errorf("expected policy_ref=ops-v0.1, got %s", result.PolicyRef)
	}
}

func TestValidate_Denied(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":      false,
			"risk_level": "high",
			"reasons":    []string{"namespace is kube-system"},
			"rule_ids":   []string{"k8s.protected_namespace"},
			"hints":      []string{"Use a different namespace"},
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	result, _, err := c.Validate(context.Background(), testInvocation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Pass {
		t.Error("expected Pass=false")
	}
	if result.RiskLevel != "high" {
		t.Errorf("expected risk_level=high, got %s", result.RiskLevel)
	}
	if result.Source != "api" {
		t.Errorf("expected Source=api, got %s", result.Source)
	}
	if len(result.Reasons) != 1 || result.Reasons[0] != "namespace is kube-system" {
		t.Errorf("unexpected reasons: %v", result.Reasons)
	}
}

func TestValidate_Unauthorized(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "bad-key"})
	_, _, err := c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
	if IsReachabilityError(err) {
		t.Error("401 should not be a reachability error")
	}
}

func TestValidate_Forbidden(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
	if IsReachabilityError(err) {
		t.Error("403 should not be a reachability error")
	}
}

func TestValidate_ServerError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrServerError) {
		t.Fatalf("expected ErrServerError, got %v", err)
	}
	if !IsReachabilityError(err) {
		t.Error("500 should be a reachability error")
	}
}

func TestValidate_Unreachable(t *testing.T) {
	t.Parallel()
	// Use a listener that is immediately closed to get connection refused.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	c := New(Config{URL: "http://" + addr, APIKey: "test-key", Timeout: 1 * time.Second})
	_, _, err = c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable, got %v", err)
	}
	if !IsReachabilityError(err) {
		t.Error("unreachable should be a reachability error")
	}
}

func TestValidate_RateLimited(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected ErrRateLimited, got %v", err)
	}
	if IsReachabilityError(err) {
		t.Error("429 should not be a reachability error")
	}
}

func TestValidate_InvalidInput(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(context.Background(), testInvocation())
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if IsReachabilityError(err) {
		t.Error("422 should not be a reachability error")
	}
}

func TestValidate_UnknownFields(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"allow":         true,
			"risk_level":    "low",
			"evidence_id":   "evt-456",
			"future_field":  "should be ignored",
			"another_field": 42,
		})
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	result, _, err := c.Validate(context.Background(), testInvocation())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Pass {
		t.Error("expected Pass=true")
	}
	if result.EvidenceID != "evt-456" {
		t.Errorf("expected evidence_id=evt-456, got %s", result.EvidenceID)
	}
}

func TestValidate_Timeout(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()
	// Small sleep to ensure context is expired
	time.Sleep(5 * time.Millisecond)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(ctx, testInvocation())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable on timeout, got %v", err)
	}
	if !IsReachabilityError(err) {
		t.Error("timeout should be a reachability error")
	}
}

func TestValidate_ContextCanceled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	_, _, err := c.Validate(ctx, testInvocation())
	if !errors.Is(err, ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable on cancel, got %v", err)
	}
}

func TestPing_Success(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Errorf("expected /healthz, got %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := New(Config{URL: srv.URL, APIKey: "test-key"})
	if err := c.Ping(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPing_Unreachable(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()

	c := New(Config{URL: "http://" + addr, APIKey: "test-key", Timeout: 1 * time.Second})
	if err := c.Ping(context.Background()); err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestNew_DefaultTimeout(t *testing.T) {
	t.Parallel()
	c := New(Config{URL: "http://localhost:9999", APIKey: "key"})
	if c.http.Timeout != 30*time.Second {
		t.Errorf("expected default 30s timeout, got %v", c.http.Timeout)
	}
}

func TestNew_CustomTimeout(t *testing.T) {
	t.Parallel()
	c := New(Config{URL: "http://localhost:9999", APIKey: "key", Timeout: 10 * time.Second})
	if c.http.Timeout != 10*time.Second {
		t.Errorf("expected 10s timeout, got %v", c.http.Timeout)
	}
}

func TestURL(t *testing.T) {
	t.Parallel()
	c := New(Config{URL: "https://api.evidra.rest", APIKey: "key"})
	if c.URL() != "https://api.evidra.rest" {
		t.Errorf("expected https://api.evidra.rest, got %s", c.URL())
	}
}
