package api

import (
	"log/slog"
	"net/http"

	"samebits.com/evidra/internal/evidence"
)

// handlePubkey returns the Ed25519 public key in PEM format. No auth required.
func handlePubkey(signer *evidence.Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pem, err := signer.PublicKeyPEM()
		if err != nil {
			slog.Error("pubkey handler", "error", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}
		w.Header().Set("Content-Type", "application/x-pem-file")
		w.WriteHeader(http.StatusOK)
		w.Write(pem)
	}
}
