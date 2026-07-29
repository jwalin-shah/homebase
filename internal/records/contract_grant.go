package records

// This file owns the authenticated, atomic Contract + CapabilityGrant
// admission boundary. A worker/Bridge may consume the pair, but cannot mint
// either authority record.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"homebase/internal/journal"
)

type ContractGrantCommitResult struct {
	Sequence uint64
	Existing bool
	Contract Record
	Grant    Record
}

type contractGrantCommit struct {
	Contract json.RawMessage `json:"contract"`
	Grant    json.RawMessage `json:"grant"`
}

type storedContractGrant struct {
	contractID string
	grantID    string
	canonical  []byte
	sequence   uint64
}

type preparedContractGrant struct {
	canonical         []byte
	contractCanonical []byte
	grantCanonical    []byte
	contract          Record
	grant             Record
}

// AppendContractAndGrant persists the approved task contract and its scoped
// capability grant in one journal entry. The caller must already have
// authenticated the owner authority; this package enforces the record schema,
// lineage, and atomic/idempotent storage semantics.
func (s *Store) AppendContractAndGrant(contractRaw, grantRaw []byte) (ContractGrantCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return ContractGrantCommitResult{}, err
	}
	prepared, err := s.prepareContractGrantCommit(contractRaw, grantRaw, true)
	if err != nil {
		return ContractGrantCommitResult{}, err
	}
	if existing, ok := s.contractGrants[prepared.contract.ID]; ok {
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return ContractGrantCommitResult{
				Sequence: existing.sequence,
				Existing: true,
				Contract: prepared.contract,
				Grant:    prepared.grant,
			}, nil
		}
		return ContractGrantCommitResult{}, fmt.Errorf("%w: contract %s", ErrConflict, prepared.contract.ID)
	}

	payload, err := journal.EncodeRecord(journal.RecordKindContractGrantCommit, prepared.canonical)
	if err != nil {
		return ContractGrantCommitResult{}, fmt.Errorf("encode contract/grant commit: %w", err)
	}
	sequence, err := s.journal.Append(payload)
	if err != nil {
		return ContractGrantCommitResult{}, s.poison(fmt.Errorf("append contract/grant commit: %w", err))
	}
	s.applyPreparedContractGrant(prepared, sequence)
	return ContractGrantCommitResult{Sequence: sequence, Contract: prepared.contract, Grant: prepared.grant}, nil
}

func (s *Store) prepareContractGrantCommit(contractRaw, grantRaw []byte, allowExisting bool) (preparedContractGrant, error) {
	contract, contractCanonical, err := parseAndValidate(contractRaw)
	if err != nil {
		return preparedContractGrant{}, err
	}
	if contract.Kind != "Contract" {
		return preparedContractGrant{}, invalid("contract/grant commit contract must have kind Contract")
	}
	grant, grantCanonical, err := parseAndValidate(grantRaw)
	if err != nil {
		return preparedContractGrant{}, err
	}
	if grant.Kind != "CapabilityGrant" {
		return preparedContractGrant{}, invalid("contract/grant commit grant must have kind CapabilityGrant")
	}
	grantFields, err := objectFields(grant.Payload)
	if err != nil {
		return preparedContractGrant{}, err
	}
	grantContractID, ok := stringValue(grantFields["contract_id"])
	if !ok || grantContractID != contract.ID {
		return preparedContractGrant{}, invalid("CapabilityGrant contract_id must reference committed Contract %q", contract.ID)
	}

	candidate := s.cloneForVerification()
	if existing, ok := candidate.records[contract.ID]; ok {
		if !allowExisting || !bytes.Equal(existing.canonical, contractCanonical) {
			return preparedContractGrant{}, fmt.Errorf("%w: contract %s", ErrConflict, contract.ID)
		}
	} else {
		if err := candidate.validateReferences(contract); err != nil {
			return preparedContractGrant{}, err
		}
		candidate.records[contract.ID] = storedRecord{record: contract, canonical: contractCanonical}
	}
	if existing, ok := candidate.records[grant.ID]; ok {
		if !allowExisting || !bytes.Equal(existing.canonical, grantCanonical) {
			return preparedContractGrant{}, fmt.Errorf("%w: grant %s", ErrConflict, grant.ID)
		}
	} else {
		if err := candidate.validateReferences(grant); err != nil {
			return preparedContractGrant{}, err
		}
		candidate.records[grant.ID] = storedRecord{record: grant, canonical: grantCanonical}
	}
	commitRaw, err := json.Marshal(contractGrantCommit{Contract: contractCanonical, Grant: grantCanonical})
	if err != nil {
		return preparedContractGrant{}, fmt.Errorf("encode contract/grant commit: %w", err)
	}
	canonical, err := canonicalObject(commitRaw)
	if err != nil {
		return preparedContractGrant{}, fmt.Errorf("canonicalize contract/grant commit: %w", err)
	}
	return preparedContractGrant{
		canonical: canonical, contractCanonical: contractCanonical, grantCanonical: grantCanonical,
		contract: contract, grant: grant,
	}, nil
}

func (s *Store) applyPreparedContractGrant(prepared preparedContractGrant, sequence uint64) {
	s.records[prepared.contract.ID] = storedRecord{record: prepared.contract, canonical: prepared.contractCanonical}
	s.records[prepared.grant.ID] = storedRecord{record: prepared.grant, canonical: prepared.grantCanonical}
	s.contractGrants[prepared.contract.ID] = storedContractGrant{
		contractID: prepared.contract.ID, grantID: prepared.grant.ID,
		canonical: bytes.Clone(prepared.canonical), sequence: sequence,
	}
}

func (s *Store) replayContractGrantCommit(sequence uint64, raw []byte) error {
	commit, err := decodeContractGrantCommit(raw)
	if err != nil {
		return err
	}
	prepared, err := s.prepareContractGrantCommit(commit.Contract, commit.Grant, true)
	if err != nil {
		return err
	}
	if existing, ok := s.contractGrants[prepared.contract.ID]; ok {
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return nil
		}
		return fmt.Errorf("%w: contract %s", ErrConflict, prepared.contract.ID)
	}
	s.applyPreparedContractGrant(prepared, sequence)
	return nil
}

func decodeContractGrantCommit(raw []byte) (contractGrantCommit, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return contractGrantCommit{}, invalid("contract/grant commit: %v", err)
	}
	for key := range fields {
		if key != "contract" && key != "grant" {
			return contractGrantCommit{}, invalid("contract/grant commit has unknown field %q", key)
		}
	}
	contractRaw, ok := fields["contract"]
	if !ok {
		return contractGrantCommit{}, invalid("contract/grant commit missing contract")
	}
	grantRaw, ok := fields["grant"]
	if !ok {
		return contractGrantCommit{}, invalid("contract/grant commit missing grant")
	}
	contractCanonical, err := CanonicalJSONValue(contractRaw)
	if err != nil {
		return contractGrantCommit{}, invalid("contract/grant commit contract: %v", err)
	}
	grantCanonical, err := CanonicalJSONValue(grantRaw)
	if err != nil {
		return contractGrantCommit{}, invalid("contract/grant commit grant: %v", err)
	}
	return contractGrantCommit{Contract: contractCanonical, Grant: grantCanonical}, nil
}
