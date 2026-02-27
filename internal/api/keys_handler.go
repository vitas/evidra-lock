package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"samebits.com/evidra/internal/store"
)

const (
	rateLimit   = 3         // max requests per window
	rateWindow  = time.Hour // sliding window
	maxLabelLen = 128
)

// rateLimiter is a simple in-memory sliding-window rate limiter keyed by IP.
type rateLimiter struct {
	mu    sync.Mutex
	store map[string][]time.Time
}

var keyIssuanceRL = &rateLimiter{store: make(map[string][]time.Time)}

// allow returns true if the caller is within the rate limit.
func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rateWindow)

	ts := rl.store[ip]
	// Slide window: drop timestamps older than the window.
	fresh := ts[:0]
	for _, t := range ts {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}
	if len(fresh) >= rateLimit {
		rl.store[ip] = fresh
		return false
	}
	rl.store[ip] = append(fresh, now)
	return true
}

// handleKeys handles POST /v1/keys — dynamic API key issuance (Phase 1).
// When ks is nil (Phase 0), returns 501.
// inviteSecret: if non-empty, X-Invite-Token header must match.
func handleKeys(ks *store.KeyStore, inviteSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ks == nil {
			writeError(w, http.StatusNotImplemented,
				"key self-service requires a database; contact the admin to obtain an API key")
			return
		}

		// Invite gate.
		if inviteSecret != "" && r.Header.Get("X-Invite-Token") != inviteSecret {
			writeError(w, http.StatusForbidden, "invalid invite token")
			return
		}

		// Rate limit by client IP.
		ip := clientIP(r)
		if !keyIssuanceRL.allow(ip) {
			w.Header().Set("Retry-After", "3600")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded: 3 keys per hour per IP")
			return
		}

		// Parse optional label from JSON body.
		var label string
		if ct := r.Header.Get("Content-Type"); strings.Contains(ct, "application/json") {
			var body struct {
				Label string `json:"label"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err == nil {
				label = body.Label
			}
		}
		if len(label) > maxLabelLen {
			writeError(w, http.StatusBadRequest, "label exceeds 128 characters")
			return
		}

		plaintext, rec, err := ks.CreateKey(r.Context(), label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create key")
			return
		}

		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusCreated, map[string]any{
			"key":        plaintext,
			"prefix":     rec.Prefix,
			"tenant_id":  rec.TenantID,
			"created_at": rec.CreatedAt.Format(time.RFC3339),
		})
	}
}

// clientIP extracts the real client IP from X-Forwarded-For or RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) address — closest to client.
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	// Strip port from RemoteAddr.
	addr := r.RemoteAddr
	if i := strings.LastIndex(addr, ":"); i != -1 {
		return addr[:i]
	}
	return addr
}
