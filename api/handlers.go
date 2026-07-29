package api

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"homebase/internal/application"
	"homebase/internal/domain"
	"homebase/internal/ledger"
	"homebase/internal/promotion"
	"homebase/internal/records"
	"homebase/internal/signing"
	"homebase/internal/types"
	"homebase/internal/validation"
)

// Server binds the HTTP layer to our formal Graph Engine.
type Server struct {
	validator   *validation.Validator
	signer      *signing.Signer
	store       *ledger.Store
	recordStore *records.Store
	promotion   *promotion.Service
	bridgeKey   ed25519.PublicKey
	attemptSvc  *application.AttemptService
}

// NewServer initializes the API.
func NewServer(v *validation.Validator, s *signing.Signer, st *ledger.Store) *Server {
	// For milestone 1, we use a mock repository (non-durable) since persistence is milestone 3.
	return &Server{validator: v, signer: s, store: st, attemptSvc: application.NewAttemptService(application.NewMockAttemptRepository())}
}

// NewServerWithRecords adds the typed shared-record boundary without changing
// the legacy decision endpoint. External HTTP callers are limited to
// observations/proposals; authoritative records require an owner-specific
// authenticated path.
func NewServerWithRecords(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store) *Server {
	server := NewServer(v, s, st)
	server.recordStore = rs
	return server
}

// NewServerWithBridge adds the authenticated Bridge verification path. The
// public key is copied so callers cannot mutate the verifier after startup.
func NewServerWithBridge(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, bridgeKey ed25519.PublicKey) *Server {
	server := NewServerWithRecords(v, s, st, rs)
	server.bridgeKey = append(ed25519.PublicKey(nil), bridgeKey...)
	return server
}

// NewServerWithPromotion adds the authenticated transcript-promotion path.
// The caller must supply a service with persistent receipt keys and a real
// transport authenticator; this constructor never manufactures authority.
func NewServerWithPromotion(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, ps *promotion.Service) *Server {
	server := NewServerWithRecords(v, s, st, rs)
	server.promotion = ps
	return server
}

// NewServerWithPromotionAndBridge combines the two owner-specific typed
// ingress paths without weakening either authentication boundary.
func NewServerWithPromotionAndBridge(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, ps *promotion.Service, bridgeKey ed25519.PublicKey) *Server {
	server := NewServerWithBridge(v, s, st, rs, bridgeKey)
	server.promotion = ps
	return server
}

// HandleAppendExternalRecord is the only public ingress for Trajectory and
// other untrusted producers. HomeBase validates the complete record envelope,
// verifies its payload hash, and fsyncs it before returning success.
func (s *Server) HandleAppendExternalRecord(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.recordStore == nil {
		http.Error(w, "record store unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var raw json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil {
		http.Error(w, "invalid record JSON", http.StatusBadRequest)
		return
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "request must contain one JSON record", http.StatusBadRequest)
		return
	}
	result, err := s.recordStore.AppendExternal(raw)
	if err != nil {
		switch {
		case errors.Is(err, records.ErrAuthorityRequired):
			http.Error(w, err.Error(), http.StatusForbidden)
		case errors.Is(err, records.ErrInvalidRecord):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, records.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "failed to persist record", http.StatusInternalServerError)
		}
		return
	}
	status := http.StatusCreated
	if result.Existing {
		status = http.StatusOK
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":       result.Record.ID,
		"kind":     result.Record.Kind,
		"existing": result.Existing,
		"sequence": result.Sequence,
	})
}

// HandleAppendBridgeVerification accepts only a Bridge-signed transport
// receipt. HomeBase then derives the Claim/Proof/VerificationReceipt records,
// checks them against the pre-existing Contract and CapabilityGrant, and
// commits the complete set in one journal entry.
func (s *Server) HandleAppendBridgeVerification(w http.ResponseWriter, r *http.Request) {
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
	signature, err := decodeSignature(r.Header.Get("X-Bridge-Verification-Signature"))
	if err != nil || !ed25519.Verify(s.bridgeKey, rawCanonicalJSON(raw), signature) {
		http.Error(w, "Bridge verification signature failed", http.StatusUnauthorized)
		return
	}
	result, err := s.recordStore.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		switch {
		case errors.Is(err, records.ErrAuthorityRequired):
			http.Error(w, err.Error(), http.StatusForbidden)
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

func rawCanonicalJSON(raw []byte) []byte {
	canonical, err := records.CanonicalJSONValue(raw)
	if err != nil {
		return nil
	}
	return canonical
}

// HandlePromoteTranscript admits a transcript-derived decision only through
// the authenticated promotion service. A successful response means the
// evidence, decision, and signed receipt were committed together.
func (s *Server) HandlePromoteTranscript(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.promotion == nil {
		http.Error(w, "promotion service unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	var raw json.RawMessage
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&raw); err != nil {
		http.Error(w, "invalid promotion JSON", http.StatusBadRequest)
		return
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "request must contain one promotion case", http.StatusBadRequest)
		return
	}
	signature, err := decodeSignature(r.Header.Get("X-HomeBase-Transcript-Signature"))
	if err != nil {
		http.Error(w, "missing or malformed transcript signature", http.StatusUnauthorized)
		return
	}
	outcome, promoteErr := s.promotion.Promote(r.Context(), raw, signature)
	if promoteErr != nil {
		switch {
		case errors.Is(promoteErr, promotion.ErrUnauthenticated):
			http.Error(w, "transcript authentication failed", http.StatusUnauthorized)
		case errors.Is(promoteErr, promotion.ErrConflict):
			http.Error(w, "promotion conflicts with an existing decision", http.StatusConflict)
		case errors.Is(promoteErr, promotion.ErrInvalidPromotion):
			writeJSON(w, http.StatusUnprocessableEntity, outcome)
		default:
			http.Error(w, "promotion was not durably committed", http.StatusInternalServerError)
		}
		return
	}
	status := http.StatusCreated
	if outcome.Existing {
		status = http.StatusOK
	}
	writeJSON(w, status, outcome)
}

func decodeSignature(value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("signature is required")
	}
	if decoded, err := hex.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.StdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(value); err == nil {
		return decoded, nil
	}
	return nil, errors.New("signature is not hex or base64")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

// RecordRequest is the JSON payload Orbit or Bridge sends to HomeBase.
// We enforce the strict Assurance Case schema at the boundary.
type RecordRequest struct {
	ID         string   `json:"id"`
	Claim      string   `json:"claim"`
	Model      string   `json:"model"`
	Argument   string   `json:"argument"`
	AxiomIDs   []string `json:"axioms"`
	Evidence   string   `json:"evidence"`
	RecordedBy string   `json:"recorded_by"`
}

// HandleRecordDecision is the main entry point. It maps incoming JSON directly
// into the strict 5-move state machine.
func (s *Server) HandleRecordDecision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request body", http.StatusBadRequest)
		return
	}

	// Map generic strings to mathematically sealed AxiomIDs
	var axioms []types.AxiomID
	for _, a := range req.AxiomIDs {
		axioms = append(axioms, types.InternalNewAxiomID(a))
	}

	// 1. Lock the input into the Immutability struct (S0: PLAN)
	subject := []types.Subject{
		types.NewSubject(req.ID, map[string]string{"sha256": "placeholder"}),
	}
	assuranceCase := types.NewAssuranceCase(req.ID, subject, req.Claim, req.Model, req.Argument, axioms, req.Evidence, req.RecordedBy)

	// 2. Parse the AttemptID
	aid, err := domain.ParseAttemptID(req.ID)
	if err != nil {
		http.Error(w, "Invalid AttemptID", http.StatusBadRequest)
		return
	}

	// 3. Route through the application service
	if err := s.validator.VerifyAxiomsExist(r.Context(), axioms); err != nil {
		// Fails validation, so we propose recovery
		cmd := domain.CommandProposeRecovery{AttemptID: aid}
		if svcErr := s.attemptSvc.ExecuteCommand(r.Context(), cmd); svcErr != nil {
			http.Error(w, svcErr.Error(), http.StatusConflict)
			return
		}

		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	// Execution successful, conclude attempt
	cmd := domain.CommandConclude{AttemptID: aid}
	if svcErr := s.attemptSvc.ExecuteCommand(r.Context(), cmd); svcErr != nil {
		http.Error(w, svcErr.Error(), http.StatusConflict)
		return
	}

	// 4. Seal the decision with Ed25519 (Invariant 4)
	signed, err := s.signer.SignDecision(assuranceCase)
	if err != nil {
		http.Error(w, "Failed to cryptographically sign decision", http.StatusInternalServerError)
		return
	}

	// 5. Flush to the physical disk via fsync (Invariant 3)
	// Cast the types.DSSEEnvelope to the ledger's Goose-extracted envelope
	ledgerEnv := ledger.DSSEEnvelope{
		Payload:     signed.Payload(),
		PayloadType: signed.PayloadType(),
	}
	// copy signatures
	for _, sig := range signed.Signatures() {
		ledgerEnv.Signatures = append(ledgerEnv.Signatures, sig.Sig())
	}

	if err := s.store.Append(ledgerEnv); err != nil {
		http.Error(w, "Failed to durably write to ledger", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "SUCCESS",
		"id":     req.ID,
		"state":  "COMPLETE",
	})
}
