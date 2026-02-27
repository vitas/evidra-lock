package api

import (
	"context"
	"net/http"
)

// handleHealthz returns 200 OK. No auth required. Phase 0+.
func handleHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// Pinger can verify database connectivity.
type Pinger interface {
	Ping(ctx context.Context) error
}

// handleReadyz returns 200 if the database is reachable, 503 otherwise.
// No auth required. Phase 1+.
func handleReadyz(db Pinger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"status": "degraded",
				"error":  "database unreachable",
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
