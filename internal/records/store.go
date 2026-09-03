// Package records owns HomeBase's durable, typed record boundary.  The
// running-machine contract schema remains the schema authority; this package
// enforces the invariants needed before a record is admitted to the journal.
package records

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"homebase/internal/journal"
	"io"
	"math/big"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidRecord     = errors.New("invalid shared record")
	ErrConflict          = errors.New("record id conflicts with an existing record")
	ErrNotFound          = errors.New("record not found")
	ErrAuthorityRequired = errors.New("record requires an authenticated authority path")
	ErrJournalUncertain  = errors.New("journal commit state is uncertain; reopen HomeBase before retrying")
)

var (
	sha256Pattern    = regexp.MustCompile(`^[0-9a-f]{64}$`)
	gitObjectPattern = regexp.MustCompile(`^[0-9a-f]{40,64}$`)
	datePattern      = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
)

const (
	AuthorityHumanDecision     = "human_decision"
	AuthorityAuthoritative     = "authoritative_fact"
	AuthorityVerifiedEvidence  = "verified_evidence"
	AuthorityWorkerObservation = "worker_observation"
	AuthorityAgentProposal     = "agent_proposal"
	AuthorityUntrustedText     = "untrusted_text"
)

// Record is the v1 record envelope defined by
// running-machine-contracts/schemas/record-envelope-v1.schema.json.
// Strings are retained rather than converted to time.Time so the exact
// contract representation is preserved at the authority boundary.
type Record struct {
	Kind           string          `json:"kind"`
	Version        string          `json:"version"`
	ID             string          `json:"id"`
	SourceRefs     []SourceRef     `json:"source_refs"`
	ContentHash    string          `json:"content_hash"`
	CapturedAt     string          `json:"captured_at"`
	AuthorityClass string          `json:"authority_class"`
	Freshness      Freshness       `json:"freshness"`
	Status         string          `json:"status"`
	Source         Source          `json:"source"`
	Payload        json.RawMessage `json:"payload"`
}

type SourceRef struct {
	Kind        string `json:"kind"`
	ID          string `json:"id"`
	ContentHash string `json:"content_hash,omitempty"`
}

type Source struct {
	ID   string `json:"id"`
	Role string `json:"role"`
}

type Freshness struct {
	Mode       string  `json:"mode"`
	ValidUntil *string `json:"valid_until"`
	Reason     string  `json:"reason,omitempty"`
}

type AppendResult struct {
	Sequence uint64
	Existing bool
	Record   Record
}

// PromotionCommitResult is returned after one journal entry durably commits
// the evidence, decision, and verifier receipt produced by transcript
// promotion. The receipt is opaque to this package; promotion owns its
// cryptographic schema while records owns the durable atomic boundary.
type PromotionCommitResult struct {
	Sequence uint64
	Existing bool
	Decision Record
	Evidence Record
	Receipt  json.RawMessage
}

type promotionCommit struct {
	Decision json.RawMessage `json:"decision"`
	Evidence json.RawMessage `json:"evidence"`
	Receipt  json.RawMessage `json:"receipt"`
}

type storedPromotion struct {
	decisionID string
	evidenceID string
	canonical  []byte
	receipt    json.RawMessage
	sequence   uint64
}

type Store struct {
	mu                     sync.Mutex
	journal                *journal.BinaryJournal
	records                map[string]storedRecord
	promotions             map[string]storedPromotion
	verifications          map[string]storedVerification
	contractGrants         map[string]storedContractGrant
	specificationDecisions map[string]storedSpecificationDecision
	poisoned               error
	now                    func() time.Time
}

type storedRecord struct {
	record    Record
	canonical []byte
}

func NewStore(j *journal.BinaryJournal) (*Store, error) {
	return newStore(j, time.Now)
}

// NewStoreWithClock is the deterministic constructor used by boundary tests.
// Production uses NewStore, which binds submission freshness to HomeBase's
// wall clock rather than a caller-provided timestamp.
func NewStoreWithClock(j *journal.BinaryJournal, clock func() time.Time) (*Store, error) {
	if clock == nil {
		return nil, fmt.Errorf("%w: clock is required", ErrInvalidRecord)
	}
	return newStore(j, clock)
}

func newStore(j *journal.BinaryJournal, clock func() time.Time) (*Store, error) {
	if j == nil {
		return nil, fmt.Errorf("%w: journal is required", ErrInvalidRecord)
	}
	s := &Store{journal: j, now: clock, records: make(map[string]storedRecord), promotions: make(map[string]storedPromotion), verifications: make(map[string]storedVerification), contractGrants: make(map[string]storedContractGrant), specificationDecisions: make(map[string]storedSpecificationDecision)}
	if err := j.Replay(func(seq uint64, payload []byte) error {
		envelope, err := journal.DecodeRecord(payload)
		if err != nil {
			return fmt.Errorf("shared record journal entry %d: %w", seq, err)
		}
		if envelope.Kind == journal.RecordKindPromotionCommit {
			commit, err := decodePromotionCommit(envelope.Payload)
			if err != nil {
				return fmt.Errorf("promotion commit journal entry %d: %w", seq, err)
			}
			evidence, evidenceCanonical, err := parseAndValidate(commit.Evidence)
			if err != nil || evidence.Kind != "Evidence" {
				return fmt.Errorf("promotion commit journal entry %d evidence: %w", seq, invalid("expected Evidence record: %v", err))
			}
			if err := s.load(evidence, evidenceCanonical); err != nil {
				return fmt.Errorf("promotion commit journal entry %d evidence: %w", seq, err)
			}
			decision, decisionCanonical, err := parseAndValidate(commit.Decision)
			if err != nil || decision.Kind != "Decision" {
				return fmt.Errorf("promotion commit journal entry %d decision: %w", seq, invalid("expected Decision record: %v", err))
			}
			if err := s.load(decision, decisionCanonical); err != nil {
				return fmt.Errorf("promotion commit journal entry %d decision: %w", seq, err)
			}
			if err := ensureDecisionEvidenceReference(decision, evidence); err != nil {
				return fmt.Errorf("promotion commit journal entry %d lineage: %w", seq, err)
			}
			commitCanonical, err := canonicalObject(envelope.Payload)
			if err != nil {
				return fmt.Errorf("promotion commit journal entry %d canonical form: %w", seq, err)
			}
			if existing, ok := s.promotions[decision.ID]; ok && !bytes.Equal(existing.canonical, commitCanonical) {
				return fmt.Errorf("promotion commit journal entry %d: %w: %s", seq, ErrConflict, decision.ID)
			}
			s.promotions[decision.ID] = storedPromotion{decisionID: decision.ID, evidenceID: evidence.ID, canonical: commitCanonical, receipt: bytes.Clone(commit.Receipt), sequence: seq}
			return nil
		}
		if envelope.Kind == journal.RecordKindVerificationCommit {
			if err := s.replayVerificationCommit(seq, envelope.Payload); err != nil {
				return fmt.Errorf("verification commit journal entry %d: %w", seq, err)
			}
			return nil
		}
		if envelope.Kind == journal.RecordKindContractGrantCommit {
			if err := s.replayContractGrantCommit(seq, envelope.Payload); err != nil {
				return fmt.Errorf("contract/grant commit journal entry %d: %w", seq, err)
			}
			return nil
		}
		if envelope.Kind == journal.RecordKindSpecificationDecisionCommit {
			if err := s.replaySpecificationDecisionCommit(seq, envelope.Payload); err != nil {
				return fmt.Errorf("specification/decision commit journal entry %d: %w", seq, err)
			}
			return nil
		}
		if envelope.Kind != journal.RecordKindSharedRecord {
			return fmt.Errorf("unsupported shared-record journal kind %q", envelope.Kind)
		}
		record, canonical, err := parseAndValidate(envelope.Payload)
		if err != nil {
			return fmt.Errorf("shared record journal entry %d: %w", seq, err)
		}
		if err := s.load(record, canonical); err != nil {
			return fmt.Errorf("shared record journal entry %d: %w", seq, err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Append(raw []byte) (AppendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return AppendResult{}, err
	}

	record, canonical, err := parseAndValidate(raw)
	if err != nil {
		return AppendResult{}, err
	}
	return s.appendValidated(record, canonical)
}

// AppendExternal admits only observations and proposals from an external
// producer. Decisions, contracts, grants, proofs, and verification receipts
// must arrive through an authenticated owner-specific path; accepting those
// from an arbitrary HTTP caller would make the record schema cosmetic.
func (s *Store) AppendExternal(raw []byte) (AppendResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return AppendResult{}, err
	}

	record, canonical, err := parseAndValidate(raw)
	if err != nil {
		return AppendResult{}, err
	}
	if record.AuthorityClass != AuthorityUntrustedText && record.AuthorityClass != AuthorityWorkerObservation && record.AuthorityClass != AuthorityAgentProposal {
		return AppendResult{}, fmt.Errorf("%w: %s/%s", ErrAuthorityRequired, record.Kind, record.AuthorityClass)
	}
	return s.appendValidated(record, canonical)
}

// AppendPromotionCommit atomically persists the evidence, decision, and
// authority-owned receipt in one journal entry. Promotion is the only caller
// allowed to create this bundle; records still validates the typed record
// envelopes and the decision-to-evidence lineage.
func (s *Store) AppendPromotionCommit(decisionRaw, evidenceRaw, receiptRaw []byte) (PromotionCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return PromotionCommitResult{}, err
	}

	decision, decisionCanonical, err := parseAndValidate(decisionRaw)
	if err != nil {
		return PromotionCommitResult{}, err
	}
	if decision.Kind != "Decision" {
		return PromotionCommitResult{}, invalid("promotion decision must have kind Decision")
	}
	evidence, evidenceCanonical, err := parseAndValidate(evidenceRaw)
	if err != nil {
		return PromotionCommitResult{}, err
	}
	if evidence.Kind != "Evidence" {
		return PromotionCommitResult{}, invalid("promotion evidence must have kind Evidence")
	}
	if err := ensureDecisionEvidenceReference(decision, evidence); err != nil {
		return PromotionCommitResult{}, err
	}
	receiptCanonical, err := canonicalObject(receiptRaw)
	if err != nil {
		return PromotionCommitResult{}, invalid("promotion receipt: %v", err)
	}

	commit := promotionCommit{
		Decision: decisionCanonical,
		Evidence: evidenceCanonical,
		Receipt:  receiptCanonical,
	}
	commitRaw, err := json.Marshal(commit)
	if err != nil {
		return PromotionCommitResult{}, fmt.Errorf("encode promotion commit: %w", err)
	}
	commitCanonical, err := canonicalObject(commitRaw)
	if err != nil {
		return PromotionCommitResult{}, fmt.Errorf("canonicalize promotion commit: %w", err)
	}

	if existing, ok := s.promotions[decision.ID]; ok {
		if bytes.Equal(existing.canonical, commitCanonical) {
			return PromotionCommitResult{Existing: true, Decision: decision, Evidence: evidence, Receipt: receiptCanonical}, nil
		}
		return PromotionCommitResult{}, fmt.Errorf("%w: promotion decision %s", ErrConflict, decision.ID)
	}
	if existing, ok := s.records[evidence.ID]; ok && !bytes.Equal(existing.canonical, evidenceCanonical) {
		return PromotionCommitResult{}, fmt.Errorf("%w: evidence %s", ErrConflict, evidence.ID)
	}
	if existing, ok := s.records[decision.ID]; ok && !bytes.Equal(existing.canonical, decisionCanonical) {
		return PromotionCommitResult{}, fmt.Errorf("%w: decision %s", ErrConflict, decision.ID)
	}
	if err := s.validateReferences(evidence); err != nil {
		return PromotionCommitResult{}, err
	}
	if err := s.validateReferences(decision); err != nil {
		return PromotionCommitResult{}, err
	}

	payload, err := journal.EncodeRecord(journal.RecordKindPromotionCommit, commitCanonical)
	if err != nil {
		return PromotionCommitResult{}, fmt.Errorf("encode promotion journal record: %w", err)
	}
	sequence, err := s.journal.Append(payload)
	if err != nil {
		return PromotionCommitResult{}, s.poison(fmt.Errorf("append promotion commit: %w", err))
	}
	s.records[evidence.ID] = storedRecord{record: evidence, canonical: evidenceCanonical}
	s.records[decision.ID] = storedRecord{record: decision, canonical: decisionCanonical}
	s.promotions[decision.ID] = storedPromotion{decisionID: decision.ID, evidenceID: evidence.ID, canonical: commitCanonical, receipt: bytes.Clone(receiptCanonical), sequence: sequence}
	return PromotionCommitResult{Sequence: sequence, Decision: decision, Evidence: evidence, Receipt: receiptCanonical}, nil
}

func (s *Store) appendValidated(record Record, canonical []byte) (AppendResult, error) {
	if existing, ok := s.records[record.ID]; ok {
		if bytes.Equal(existing.canonical, canonical) {
			return AppendResult{Existing: true, Record: existing.record}, nil
		}
		return AppendResult{}, fmt.Errorf("%w: %s", ErrConflict, record.ID)
	}
	if err := s.validateReferences(record); err != nil {
		return AppendResult{}, err
	}

	payload, err := journal.EncodeRecord(journal.RecordKindSharedRecord, canonical)
	if err != nil {
		return AppendResult{}, fmt.Errorf("encode shared record: %w", err)
	}
	seq, err := s.journal.Append(payload)
	if err != nil {
		return AppendResult{}, s.poison(fmt.Errorf("append shared record: %w", err))
	}
	s.records[record.ID] = storedRecord{record: record, canonical: canonical}
	return AppendResult{Sequence: seq, Record: record}, nil
}

func (s *Store) ensureHealthy() error {
	if s.poisoned != nil {
		return s.poisoned
	}
	return nil
}

func (s *Store) poison(err error) error {
	s.poisoned = fmt.Errorf("%w: %v", ErrJournalUncertain, err)
	return s.poisoned
}

func (s *Store) Get(id string) (Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[id]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record.record, nil
}

func (s *Store) List() []Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.records))
	for id := range s.records {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]Record, 0, len(ids))
	for _, id := range ids {
		result = append(result, s.records[id].record)
	}
	return result
}

// ListPromotionCommits returns the durable promotion bundles in decision-ID
// order. It is a projection API: callers can rebuild replay guards and
// indexes, but cannot mutate the authority store through it.
func (s *Store) ListPromotionCommits() []PromotionCommitResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.promotions))
	for id := range s.promotions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	result := make([]PromotionCommitResult, 0, len(ids))
	for _, id := range ids {
		promotion := s.promotions[id]
		decision := s.records[promotion.decisionID].record
		evidence := s.records[promotion.evidenceID].record
		result = append(result, PromotionCommitResult{
			Sequence: promotion.sequence,
			Decision: decision,
			Evidence: evidence,
			Receipt:  bytes.Clone(promotion.receipt),
		})
	}
	return result
}

func (s *Store) load(record Record, canonical []byte) error {
	if existing, ok := s.records[record.ID]; ok {
		if bytes.Equal(existing.canonical, canonical) {
			return nil
		}
		return fmt.Errorf("%w: %s", ErrConflict, record.ID)
	}
	if err := s.validateReferences(record); err != nil {
		return err
	}
	s.records[record.ID] = storedRecord{record: record, canonical: canonical}
	return nil
}

func ensureDecisionEvidenceReference(decision, evidence Record) error {
	for _, ref := range decision.SourceRefs {
		if strings.EqualFold(ref.Kind, "evidence") && ref.ID == evidence.ID && ref.ContentHash == evidence.ContentHash {
			return nil
		}
	}
	return invalid("Decision %q must reference Evidence %q with its content hash", decision.ID, evidence.ID)
}

func (s *Store) validateReferences(record Record) error {
	fields, err := objectFields(record.Payload)
	if err != nil {
		return invalid("%s payload: %v", record.Kind, err)
	}
	switch record.Kind {
	case "Specification":
		if err := s.validateSpecification(record, fields); err != nil {
			return err
		}
	case "Contract":
		if err := s.validateContractSpecification(record, fields); err != nil {
			return err
		}
	case "CapabilityGrant":
		contractID, _ := stringValue(fields["contract_id"])
		contract, ok := s.records[contractID]
		if !ok || contract.record.Kind != "Contract" {
			return invalid("CapabilityGrant contract_id %q does not reference an existing Contract", contractID)
		}
		contractFields, err := objectFields(contract.record.Payload)
		if err != nil {
			return invalid("referenced Contract %q: %v", contractID, err)
		}
		for _, pair := range [][2]string{{"context_hash", "context_hash"}, {"task_id", "task_id"}, {"worker_id", "worker_id"}, {"idempotency_key", "idempotency_key"}, {"specification_id", "specification_id"}, {"specification_digest", "specification_digest"}} {
			grantValue, _ := stringValue(fields[pair[0]])
			contractValue, _ := stringValue(contractFields[pair[1]])
			if grantValue != contractValue {
				return invalid("CapabilityGrant %s does not match Contract %q", pair[0], contractID)
			}
		}
		key, _ := stringValue(fields["idempotency_key"])
		for _, existing := range s.records {
			if existing.record.Kind != "CapabilityGrant" {
				continue
			}
			existingFields, _ := objectFields(existing.record.Payload)
			if existingKey, _ := stringValue(existingFields["idempotency_key"]); existingKey == key {
				return invalid("duplicate CapabilityGrant idempotency_key %q", key)
			}
		}
	case "Observation":
		grantID, _ := stringValue(fields["grant_id"])
		grant, ok := s.records[grantID]
		if !ok || grant.record.Kind != "CapabilityGrant" {
			return invalid("Observation grant_id %q does not reference an existing CapabilityGrant", grantID)
		}
		grantFields, err := objectFields(grant.record.Payload)
		if err != nil {
			return invalid("referenced CapabilityGrant %q: %v", grantID, err)
		}
		for _, pair := range [][2]string{{"task_id", "task_id"}, {"worker_id", "worker_id"}} {
			observationValue, _ := stringValue(fields[pair[0]])
			grantValue, _ := stringValue(grantFields[pair[1]])
			if observationValue != grantValue {
				return invalid("Observation %s does not match CapabilityGrant %q", pair[0], grantID)
			}
		}
		effect, _ := objectFields(fields["effect"])
		path, _ := stringValue(effect["path"])
		allowed, _ := stringArray(grantFields["allowed_paths"])
		if !pathAllowed(path, allowed) {
			return invalid("Observation path %q is outside CapabilityGrant %q", path, grantID)
		}
	case "VerificationReceipt":
		contractID, ok := stringValue(fields["contract_id"])
		if !ok {
			return invalid("VerificationReceipt contract_id must be a non-empty string")
		}
		contract, err := s.requireRecordID(contractID, "contract_id", "Contract")
		if err != nil {
			return err
		}
		grantID, ok := stringValue(fields["grant_id"])
		if !ok {
			return invalid("VerificationReceipt grant_id must be a non-empty string")
		}
		grant, err := s.requireRecordID(grantID, "grant_id", "CapabilityGrant")
		if err != nil {
			return err
		}
		taskID, _ := stringValue(fields["task_id"])
		workerID, _ := stringValue(fields["worker_id"])
		contractFields, err := objectFields(contract.record.Payload)
		if err != nil {
			return invalid("referenced Contract %q: %v", contractID, err)
		}
		if !sameStringField(contractFields, "task_id", taskID) || !sameStringField(contractFields, "worker_id", workerID) {
			return invalid("VerificationReceipt does not match Contract %q task or worker", contractID)
		}
		grantFields, err := objectFields(grant.record.Payload)
		if err != nil {
			return invalid("referenced CapabilityGrant %q: %v", grantID, err)
		}
		for _, field := range []string{"contract_id", "task_id", "worker_id"} {
			want := map[string]string{"contract_id": contractID, "task_id": taskID, "worker_id": workerID}[field]
			if !sameStringField(grantFields, field, want) {
				return invalid("VerificationReceipt does not match CapabilityGrant %q field %s", grantID, field)
			}
		}
		if err := validateVerificationFreshness(fields, contractFields, grantFields); err != nil {
			return err
		}

		workerRef, workerRecord, err := s.resolveSourceRef(fields["worker_claim_ref"], "worker_claim_ref", "Claim", "Observation")
		if err != nil {
			return err
		}
		if workerRecord.record.Source.ID != workerID || (workerRecord.record.Source.Role != "worker" && workerRecord.record.Source.Role != "agent") {
			return invalid("VerificationReceipt worker_claim_ref %q is not owned by worker %q", workerRef.ID, workerID)
		}
		if workerRecord.record.Kind == "Observation" {
			workerFields, err := objectFields(workerRecord.record.Payload)
			if err != nil {
				return invalid("referenced Observation %q: %v", workerRef.ID, err)
			}
			if !sameStringField(workerFields, "task_id", taskID) || !sameStringField(workerFields, "worker_id", workerID) || !sameStringField(workerFields, "grant_id", grantID) {
				return invalid("VerificationReceipt Observation %q is not bound to its task, grant, and worker", workerRef.ID)
			}
		}

		evidenceRefs, err := arrayValues(fields["evidence_refs"])
		if err != nil {
			return invalid("VerificationReceipt evidence_refs: %v", err)
		}
		evidenceIDs := make(map[string]bool, len(evidenceRefs))
		for _, rawRef := range evidenceRefs {
			ref, evidenceRecord, err := s.resolveSourceRef(rawRef, "evidence_ref", "Proof", "Evidence")
			if err != nil {
				return err
			}
			evidenceIDs[evidenceRecord.record.ID] = true
			if ref.ContentHash != "" && ref.ContentHash != evidenceRecord.record.ContentHash {
				return invalid("VerificationReceipt evidence_ref %q content_hash does not match", ref.ID)
			}
		}
		checks, err := arrayValues(fields["checks"])
		if err != nil {
			return invalid("VerificationReceipt checks: %v", err)
		}
		for _, rawCheck := range checks {
			check, err := objectFields(rawCheck)
			if err != nil {
				return invalid("VerificationReceipt check: %v", err)
			}
			ref, evidenceRecord, err := s.resolveSourceRef(check["evidence_ref"], "check.evidence_ref", "Proof", "Evidence")
			if err != nil {
				return err
			}
			if !evidenceIDs[evidenceRecord.record.ID] {
				return invalid("VerificationReceipt check evidence_ref %q is not listed in evidence_refs", ref.ID)
			}
		}
	}
	return nil
}

// validateSpecification enforces the semantic authority boundary for an
// upstream specification. A proposed specification may be stored as
// evidence/provenance, but only a captain-approved specification with a
// durable Decision reference can authorize a Contract.
func (s *Store) validateSpecification(record Record, fields map[string]json.RawMessage) error {
	if record.Status == "proposed" {
		if record.AuthorityClass != AuthorityAgentProposal {
			return invalid("proposed Specification requires agent_proposal authority")
		}
		return nil
	}
	if record.Status != "approved" {
		return nil
	}
	if record.AuthorityClass != AuthorityHumanDecision || record.Source.Role != "captain" {
		return invalid("approved Specification requires human_decision from captain")
	}
	approvalRaw, ok := fields["approval_ref"]
	if !ok {
		return invalid("approved Specification requires payload.approval_ref")
	}
	approval, err := parseDecisionApprovalRef(approvalRaw, "Specification approval_ref")
	if err != nil {
		return err
	}
	if approval.Kind != "decision" {
		return invalid("Specification approval_ref must reference a decision")
	}
	decision, ok := s.records[approval.ID]
	if !ok || decision.record.Kind != "Decision" {
		return invalid("Specification approval_ref %q does not reference an existing Decision", approval.ID)
	}
	if decision.record.AuthorityClass != AuthorityHumanDecision || decision.record.Status != "approved" || decision.record.Source.Role != "captain" {
		return invalid("Specification approval_ref %q is not a captain-approved Decision", approval.ID)
	}
	decisionFields, err := objectFields(decision.record.Payload)
	if err != nil {
		return invalid("approval Decision %q payload: %v", approval.ID, err)
	}
	specificationRefRaw, ok := decisionFields["specification_ref"]
	if !ok {
		return invalid("approval Decision %q requires payload.specification_ref", approval.ID)
	}
	specificationRef, err := parseRequiredSourceRef(specificationRefRaw, "approval Decision specification_ref")
	if err != nil {
		return err
	}
	if specificationRef.Kind != "specification" || specificationRef.ID != record.ID || specificationRef.ContentHash != record.ContentHash {
		return invalid("approval Decision %q is not bound to Specification %q and its content hash", approval.ID, record.ID)
	}
	return nil
}

// validateContractSpecification resolves the exact specification named by a
// Contract. Shape-valid identifiers are not enough: the record must exist,
// its payload hash must match, its authority must be approved, and the
// Contract envelope must carry the same hash-bound source reference.
func (s *Store) validateContractSpecification(record Record, fields map[string]json.RawMessage) error {
	specificationID, ok := stringValue(fields["specification_id"])
	if !ok {
		return invalid("Contract specification_id is required")
	}
	specificationDigest, ok := stringValue(fields["specification_digest"])
	if !ok || !sha256Pattern.MatchString(specificationDigest) {
		return invalid("Contract specification_digest must be a lowercase SHA-256 digest")
	}
	specification, ok := s.records[specificationID]
	if !ok || specification.record.Kind != "Specification" {
		return invalid("Contract specification_id %q does not reference an existing Specification", specificationID)
	}
	if specification.record.ContentHash != specificationDigest {
		return invalid("Contract specification_digest does not match Specification %q", specificationID)
	}
	if specification.record.Status != "approved" || specification.record.AuthorityClass != AuthorityHumanDecision || specification.record.Source.Role != "captain" {
		return invalid("Contract Specification %q is not captain-approved", specificationID)
	}
	specificationFields, err := objectFields(specification.record.Payload)
	if err != nil {
		return invalid("referenced Specification %q payload: %v", specificationID, err)
	}
	approval, err := parseDecisionApprovalRef(specificationFields["approval_ref"], "Specification approval_ref")
	if err != nil {
		return err
	}
	decisionRefFound := false
	for _, ref := range record.SourceRefs {
		if ref.Kind != "decision" {
			continue
		}
		decision, ok := s.records[ref.ID]
		if !ok || decision.record.Kind != "Decision" {
			return invalid("Contract decision source_ref %q does not reference an existing Decision", ref.ID)
		}
		if ref.ContentHash != decision.record.ContentHash {
			return invalid("Contract decision source_ref %q content_hash does not match", ref.ID)
		}
		if ref.ID == approval.ID {
			decisionRefFound = true
		}
	}
	if !decisionRefFound {
		return invalid("Contract must include the Specification approval Decision source_ref")
	}
	found := false
	for _, ref := range record.SourceRefs {
		if ref.Kind != "specification" || ref.ID != specificationID {
			continue
		}
		if ref.ContentHash != specificationDigest {
			return invalid("Contract specification source_ref %q content_hash does not match", specificationID)
		}
		found = true
	}
	if !found {
		return invalid("Contract must include a hash-bound specification source_ref")
	}
	return nil
}

func parseRequiredSourceRef(raw []byte, field string) (SourceRef, error) {
	if err := requireSourceRef(raw, field); err != nil {
		return SourceRef{}, invalid("%v", err)
	}
	var ref SourceRef
	if err := json.Unmarshal(raw, &ref); err != nil || ref.Kind == "" || ref.ID == "" || !sha256Pattern.MatchString(ref.ContentHash) {
		return SourceRef{}, invalid("%s requires kind, id, and content_hash", field)
	}
	return ref, nil
}

func parseDecisionApprovalRef(raw []byte, field string) (SourceRef, error) {
	if err := requireSourceRef(raw, field); err != nil {
		return SourceRef{}, invalid("%v", err)
	}
	var ref map[string]json.RawMessage
	if err := json.Unmarshal(raw, &ref); err != nil {
		return SourceRef{}, invalid("%s must be an object", field)
	}
	if len(ref) != 2 {
		return SourceRef{}, invalid("%s requires exactly kind and id; content_hash would create a circular approval binding", field)
	}
	var parsed SourceRef
	if err := json.Unmarshal(raw, &parsed); err != nil || parsed.Kind != "decision" || parsed.ID == "" || parsed.ContentHash != "" {
		return SourceRef{}, invalid("%s requires kind=decision and id, without content_hash", field)
	}
	return parsed, nil
}

func validateVerificationFreshness(receiptFields, contractFields, grantFields map[string]json.RawMessage) error {
	verifiedAtText, ok := stringValue(receiptFields["verified_at"])
	if !ok {
		return invalid("VerificationReceipt verified_at must be a non-empty timestamp")
	}
	verifiedAt, err := time.Parse("2006-01-02T15:04:05Z", verifiedAtText)
	if err != nil {
		return invalid("VerificationReceipt verified_at: %v", err)
	}
	issuedAtText, ok := stringValue(grantFields["issued_at"])
	if !ok {
		return invalid("CapabilityGrant issued_at is missing for receipt freshness")
	}
	issuedAt, err := time.Parse("2006-01-02T15:04:05Z", issuedAtText)
	if err != nil {
		return invalid("CapabilityGrant issued_at: %v", err)
	}
	expiresAtText, ok := stringValue(grantFields["expires_at"])
	if !ok {
		return invalid("CapabilityGrant expires_at is missing for receipt freshness")
	}
	expiresAt, err := time.Parse("2006-01-02T15:04:05Z", expiresAtText)
	if err != nil {
		return invalid("CapabilityGrant expires_at: %v", err)
	}
	contextValidUntilText, ok := stringValue(contractFields["context_valid_until"])
	if !ok {
		return invalid("Contract context_valid_until is missing for receipt freshness")
	}
	contextValidUntil, err := time.Parse("2006-01-02T15:04:05Z", contextValidUntilText)
	if err != nil {
		return invalid("Contract context_valid_until: %v", err)
	}
	if verifiedAt.Before(issuedAt) {
		return invalid("VerificationReceipt verified_at precedes CapabilityGrant issued_at")
	}
	if verifiedAt.After(expiresAt) {
		return invalid("VerificationReceipt verified_at is after CapabilityGrant expires_at")
	}
	if verifiedAt.After(contextValidUntil) {
		return invalid("VerificationReceipt verified_at is after Contract context_valid_until")
	}
	return nil
}

func (s *Store) requireRecordID(id, field, expectedKind string) (storedRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return storedRecord{}, invalid("VerificationReceipt %s references missing record %q", field, id)
	}
	if record.record.Kind != expectedKind {
		return storedRecord{}, invalid("VerificationReceipt %s references %q, which is %s not %s", field, id, record.record.Kind, expectedKind)
	}
	return record, nil
}

func (s *Store) resolveSourceRef(raw []byte, field string, allowedKinds ...string) (SourceRef, storedRecord, error) {
	refFields, err := objectFields(raw)
	if err != nil {
		return SourceRef{}, storedRecord{}, invalid("VerificationReceipt %s: %v", field, err)
	}
	var ref SourceRef
	if err := json.Unmarshal(raw, &ref); err != nil || ref.ID == "" || ref.Kind == "" {
		return SourceRef{}, storedRecord{}, invalid("VerificationReceipt %s is not a valid source reference", field)
	}
	record, ok := s.records[ref.ID]
	if !ok {
		return SourceRef{}, storedRecord{}, invalid("VerificationReceipt %s references missing record %q", field, ref.ID)
	}
	allowed := false
	for _, kind := range allowedKinds {
		if record.record.Kind == kind && sourceRefKind(kind) == strings.ToLower(ref.Kind) {
			allowed = true
			break
		}
	}
	if !allowed {
		return SourceRef{}, storedRecord{}, invalid("VerificationReceipt %s kind %q does not match referenced %s", field, ref.Kind, record.record.Kind)
	}
	if hash, ok := refFields["content_hash"]; ok {
		var contentHash string
		if err := json.Unmarshal(hash, &contentHash); err != nil || contentHash != record.record.ContentHash {
			return SourceRef{}, storedRecord{}, invalid("VerificationReceipt %s content_hash does not match referenced record", field)
		}
	}
	return ref, record, nil
}

func sourceRefKind(kind string) string {
	switch kind {
	case "CapabilityGrant":
		return "grant"
	case "VerificationReceipt":
		return "verificationreceipt"
	default:
		return strings.ToLower(kind)
	}
}

func sameStringField(fields map[string]json.RawMessage, key, want string) bool {
	got, ok := stringValue(fields[key])
	return ok && got == want
}

func stringValue(raw []byte) (string, bool) {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" {
		return "", false
	}
	return value, true
}

func stringArray(raw []byte) ([]string, bool) {
	values, err := arrayValues(raw)
	if err != nil {
		return nil, false
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		parsed, ok := stringValue(value)
		if !ok {
			return nil, false
		}
		result = append(result, parsed)
	}
	return result, true
}

func pathAllowed(path string, allowed []string) bool {
	for _, candidate := range allowed {
		if path == candidate || strings.HasSuffix(candidate, "/") && strings.HasPrefix(path, candidate) {
			return true
		}
	}
	return false
}

func parseAndValidate(raw []byte) (Record, []byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return Record{}, nil, invalid("record is not an object: %v", err)
	}
	for key := range fields {
		switch key {
		case "kind", "version", "id", "source_refs", "content_hash", "captured_at", "authority_class", "freshness", "status", "source", "payload":
		default:
			return Record{}, nil, invalid("unknown top-level field %q", key)
		}
	}
	for _, key := range []string{"kind", "version", "id", "source_refs", "content_hash", "captured_at", "authority_class", "freshness", "status", "source", "payload"} {
		if _, ok := fields[key]; !ok {
			return Record{}, nil, invalid("missing top-level field %q", key)
		}
	}
	var record Record
	if err := json.Unmarshal(raw, &record); err != nil {
		return Record{}, nil, invalid("decode record: %v", err)
	}
	if err := record.validate(fields, fields["payload"]); err != nil {
		return Record{}, nil, err
	}
	canonicalPayload, err := canonicalObject(record.Payload)
	if err != nil {
		return Record{}, nil, invalid("payload: %v", err)
	}
	digest := sha256.Sum256(canonicalPayload)
	if record.ContentHash != hex.EncodeToString(digest[:]) {
		return Record{}, nil, invalid("content_hash does not match canonical payload")
	}
	canonical, err := canonicalObject(raw)
	if err != nil {
		return Record{}, nil, invalid("record: %v", err)
	}
	// Store the normalized object, ensuring whitespace and key order cannot
	// create a second representation of the same record ID.
	return record, canonical, nil
}

func (r Record) validate(fields map[string]json.RawMessage, payloadRaw []byte) error {
	if r.Kind == "" || r.Version == "" || r.ID == "" || r.ContentHash == "" || r.CapturedAt == "" || r.AuthorityClass == "" || r.Status == "" {
		return invalid("required scalar is empty")
	}
	if !isOneOf(r.Kind, "Claim", "Evidence", "Decision", "Contract", "CapabilityGrant", "Observation", "Proposal", "Proof", "VerificationReceipt", "Challenge", "Specification") {
		return invalid("unsupported record kind %q", r.Kind)
	}
	if r.Version != "1" {
		return invalid("unsupported record version %q", r.Version)
	}
	if len(r.SourceRefs) == 0 {
		return invalid("source_refs must not be empty")
	}
	for i, ref := range r.SourceRefs {
		if ref.Kind == "" || ref.ID == "" {
			return invalid("source_refs[%d] requires kind and id", i)
		}
		if ref.ContentHash != "" && !sha256Pattern.MatchString(ref.ContentHash) {
			return invalid("source_refs[%d] content_hash is not lowercase sha256", i)
		}
	}
	if err := validateSourceRefsRaw(fields["source_refs"]); err != nil {
		return err
	}
	if !sha256Pattern.MatchString(r.ContentHash) {
		return invalid("content_hash is not lowercase sha256")
	}
	if !datePattern.MatchString(r.CapturedAt) {
		return invalid("captured_at must be UTC without fractional seconds")
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z", r.CapturedAt); err != nil {
		return invalid("captured_at is not a valid timestamp: %v", err)
	}
	if !isOneOf(r.AuthorityClass, AuthorityHumanDecision, AuthorityAuthoritative, AuthorityVerifiedEvidence, AuthorityWorkerObservation, AuthorityAgentProposal, AuthorityUntrustedText) {
		return invalid("unsupported authority_class %q", r.AuthorityClass)
	}
	freshnessFields, err := objectFields(fields["freshness"])
	if err != nil {
		return invalid("freshness: %v", err)
	}
	for key := range freshnessFields {
		if key != "mode" && key != "valid_until" && key != "reason" {
			return invalid("freshness has unknown field %q", key)
		}
	}
	for _, key := range []string{"mode", "valid_until"} {
		if _, ok := freshnessFields[key]; !ok {
			return invalid("freshness missing %q", key)
		}
	}
	if err := r.Freshness.validate(); err != nil {
		return err
	}
	if !isOneOf(r.Status, "proposed", "approved", "active", "observed", "verified", "rejected", "expired", "stale", "superseded", "challenged") {
		return invalid("unsupported status %q", r.Status)
	}
	if r.Source.ID == "" || !isOneOf(r.Source.Role, "captain", "portfolio", "homebase", "bridge", "mintmux", "trajectory", "knowledge_engine", "axioms", "agent", "worker", "verifier", "git", "github", "system") {
		return invalid("source requires a known role and non-empty id")
	}
	if err := validateSourceRaw(fields["source"]); err != nil {
		return err
	}
	if r.Kind == "Decision" || r.Kind == "Contract" {
		if r.AuthorityClass != AuthorityHumanDecision || !isOneOf(r.Source.Role, "captain", "portfolio", "homebase") {
			return invalid("%s requires human_decision from captain, portfolio, or homebase", r.Kind)
		}
	}
	if r.Kind == "Specification" {
		if !isOneOf(r.Status, "proposed", "approved", "superseded", "challenged") {
			return invalid("Specification has unsupported status %q", r.Status)
		}
		if r.Status == "proposed" && r.AuthorityClass != AuthorityAgentProposal {
			return invalid("proposed Specification requires agent_proposal authority")
		}
		if r.Status == "approved" && (r.AuthorityClass != AuthorityHumanDecision || r.Source.Role != "captain") {
			return invalid("approved Specification requires human_decision from captain")
		}
	}
	if roles := expectedSourceRoles(r.Kind); len(roles) > 0 && !contains(roles, r.Source.Role) {
		return invalid("%s requires source role in %v", r.Kind, roles)
	}
	if r.Kind == "VerificationReceipt" && (r.AuthorityClass != AuthorityVerifiedEvidence || r.Status != "verified" || r.Source.Role != "verifier") {
		return invalid("VerificationReceipt requires verified evidence, verified status, and verifier source")
	}
	if r.Kind == "VerificationReceipt" {
		payloadFields, err := objectFields(payloadRaw)
		if err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
		var verifierID string
		if err := json.Unmarshal(payloadFields["verifier_id"], &verifierID); err != nil || verifierID != r.Source.ID {
			return invalid("VerificationReceipt verifier_id must equal source.id")
		}
	}
	if err := validatePayload(r.Kind, r.AuthorityClass, payloadRaw); err != nil {
		return err
	}
	return nil
}

func (f Freshness) validate() error {
	if !isOneOf(f.Mode, "time_bound", "source_bound", "immutable") {
		return invalid("freshness requires a known mode")
	}
	if f.ValidUntil != nil && *f.ValidUntil != "" {
		if !datePattern.MatchString(*f.ValidUntil) {
			return invalid("freshness.valid_until must be UTC without fractional seconds or null")
		}
		if _, err := time.Parse("2006-01-02T15:04:05Z", *f.ValidUntil); err != nil {
			return invalid("freshness.valid_until is not a valid timestamp: %v", err)
		}
	}
	return nil
}

func validatePayload(kind, authority string, raw []byte) error {
	fields, err := objectFields(raw)
	if err != nil {
		return invalid("%s payload: %v", kind, err)
	}
	required := map[string][]string{
		"Claim":               {"statement", "subject_refs"},
		"Evidence":            {"evidence_type", "subject_refs", "observed_digest"},
		"Decision":            {"decision", "scope", "decided_by"},
		"Contract":            {"task_id", "repository", "base_commit", "allowed_paths", "forbidden_paths", "context_hash", "context_valid_until", "idempotency_key", "worker_id", "verifier_id", "acceptance", "publication", "specification_id", "specification_digest"},
		"CapabilityGrant":     {"grant_id", "contract_id", "task_id", "worker_id", "allowed_paths", "commands", "issued_at", "expires_at", "context_hash", "idempotency_key", "effect_id", "specification_id", "specification_digest"},
		"Observation":         {"task_id", "grant_id", "worker_id", "effect"},
		"Proposal":            {"task_id", "worker_id", "proposal"},
		"Proof":               {"proof_type", "proof_command", "result", "subject_refs", "verifier_id"},
		"VerificationReceipt": {"task_id", "contract_id", "grant_id", "worker_id", "verifier_id", "tree_digest", "subject", "checks", "evidence_refs", "worker_claim_ref", "verified_at"},
		"Challenge":           {"target_ref", "challenge_type", "reason", "raised_by"},
		"Specification":       {"purpose", "scope", "non_goals", "requirements", "proof_obligations", "golden_scenarios", "context_sources", "assumptions", "admission_policy", "revision_policy"},
	}[kind]
	for key := range fields {
		if !contains(required, key) && !allowedOptional(kind, key) {
			return invalid("%s payload has unknown field %q", kind, key)
		}
	}
	for _, key := range required {
		if _, ok := fields[key]; !ok {
			return invalid("%s payload missing %q", kind, key)
		}
	}
	if expected := expectedAuthority(kind); expected != "" && authority != expected {
		return invalid("%s requires authority_class %q", kind, expected)
	}
	for _, key := range []string{"statement", "evidence_type", "decision", "scope", "decided_by", "task_id", "repository", "idempotency_key", "worker_id", "verifier_id", "proposal", "proof_type", "proof_command", "challenge_type", "reason", "raised_by", "grant_id", "contract_id", "effect_id"} {
		if kind == "Specification" && key == "scope" {
			continue
		}
		if rawValue, ok := fields[key]; ok {
			if err := requireNonEmptyString(rawValue, key); err != nil {
				return invalid("%s payload: %v", kind, err)
			}
		}
	}
	if kind == "Evidence" {
		if err := requireSHA(fields["observed_digest"], "observed_digest"); err != nil {
			return invalid("Evidence payload: %v", err)
		}
	}
	if kind == "Proof" {
		if err := requireOneOfString(fields["result"], "result", "passed", "failed"); err != nil {
			return invalid("Proof payload: %v", err)
		}
	}
	if kind == "Contract" {
		if err := requireGitObject(fields["base_commit"], "base_commit"); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireSHA(fields["context_hash"], "context_hash"); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireOneOfString(fields["publication"], "publication", "prohibited"); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireDate(fields["context_valid_until"], "context_valid_until"); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireNonEmptyStringArray(fields["allowed_paths"], "allowed_paths", true); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireNonEmptyStringArray(fields["forbidden_paths"], "forbidden_paths", true); err != nil {
			return invalid("Contract payload: %v", err)
		}
		if err := requireNonEmptyStringArray(fields["acceptance"], "acceptance", true); err != nil {
			return invalid("Contract payload: %v", err)
		}
	}
	if kind == "CapabilityGrant" {
		if err := requireSHA(fields["context_hash"], "context_hash"); err != nil {
			return invalid("CapabilityGrant payload: %v", err)
		}
		if err := requireDate(fields["issued_at"], "issued_at"); err != nil {
			return invalid("CapabilityGrant payload: %v", err)
		}
		if err := requireDate(fields["expires_at"], "expires_at"); err != nil {
			return invalid("CapabilityGrant payload: %v", err)
		}
		if err := requireNonEmptyStringArray(fields["allowed_paths"], "allowed_paths", true); err != nil {
			return invalid("CapabilityGrant payload: %v", err)
		}
		if err := requireNonEmptyStringArray(fields["commands"], "commands", false); err != nil {
			return invalid("CapabilityGrant payload: %v", err)
		}
	}
	if kind == "Observation" {
		effect, err := objectFields(fields["effect"])
		if err != nil {
			return invalid("Observation payload effect: %v", err)
		}
		if err := requireOneOfString(effect["operation"], "effect.operation", "read", "write", "execute"); err != nil {
			return invalid("Observation payload: %v", err)
		}
		if err := requirePath(effect["path"], "effect.path"); err != nil {
			return invalid("Observation payload: %v", err)
		}
		for key := range effect {
			if key != "operation" && key != "path" && key != "result_digest" {
				return invalid("Observation effect has unknown field %q", key)
			}
		}
		if digest, ok := effect["result_digest"]; ok {
			if err := requireSHA(digest, "effect.result_digest"); err != nil {
				return invalid("Observation payload: %v", err)
			}
		}
	}
	if kind == "VerificationReceipt" {
		verifierID, _ := stringValue(fields["verifier_id"])
		if rawAttestation, ok := fields["attestation"]; ok {
			attestation, err := objectFields(rawAttestation)
			if err != nil {
				return invalid("VerificationReceipt attestation: %v", err)
			}
			for key := range attestation {
				if key != "scheme" && key != "key_id" && key != "signature" {
					return invalid("VerificationReceipt attestation has unknown field %q", key)
				}
			}
			if err := requireNonEmptyString(attestation["scheme"], "attestation.scheme"); err != nil {
				return invalid("VerificationReceipt attestation: %v", err)
			}
			if err := requireNonEmptyString(attestation["key_id"], "attestation.key_id"); err != nil {
				return invalid("VerificationReceipt attestation: %v", err)
			}
			if err := requireHexDigest(attestation["signature"], "attestation.signature", 128); err != nil {
				return invalid("VerificationReceipt attestation: %v", err)
			}
		} else if verifierID == productionVerifierID {
			return invalid("VerificationReceipt production verifier requires attestation")
		}
		for _, key := range []string{"tree_digest"} {
			if err := requireSHA(fields[key], key); err != nil {
				return invalid("VerificationReceipt payload: %v", err)
			}
		}
		if err := requireDate(fields["verified_at"], "verified_at"); err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
		subject, err := objectFields(fields["subject"])
		if err != nil {
			return invalid("VerificationReceipt subject: %v", err)
		}
		if err := requireOneOfString(subject["kind"], "subject.kind", "git_tree"); err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
		if err := requireNonEmptyString(subject["name"], "subject.name"); err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
		digest, err := objectFields(subject["digest"])
		if err != nil {
			return invalid("VerificationReceipt subject.digest: %v", err)
		}
		if err := requireSHA(digest["sha256"], "subject.digest.sha256"); err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
		var subjectDigest, treeDigest string
		if err := json.Unmarshal(digest["sha256"], &subjectDigest); err != nil {
			return invalid("VerificationReceipt subject digest: %v", err)
		}
		if err := json.Unmarshal(fields["tree_digest"], &treeDigest); err != nil || subjectDigest != treeDigest {
			return invalid("VerificationReceipt subject digest must equal tree_digest")
		}
		var subjectName string
		if err := json.Unmarshal(subject["name"], &subjectName); err != nil || subjectName == "" {
			return invalid("VerificationReceipt subject.name must be non-empty")
		}
		for key := range subject {
			if key != "kind" && key != "name" && key != "digest" {
				return invalid("VerificationReceipt subject has unknown field %q", key)
			}
		}
		checks, err := arrayValues(fields["checks"])
		if err != nil || len(checks) == 0 {
			return invalid("VerificationReceipt checks must be non-empty array")
		}
		for i, checkRaw := range checks {
			check, err := objectFields(checkRaw)
			if err != nil {
				return invalid("VerificationReceipt check %d: %v", i, err)
			}
			for key := range check {
				if key != "name" && key != "result" && key != "evidence_ref" && key != "proof_command" && key != "provenance" {
					return invalid("VerificationReceipt check has unknown field %q", key)
				}
			}
			if err := requireNonEmptyString(check["name"], "check.name"); err != nil {
				return invalid("VerificationReceipt check %d: %v", i, err)
			}
			if err := requireOneOfString(check["result"], "check.result", "passed"); err != nil {
				return invalid("VerificationReceipt check %d: %v", i, err)
			}
			if err := requireSourceRef(check["evidence_ref"], "check.evidence_ref"); err != nil {
				return invalid("VerificationReceipt check %d: %v", i, err)
			}
			if rawProvenance, ok := check["provenance"]; ok {
				if _, ok := check["proof_command"]; !ok {
					return invalid("VerificationReceipt check %d with provenance is missing proof_command", i)
				}
				var provenance bridgeProvenance
				if err := json.Unmarshal(rawProvenance, &provenance); err != nil {
					return invalid("VerificationReceipt check %d provenance: %v", i, err)
				}
				if err := validateBridgeProvenance(provenance, subjectName); err != nil {
					return invalid("VerificationReceipt check %d provenance: %v", i, err)
				}
			} else if receiptVerifierID, ok := stringValue(fields["verifier_id"]); ok && (receiptVerifierID == legacyVerifierID || receiptVerifierID == productionVerifierID) {
				return invalid("VerificationReceipt check %d is missing provenance for bridge:verifier", i)
			}
		}
		for _, key := range []string{"evidence_refs"} {
			values, err := arrayValues(fields[key])
			if err != nil || len(values) == 0 {
				return invalid("VerificationReceipt %s must be non-empty array", key)
			}
			for _, value := range values {
				if err := requireSourceRef(value, key); err != nil {
					return invalid("VerificationReceipt payload: %v", err)
				}
			}
		}
		if err := requireSourceRef(fields["worker_claim_ref"], "worker_claim_ref"); err != nil {
			return invalid("VerificationReceipt payload: %v", err)
		}
	}
	if kind == "Claim" || kind == "Evidence" || kind == "Proof" {
		if values, err := arrayValues(fields["subject_refs"]); err != nil || len(values) == 0 {
			return invalid("%s subject_refs must be non-empty array", kind)
		} else {
			for _, value := range values {
				if err := requireSourceRef(value, "subject_refs"); err != nil {
					return invalid("%s payload: %v", kind, err)
				}
			}
		}
	}
	if kind == "Challenge" {
		if err := requireSourceRef(fields["target_ref"], "target_ref"); err != nil {
			return invalid("Challenge payload: %v", err)
		}
	}
	return nil
}

func expectedAuthority(kind string) string {
	switch kind {
	case "Decision", "Contract":
		return AuthorityHumanDecision
	case "CapabilityGrant":
		return AuthorityAuthoritative
	case "Observation":
		return AuthorityWorkerObservation
	case "Proposal":
		return AuthorityAgentProposal
	case "Proof", "VerificationReceipt":
		return AuthorityVerifiedEvidence
	case "Specification":
		return ""
	}
	return ""
}

func expectedSourceRoles(kind string) []string {
	switch kind {
	case "Decision", "Contract":
		return []string{"captain", "portfolio", "homebase"}
	case "CapabilityGrant":
		return []string{"bridge"}
	case "Observation", "Proposal":
		return []string{"worker", "agent"}
	case "Proof":
		return []string{"verifier", "axioms"}
	case "VerificationReceipt":
		return []string{"verifier"}
	case "Specification":
		return []string{"system", "captain", "portfolio", "knowledge_engine"}
	default:
		return nil
	}
}

func allowedOptional(kind, key string) bool {
	if kind == "Decision" && key == "specification_ref" {
		return true
	}
	if kind == "Observation" && key == "effect" {
		return true
	}
	if kind == "VerificationReceipt" && key == "attestation" {
		return true
	}
	if kind == "Specification" && key == "approval_ref" {
		return true
	}
	return false
}

func objectFields(raw []byte) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return nil, errors.New("is required")
	}
	canonical, err := canonicalObject(raw)
	if err != nil {
		return nil, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(canonical, &fields); err != nil || fields == nil {
		return nil, errors.New("must be an object")
	}
	return fields, nil
}

// DecodeStrictJSON parses exactly one JSON value and rejects duplicate object
// keys. It is exported for typed authority boundaries that must not let two
// syntactic representations silently acquire different meanings.
func DecodeStrictJSON(raw []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeStrictValue(decoder)
	if err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("contains more than one JSON value")
		}
		return nil, err
	}
	return value, nil
}

// CanonicalJSONValue applies the repository's deterministic JSON encoding to
// any JSON object/array/scalar. Integer numbers are normalized; other numeric
// forms are rejected so callers cannot create language-dependent digests.
func CanonicalJSONValue(raw []byte) ([]byte, error) {
	value, err := DecodeStrictJSON(raw)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err := writeCanonicalValue(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// CanonicalContentHash returns the SHA-256 digest of a canonical JSON object.
// It is intended for generated typed payloads and receipts.
func CanonicalContentHash(raw []byte) (string, error) {
	canonical, err := canonicalObject(raw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func decodePromotionCommit(raw []byte) (promotionCommit, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return promotionCommit{}, invalid("promotion commit: %v", err)
	}
	for key := range fields {
		if key != "decision" && key != "evidence" && key != "receipt" {
			return promotionCommit{}, invalid("promotion commit has unknown field %q", key)
		}
	}
	var commit promotionCommit
	for key, target := range map[string]*json.RawMessage{
		"decision": &commit.Decision,
		"evidence": &commit.Evidence,
		"receipt":  &commit.Receipt,
	} {
		rawValue, ok := fields[key]
		if !ok || len(rawValue) == 0 {
			return promotionCommit{}, invalid("promotion commit missing %q", key)
		}
		canonical, err := CanonicalJSONValue(rawValue)
		if err != nil {
			return promotionCommit{}, invalid("promotion commit %s: %v", key, err)
		}
		if _, err := objectFields(canonical); err != nil {
			return promotionCommit{}, invalid("promotion commit %s must be an object", key)
		}
		*target = canonical
	}
	return commit, nil
}

func canonicalObject(raw []byte) ([]byte, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeStrictValue(decoder)
	if err != nil {
		return nil, err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("contains more than one JSON value")
		}
		return nil, err
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, errors.New("must be an object")
	}
	var out bytes.Buffer
	if err := writeCanonical(&out, value); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// writeCanonical matches the existing contract checker's JSON encoding:
// recursively sorted object keys, compact separators, and ensure_ascii=true.
// The record v1 payload schema contains no numeric fields, so numeric values
// are rejected rather than allowing language-specific number normalization.
func writeCanonical(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if value {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		writeCanonicalString(out, value)
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonical(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonicalString(out, key)
			out.WriteByte(':')
			if err := writeCanonical(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
	case json.Number:
		return errors.New("numeric JSON values are not supported by record canonicalization")
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	return nil
}

func writeCanonicalValue(out *bytes.Buffer, value any) error {
	switch value := value.(type) {
	case json.Number:
		integer := value.String()
		if strings.HasPrefix(integer, "-") {
			integer = integer[1:]
		}
		if integer == "" || strings.Trim(integer, "0123456789") != "" {
			return fmt.Errorf("non-integer JSON number %q is unsupported", value.String())
		}
		var normalized big.Int
		if _, ok := normalized.SetString(value.String(), 10); !ok {
			return fmt.Errorf("invalid JSON integer %q", value.String())
		}
		out.WriteString(normalized.String())
		return nil
	case []any:
		out.WriteByte('[')
		for i, item := range value {
			if i > 0 {
				out.WriteByte(',')
			}
			if err := writeCanonicalValue(out, item); err != nil {
				return err
			}
		}
		out.WriteByte(']')
		return nil
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		out.WriteByte('{')
		for i, key := range keys {
			if i > 0 {
				out.WriteByte(',')
			}
			writeCanonicalString(out, key)
			out.WriteByte(':')
			if err := writeCanonicalValue(out, value[key]); err != nil {
				return err
			}
		}
		out.WriteByte('}')
		return nil
	case []byte:
		return errors.New("byte slices are not JSON values")
	case nil:
		out.WriteString("null")
	case bool:
		if value {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		writeCanonicalString(out, value)
	default:
		return fmt.Errorf("unsupported JSON value %T", value)
	}
	return nil
}

func writeCanonicalString(out *bytes.Buffer, value string) {
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\':
			out.WriteString(`\\`)
		case '"':
			out.WriteString(`\"`)
		case '\b':
			out.WriteString(`\b`)
		case '\f':
			out.WriteString(`\f`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		case '\t':
			out.WriteString(`\t`)
		default:
			if r < 0x20 || r > 0x7e {
				if r <= 0xffff {
					fmt.Fprintf(out, `\u%04x`, r)
				} else {
					r -= 0x10000
					fmt.Fprintf(out, `\u%04x\u%04x`, 0xd800+(r>>10), 0xdc00+(r&0x3ff))
				}
			} else {
				out.WriteRune(r)
			}
		}
	}
	out.WriteByte('"')
}

// decodeStrictValue rejects duplicate object keys and malformed trailing
// values. encoding/json's ordinary Unmarshal silently keeps the last duplicate
// key, which is unsafe at an authority boundary because two readers could
// reason about different representations of the same record.
func decodeStrictValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch delimiter := token.(type) {
	case json.Delim:
		switch delimiter {
		case '{':
			object := make(map[string]any)
			for decoder.More() {
				keyToken, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				key, ok := keyToken.(string)
				if !ok {
					return nil, errors.New("object key is not a string")
				}
				if _, exists := object[key]; exists {
					return nil, fmt.Errorf("duplicate object key %q", key)
				}
				value, err := decodeStrictValue(decoder)
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim('}') {
				return nil, errors.New("object is not closed")
			}
			return object, nil
		case '[':
			array := make([]any, 0)
			for decoder.More() {
				value, err := decodeStrictValue(decoder)
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			closing, err := decoder.Token()
			if err != nil || closing != json.Delim(']') {
				return nil, errors.New("array is not closed")
			}
			return array, nil
		default:
			return nil, fmt.Errorf("unexpected JSON delimiter %q", delimiter)
		}
	default:
		return token, nil
	}
}

func arrayValues(raw []byte) ([]json.RawMessage, error) {
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func requireSourceRef(raw []byte, name string) error {
	fields, err := objectFields(raw)
	if err != nil {
		return fmt.Errorf("%s must be an object", name)
	}
	for key := range fields {
		if key != "kind" && key != "id" && key != "content_hash" {
			return fmt.Errorf("%s has unknown field %q", name, key)
		}
	}
	if err := requireNonEmptyString(fields["kind"], name+".kind"); err != nil {
		return err
	}
	if err := requireNonEmptyString(fields["id"], name+".id"); err != nil {
		return err
	}
	if hash, ok := fields["content_hash"]; ok {
		if err := requireSHA(hash, name+".content_hash"); err != nil {
			return err
		}
	}
	return nil
}

func validateSourceRefsRaw(raw []byte) error {
	values, err := arrayValues(raw)
	if err != nil || len(values) == 0 {
		return invalid("source_refs must be a non-empty array")
	}
	for i, value := range values {
		if err := requireSourceRef(value, fmt.Sprintf("source_refs[%d]", i)); err != nil {
			return invalid("%v", err)
		}
	}
	return nil
}

func validateSourceRaw(raw []byte) error {
	fields, err := objectFields(raw)
	if err != nil {
		return invalid("source: %v", err)
	}
	for key := range fields {
		if key != "id" && key != "role" {
			return invalid("source has unknown field %q", key)
		}
	}
	if err := requireNonEmptyString(fields["id"], "source.id"); err != nil {
		return invalid("%v", err)
	}
	return nil
}

func requireNonEmptyString(raw []byte, name string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must be a non-empty string", name)
	}
	return nil
}

func requireOneOfString(raw []byte, name string, values ...string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !isOneOf(value, values...) {
		return fmt.Errorf("%s has an unsupported value", name)
	}
	return nil
}

func requireSHA(raw []byte, name string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !sha256Pattern.MatchString(value) {
		return fmt.Errorf("%s must be lowercase sha256", name)
	}
	return nil
}

func requireHexDigest(raw []byte, name string, length int) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || len(value) != length {
		return fmt.Errorf("%s must be %d lowercase hexadecimal characters", name, length)
	}
	if _, err := hex.DecodeString(value); err != nil || strings.ToLower(value) != value {
		return fmt.Errorf("%s must be %d lowercase hexadecimal characters", name, length)
	}
	return nil
}

func requireGitObject(raw []byte, name string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !gitObjectPattern.MatchString(value) {
		return fmt.Errorf("%s must be a git object id", name)
	}
	return nil
}

func requireDate(raw []byte, name string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !datePattern.MatchString(value) {
		return fmt.Errorf("%s must be UTC without fractional seconds", name)
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z", value); err != nil {
		return fmt.Errorf("%s is invalid: %v", name, err)
	}
	return nil
}

func requirePath(raw []byte, name string) error {
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || value == "" || strings.HasPrefix(value, "/") || strings.Contains(value, "../") || strings.HasSuffix(value, "/..") || value == ".." {
		return fmt.Errorf("%s must be a relative path without parent traversal", name)
	}
	return nil
}

func requireNonEmptyStringArray(raw []byte, name string, paths bool) error {
	values, err := arrayValues(raw)
	if err != nil || len(values) == 0 {
		return fmt.Errorf("%s must be a non-empty string array", name)
	}
	for _, value := range values {
		if paths {
			if err := requirePath(value, name); err != nil {
				return err
			}
		} else if err := requireNonEmptyString(value, name); err != nil {
			return err
		}
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRecord, fmt.Sprintf(format, args...))
}
func isOneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
