package api

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"time"

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
	validator     *validation.Validator
	signer        *signing.Signer
	store         *ledger.Store
	recordStore   *records.Store
	promotion     *promotion.Service
	bridgeKey     ed25519.PublicKey
	verifierKey   ed25519.PublicKey
	verifierKeyID string
	contractKey   ed25519.PublicKey
	// admissionPrivate signs the response to Bridge's read-only authority
	// check. Request authentication alone is insufficient: without this key,
	// a local proxy could forge a matching 200 response after the request was
	// accepted by HomeBase.
	admissionPrivate ed25519.PrivateKey
	attemptSvc       *application.AttemptService
	now              func() time.Time
}

// NewServer initializes the API.
func NewServer(v *validation.Validator, s *signing.Signer, st *ledger.Store) *Server {
	// For milestone 1, we use a mock repository (non-durable) since persistence is milestone 3.
	return &Server{validator: v, signer: s, store: st, attemptSvc: application.NewAttemptService(application.NewMockAttemptRepository()), now: time.Now}
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
	server := NewServerWithAuthorities(v, s, st, rs, ps, nil, bridgeKey)
	return server
}

// NewServerWithAuthorities wires the owner-authenticated Contract/Grant path,
// transcript promotion, and Bridge verification path together. Each public
// key is copied and an absent key leaves only that specific endpoint
// unavailable.
func NewServerWithAuthorities(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, ps *promotion.Service, contractKey, bridgeKey ed25519.PublicKey) *Server {
	server := NewServerWithBridge(v, s, st, rs, bridgeKey)
	server.promotion = ps
	server.contractKey = append(ed25519.PublicKey(nil), contractKey...)
	return server
}

// NewServerWithAuthoritiesAndAdmissionResponse adds the HomeBase-owned key
// required to authenticate successful Bridge admission responses.
func NewServerWithAuthoritiesAndAdmissionResponse(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, ps *promotion.Service, contractKey, bridgeKey ed25519.PublicKey, admissionPrivate ed25519.PrivateKey) *Server {
	server := NewServerWithAuthorities(v, s, st, rs, ps, contractKey, bridgeKey)
	server.admissionPrivate = append(ed25519.PrivateKey(nil), admissionPrivate...)
	return server
}

// NewServerWithAuthoritiesAndAdmissionResponseAndVerifier adds the enrolled
// verifier public key. The Bridge transport key authenticates the caller;
// this separate key authenticates the verifier-owned receipt itself.
func NewServerWithAuthoritiesAndAdmissionResponseAndVerifier(v *validation.Validator, s *signing.Signer, st *ledger.Store, rs *records.Store, ps *promotion.Service, contractKey, bridgeKey ed25519.PublicKey, admissionPrivate ed25519.PrivateKey, verifierPublic ed25519.PublicKey, verifierKeyID string) *Server {
	server := NewServerWithAuthoritiesAndAdmissionResponse(v, s, st, rs, ps, contractKey, bridgeKey, admissionPrivate)
	server.verifierKey = append(ed25519.PublicKey(nil), verifierPublic...)
	server.verifierKeyID = strings.TrimSpace(verifierKeyID)
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

// HandleAppendContractGrant admits the captain-approved Specification, the
// Contract, and its scoped CapabilityGrant as one owner-signed journal commit.
// Bridge and workers have no route that can create or replace any of them.
func (s *Server) HandleAppendContractGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.recordStore == nil || len(s.contractKey) != ed25519.PublicKeySize {
		http.Error(w, "Contract authority service unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var request struct {
		Specification json.RawMessage `json:"specification"`
		Contract      json.RawMessage `json:"contract"`
		Grant         json.RawMessage `json:"grant"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid Contract/Grant JSON", http.StatusBadRequest)
		return
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "request must contain one Contract/Grant bundle", http.StatusBadRequest)
		return
	}
	canonical := rawCanonicalJSON(mustMarshalAuthorityRequest(request))
	signature, err := decodeSignature(r.Header.Get("X-HomeBase-Contract-Signature"))
	if err != nil || !ed25519.Verify(s.contractKey, canonical, signature) {
		http.Error(w, "Contract authority signature failed", http.StatusUnauthorized)
		return
	}
	result, err := s.recordStore.AppendContractAndGrant(request.Specification, request.Contract, request.Grant)
	if err != nil {
		switch {
		case errors.Is(err, records.ErrInvalidRecord):
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		case errors.Is(err, records.ErrConflict):
			http.Error(w, err.Error(), http.StatusConflict)
		default:
			http.Error(w, "failed to persist Contract/Grant", http.StatusInternalServerError)
		}
		return
	}
	status := http.StatusCreated
	if result.Existing {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"contract_id": result.Contract.ID,
		"grant_id":    result.Grant.ID,
		"existing":    result.Existing,
		"sequence":    result.Sequence,
	})
}

// HandleCheckContractGrant lets Bridge prove that the owner-authenticated
// Contract + CapabilityGrant pair exists before it creates a worktree. This
// endpoint is read-only: Bridge cannot mint, replace, or extend authority.
func (s *Server) HandleCheckContractGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.recordStore == nil || len(s.bridgeKey) != ed25519.PublicKeySize || len(s.admissionPrivate) != ed25519.PrivateKeySize {
		http.Error(w, "Bridge admission service unavailable", http.StatusServiceUnavailable)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	var request struct {
		ContractID          string   `json:"contract_id"`
		GrantID             string   `json:"grant_id"`
		TaskID              string   `json:"task_id"`
		WorkerID            string   `json:"worker_id"`
		Repository          string   `json:"repository"`
		BaseCommit          string   `json:"base_commit"`
		AllowedPaths        []string `json:"allowed_paths"`
		ForbiddenPaths      []string `json:"forbidden_paths"`
		Acceptance          []string `json:"acceptance"`
		Commands            []string `json:"commands"`
		ContextHash         string   `json:"context_hash"`
		SpecificationID     string   `json:"specification_id"`
		SpecificationDigest string   `json:"specification_digest"`
		VerifierID          string   `json:"verifier_id"`
		IdempotencyKey      string   `json:"idempotency_key"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid Contract/Grant check JSON", http.StatusBadRequest)
		return
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		http.Error(w, "request must contain one Contract/Grant check", http.StatusBadRequest)
		return
	}
	canonical := rawCanonicalJSON(mustMarshalAuthorityRequest(request))
	signature, err := decodeSignature(r.Header.Get("X-Bridge-Contract-Check-Signature"))
	if err != nil || !ed25519.Verify(s.bridgeKey, canonical, signature) {
		http.Error(w, "Bridge admission signature failed", http.StatusUnauthorized)
		return
	}
	contract, err := s.recordStore.Get(request.ContractID)
	if err != nil || contract.Kind != "Contract" {
		http.Error(w, "Contract not found", http.StatusNotFound)
		return
	}
	grant, err := s.recordStore.Get(request.GrantID)
	if err != nil || grant.Kind != "CapabilityGrant" {
		http.Error(w, "CapabilityGrant not found", http.StatusNotFound)
		return
	}
	if contract.Status != "approved" || grant.Status != "active" {
		http.Error(w, "Contract/CapabilityGrant is not currently active", http.StatusPreconditionFailed)
		return
	}
	now := s.now().UTC()
	if !liveAuthorityFreshness(contract, now) || !liveAuthorityFreshness(grant, now) {
		http.Error(w, "Contract/CapabilityGrant freshness is not current", http.StatusPreconditionFailed)
		return
	}
	contractFields, err := objectFields(contract.Payload)
	if err != nil ||
		!sameStringField(contractFields, "task_id", request.TaskID) ||
		!sameStringField(contractFields, "worker_id", request.WorkerID) ||
		!sameStringField(contractFields, "repository", request.Repository) ||
		!sameStringField(contractFields, "base_commit", request.BaseCommit) ||
		!sameStringField(contractFields, "context_hash", request.ContextHash) ||
		!sameStringField(contractFields, "specification_id", request.SpecificationID) ||
		!sameStringField(contractFields, "specification_digest", request.SpecificationDigest) ||
		!sameStringField(contractFields, "verifier_id", request.VerifierID) ||
		!sameStringField(contractFields, "idempotency_key", request.IdempotencyKey) ||
		!sameStringArray(contractFields, "allowed_paths", request.AllowedPaths) ||
		!sameStringArray(contractFields, "forbidden_paths", request.ForbiddenPaths) ||
		!sameStringArray(contractFields, "acceptance", request.Acceptance) {
		http.Error(w, "Contract scope does not match request", http.StatusPreconditionFailed)
		return
	}
	grantFields, err := objectFields(grant.Payload)
	if err != nil ||
		!sameStringField(grantFields, "contract_id", request.ContractID) ||
		!sameStringField(grantFields, "task_id", request.TaskID) ||
		!sameStringField(grantFields, "worker_id", request.WorkerID) ||
		!sameStringField(grantFields, "context_hash", request.ContextHash) ||
		!sameStringField(grantFields, "specification_id", request.SpecificationID) ||
		!sameStringField(grantFields, "specification_digest", request.SpecificationDigest) ||
		!sameStringField(grantFields, "idempotency_key", request.IdempotencyKey) ||
		!sameStringArray(grantFields, "allowed_paths", request.AllowedPaths) ||
		!sameStringArray(grantFields, "commands", request.Commands) {
		http.Error(w, "CapabilityGrant scope does not match request", http.StatusPreconditionFailed)
		return
	}
	contextValidUntil, ok := stringValue(contractFields["context_valid_until"])
	if !ok {
		http.Error(w, "Contract context expiry is invalid", http.StatusPreconditionFailed)
		return
	}
	contextExpiry, err := time.Parse("2006-01-02T15:04:05Z", contextValidUntil)
	if err != nil || !s.now().UTC().Before(contextExpiry) {
		http.Error(w, "Contract context is expired", http.StatusPreconditionFailed)
		return
	}
	expiresAt, ok := stringValue(grantFields["expires_at"])
	if !ok {
		http.Error(w, "CapabilityGrant expiry is invalid", http.StatusPreconditionFailed)
		return
	}
	expires, err := time.Parse("2006-01-02T15:04:05Z", expiresAt)
	if err != nil || !s.now().UTC().Before(expires) {
		http.Error(w, "CapabilityGrant is expired", http.StatusPreconditionFailed)
		return
	}
	canonicalRequest := rawCanonicalJSON(mustMarshalAuthorityRequest(request))
	requestHash := sha256.Sum256(canonicalRequest)
	validUntil := expiresAt
	if contextExpiry.Before(expires) {
		validUntil = contextExpiry.Format("2006-01-02T15:04:05Z")
	}
	response := map[string]any{
		"contract_id":  contract.ID,
		"grant_id":     grant.ID,
		"task_id":      request.TaskID,
		"worker_id":    request.WorkerID,
		"context_hash": contractFields["context_hash"],
		"request_hash": hex.EncodeToString(requestHash[:]),
		"valid_until":  validUntil,
	}
	rawResponse, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "failed to encode admission response", http.StatusInternalServerError)
		return
	}
	canonicalResponse, err := records.CanonicalJSONValue(rawResponse)
	if err != nil {
		http.Error(w, "failed to canonicalize admission response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-HomeBase-Admission-Signature", hex.EncodeToString(ed25519.Sign(s.admissionPrivate, canonicalResponse)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(rawResponse)
	_, _ = w.Write([]byte("\n"))
}

func liveAuthorityFreshness(record records.Record, now time.Time) bool {
	if record.Freshness.Mode != "time_bound" || record.Freshness.ValidUntil == nil {
		return false
	}
	expires, err := time.Parse("2006-01-02T15:04:05Z", *record.Freshness.ValidUntil)
	return err == nil && now.Before(expires)
}

func mustMarshalAuthorityRequest(value any) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func objectFields(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("expected JSON object")
	}
	return fields, nil
}

func sameStringField(fields map[string]json.RawMessage, key, want string) bool {
	got, ok := stringValue(fields[key])
	return ok && got == want
}

func sameStringArray(fields map[string]json.RawMessage, key string, want []string) bool {
	var got []string
	if err := json.Unmarshal(fields[key], &got); err != nil {
		return false
	}
	return reflect.DeepEqual(got, want)
}

func stringValue(raw json.RawMessage) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", false
	}
	return value, true
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
	if err := records.VerifyBridgeReceiptAttestation(raw, s.verifierKey, s.verifierKeyID); err != nil {
		http.Error(w, "verifier attestation failed: "+err.Error(), http.StatusUnauthorized)
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
