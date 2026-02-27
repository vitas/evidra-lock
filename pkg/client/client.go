package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"samebits.com/evidra/pkg/invocation"
	"samebits.com/evidra/pkg/validate"
)

// Config holds API connection settings.
type Config struct {
	URL     string        // base URL without path, e.g. "https://api.evidra.rest" (client appends /v1/validate, /healthz)
	APIKey  string        // Bearer token
	Timeout time.Duration // HTTP timeout (default: 30s)
}

// Client sends evaluation requests to the Evidra API.
type Client struct {
	config Config
	http   *http.Client
}

// New creates a new API client. Does NOT check reachability.
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		config: cfg,
		http:   &http.Client{Timeout: timeout},
	}
}

// URL returns the configured API URL (for error messages).
func (c *Client) URL() string {
	return c.config.URL
}

// apiValidateResponse is the JSON shape returned by POST /v1/validate.
// Private — only used for deserialization. Unknown fields are silently ignored.
type apiValidateResponse struct {
	Allow      bool     `json:"allow"`
	RiskLevel  string   `json:"risk_level"`
	Reason     string   `json:"reason"`
	Reasons    []string `json:"reasons"`
	RuleIDs    []string `json:"rule_ids"`
	Hints      []string `json:"hints"`
	EvidenceID string   `json:"evidence_id"`
	PolicyRef  string   `json:"policy_ref"`
}

func (r *apiValidateResponse) toResult() validate.Result {
	return validate.Result{
		Pass:       r.Allow,
		RiskLevel:  r.RiskLevel,
		EvidenceID: r.EvidenceID,
		Reasons:    r.Reasons,
		RuleIDs:    r.RuleIDs,
		Hints:      r.Hints,
		Source:     "api",
		PolicyRef:  r.PolicyRef,
	}
}

// Validate sends a ToolInvocation to POST /v1/validate and returns a Result.
// Sets X-Request-ID header (random UUID) for server log correlation.
// Returns (Result, requestID, error).
func (c *Client) Validate(ctx context.Context, inv invocation.ToolInvocation) (validate.Result, string, error) {
	reqID := newRequestID()

	body, err := json.Marshal(inv)
	if err != nil {
		return validate.Result{}, reqID, fmt.Errorf("%w: marshal request: %v", ErrInvalidInput, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.URL+"/v1/validate", bytes.NewReader(body))
	if err != nil {
		return validate.Result{}, reqID, fmt.Errorf("%w: create request: %v", ErrUnreachable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("X-Request-ID", reqID)

	resp, err := c.http.Do(req)
	if err != nil {
		return validate.Result{}, reqID, classifyTransportError(err)
	}
	defer resp.Body.Close()

	if err := classifyHTTPStatus(resp); err != nil {
		return validate.Result{}, reqID, err
	}

	var apiResp apiValidateResponse
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
	if err != nil {
		return validate.Result{}, reqID, fmt.Errorf("%w: read response: %v", ErrServerError, err)
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return validate.Result{}, reqID, fmt.Errorf("%w: decode response: %v", ErrServerError, err)
	}

	return apiResp.toResult(), reqID, nil
}

// Ping checks if the API is reachable (GET /healthz).
// NOT used on the hot path. Reserved for `evidra doctor` or optional startup check.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.config.URL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return classifyTransportError(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: healthz returned HTTP %d", ErrServerError, resp.StatusCode)
	}
	return nil
}

// classifyTransportError maps transport-level failures to ErrUnreachable.
func classifyTransportError(err error) error {
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		(errors.As(err, &netErr) && netErr.Timeout()) {
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	}
	// Connection refused, DNS resolution failure, etc.
	return fmt.Errorf("%w: %v", ErrUnreachable, err)
}

// classifyHTTPStatus maps non-200 HTTP status codes to sentinel errors.
func classifyHTTPStatus(resp *http.Response) error {
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return nil
	case resp.StatusCode == 401:
		return ErrUnauthorized
	case resp.StatusCode == 403:
		return ErrForbidden
	case resp.StatusCode == 422:
		return ErrInvalidInput
	case resp.StatusCode == 429:
		return ErrRateLimited
	case resp.StatusCode >= 500:
		return fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
	default:
		return fmt.Errorf("unexpected HTTP status: %d", resp.StatusCode)
	}
}

// newRequestID generates a random UUID v4 for X-Request-ID.
func newRequestID() string {
	var uuid [16]byte
	_, _ = rand.Read(uuid[:])
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
}
