package api

import (
	"encoding/json"
	"net/http"

	"homebase/internal/domain"
	"homebase/internal/ledger"
	"homebase/internal/signing"
	"homebase/internal/types"
	"homebase/internal/validation"
)

// Server binds the HTTP layer to our formal Graph Engine.
type Server struct {
	validator *validation.Validator
	signer    *signing.Signer
	store     *ledger.Store
}

// NewServer initializes the API.
func NewServer(v *validation.Validator, s *signing.Signer, st *ledger.Store) *Server {
	return &Server{validator: v, signer: s, store: st}
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

	// 2. Initialize the domain state
	state := domain.AttemptState{ID: domain.AttemptID(req.ID)}

	// 3. Route through the pure reducer
	if err := s.validator.VerifyAxiomsExist(r.Context(), axioms); err != nil {
		// Fails validation, so we propose recovery
		cmd := domain.CommandProposeRecovery{AttemptID: state.ID}
		decision := domain.Decide(state, cmd)
		for _, e := range decision.Events {
			state = domain.Apply(state, e)
		}
		
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}
	
	// Execution successful, conclude attempt
	cmd := domain.CommandConclude{AttemptID: state.ID}
	decision := domain.Decide(state, cmd)
	for _, e := range decision.Events {
		state = domain.Apply(state, e)
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
