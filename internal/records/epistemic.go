package records

import (
	"bytes"
	"encoding/json"
	"fmt"

	"homebase/internal/journal"
)

// EpistemicAcceptanceCommitResult is one atomic research-evidence bundle.
// Its receipt attests to provenance validation and intake, not to truth of claims.
type EpistemicAcceptanceCommitResult struct {
	Sequence   uint64
	Existing   bool
	Submission string
	Records    []Record
	Receipt    json.RawMessage
}

type epistemicAcceptanceCommit struct {
	Submission string            `json:"submission_id"`
	RequestSHA string            `json:"request_sha256"`
	Records    []json.RawMessage `json:"records"`
	Receipt    json.RawMessage   `json:"receipt"`
}

type storedEpistemicAcceptance struct {
	canonical  []byte
	requestSHA string
	sequence   uint64
	receipt    json.RawMessage
	records    []Record
}

// AppendEpistemicAcceptanceCommit atomically stores Evidence, Claim and Proof
// records plus a signed epistemic receipt in a dedicated journal event. It is
// separate from transcript promotion and never creates captain approval.
func (s *Store) AppendEpistemicAcceptanceCommit(submission, requestSHA string, recordRaw [][]byte, receiptRaw []byte) (EpistemicAcceptanceCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return EpistemicAcceptanceCommitResult{}, err
	}
	commit, parsed, _, err := s.validateEpistemicAcceptance(submission, requestSHA, recordRaw, receiptRaw)
	if err != nil {
		return EpistemicAcceptanceCommitResult{}, err
	}
	if existing, ok := s.epistemicAcceptances[submission]; ok {
		if existing.requestSHA == requestSHA {
			return EpistemicAcceptanceCommitResult{Sequence: existing.sequence, Existing: true, Submission: submission, Records: append([]Record(nil), existing.records...), Receipt: bytes.Clone(existing.receipt)}, nil
		}
		return EpistemicAcceptanceCommitResult{}, fmt.Errorf("%w: epistemic submission %s", ErrConflict, submission)
	}
	for _, record := range parsed {
		if _, ok := s.records[record.ID]; ok {
			return EpistemicAcceptanceCommitResult{}, fmt.Errorf("%w: record %s already belongs to another commit", ErrConflict, record.ID)
		}
	}
	payloadRaw, err := json.Marshal(commit)
	if err != nil {
		return EpistemicAcceptanceCommitResult{}, fmt.Errorf("encode epistemic acceptance commit: %w", err)
	}
	payloadCanonical, err := canonicalObject(payloadRaw)
	if err != nil {
		return EpistemicAcceptanceCommitResult{}, fmt.Errorf("canonicalize epistemic acceptance commit: %w", err)
	}
	envelope, err := journal.EncodeRecord(journal.RecordKindEpistemicAcceptanceCommit, payloadCanonical)
	if err != nil {
		return EpistemicAcceptanceCommitResult{}, fmt.Errorf("encode epistemic acceptance journal record: %w", err)
	}
	sequence, err := s.journal.Append(envelope)
	if err != nil {
		return EpistemicAcceptanceCommitResult{}, s.poison(fmt.Errorf("append epistemic acceptance commit: %w", err))
	}
	for i, record := range parsed {
		_, normalized, parseErr := parseAndValidate(recordRaw[i])
		if parseErr != nil { // Already validated above; retain fail-closed behavior.
			return EpistemicAcceptanceCommitResult{}, s.poison(parseErr)
		}
		s.records[record.ID] = storedRecord{record: record, canonical: normalized}
	}
	s.epistemicAcceptances[submission] = storedEpistemicAcceptance{canonical: payloadCanonical, requestSHA: requestSHA, sequence: sequence, receipt: bytes.Clone(commit.Receipt), records: append([]Record(nil), parsed...)}
	return EpistemicAcceptanceCommitResult{Sequence: sequence, Submission: submission, Records: parsed, Receipt: bytes.Clone(commit.Receipt)}, nil
}

func (s *Store) validateEpistemicAcceptance(submission, requestSHA string, recordRaw [][]byte, receiptRaw []byte) (epistemicAcceptanceCommit, []Record, []byte, error) {
	if submission == "" || !sha256Pattern.MatchString(requestSHA) || len(recordRaw) < 3 {
		return epistemicAcceptanceCommit{}, nil, nil, invalid("epistemic submission and Evidence, Claim, and Proof records are required")
	}
	parsed := make([]Record, 0, len(recordRaw))
	canonicalRecords := make([]json.RawMessage, 0, len(recordRaw))
	byID := make(map[string]Record, len(recordRaw))
	evidence := make(map[string]Record)
	claims := make(map[string]Record)
	var proof *Record
	for _, raw := range recordRaw {
		record, canonical, err := parseAndValidate(raw)
		if err != nil {
			return epistemicAcceptanceCommit{}, nil, nil, err
		}
		if _, duplicate := byID[record.ID]; duplicate {
			return epistemicAcceptanceCommit{}, nil, nil, invalid("duplicate record id %q in epistemic bundle", record.ID)
		}
		if err := s.validateReferences(record); err != nil {
			return epistemicAcceptanceCommit{}, nil, nil, err
		}
		byID[record.ID] = record
		switch record.Kind {
		case "Evidence":
			evidence[record.ID] = record
		case "Claim":
			claims[record.ID] = record
		case "Proof":
			if proof != nil {
				return epistemicAcceptanceCommit{}, nil, nil, invalid("epistemic bundle must contain exactly one acceptance Proof")
			}
			copy := record
			proof = &copy
		default:
			return epistemicAcceptanceCommit{}, nil, nil, invalid("epistemic bundle cannot contain %s records", record.Kind)
		}
		parsed = append(parsed, record)
		canonicalRecords = append(canonicalRecords, canonical)
	}
	if len(evidence) == 0 || len(claims) == 0 || proof == nil {
		return epistemicAcceptanceCommit{}, nil, nil, invalid("epistemic bundle requires Evidence, Claim, and one signed acceptance Proof")
	}
	for _, claim := range claims {
		found := false
		payload, _ := objectFields(claim.Payload)
		status, _ := stringValue(payload["epistemic_status"])
		for _, ref := range claim.SourceRefs {
			if ref.Kind == "evidence" {
				if source, ok := evidence[ref.ID]; ok && (ref.ContentHash == "" || ref.ContentHash == source.ContentHash) {
					found = true
				}
			}
		}
		if !found && status != "REJECTED" {
			return epistemicAcceptanceCommit{}, nil, nil, invalid("Claim %q must reference bundled Evidence with matching content hash", claim.ID)
		}
	}
	proofEvidence := make(map[string]bool)
	proofClaims := make(map[string]bool)
	for _, ref := range proof.SourceRefs {
		if ref.Kind == "evidence" {
			if source, ok := evidence[ref.ID]; !ok || ref.ContentHash != source.ContentHash {
				return epistemicAcceptanceCommit{}, nil, nil, invalid("acceptance Proof references missing or mismatched Evidence %q", ref.ID)
			}
			proofEvidence[ref.ID] = true
		}
		if ref.Kind == "claim" {
			if source, ok := claims[ref.ID]; !ok || ref.ContentHash != source.ContentHash {
				return epistemicAcceptanceCommit{}, nil, nil, invalid("acceptance Proof references missing or mismatched Claim %q", ref.ID)
			}
			proofClaims[ref.ID] = true
		}
	}
	if len(proofEvidence) != len(evidence) || len(proofClaims) != len(claims) {
		return epistemicAcceptanceCommit{}, nil, nil, invalid("acceptance Proof must bind every Evidence and Claim record")
	}
	if err := validateEpistemicReceipt(proof, submission, requestSHA); err != nil {
		return epistemicAcceptanceCommit{}, nil, nil, err
	}
	receiptCanonical, err := canonicalObject(receiptRaw)
	if err != nil {
		return epistemicAcceptanceCommit{}, nil, nil, invalid("epistemic receipt: %v", err)
	}
	proofPayload, _ := objectFields(proof.Payload)
	var embedded json.RawMessage
	_ = json.Unmarshal(proofPayload["epistemic_acceptance_receipt"], &embedded)
	embeddedCanonical, err := canonicalObject(embedded)
	if err != nil || !bytes.Equal(receiptCanonical, embeddedCanonical) {
		return epistemicAcceptanceCommit{}, nil, nil, invalid("Proof receipt payload does not match receipt object")
	}
	commit := epistemicAcceptanceCommit{Submission: submission, RequestSHA: requestSHA, Records: canonicalRecords, Receipt: receiptCanonical}
	commitRaw, err := json.Marshal(commit)
	if err != nil {
		return epistemicAcceptanceCommit{}, nil, nil, err
	}
	canonical, err := canonicalObject(commitRaw)
	return commit, parsed, canonical, err
}

func validateEpistemicReceipt(proof *Record, submission, requestSHA string) error {
	fields, err := objectFields(proof.Payload)
	if err != nil {
		return invalid("acceptance Proof payload: %v", err)
	}
	var receipt struct {
		SubmissionID  string `json:"submission_id"`
		SubmissionSHA string `json:"submission_sha256"`
		Accepted      bool   `json:"accepted"`
	}
	if err := json.Unmarshal(fields["epistemic_acceptance_receipt"], &receipt); err != nil || receipt.SubmissionID != submission || receipt.SubmissionSHA != requestSHA || !receipt.Accepted {
		return invalid("acceptance Proof must contain accepted receipt for submission %q", submission)
	}
	return nil
}

func (s *Store) replayEpistemicAcceptanceCommit(sequence uint64, raw []byte) error {
	var commit epistemicAcceptanceCommit
	if err := json.Unmarshal(raw, &commit); err != nil {
		return invalid("decode epistemic acceptance commit: %v", err)
	}
	if commit.Submission == "" || !sha256Pattern.MatchString(commit.RequestSHA) || len(commit.Records) < 3 || len(commit.Receipt) == 0 {
		return invalid("epistemic acceptance commit is incomplete")
	}
	recordRaw := make([][]byte, len(commit.Records))
	for i := range commit.Records {
		recordRaw[i] = bytes.Clone(commit.Records[i])
	}
	validated, parsed, canonical, err := s.validateEpistemicAcceptance(commit.Submission, commit.RequestSHA, recordRaw, commit.Receipt)
	if err != nil {
		return err
	}
	_ = validated
	if previous, ok := s.epistemicAcceptances[commit.Submission]; ok {
		if previous.requestSHA != commit.RequestSHA {
			return fmt.Errorf("%w: epistemic submission %s", ErrConflict, commit.Submission)
		}
		return nil
	}
	for i, record := range parsed {
		_, normalized, err := parseAndValidate(recordRaw[i])
		if err != nil {
			return err
		}
		if err := s.load(record, normalized); err != nil {
			return err
		}
	}
	s.epistemicAcceptances[commit.Submission] = storedEpistemicAcceptance{canonical: canonical, requestSHA: commit.RequestSHA, sequence: sequence, receipt: bytes.Clone(commit.Receipt), records: append([]Record(nil), parsed...)}
	return nil
}
