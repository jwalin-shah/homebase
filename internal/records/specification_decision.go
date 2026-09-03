package records

// This file owns the authenticated, atomic Specification + Decision
// admission boundary: the smallest owner-signed path that can create a
// captain-approved Specification together with the Decision that approves
// it. Nothing else may mint either record, and AppendExternal continues to
// reject both kinds outright.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"homebase/internal/journal"
)

// SpecificationDecisionCommitResult reports the durable outcome of one
// Specification+Decision commit, whether newly written or replayed as an
// identical duplicate.
type SpecificationDecisionCommitResult struct {
	Sequence      uint64
	Existing      bool
	Specification Record
	Decision      Record
}

type specificationDecisionCommit struct {
	Specification json.RawMessage `json:"specification"`
	Decision      json.RawMessage `json:"decision"`
}

type storedSpecificationDecision struct {
	specificationID string
	decisionID      string
	canonical       []byte
	sequence        uint64
}

type preparedSpecificationDecision struct {
	canonical              []byte
	specificationCanonical []byte
	decisionCanonical      []byte
	specification          Record
	decision               Record
}

// AppendSpecificationAndDecision persists a captain-approved Specification
// and its approving Decision in one journal entry. The caller must already
// have authenticated the owner authority (the captain/contract signing key);
// this package still enforces the full approval chain, record schema, and
// atomic/idempotent storage semantics. Because the Specification's
// approval_ref and the Decision's specification_ref cite each other by ID
// and content hash, neither record can be admitted alone through the
// generic external boundary.
func (s *Store) AppendSpecificationAndDecision(specificationRaw, decisionRaw []byte) (SpecificationDecisionCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return SpecificationDecisionCommitResult{}, err
	}
	prepared, err := s.prepareSpecificationDecisionCommit(specificationRaw, decisionRaw, true)
	if err != nil {
		return SpecificationDecisionCommitResult{}, err
	}
	if existing, ok := s.specificationDecisions[prepared.specification.ID]; ok {
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return SpecificationDecisionCommitResult{
				Sequence:      existing.sequence,
				Existing:      true,
				Specification: prepared.specification,
				Decision:      prepared.decision,
			}, nil
		}
		return SpecificationDecisionCommitResult{}, fmt.Errorf("%w: specification %s", ErrConflict, prepared.specification.ID)
	}

	payload, err := journal.EncodeRecord(journal.RecordKindSpecificationDecisionCommit, prepared.canonical)
	if err != nil {
		return SpecificationDecisionCommitResult{}, fmt.Errorf("encode specification/decision commit: %w", err)
	}
	sequence, err := s.journal.Append(payload)
	if err != nil {
		return SpecificationDecisionCommitResult{}, s.poison(fmt.Errorf("append specification/decision commit: %w", err))
	}
	s.applyPreparedSpecificationDecision(prepared, sequence)
	return SpecificationDecisionCommitResult{Sequence: sequence, Specification: prepared.specification, Decision: prepared.decision}, nil
}

func (s *Store) prepareSpecificationDecisionCommit(specificationRaw, decisionRaw []byte, allowExisting bool) (preparedSpecificationDecision, error) {
	specification, specificationCanonical, err := parseAndValidate(specificationRaw)
	if err != nil {
		return preparedSpecificationDecision{}, err
	}
	if specification.Kind != "Specification" {
		return preparedSpecificationDecision{}, invalid("specification/decision commit specification must have kind Specification")
	}
	if specification.Status != "approved" {
		return preparedSpecificationDecision{}, invalid("specification/decision commit specification must have status approved")
	}
	decision, decisionCanonical, err := parseAndValidate(decisionRaw)
	if err != nil {
		return preparedSpecificationDecision{}, err
	}
	if decision.Kind != "Decision" {
		return preparedSpecificationDecision{}, invalid("specification/decision commit decision must have kind Decision")
	}
	if decision.Status != "approved" || decision.Source.Role != "captain" {
		return preparedSpecificationDecision{}, invalid("specification/decision commit decision must be a captain-approved Decision")
	}
	decisionFields, err := objectFields(decision.Payload)
	if err != nil {
		return preparedSpecificationDecision{}, err
	}
	specificationRefRaw, ok := decisionFields["specification_ref"]
	if !ok {
		return preparedSpecificationDecision{}, invalid("specification/decision commit decision requires payload.specification_ref")
	}
	specificationRef, err := parseRequiredSourceRef(specificationRefRaw, "specification/decision commit decision specification_ref")
	if err != nil {
		return preparedSpecificationDecision{}, err
	}
	if specificationRef.Kind != "specification" || specificationRef.ID != specification.ID || specificationRef.ContentHash != specification.ContentHash {
		return preparedSpecificationDecision{}, invalid("specification/decision commit decision specification_ref does not match Specification %q and its content hash", specification.ID)
	}

	candidate := s.cloneForVerification()
	if existing, ok := candidate.records[decision.ID]; ok {
		if !allowExisting || !bytes.Equal(existing.canonical, decisionCanonical) {
			return preparedSpecificationDecision{}, fmt.Errorf("%w: decision %s", ErrConflict, decision.ID)
		}
	} else {
		if err := candidate.validateReferences(decision); err != nil {
			return preparedSpecificationDecision{}, err
		}
		candidate.records[decision.ID] = storedRecord{record: decision, canonical: decisionCanonical}
	}
	if existing, ok := candidate.records[specification.ID]; ok {
		if !allowExisting || !bytes.Equal(existing.canonical, specificationCanonical) {
			return preparedSpecificationDecision{}, fmt.Errorf("%w: specification %s", ErrConflict, specification.ID)
		}
	} else {
		if err := candidate.validateReferences(specification); err != nil {
			return preparedSpecificationDecision{}, err
		}
		candidate.records[specification.ID] = storedRecord{record: specification, canonical: specificationCanonical}
	}

	commitRaw, err := json.Marshal(specificationDecisionCommit{Specification: specificationCanonical, Decision: decisionCanonical})
	if err != nil {
		return preparedSpecificationDecision{}, fmt.Errorf("encode specification/decision commit: %w", err)
	}
	canonical, err := canonicalObject(commitRaw)
	if err != nil {
		return preparedSpecificationDecision{}, fmt.Errorf("canonicalize specification/decision commit: %w", err)
	}
	return preparedSpecificationDecision{
		canonical: canonical, specificationCanonical: specificationCanonical, decisionCanonical: decisionCanonical,
		specification: specification, decision: decision,
	}, nil
}

func (s *Store) applyPreparedSpecificationDecision(prepared preparedSpecificationDecision, sequence uint64) {
	s.records[prepared.decision.ID] = storedRecord{record: prepared.decision, canonical: prepared.decisionCanonical}
	s.records[prepared.specification.ID] = storedRecord{record: prepared.specification, canonical: prepared.specificationCanonical}
	s.specificationDecisions[prepared.specification.ID] = storedSpecificationDecision{
		specificationID: prepared.specification.ID, decisionID: prepared.decision.ID,
		canonical: bytes.Clone(prepared.canonical), sequence: sequence,
	}
}

func (s *Store) replaySpecificationDecisionCommit(sequence uint64, raw []byte) error {
	commit, err := decodeSpecificationDecisionCommit(raw)
	if err != nil {
		return err
	}
	prepared, err := s.prepareSpecificationDecisionCommit(commit.Specification, commit.Decision, true)
	if err != nil {
		return err
	}
	if existing, ok := s.specificationDecisions[prepared.specification.ID]; ok {
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return nil
		}
		return fmt.Errorf("%w: specification %s", ErrConflict, prepared.specification.ID)
	}
	s.applyPreparedSpecificationDecision(prepared, sequence)
	return nil
}

func decodeSpecificationDecisionCommit(raw []byte) (specificationDecisionCommit, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return specificationDecisionCommit{}, invalid("specification/decision commit: %v", err)
	}
	for key := range fields {
		if key != "specification" && key != "decision" {
			return specificationDecisionCommit{}, invalid("specification/decision commit has unknown field %q", key)
		}
	}
	specificationRaw, ok := fields["specification"]
	if !ok {
		return specificationDecisionCommit{}, invalid("specification/decision commit missing specification")
	}
	decisionRaw, ok := fields["decision"]
	if !ok {
		return specificationDecisionCommit{}, invalid("specification/decision commit missing decision")
	}
	specificationCanonical, err := CanonicalJSONValue(specificationRaw)
	if err != nil {
		return specificationDecisionCommit{}, invalid("specification/decision commit specification: %v", err)
	}
	decisionCanonical, err := CanonicalJSONValue(decisionRaw)
	if err != nil {
		return specificationDecisionCommit{}, invalid("specification/decision commit decision: %v", err)
	}
	return specificationDecisionCommit{Specification: specificationCanonical, Decision: decisionCanonical}, nil
}
