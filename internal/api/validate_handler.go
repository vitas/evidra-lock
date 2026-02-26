package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"samebits.com/evidra/internal/auth"
	"samebits.com/evidra/internal/engine"
	"samebits.com/evidra/internal/evidence"
	"samebits.com/evidra/pkg/invocation"
)

// handleValidate evaluates policy and returns a signed evidence record.
// Deny = HTTP 200 with decision.allow=false. Only errors produce non-200.
func handleValidate(eng *engine.Adapter, signer *evidence.Signer, builderCfg evidence.BuilderConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var inv invocation.ToolInvocation
		if err := json.NewDecoder(r.Body).Decode(&inv); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
			return
		}

		if err := validatePayloadFields(inv); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		dec, err := eng.Evaluate(r.Context(), inv)
		if err != nil {
			// ValidateStructure errors are client errors.
			if isValidationError(err) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			slog.Error("policy evaluation", "error", err)
			writeError(w, http.StatusInternalServerError, "policy evaluation failed")
			return
		}

		cfg := builderCfg
		cfg.TenantID = auth.TenantID(r.Context())

		rec, err := evidence.BuildRecord(cfg, dec, inv)
		if err != nil {
			slog.Error("build evidence record", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		payload := evidence.BuildSigningPayload(&rec)
		sig := signer.Sign([]byte(payload))
		rec.SigningPayload = payload
		rec.Signature = base64.StdEncoding.EncodeToString(sig)

		slog.Info("evaluate",
			"event_id", rec.EventID,
			"tenant_id", rec.TenantID,
			"tool", rec.Tool,
			"operation", rec.Operation,
			"allow", rec.Decision.Allow,
			"risk_level", rec.Decision.RiskLevel,
		)

		writeJSON(w, http.StatusOK, rec)
	}
}

// validatePayloadFields rejects newline characters in fields that appear
// verbatim in the signing payload. See security.md.
func validatePayloadFields(inv invocation.ToolInvocation) error {
	checks := []struct {
		name, value string
	}{
		{"actor.type", inv.Actor.Type},
		{"actor.id", inv.Actor.ID},
		{"actor.origin", inv.Actor.Origin},
		{"tool", inv.Tool},
		{"operation", inv.Operation},
		{"environment", inv.Environment},
	}
	for _, c := range checks {
		if strings.ContainsAny(c.value, "\n\r") {
			return fmt.Errorf("field %q must not contain newline characters", c.name)
		}
	}
	return nil
}

// isValidationError returns true if the error originated from ValidateStructure.
func isValidationError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "is required") ||
		strings.Contains(msg, "must be") ||
		strings.Contains(msg, "unknown")
}
