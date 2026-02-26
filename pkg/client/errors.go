package client

import "errors"

// Sentinel errors returned by Client.Validate.
// Use errors.Is() to check.
var (
	ErrUnreachable  = errors.New("api_unreachable") // connect refused, DNS, timeout
	ErrUnauthorized = errors.New("unauthorized")    // 401
	ErrForbidden    = errors.New("forbidden")       // 403
	ErrRateLimited  = errors.New("rate_limited")    // 429
	ErrServerError  = errors.New("server_error")    // 5xx
	ErrInvalidInput = errors.New("invalid_input")   // 422
)

// IsReachabilityError returns true for errors that can trigger fallback.
// Only ErrUnreachable and ErrServerError qualify.
// Auth errors (401/403), validation (422), and rate limit (429) always fail immediately.
func IsReachabilityError(err error) bool {
	return errors.Is(err, ErrUnreachable) || errors.Is(err, ErrServerError)
}
