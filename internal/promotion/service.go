// Package promotion owns the only path that can turn an authenticated
// transcript-derived proposition into a durable HomeBase Decision.
//
// The contract is deliberately boring: strict JSON, content hashes, exact
// source spans, explicit approval language, an authenticated principal, a
// durable nonce, and one atomic records-journal commit. LLM output may propose
// a case, but it cannot satisfy this package's authority checks by itself.
package promotion

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"homebase/internal/records"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidPromotion = errors.New("invalid transcript promotion")
	ErrUnauthenticated  = errors.New("transcript authentication failed")
	ErrReplay           = errors.New("transcript approval replay")
	ErrConflict         = errors.New("transcript promotion conflicts with an existing decision")
)

var (
	sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
	datePattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
	negated       = regexp.MustCompile(`\b(do not|don't|not)\s+approve\b`)
)

// VerifyFunc verifies the authenticated transport/session signature over the
// canonical transcript digest. The auth.verified JSON field is metadata and
// is never accepted as proof by this package.
type VerifyFunc func(context.Context, string, string, []byte) error

// Ed25519Verifier binds one authenticated principal to one public key. The
// signed message is the canonical transcript digest, so a signature cannot be
// replayed against a different transcript without changing the digest.
func Ed25519Verifier(principal string, publicKey ed25519.PublicKey) VerifyFunc {
	key := append(ed25519.PublicKey(nil), publicKey...)
	return func(_ context.Context, claimedPrincipal, transcriptDigest string, signature []byte) error {
		if claimedPrincipal != principal || len(key) != ed25519.PublicKeySize || !ed25519.Verify(key, []byte(transcriptDigest), signature) {
			return errors.New("principal or signature mismatch")
		}
		return nil
	}
}

type Auth struct {
	Principal string `json:"principal"`
	Method    string `json:"method"`
	Verified  bool   `json:"verified"`
}

type Turn struct {
	TurnID      string `json:"turn_id"`
	Sequence    int    `json:"sequence"`
	Role        string `json:"role"`
	Content     string `json:"content"`
	ContentHash string `json:"content_hash"`
}

type Transcript struct {
	SourceID     string `json:"source_id"`
	SessionID    string `json:"session_id"`
	CapturedAt   string `json:"captured_at"`
	ContentHash  string `json:"content_hash"`
	Completeness string `json:"completeness"`
	Auth         Auth   `json:"auth"`
	Turns        []Turn `json:"turns"`
}

type Span struct {
	TurnID    string `json:"turn_id"`
	Start     int    `json:"start"`
	End       int    `json:"end"`
	QuoteHash string `json:"quote_hash"`
}

type Candidate struct {
	DecisionID      string `json:"decision_id"`
	Proposition     string `json:"proposition"`
	Scope           string `json:"scope"`
	OptionID        string `json:"option_id"`
	SourceSpans     []Span `json:"source_spans"`
	ContextDigest   string `json:"context_digest"`
	PropositionHash string `json:"proposition_hash"`
	ApprovalID      string `json:"approval_id"`
}

type Confirmation struct {
	DecisionID  string `json:"decision_id"`
	OptionID    string `json:"option_id"`
	Scope       string `json:"scope"`
	Proposition string `json:"proposition"`
}

type Approval struct {
	ApprovalID      string       `json:"approval_id"`
	DecisionID      string       `json:"decision_id"`
	Principal       string       `json:"principal"`
	OptionID        string       `json:"option_id"`
	PropositionHash string       `json:"proposition_hash"`
	ContextDigest   string       `json:"context_digest"`
	ApprovedAt      string       `json:"approved_at"`
	ExpiresAt       string       `json:"expires_at"`
	Nonce           string       `json:"nonce"`
	SourceTurnID    string       `json:"source_turn_id"`
	SourceSpan      Span         `json:"source_span"`
	Confirmation    Confirmation `json:"confirmation"`
}

type Case struct {
	Kind       string     `json:"kind"`
	Version    string     `json:"version"`
	CaseID     string     `json:"case_id"`
	Transcript Transcript `json:"transcript"`
	Candidate  Candidate  `json:"candidate"`
	Approvals  []Approval `json:"approvals"`
}

// Receipt is signed by HomeBase and is the durable proof that one decision was
// admitted through this exact promotion boundary.
type Receipt struct {
	Version          string `json:"version"`
	ReceiptID        string `json:"receipt_id"`
	CaseID           string `json:"case_id"`
	DecisionID       string `json:"decision_id"`
	EvidenceID       string `json:"evidence_id"`
	Principal        string `json:"principal"`
	TranscriptDigest string `json:"transcript_digest"`
	PropositionHash  string `json:"proposition_hash"`
	ContextDigest    string `json:"context_digest"`
	ApprovalID       string `json:"approval_id"`
	Nonce            string `json:"nonce"`
	EvaluatedAt      string `json:"evaluated_at"`
	DecisionHash     string `json:"decision_hash"`
	EvidenceHash     string `json:"evidence_hash"`
	Signature        string `json:"signature"`
}

type Outcome struct {
	ContractVersion string   `json:"contract_version"`
	CaseID          string   `json:"case_id"`
	DecisionID      string   `json:"decision_id"`
	Accepted        bool     `json:"accepted"`
	Existing        bool     `json:"existing"`
	Errors          []string `json:"errors"`
	EvaluatedAt     string   `json:"evaluated_at"`
	Receipt         *Receipt `json:"receipt,omitempty"`
}

type Service struct {
	records     *records.Store
	verify      VerifyFunc
	receiptPriv ed25519.PrivateKey
	receiptPub  ed25519.PublicKey
	now         func() time.Time
	mu          sync.Mutex
	byNonce     map[string]Receipt
}

func NewService(store *records.Store, verify VerifyFunc, receiptPrivate ed25519.PrivateKey, now func() time.Time) (*Service, error) {
	if store == nil || verify == nil || len(receiptPrivate) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: records store, verifier, and receipt private key are required", ErrInvalidPromotion)
	}
	if now == nil {
		now = time.Now
	}
	service := &Service{
		records:     store,
		verify:      verify,
		receiptPriv: append(ed25519.PrivateKey(nil), receiptPrivate...),
		receiptPub:  append(ed25519.PublicKey(nil), receiptPrivate.Public().(ed25519.PublicKey)...),
		now:         now,
		byNonce:     make(map[string]Receipt),
	}
	for _, commit := range store.ListPromotionCommits() {
		receipt, err := decodeReceipt(commit.Receipt)
		if err != nil {
			return nil, fmt.Errorf("%w: replay receipt: %v", ErrInvalidPromotion, err)
		}
		if err := service.verifyReceipt(receipt, commit.Decision, commit.Evidence); err != nil {
			return nil, fmt.Errorf("%w: replay receipt: %v", ErrInvalidPromotion, err)
		}
		key := nonceKey(receipt.Principal, receipt.Nonce)
		if previous, ok := service.byNonce[key]; ok && previous.ReceiptID != receipt.ReceiptID {
			return nil, fmt.Errorf("%w: duplicate durable nonce", ErrInvalidPromotion)
		}
		service.byNonce[key] = receipt
	}
	return service, nil
}

func (s *Service) Promote(ctx context.Context, rawCase, detachedSignature []byte) (Outcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now().UTC()
	caseValue, errorsFound := validateCase(rawCase, now)
	outcome := Outcome{ContractVersion: "transcript-promotion-v1", CaseID: caseValue.CaseID, DecisionID: caseValue.Candidate.DecisionID, EvaluatedAt: formatTime(now), Errors: errorsFound}
	if len(errorsFound) != 0 {
		return outcome, fmt.Errorf("%w: %s", ErrInvalidPromotion, strings.Join(errorsFound, ","))
	}
	transcriptDigest, err := digestValue(transcriptDigestValue(caseValue.Transcript))
	if err != nil {
		return outcome, fmt.Errorf("%w: transcript digest: %v", ErrInvalidPromotion, err)
	}
	if err := s.verify(ctx, caseValue.Transcript.Auth.Principal, transcriptDigest, detachedSignature); err != nil {
		outcome.Errors = []string{"authentication_verifier_rejected"}
		return outcome, fmt.Errorf("%w: %v", ErrUnauthenticated, err)
	}
	approval := caseValue.Approvals[0]
	key := nonceKey(approval.Principal, approval.Nonce)
	if previous, ok := s.byNonce[key]; ok {
		if previous.DecisionID == caseValue.Candidate.DecisionID && previous.PropositionHash == caseValue.Candidate.PropositionHash && previous.ContextDigest == caseValue.Candidate.ContextDigest {
			outcome.Accepted = true
			outcome.Existing = true
			outcome.Receipt = &previous
			return outcome, nil
		}
		outcome.Errors = []string{"replayed_approval"}
		return outcome, fmt.Errorf("%w: nonce %s", ErrConflict, key)
	}

	evidenceRaw, evidence, err := buildEvidence(caseValue, transcriptDigest)
	if err != nil {
		return outcome, fmt.Errorf("%w: evidence: %v", ErrInvalidPromotion, err)
	}
	decisionRaw, decision, err := buildDecision(caseValue, evidence, approval)
	if err != nil {
		return outcome, fmt.Errorf("%w: decision: %v", ErrInvalidPromotion, err)
	}
	receipt := Receipt{
		Version:          "1",
		ReceiptID:        "receipt:promotion:" + approval.ApprovalID + ":" + approval.Nonce,
		CaseID:           caseValue.CaseID,
		DecisionID:       decision.ID,
		EvidenceID:       evidence.ID,
		Principal:        approval.Principal,
		TranscriptDigest: transcriptDigest,
		PropositionHash:  caseValue.Candidate.PropositionHash,
		ContextDigest:    caseValue.Candidate.ContextDigest,
		ApprovalID:       approval.ApprovalID,
		Nonce:            approval.Nonce,
		EvaluatedAt:      formatTime(now),
		DecisionHash:     decision.ContentHash,
		EvidenceHash:     evidence.ContentHash,
	}
	if err := s.signReceipt(&receipt); err != nil {
		return outcome, fmt.Errorf("%w: sign receipt: %v", ErrInvalidPromotion, err)
	}
	receiptRaw, err := json.Marshal(receipt)
	if err != nil {
		return outcome, fmt.Errorf("%w: encode receipt: %v", ErrInvalidPromotion, err)
	}
	commit, err := s.records.AppendPromotionCommit(decisionRaw, evidenceRaw, receiptRaw)
	if err != nil {
		if errors.Is(err, records.ErrConflict) {
			return outcome, fmt.Errorf("%w: %v", ErrConflict, err)
		}
		return outcome, err
	}
	s.byNonce[key] = receipt
	outcome.Accepted = true
	outcome.Existing = commit.Existing
	outcome.Receipt = &receipt
	return outcome, nil
}

func (s *Service) signReceipt(receipt *Receipt) error {
	unsigned, err := canonicalReceipt(*receipt)
	if err != nil {
		return err
	}
	signature := ed25519.Sign(s.receiptPriv, unsigned)
	receipt.Signature = hex.EncodeToString(signature)
	return nil
}

func (s *Service) verifyReceipt(receipt Receipt, decision, evidence records.Record) error {
	if receipt.Version != "1" || receipt.ReceiptID == "" || receipt.CaseID == "" || receipt.Principal == "" || receipt.DecisionID != decision.ID || receipt.EvidenceID != evidence.ID || receipt.DecisionHash != decision.ContentHash || receipt.EvidenceHash != evidence.ContentHash || !validHash(receipt.TranscriptDigest) || !validHash(receipt.PropositionHash) || !validHash(receipt.ContextDigest) || !validHash(receipt.DecisionHash) || !validHash(receipt.EvidenceHash) || receipt.ApprovalID == "" || receipt.Nonce == "" || !validTime(receipt.EvaluatedAt) || receipt.Signature == "" {
		return errors.New("receipt does not bind its committed records")
	}
	signature, err := hex.DecodeString(receipt.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return errors.New("receipt signature is not valid hex")
	}
	unsigned, err := canonicalReceipt(receipt)
	if err != nil {
		return err
	}
	if !ed25519.Verify(s.receiptPub, unsigned, signature) {
		return errors.New("receipt signature did not verify")
	}
	return nil
}

func validateCase(raw []byte, now time.Time) (Case, []string) {
	var value Case
	var found []string
	root, err := strictFields(raw)
	if err != nil {
		return value, []string{"invalid_case_fields"}
	}
	checkExact(root, []string{"kind", "version", "case_id", "transcript", "candidate", "approvals", "expected"}, []string{"kind", "version", "case_id", "transcript", "candidate", "approvals"}, &found, "case")
	if err := json.Unmarshal(raw, &value); err != nil {
		return value, []string{"invalid_case_types"}
	}
	if value.Kind != "TranscriptPromotionCase" {
		found = append(found, "invalid_kind")
	}
	if value.Version != "1" {
		found = append(found, "unsupported_version")
	}
	if value.CaseID == "" {
		found = append(found, "invalid_case_id")
	}

	transcriptRaw, ok := root["transcript"]
	transcriptFields, transcriptErr := strictFields(transcriptRaw)
	if !ok || transcriptErr != nil {
		found = append(found, "invalid_transcript_fields")
	} else {
		checkExact(transcriptFields, []string{"source_id", "session_id", "captured_at", "content_hash", "completeness", "auth", "turns"}, []string{"source_id", "session_id", "captured_at", "content_hash", "completeness", "auth", "turns"}, &found, "transcript")
		if value.Transcript.SourceID == "" || value.Transcript.SessionID == "" {
			found = append(found, "invalid_transcript_identity")
		}
		if !validTime(value.Transcript.CapturedAt) {
			found = append(found, "invalid_transcript_captured_at")
		}
		if !validHash(value.Transcript.ContentHash) {
			found = append(found, "invalid_transcript_content_hash")
		}
		if value.Transcript.Completeness != "complete" {
			found = append(found, "incomplete_transcript")
		}
		if value.Transcript.Auth.Principal == "" || value.Transcript.Auth.Method != "authenticated_session" || !value.Transcript.Auth.Verified {
			found = append(found, "approval_source_not_authenticated")
		}
		if !hasExactAuthFields(transcriptFields["auth"]) {
			found = append(found, "invalid_transcript_auth")
		}
		turnsRaw, turnsErr := arrayRaw(transcriptFields["turns"])
		if turnsErr != nil || len(value.Transcript.Turns) == 0 || len(turnsRaw) != len(value.Transcript.Turns) {
			found = append(found, "invalid_transcript_turns")
		} else {
			turnMap := make(map[string]Turn, len(value.Transcript.Turns))
			sequences := make(map[int]bool, len(value.Transcript.Turns))
			for i, turn := range value.Transcript.Turns {
				fields, fieldErr := strictFields(turnsRaw[i])
				if fieldErr != nil {
					found = append(found, "invalid_turn_fields")
					continue
				}
				checkExact(fields, []string{"turn_id", "sequence", "role", "content", "content_hash"}, []string{"turn_id", "sequence", "role", "content", "content_hash"}, &found, "turn")
				if turn.TurnID == "" || turn.Sequence < 1 || sequences[turn.Sequence] || turn.ContentHash == "" || !validHash(turn.ContentHash) {
					found = append(found, "invalid_turn")
				}
				if turn.Role != "user" && turn.Role != "assistant" && turn.Role != "tool" && turn.Role != "system" {
					found = append(found, "invalid_turn_role")
				}
				if sha256Text(turn.Content) != turn.ContentHash {
					found = append(found, "turn_content_hash_mismatch")
				}
				if _, exists := turnMap[turn.TurnID]; exists {
					found = append(found, "duplicate_turn_id")
				}
				turnMap[turn.TurnID] = turn
				sequences[turn.Sequence] = true
				if i > 0 && value.Transcript.Turns[i-1].Sequence >= turn.Sequence {
					found = append(found, "turn_order_mismatch")
				}
			}
			for sequence := 1; sequence <= len(value.Transcript.Turns); sequence++ {
				if !sequences[sequence] {
					found = append(found, "non_contiguous_turn_sequence")
				}
			}
			if expected, digestErr := digestValue(transcriptDigestValue(value.Transcript)); digestErr != nil || expected != value.Transcript.ContentHash {
				found = append(found, "transcript_content_hash_mismatch")
			}
		}
	}

	candidateRaw, candidateOK := root["candidate"]
	candidateFields, candidateErr := strictFields(candidateRaw)
	if !candidateOK || candidateErr != nil {
		found = append(found, "invalid_candidate_fields")
	} else {
		checkExact(candidateFields, []string{"decision_id", "proposition", "scope", "option_id", "source_spans", "context_digest", "proposition_hash", "approval_id"}, []string{"decision_id", "proposition", "scope", "option_id", "source_spans", "context_digest", "proposition_hash", "approval_id"}, &found, "candidate")
		if value.Candidate.DecisionID == "" || value.Candidate.Proposition == "" || value.Candidate.Scope == "" || value.Candidate.OptionID == "" || value.Candidate.ApprovalID == "" || !validHash(value.Candidate.ContextDigest) || !validHash(value.Candidate.PropositionHash) {
			found = append(found, "invalid_candidate")
		}
		propositionHash, _ := digestValue(map[string]any{"decision_id": value.Candidate.DecisionID, "proposition": value.Candidate.Proposition, "scope": value.Candidate.Scope, "option_id": value.Candidate.OptionID})
		if propositionHash != value.Candidate.PropositionHash {
			found = append(found, "candidate_proposition_hash_mismatch")
		}
		contextDigest, _ := digestValue(map[string]any{"contract_version": "transcript-promotion-v1", "decision_id": value.Candidate.DecisionID, "transcript_hash": value.Transcript.ContentHash})
		if contextDigest != value.Candidate.ContextDigest {
			found = append(found, "candidate_context_digest_mismatch")
		}
		spansRaw, spansErr := arrayRaw(candidateFields["source_spans"])
		if spansErr != nil || len(value.Candidate.SourceSpans) == 0 || len(spansRaw) != len(value.Candidate.SourceSpans) {
			found = append(found, "invalid_candidate_source_spans")
		} else {
			turnMap := make(map[string]Turn, len(value.Transcript.Turns))
			for _, turn := range value.Transcript.Turns {
				turnMap[turn.TurnID] = turn
			}
			quotes := make([]string, 0, len(value.Candidate.SourceSpans))
			for i, span := range value.Candidate.SourceSpans {
				quote, spanErrors := validateSpan(span, spansRaw[i], turnMap, fmt.Sprintf("candidate_span_%d", i))
				found = append(found, spanErrors...)
				if quote != "" {
					quotes = append(quotes, quote)
				}
			}
			contained := false
			for _, quote := range quotes {
				if strings.Contains(strings.ToLower(quote), strings.ToLower(value.Candidate.Proposition)) {
					contained = true
				}
			}
			if !contained {
				found = append(found, "candidate_source_not_contain_proposition")
			}
		}
	}

	approvalsRaw, approvalsErr := arrayRaw(root["approvals"])
	if approvalsErr != nil || len(value.Approvals) != 1 || len(approvalsRaw) != len(value.Approvals) {
		found = append(found, "invalid_approvals")
	} else {
		approval := value.Approvals[0]
		fields, fieldErr := strictFields(approvalsRaw[0])
		if fieldErr != nil {
			found = append(found, "invalid_approval_0_fields")
		} else {
			checkExact(fields, []string{"approval_id", "decision_id", "principal", "option_id", "proposition_hash", "context_digest", "approved_at", "expires_at", "nonce", "source_turn_id", "source_span", "confirmation"}, []string{"approval_id", "decision_id", "principal", "option_id", "proposition_hash", "context_digest", "approved_at", "expires_at", "nonce", "source_turn_id", "source_span", "confirmation"}, &found, "approval")
			approvedAt, approvedOK := parseContractTime(approval.ApprovedAt)
			expiresAt, expiresOK := parseContractTime(approval.ExpiresAt)
			if approval.ApprovalID == "" || approval.DecisionID == "" || approval.Principal == "" || approval.OptionID == "" || approval.Nonce == "" || approval.SourceTurnID == "" || !validHash(approval.PropositionHash) || !validHash(approval.ContextDigest) {
				found = append(found, "invalid_approval")
			}
			if approvedOK && approvedAt.After(now) {
				found = append(found, "future_approval")
			}
			if expiresOK && !expiresAt.After(now) {
				found = append(found, "stale_approval")
			}
			if approvedOK && expiresOK && !expiresAt.After(approvedAt) {
				found = append(found, "approval_expiry_before_approval")
			}
			turnMap := make(map[string]Turn, len(value.Transcript.Turns))
			for _, turn := range value.Transcript.Turns {
				turnMap[turn.TurnID] = turn
			}
			sourceTurn, sourceOK := turnMap[approval.SourceTurnID]
			if !sourceOK {
				found = append(found, "approval_source_turn_missing")
			} else if sourceTurn.Role != "user" {
				found = append(found, "untrusted_turn_cannot_authorize")
			}
			spanRaw := fields["source_span"]
			if approval.SourceSpan.TurnID != approval.SourceTurnID {
				found = append(found, "approval_source_turn_mismatch")
			}
			quote, spanErrors := validateSpan(approval.SourceSpan, spanRaw, turnMap, "approval_0_source_span")
			found = append(found, spanErrors...)
			if sourceOK {
				expectedNonce, nonceErr := digestValue(map[string]any{"contract_version": "transcript-promotion-v1", "source_id": value.Transcript.SourceID, "session_id": value.Transcript.SessionID, "source_turn_id": approval.SourceTurnID, "source_span": approval.SourceSpan, "source_turn_content_hash": sourceTurn.ContentHash})
				if nonceErr != nil || expectedNonce != approval.Nonce {
					found = append(found, "nonce_not_bound_to_source_event")
				}
			}
			if quote != "" {
				normalized := strings.ToLower(quote)
				if negated.MatchString(normalized) {
					found = append(found, "negated_approval")
				} else if !strings.Contains(normalized, "approve") {
					found = append(found, "ambiguous_approval")
				}
				prefix := regexp.MustCompile(`^\s*approve\s+decision\s+` + regexp.QuoteMeta(strings.ToLower(value.Candidate.DecisionID)) + `\s*,\s*option\s+` + regexp.QuoteMeta(strings.ToLower(value.Candidate.OptionID)) + `(?:\b|,)`)
				if !prefix.MatchString(normalized) || !strings.Contains(normalized, strings.ToLower(value.Candidate.Scope)) || !strings.Contains(normalized, strings.ToLower(value.Candidate.Proposition)) {
					found = append(found, "approval_source_not_explicit")
				}
			}
			if approval.Confirmation.DecisionID != value.Candidate.DecisionID || approval.Confirmation.OptionID != value.Candidate.OptionID || approval.Confirmation.Scope != value.Candidate.Scope || approval.Confirmation.Proposition != value.Candidate.Proposition {
				found = append(found, "approval_confirmation_mismatch")
			}
			if approval.ApprovalID != value.Candidate.ApprovalID || approval.DecisionID != value.Candidate.DecisionID || approval.OptionID != value.Candidate.OptionID || approval.PropositionHash != value.Candidate.PropositionHash || approval.ContextDigest != value.Candidate.ContextDigest {
				found = append(found, "approval_candidate_mismatch")
			}
			if approval.Principal != value.Transcript.Auth.Principal {
				found = append(found, "approval_principal_mismatch")
			}
		}
	}
	return value, uniqueSorted(found)
}

func buildEvidence(value Case, transcriptDigest string) ([]byte, records.Record, error) {
	payload := map[string]any{
		"evidence_type":   "authenticated-transcript-promotion",
		"subject_refs":    []any{map[string]any{"kind": "trajectory_session", "id": value.Transcript.SessionID}},
		"observed_digest": transcriptDigest,
	}
	contentHash, err := records.CanonicalContentHash(mustJSON(payload))
	if err != nil {
		return nil, records.Record{}, err
	}
	evidence := map[string]any{
		"kind": "Evidence", "version": "1", "id": "evidence:promotion:" + value.CaseID + ":" + transcriptDigest,
		"source_refs":  []any{map[string]any{"kind": "trajectory_session", "id": value.Transcript.SessionID, "content_hash": transcriptDigest}},
		"content_hash": contentHash, "captured_at": value.Transcript.CapturedAt,
		"authority_class": records.AuthorityUntrustedText, "freshness": map[string]any{"mode": "immutable", "valid_until": nil, "reason": "authenticated transcript is retained as evidence; interpretation remains untrusted"},
		"status": "observed", "source": map[string]any{"id": value.Transcript.SourceID, "role": "trajectory"}, "payload": payload,
	}
	raw := mustJSON(evidence)
	parsed, err := parseRecord(raw)
	return raw, parsed, err
}

func buildDecision(value Case, evidence records.Record, approval Approval) ([]byte, records.Record, error) {
	payload := map[string]any{"decision": value.Candidate.Proposition, "scope": value.Candidate.Scope, "decided_by": approval.Principal}
	contentHash, err := records.CanonicalContentHash(mustJSON(payload))
	if err != nil {
		return nil, records.Record{}, err
	}
	decision := map[string]any{
		"kind": "Decision", "version": "1", "id": value.Candidate.DecisionID,
		"source_refs":  []any{map[string]any{"kind": "evidence", "id": evidence.ID, "content_hash": evidence.ContentHash}},
		"content_hash": contentHash, "captured_at": approval.ApprovedAt,
		"authority_class": records.AuthorityHumanDecision, "freshness": map[string]any{"mode": "time_bound", "valid_until": approval.ExpiresAt},
		"status": "approved", "source": map[string]any{"id": approval.Principal, "role": "captain"}, "payload": payload,
	}
	raw := mustJSON(decision)
	parsed, err := parseRecord(raw)
	return raw, parsed, err
}

func parseRecord(raw []byte) (records.Record, error) {
	// The records package is the schema authority; append is intentionally not
	// performed here because promotion must build both records before commit.
	value, err := records.DecodeStrictJSON(raw)
	if err != nil {
		return records.Record{}, err
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return records.Record{}, err
	}
	// Validate by using a temporary in-memory journal/store would obscure the
	// atomicity boundary. The final AppendPromotionCommit repeats full validation
	// and returns the authoritative typed records.
	var result records.Record
	if err := json.Unmarshal(encoded, &result); err != nil {
		return records.Record{}, err
	}
	return result, nil
}

func canonicalReceipt(receipt Receipt) ([]byte, error) {
	value := map[string]any{
		"version": receipt.Version, "receipt_id": receipt.ReceiptID, "case_id": receipt.CaseID, "decision_id": receipt.DecisionID, "evidence_id": receipt.EvidenceID,
		"principal": receipt.Principal, "transcript_digest": receipt.TranscriptDigest, "proposition_hash": receipt.PropositionHash, "context_digest": receipt.ContextDigest,
		"approval_id": receipt.ApprovalID, "nonce": receipt.Nonce, "evaluated_at": receipt.EvaluatedAt, "decision_hash": receipt.DecisionHash, "evidence_hash": receipt.EvidenceHash,
	}
	return records.CanonicalJSONValue(mustJSON(value))
}

func decodeReceipt(raw []byte) (Receipt, error) {
	fields, err := strictFields(raw)
	if err != nil {
		return Receipt{}, err
	}
	required := []string{"version", "receipt_id", "case_id", "decision_id", "evidence_id", "principal", "transcript_digest", "proposition_hash", "context_digest", "approval_id", "nonce", "evaluated_at", "decision_hash", "evidence_hash", "signature"}
	allowed := make(map[string]bool, len(required))
	for _, key := range required {
		allowed[key] = true
		if _, ok := fields[key]; !ok {
			return Receipt{}, fmt.Errorf("receipt missing %q", key)
		}
	}
	for key := range fields {
		if !allowed[key] {
			return Receipt{}, fmt.Errorf("receipt has unknown field %q", key)
		}
	}
	var receipt Receipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return Receipt{}, err
	}
	return receipt, nil
}

func validateSpan(span Span, raw []byte, turns map[string]Turn, name string) (string, []string) {
	var found []string
	fields, err := strictFields(raw)
	if err != nil {
		return "", []string{"invalid_" + name}
	}
	checkExact(fields, []string{"turn_id", "start", "end", "quote_hash"}, []string{"turn_id", "start", "end", "quote_hash"}, &found, name)
	turn, ok := turns[span.TurnID]
	if span.TurnID == "" || span.Start < 0 || span.End <= span.Start || !validHash(span.QuoteHash) {
		found = append(found, "invalid_"+name)
	}
	if !ok {
		return "", append(found, "unknown_"+name+"_turn")
	}
	runes := []rune(turn.Content)
	if span.Start >= 0 && span.End > span.Start && span.End <= len(runes) {
		quote := string(runes[span.Start:span.End])
		if sha256Text(quote) != span.QuoteHash {
			found = append(found, name+"_quote_hash_mismatch")
		}
		return quote, found
	}
	return "", append(found, name+"_out_of_bounds")
}

func transcriptDigestValue(transcript Transcript) map[string]any {
	turns := make([]any, 0, len(transcript.Turns))
	for _, turn := range transcript.Turns {
		turns = append(turns, map[string]any{"turn_id": turn.TurnID, "sequence": turn.Sequence, "role": turn.Role, "content": turn.Content, "content_hash": turn.ContentHash})
	}
	return map[string]any{
		"source_id": transcript.SourceID, "session_id": transcript.SessionID, "captured_at": transcript.CapturedAt, "completeness": transcript.Completeness,
		"auth": map[string]any{"principal": transcript.Auth.Principal, "method": transcript.Auth.Method, "verified": transcript.Auth.Verified}, "turns": turns,
	}
}

func digestValue(value any) (string, error) {
	canonical, err := records.CanonicalJSONValue(mustJSON(value))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func sha256Text(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func strictFields(raw []byte) (map[string]json.RawMessage, error) {
	if _, err := records.DecodeStrictJSON(raw); err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return nil, errors.New("must be an object")
	}
	return fields, nil
}

func arrayRaw(raw []byte) ([]json.RawMessage, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func checkExact(fields map[string]json.RawMessage, allowed, required []string, errorsFound *[]string, name string) {
	if fields == nil {
		if errorsFound != nil {
			*errorsFound = append(*errorsFound, "invalid_"+name+"_fields")
		}
		return
	}
	allowedSet := make(map[string]bool, len(allowed))
	for _, key := range allowed {
		allowedSet[key] = true
	}
	requiredSet := make(map[string]bool, len(required))
	for _, key := range required {
		requiredSet[key] = true
	}
	for key := range fields {
		if !allowedSet[key] && errorsFound != nil {
			*errorsFound = append(*errorsFound, "invalid_"+name+"_fields")
		}
	}
	for key := range requiredSet {
		if _, ok := fields[key]; !ok && errorsFound != nil {
			*errorsFound = append(*errorsFound, "invalid_"+name+"_fields")
		}
	}
}

func hasExactAuthFields(raw []byte) bool {
	fields, err := strictFields(raw)
	if err != nil || len(fields) != 3 {
		return false
	}
	return fields["principal"] != nil && fields["method"] != nil && fields["verified"] != nil
}

func parseContractTime(value string) (time.Time, bool) {
	if !validTime(value) {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02T15:04:05Z", value)
	return parsed, err == nil
}

func validTime(value string) bool {
	_, ok := parseTimeWithoutRecursion(value)
	return ok
}

func parseTimeWithoutRecursion(value string) (time.Time, bool) {
	if !datePattern.MatchString(value) {
		return time.Time{}, false
	}
	parsed, err := time.Parse("2006-01-02T15:04:05Z", value)
	return parsed, err == nil
}

func formatTime(value time.Time) string { return value.UTC().Format("2006-01-02T15:04:05Z") }

func validHash(value string) bool { return sha256Pattern.MatchString(value) }

func nonceKey(principal, nonce string) string { return principal + ":" + nonce }

func uniqueSorted(values []string) []string {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func mustJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
