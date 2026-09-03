package api

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"homebase/internal/records"
)

// HandleAppendBridgeVerificationWithAuthority returns the production Bridge
// verification handler bound to one StoreVerifierAuthority. The same opaque,
// Store-bound authority performs both HTTP preflight verification and durable
// Store admission, so production cannot drift between two verifier trust roots.
func (s *Server) HandleAppendBridgeVerificationWithAuthority(authority records.StoreVerifierAuthority) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if s.recordStore == nil || len(s.bridgeKey) != ed25519.PublicKeySize {
			http.Error(w, "Bridge verification service unavailable", http.StatusServiceUnavailable)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
		var raw json.RawMessage
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&raw); err != nil {
			http.Error(w, "invalid Bridge verification JSON", http.StatusBadRequest)
			return
		}
		var extra json.RawMessage
		if err := decoder.Decode(&extra); err != io.EOF {
			http.Error(w, "request must contain one Bridge verification receipt", http.StatusBadRequest)
			return
		}
		var envelope struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || strings.TrimSpace(envelope.ID) == "" {
			http.Error(w, "Bridge verification receipt id is required", http.StatusBadRequest)
			return
		}
		idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
		if idempotencyKey == "" {
			http.Error(w, "Idempotency-Key is required", http.StatusBadRequest)
			return
		}
		if idempotencyKey != envelope.ID {
			http.Error(w, "Idempotency-Key does not match receipt id", http.StatusConflict)
			return
		}
		signature, err := decodeSignature(r.Header.Get("X-Bridge-Verification-Signature"))
		if err != nil || !ed25519.Verify(s.bridgeKey, rawCanonicalJSON(raw), signature) {
			http.Error(w, "Bridge verification signature failed", http.StatusUnauthorized)
			return
		}
		if err := authority.Verify(raw, s.now().UTC()); err != nil {
			if errors.Is(err, records.ErrAuthorityRequired) {
				http.Error(w, "verifier authority unavailable: "+err.Error(), http.StatusServiceUnavailable)
			} else {
				http.Error(w, "verifier attestation failed: "+err.Error(), http.StatusUnauthorized)
			}
			return
		}
		result, err := s.recordStore.AppendBridgeVerificationSubmissionAuthorized(raw, authority)
		if err != nil {
			switch {
			case errors.Is(err, records.ErrAuthorityRequired):
				http.Error(w, "verifier authority unavailable: "+err.Error(), http.StatusServiceUnavailable)
			case errors.Is(err, records.ErrInvalidRecord):
				http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			case errors.Is(err, records.ErrConflict):
				http.Error(w, err.Error(), http.StatusConflict)
			default:
				http.Error(w, "failed to persist Bridge verification", http.StatusInternalServerError)
			}
			return
		}
		status := http.StatusCreated
		if result.Existing {
			status = http.StatusOK
		}
		writeJSON(w, status, map[string]any{
			"receipt_id":   result.Receipt.ID,
			"existing":     result.Existing,
			"sequence":     result.Sequence,
			"record_count": len(result.Records),
		})
	}
}
