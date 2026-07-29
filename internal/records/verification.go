package records

// This file owns HomeBase's Bridge verification submission boundary. Bridge
// may submit worker assertions and verifier proofs, but it may not create or
// replace the Contract or CapabilityGrant that authorizes the work.

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"homebase/internal/journal"
	"sort"
	"strings"
	"time"
)

const (
	productionVerifierID = "bridge:verifier:v2"
	attestationScheme    = "ed25519-content-hash-v1"
)

type VerificationCommitResult struct {
	Sequence uint64
	Existing bool
	Records  []Record
	Receipt  Record
}

type verificationCommit struct {
	Records []json.RawMessage `json:"records"`
}

type storedVerification struct {
	receiptID string
	canonical []byte
	sequence  uint64
	recordIDs []string
}

type bridgeSubject struct {
	Kind   string `json:"kind"`
	Name   string `json:"name"`
	Digest struct {
		SHA256 string `json:"sha256"`
	} `json:"digest"`
}

type bridgeCheck struct {
	Name         string            `json:"name"`
	Result       string            `json:"result"`
	ProofCommand string            `json:"proof_command"`
	EvidenceRef  SourceRef         `json:"evidence_ref"`
	Provenance   *bridgeProvenance `json:"provenance,omitempty"`
}

type bridgeProvenance struct {
	SchemaVersion     string            `json:"schema_version"`
	Command           []string          `json:"command"`
	CommandDigest     string            `json:"command_digest"`
	EnvironmentDigest string            `json:"environment_digest"`
	OutputDigest      string            `json:"output_digest"`
	LogDigest         string            `json:"log_digest"`
	CheckoutSHA       string            `json:"checkout_sha"`
	VerifierVersion   string            `json:"verifier_version"`
	ToolVersions      map[string]string `json:"tool_versions"`
	CacheStatus       string            `json:"cache_status"`
}

type bridgeAttestation struct {
	Scheme    string `json:"scheme"`
	KeyID     string `json:"key_id"`
	Signature string `json:"signature"`
}

// bridgeReceipt is the transport shape emitted by Bridge. tree_sha and
// worker_statement are transport-only fields; the durable HomeBase payload
// intentionally remains the schema-defined VerificationReceipt payload.
type bridgeReceipt struct {
	Kind            string             `json:"kind"`
	Version         string             `json:"version"`
	ID              string             `json:"id"`
	ContentHash     string             `json:"content_hash"`
	TaskID          string             `json:"task_id"`
	ContractID      string             `json:"contract_id"`
	GrantID         string             `json:"grant_id"`
	WorkerID        string             `json:"worker_id"`
	VerifierID      string             `json:"verifier_id"`
	TreeSHA         string             `json:"tree_sha"`
	TreeDigest      string             `json:"tree_digest"`
	Subject         bridgeSubject      `json:"subject"`
	Checks          []bridgeCheck      `json:"checks"`
	EvidenceRefs    []SourceRef        `json:"evidence_refs"`
	WorkerClaimRef  SourceRef          `json:"worker_claim_ref"`
	VerifiedAt      string             `json:"verified_at"`
	WorkerStatement string             `json:"worker_statement"`
	Attestation     *bridgeAttestation `json:"attestation,omitempty"`
}

// VerifyBridgeReceiptAttestation checks the verifier-only signature after the
// Bridge transport signature has authenticated the submitting client. The two
// signatures answer different questions and must not be conflated.
func VerifyBridgeReceiptAttestation(raw []byte, publicKey ed25519.PublicKey, expectedKeyID string) error {
	receipt, err := decodeBridgeReceipt(raw)
	if err != nil {
		return err
	}
	if receipt.VerifierID != productionVerifierID {
		return nil
	}
	if len(publicKey) != ed25519.PublicKeySize {
		return fmt.Errorf("verifier attestation public key is not configured")
	}
	if receipt.Attestation == nil || receipt.Attestation.Scheme != attestationScheme || strings.TrimSpace(receipt.Attestation.KeyID) == "" {
		return invalid("production verification receipt is missing verifier attestation")
	}
	if expectedKeyID != "" && receipt.Attestation.KeyID != expectedKeyID {
		return invalid("verifier attestation key id is not enrolled")
	}
	signature, err := hex.DecodeString(receipt.Attestation.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return invalid("verifier attestation signature is malformed")
	}
	message := []byte("running-machine-verifier-attestation:v1:" + receipt.ID + ":" + receipt.ContentHash + "\x00" + receipt.WorkerStatement)
	if !ed25519.Verify(publicKey, message, signature) {
		return invalid("verifier attestation signature did not verify")
	}
	return nil
}

func (s *Store) AppendBridgeVerificationSubmission(raw []byte) (VerificationCommitResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ensureHealthy(); err != nil {
		return VerificationCommitResult{}, err
	}

	receipt, err := decodeBridgeReceipt(raw)
	if err != nil {
		return VerificationCommitResult{}, err
	}
	if existingID, existingTree, ok := s.verificationForAuthority(receipt.TaskID, receipt.ContractID, receipt.GrantID); ok && existingID != receipt.ID {
		return VerificationCommitResult{}, fmt.Errorf("%w: task %q already has verification receipt %q for tree %q", ErrConflict, receipt.TaskID, existingID, existingTree)
	}
	if existing, ok := s.verifications[receipt.ID]; ok {
		recordsRaw, err := buildBridgeRecords(receipt, s)
		if err != nil {
			return VerificationCommitResult{}, err
		}
		prepared, err := s.prepareVerificationCommit(recordsRaw, true)
		if err != nil {
			return VerificationCommitResult{}, err
		}
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return s.verificationResultForExisting(existing), nil
		}
		return VerificationCommitResult{}, fmt.Errorf("%w: verification receipt %s", ErrConflict, receipt.ID)
	}
	if err := s.validateVerificationSubmissionFreshness(receipt); err != nil {
		return VerificationCommitResult{}, err
	}

	recordsRaw, err := buildBridgeRecords(receipt, s)
	if err != nil {
		return VerificationCommitResult{}, err
	}
	prepared, err := s.prepareVerificationCommit(recordsRaw, false)
	if err != nil {
		return VerificationCommitResult{}, err
	}
	payload, err := journalEncodeVerification(prepared.canonical)
	if err != nil {
		return VerificationCommitResult{}, err
	}
	sequence, err := s.journal.Append(payload)
	if err != nil {
		return VerificationCommitResult{}, s.poison(fmt.Errorf("append verification commit: %w", err))
	}
	s.applyPreparedVerification(prepared, sequence)
	return VerificationCommitResult{Sequence: sequence, Records: prepared.records, Receipt: prepared.receipt}, nil
}

// validateVerificationSubmissionFreshness checks the authority at the moment
// HomeBase is asked to commit a new receipt. Historical records remain
// replayable after expiry, but a new effect cannot use an expired grant or
// contract. A small future-clock allowance handles bounded transport skew.
func (s *Store) validateVerificationSubmissionFreshness(receipt bridgeReceipt) error {
	contract, err := s.requireRecordID(receipt.ContractID, "contract_id", "Contract")
	if err != nil {
		return err
	}
	grant, err := s.requireRecordID(receipt.GrantID, "grant_id", "CapabilityGrant")
	if err != nil {
		return err
	}
	contractFields, err := objectFields(contract.record.Payload)
	if err != nil {
		return err
	}
	grantFields, err := objectFields(grant.record.Payload)
	if err != nil {
		return err
	}
	verifiedAt, err := time.Parse("2006-01-02T15:04:05Z", receipt.VerifiedAt)
	if err != nil {
		return invalid("VerificationReceipt verified_at: %v", err)
	}
	grantExpiresText, ok := stringValue(grantFields["expires_at"])
	if !ok {
		return invalid("CapabilityGrant expires_at is missing for receipt submission freshness")
	}
	grantExpires, err := time.Parse("2006-01-02T15:04:05Z", grantExpiresText)
	if err != nil {
		return invalid("CapabilityGrant expires_at: %v", err)
	}
	contextUntilText, ok := stringValue(contractFields["context_valid_until"])
	if !ok {
		return invalid("Contract context_valid_until is missing for receipt submission freshness")
	}
	contextUntil, err := time.Parse("2006-01-02T15:04:05Z", contextUntilText)
	if err != nil {
		return invalid("Contract context_valid_until: %v", err)
	}
	now := s.now().UTC()
	if !now.Before(grantExpires) {
		return invalid("CapabilityGrant expired before verification submission")
	}
	if !now.Before(contextUntil) {
		return invalid("Contract context expired before verification submission")
	}
	if verifiedAt.After(now.Add(2 * time.Minute)) {
		return invalid("VerificationReceipt verified_at is in the future at submission")
	}
	return nil
}

// verificationForAuthority enforces one terminal verifier receipt for one
// admitted task/authority tuple. Receipt IDs include the tree, so indexing
// only by receipt ID would incorrectly permit a second terminal receipt for a
// different tree after a retry or stale worker.
func (s *Store) verificationForAuthority(taskID, contractID, grantID string) (receiptID, treeSHA string, ok bool) {
	for _, stored := range s.records {
		if stored.record.Kind != "VerificationReceipt" {
			continue
		}
		fields, err := objectFields(stored.record.Payload)
		if err != nil {
			continue
		}
		storedTask, _ := stringValue(fields["task_id"])
		storedContract, _ := stringValue(fields["contract_id"])
		storedGrant, _ := stringValue(fields["grant_id"])
		if storedTask != taskID || storedContract != contractID || storedGrant != grantID {
			continue
		}
		storedTree := ""
		if subjectFields, subjectErr := objectFields(fields["subject"]); subjectErr == nil {
			storedTree, _ = stringValue(subjectFields["name"])
		}
		return stored.record.ID, storedTree, true
	}
	return "", "", false
}

type preparedVerification struct {
	canonical  []byte
	receiptID  string
	records    []Record
	canonicals [][]byte
	receipt    Record
}

func (s *Store) prepareVerificationCommit(rawRecords []json.RawMessage, allowExisting bool) (preparedVerification, error) {
	if len(rawRecords) < 3 {
		return preparedVerification{}, invalid("Bridge verification commit requires a worker claim, at least one proof, and a receipt")
	}

	type parsed struct {
		record    Record
		canonical []byte
	}
	parsedRecords := make([]parsed, 0, len(rawRecords))
	seenIDs := make(map[string]struct{}, len(rawRecords))
	for _, raw := range rawRecords {
		record, canonical, err := parseAndValidate(raw)
		if err != nil {
			return preparedVerification{}, err
		}
		if record.Kind != "Claim" && record.Kind != "Observation" && record.Kind != "Proof" && record.Kind != "VerificationReceipt" {
			return preparedVerification{}, fmt.Errorf("%w: Bridge cannot submit %s records", ErrAuthorityRequired, record.Kind)
		}
		if _, exists := seenIDs[record.ID]; exists {
			return preparedVerification{}, fmt.Errorf("%w: duplicate verification record %s", ErrConflict, record.ID)
		}
		seenIDs[record.ID] = struct{}{}
		parsedRecords = append(parsedRecords, parsed{record: record, canonical: canonical})
	}

	sort.Slice(parsedRecords, func(i, j int) bool {
		priority := func(kind string) int {
			switch kind {
			case "Claim", "Observation":
				return 0
			case "Proof":
				return 1
			default:
				return 2
			}
		}
		pi, pj := priority(parsedRecords[i].record.Kind), priority(parsedRecords[j].record.Kind)
		if pi != pj {
			return pi < pj
		}
		return parsedRecords[i].record.ID < parsedRecords[j].record.ID
	})

	var receipt Record
	receiptCount := 0
	for _, item := range parsedRecords {
		if item.record.Kind == "VerificationReceipt" {
			receipt = item.record
			receiptCount++
		}
	}
	if receiptCount != 1 {
		return preparedVerification{}, invalid("Bridge verification commit requires exactly one VerificationReceipt")
	}
	receiptFields, err := objectFields(receipt.Payload)
	if err != nil {
		return preparedVerification{}, invalid("Bridge receipt payload: %v", err)
	}
	workerRef, err := decodeSourceRef(receiptFields["worker_claim_ref"], "worker_claim_ref")
	if err != nil {
		return preparedVerification{}, err
	}
	evidenceRefs, err := arrayValues(receiptFields["evidence_refs"])
	if err != nil || len(evidenceRefs) == 0 {
		return preparedVerification{}, invalid("Bridge receipt evidence_refs must be non-empty")
	}
	for _, refRaw := range append([]json.RawMessage{mustSourceRefJSON(workerRef)}, evidenceRefs...) {
		ref, err := decodeSourceRef(refRaw, "verification reference")
		if err != nil {
			return preparedVerification{}, err
		}
		if _, ok := seenIDs[ref.ID]; !ok {
			return preparedVerification{}, invalid("Bridge receipt references record %q that is not in this atomic submission", ref.ID)
		}
	}
	for _, item := range parsedRecords {
		if item.record.Kind == "Proof" {
			if !sourceRefListed(item.record.ID, evidenceRefs) {
				return preparedVerification{}, invalid("Bridge submission contains unreferenced Proof %q", item.record.ID)
			}
		}
	}

	candidate := s.cloneForVerification()
	for _, item := range parsedRecords {
		if existing, exists := candidate.records[item.record.ID]; exists {
			if !allowExisting || !bytes.Equal(existing.canonical, item.canonical) {
				return preparedVerification{}, fmt.Errorf("%w: verification record %s", ErrConflict, item.record.ID)
			}
		}
		if err := candidate.validateReferences(item.record); err != nil {
			return preparedVerification{}, err
		}
		candidate.records[item.record.ID] = storedRecord{record: item.record, canonical: item.canonical}
	}

	canonicalRecords := make([]json.RawMessage, 0, len(parsedRecords))
	records := make([]Record, 0, len(parsedRecords))
	canonicals := make([][]byte, 0, len(parsedRecords))
	for _, item := range parsedRecords {
		canonicalRecords = append(canonicalRecords, item.canonical)
		records = append(records, item.record)
		canonicals = append(canonicals, item.canonical)
	}
	commitRaw, err := json.Marshal(verificationCommit{Records: canonicalRecords})
	if err != nil {
		return preparedVerification{}, fmt.Errorf("encode verification commit: %w", err)
	}
	commitCanonical, err := canonicalObject(commitRaw)
	if err != nil {
		return preparedVerification{}, fmt.Errorf("canonicalize verification commit: %w", err)
	}
	return preparedVerification{canonical: commitCanonical, receiptID: receipt.ID, records: records, canonicals: canonicals, receipt: receipt}, nil
}

func (s *Store) cloneForVerification() *Store {
	clone := &Store{records: make(map[string]storedRecord, len(s.records)), verifications: make(map[string]storedVerification, len(s.verifications))}
	for id, record := range s.records {
		clone.records[id] = storedRecord{record: record.record, canonical: bytes.Clone(record.canonical)}
	}
	for id, verification := range s.verifications {
		clone.verifications[id] = verification
	}
	return clone
}

func (s *Store) applyPreparedVerification(prepared preparedVerification, sequence uint64) {
	for index, record := range prepared.records {
		s.records[record.ID] = storedRecord{record: record, canonical: prepared.canonicals[index]}
	}
	recordIDs := make([]string, 0, len(prepared.records))
	for _, record := range prepared.records {
		recordIDs = append(recordIDs, record.ID)
	}
	s.verifications[prepared.receiptID] = storedVerification{receiptID: prepared.receiptID, canonical: bytes.Clone(prepared.canonical), sequence: sequence, recordIDs: recordIDs}
}

func (s *Store) replayVerificationCommit(sequence uint64, raw []byte) error {
	commit, err := decodeVerificationCommit(raw)
	if err != nil {
		return err
	}
	prepared, err := s.prepareVerificationCommit(commit.Records, true)
	if err != nil {
		return err
	}
	if existing, ok := s.verifications[prepared.receiptID]; ok {
		if bytes.Equal(existing.canonical, prepared.canonical) {
			return nil
		}
		return fmt.Errorf("%w: verification receipt %s", ErrConflict, prepared.receiptID)
	}
	s.applyPreparedVerification(prepared, sequence)
	return nil
}

func (s *Store) verificationResultForExisting(existing storedVerification) VerificationCommitResult {
	records := make([]Record, 0, len(existing.recordIDs))
	for _, id := range existing.recordIDs {
		if record, ok := s.records[id]; ok {
			records = append(records, record.record)
		}
	}
	receipt, ok := s.records[existing.receiptID]
	if !ok {
		return VerificationCommitResult{Sequence: existing.sequence, Existing: true, Records: records}
	}
	return VerificationCommitResult{Sequence: existing.sequence, Existing: true, Records: records, Receipt: receipt.record}
}

func decodeVerificationCommit(raw []byte) (verificationCommit, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return verificationCommit{}, invalid("verification commit: %v", err)
	}
	for key := range fields {
		if key != "records" {
			return verificationCommit{}, invalid("verification commit has unknown field %q", key)
		}
	}
	values, err := arrayValues(fields["records"])
	if err != nil || len(values) == 0 {
		return verificationCommit{}, invalid("verification commit records must be a non-empty array")
	}
	canonical := make([]json.RawMessage, 0, len(values))
	for _, value := range values {
		object, err := canonicalObject(value)
		if err != nil {
			return verificationCommit{}, invalid("verification commit record: %v", err)
		}
		canonical = append(canonical, object)
	}
	return verificationCommit{Records: canonical}, nil
}

func journalEncodeVerification(canonical []byte) ([]byte, error) {
	return journal.EncodeRecord(journal.RecordKindVerificationCommit, canonical)
}

func sourceRefListed(id string, refs []json.RawMessage) bool {
	for _, raw := range refs {
		ref, err := decodeSourceRef(raw, "evidence_ref")
		if err == nil && ref.ID == id {
			return true
		}
	}
	return false
}

func decodeSourceRef(raw []byte, field string) (SourceRef, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return SourceRef{}, invalid("%s: %v", field, err)
	}
	for key := range fields {
		if key != "kind" && key != "id" && key != "content_hash" {
			return SourceRef{}, invalid("%s has unknown field %q", field, key)
		}
	}
	var ref SourceRef
	if err := json.Unmarshal(raw, &ref); err != nil || ref.Kind == "" || ref.ID == "" {
		return SourceRef{}, invalid("%s requires kind and id", field)
	}
	return ref, nil
}

func mustSourceRefJSON(ref SourceRef) json.RawMessage {
	raw, _ := json.Marshal(ref)
	return raw
}

func decodeBridgeReceipt(raw []byte) (bridgeReceipt, error) {
	fields, err := objectFields(raw)
	if err != nil {
		return bridgeReceipt{}, invalid("Bridge verification receipt: %v", err)
	}
	allowed := map[string]bool{
		"kind": true, "version": true, "id": true, "content_hash": true,
		"task_id": true, "contract_id": true, "grant_id": true, "worker_id": true,
		"verifier_id": true, "tree_sha": true, "tree_digest": true, "subject": true,
		"checks": true, "evidence_refs": true, "worker_claim_ref": true,
		"verified_at": true, "worker_statement": true, "attestation": true,
	}
	for key := range fields {
		if !allowed[key] {
			return bridgeReceipt{}, invalid("Bridge verification receipt has unknown field %q", key)
		}
	}
	for _, key := range []string{"kind", "version", "id", "content_hash", "task_id", "contract_id", "grant_id", "worker_id", "verifier_id", "tree_sha", "tree_digest", "subject", "checks", "evidence_refs", "worker_claim_ref", "verified_at", "worker_statement"} {
		if _, ok := fields[key]; !ok {
			return bridgeReceipt{}, invalid("Bridge verification receipt missing %q", key)
		}
	}
	var receipt bridgeReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil {
		return bridgeReceipt{}, invalid("Bridge verification receipt decode: %v", err)
	}
	if receipt.Kind != "VerificationReceipt" || receipt.Version != "1" || receipt.ID == "" || receipt.TaskID == "" || receipt.ContractID == "" || receipt.GrantID == "" || receipt.WorkerID == "" || receipt.VerifierID == "" || receipt.WorkerStatement == "" {
		return bridgeReceipt{}, invalid("Bridge verification receipt has invalid identity fields")
	}
	if receipt.WorkerID == receipt.VerifierID {
		return bridgeReceipt{}, invalid("Bridge verification receipt worker and verifier identities must be distinct")
	}
	if receipt.VerifierID == productionVerifierID {
		if receipt.Attestation == nil || receipt.Attestation.Scheme != attestationScheme || strings.TrimSpace(receipt.Attestation.KeyID) == "" {
			return bridgeReceipt{}, invalid("production verification receipt attestation is missing or malformed")
		}
		signature, err := hex.DecodeString(receipt.Attestation.Signature)
		if err != nil || len(signature) != ed25519.SignatureSize {
			return bridgeReceipt{}, invalid("production verification receipt attestation signature is malformed")
		}
	}
	if receipt.ID != "receipt:"+receipt.TaskID+":"+receipt.TreeSHA {
		return bridgeReceipt{}, invalid("Bridge verification receipt id is not bound to task and tree")
	}
	if !gitObjectPattern.MatchString(receipt.TreeSHA) || !sha256Pattern.MatchString(receipt.TreeDigest) || receipt.Subject.Kind != "git_tree" || receipt.Subject.Name != receipt.TreeSHA || receipt.Subject.Digest.SHA256 != receipt.TreeDigest {
		return bridgeReceipt{}, invalid("Bridge verification receipt tree binding is invalid")
	}
	if _, err := time.Parse("2006-01-02T15:04:05Z", receipt.VerifiedAt); err != nil {
		return bridgeReceipt{}, invalid("Bridge verification receipt verified_at is invalid")
	}
	if receipt.WorkerClaimRef.Kind != "claim" || receipt.WorkerClaimRef.ID == "" || len(receipt.Checks) == 0 || len(receipt.Checks) != len(receipt.EvidenceRefs) {
		return bridgeReceipt{}, invalid("Bridge verification receipt references are invalid")
	}
	seen := make(map[string]bool, len(receipt.EvidenceRefs))
	for index, check := range receipt.Checks {
		if check.Name == "" || check.Result != "passed" || check.ProofCommand == "" || check.EvidenceRef.Kind != "proof" || check.EvidenceRef.ID == "" || check.EvidenceRef != receipt.EvidenceRefs[index] || seen[check.EvidenceRef.ID] {
			return bridgeReceipt{}, invalid("Bridge verification receipt check %d is invalid", index)
		}
		if check.Provenance == nil {
			if receipt.VerifierID == "bridge:verifier" {
				return bridgeReceipt{}, invalid("Bridge verification receipt check %d is missing provenance", index)
			}
		} else if err := validateBridgeProvenance(*check.Provenance, receipt.TreeSHA); err != nil {
			return bridgeReceipt{}, invalid("Bridge verification receipt check %d provenance: %v", index, err)
		}
		seen[check.EvidenceRef.ID] = true
	}
	if err := verifyBridgeReceiptContentHash(receipt); err != nil {
		return bridgeReceipt{}, err
	}
	return receipt, nil
}

func verifyBridgeReceiptContentHash(receipt bridgeReceipt) error {
	checks := make([]any, 0, len(receipt.Checks))
	for _, check := range receipt.Checks {
		value := map[string]any{
			"name":         check.Name,
			"result":       check.Result,
			"evidence_ref": map[string]any{"kind": check.EvidenceRef.Kind, "id": check.EvidenceRef.ID},
		}
		if check.Provenance != nil {
			value["proof_command"] = check.ProofCommand
			value["provenance"] = bridgeProvenanceAsAny(*check.Provenance)
		}
		checks = append(checks, value)
	}
	payload := map[string]any{
		"task_id": receipt.TaskID, "contract_id": receipt.ContractID, "grant_id": receipt.GrantID,
		"worker_id": receipt.WorkerID, "verifier_id": receipt.VerifierID, "tree_digest": receipt.TreeDigest,
		"subject":          map[string]any{"kind": receipt.Subject.Kind, "name": receipt.Subject.Name, "digest": map[string]any{"sha256": receipt.Subject.Digest.SHA256}},
		"checks":           checks,
		"evidence_refs":    sourceRefsAsAny(receipt.EvidenceRefs),
		"worker_claim_ref": map[string]any{"kind": receipt.WorkerClaimRef.Kind, "id": receipt.WorkerClaimRef.ID},
		"verified_at":      receipt.VerifiedAt,
	}
	hash, err := canonicalHashValue(payload)
	if err != nil {
		return invalid("Bridge verification receipt payload: %v", err)
	}
	if hash != receipt.ContentHash {
		return invalid("Bridge verification receipt content_hash does not match payload")
	}
	return nil
}

func sourceRefsAsAny(refs []SourceRef) []any {
	result := make([]any, 0, len(refs))
	for _, ref := range refs {
		value := map[string]any{"kind": ref.Kind, "id": ref.ID}
		if ref.ContentHash != "" {
			value["content_hash"] = ref.ContentHash
		}
		result = append(result, value)
	}
	return result
}

func canonicalHashValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := canonicalObject(raw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func buildBridgeRecords(receipt bridgeReceipt, s *Store) ([]json.RawMessage, error) {
	contract, err := s.requireRecordID(receipt.ContractID, "contract_id", "Contract")
	if err != nil {
		return nil, err
	}
	grant, err := s.requireRecordID(receipt.GrantID, "grant_id", "CapabilityGrant")
	if err != nil {
		return nil, err
	}
	contractFields, err := objectFields(contract.record.Payload)
	if err != nil {
		return nil, err
	}
	grantFields, err := objectFields(grant.record.Payload)
	if err != nil {
		return nil, err
	}
	for _, pair := range [][2]string{{"task_id", receipt.TaskID}, {"worker_id", receipt.WorkerID}} {
		if !sameStringField(contractFields, pair[0], pair[1]) {
			return nil, invalid("Bridge receipt does not match Contract %q field %s", receipt.ContractID, pair[0])
		}
	}
	if !sameStringField(contractFields, "verifier_id", receipt.VerifierID) {
		return nil, invalid("Bridge receipt verifier_id does not match Contract %q", receipt.ContractID)
	}
	for _, pair := range [][2]string{{"contract_id", receipt.ContractID}, {"task_id", receipt.TaskID}, {"worker_id", receipt.WorkerID}} {
		if !sameStringField(grantFields, pair[0], pair[1]) {
			return nil, invalid("Bridge receipt does not match CapabilityGrant %q field %s", receipt.GrantID, pair[0])
		}
	}

	contractRef := SourceRef{Kind: "contract", ID: contract.record.ID, ContentHash: contract.record.ContentHash}
	grantRef := SourceRef{Kind: "grant", ID: grant.record.ID, ContentHash: grant.record.ContentHash}
	claimPayload := map[string]any{"statement": receipt.WorkerStatement, "subject_refs": []any{sourceRefAsAny(contractRef), sourceRefAsAny(grantRef)}}
	claimRaw, err := bridgeEnvelope("Claim", receipt.WorkerClaimRef.ID, []SourceRef{contractRef, grantRef}, AuthorityWorkerObservation, "observed", receipt.WorkerID, "worker", receipt.VerifiedAt, "worker assertion", claimPayload)
	if err != nil {
		return nil, err
	}
	claim, claimCanonical, err := parseAndValidate(claimRaw)
	if err != nil {
		return nil, err
	}
	claimRef := SourceRef{Kind: "claim", ID: claim.ID, ContentHash: claim.ContentHash}

	recordsRaw := []json.RawMessage{claimCanonical}
	proofRefs := make([]SourceRef, 0, len(receipt.Checks))
	for _, check := range receipt.Checks {
		treeRef := SourceRef{Kind: "git_tree", ID: receipt.TreeSHA}
		proofPayload := map[string]any{"proof_type": "bridge_independent_verification", "proof_command": check.ProofCommand, "result": "passed", "subject_refs": []any{sourceRefAsAny(contractRef), sourceRefAsAny(grantRef), sourceRefAsAny(treeRef)}, "verifier_id": receipt.VerifierID}
		proofRaw, err := bridgeEnvelope("Proof", check.EvidenceRef.ID, []SourceRef{contractRef, grantRef, treeRef}, AuthorityVerifiedEvidence, "verified", receipt.VerifierID, "verifier", receipt.VerifiedAt, "independent verifier result", proofPayload)
		if err != nil {
			return nil, err
		}
		proof, proofCanonical, err := parseAndValidate(proofRaw)
		if err != nil {
			return nil, err
		}
		proofRefs = append(proofRefs, SourceRef{Kind: "proof", ID: proof.ID, ContentHash: proof.ContentHash})
		recordsRaw = append(recordsRaw, proofCanonical)
	}

	durablePayload := map[string]any{
		"task_id": receipt.TaskID, "contract_id": receipt.ContractID, "grant_id": receipt.GrantID,
		"worker_id": receipt.WorkerID, "verifier_id": receipt.VerifierID, "tree_digest": receipt.TreeDigest,
		"subject": map[string]any{"kind": "git_tree", "name": receipt.TreeSHA, "digest": map[string]any{"sha256": receipt.TreeDigest}},
		"checks":  durableChecks(receipt.Checks), "evidence_refs": sourceRefsAsAny(receipt.EvidenceRefs),
		"worker_claim_ref": sourceRefAsAny(receipt.WorkerClaimRef), "verified_at": receipt.VerifiedAt,
	}
	contentHash, err := canonicalHashValue(durablePayload)
	if err != nil {
		return nil, err
	}
	if contentHash != receipt.ContentHash {
		return nil, invalid("Bridge receipt payload does not match durable record payload")
	}
	receiptSourceRefs := append([]SourceRef{contractRef, grantRef, claimRef}, proofRefs...)
	receiptRaw, err := bridgeEnvelope("VerificationReceipt", receipt.ID, receiptSourceRefs, AuthorityVerifiedEvidence, "verified", receipt.VerifierID, "verifier", receipt.VerifiedAt, "independent verification bound to contract, grant, and Git tree", durablePayload)
	if err != nil {
		return nil, err
	}
	recordsRaw = append(recordsRaw, receiptRaw)
	return recordsRaw, nil
}

func durableChecks(checks []bridgeCheck) []any {
	result := make([]any, 0, len(checks))
	for _, check := range checks {
		value := map[string]any{"name": check.Name, "result": check.Result, "evidence_ref": map[string]any{"kind": check.EvidenceRef.Kind, "id": check.EvidenceRef.ID}}
		if check.Provenance != nil {
			value["proof_command"] = check.ProofCommand
			value["provenance"] = bridgeProvenanceAsAny(*check.Provenance)
		}
		result = append(result, value)
	}
	return result
}

func validateBridgeProvenance(provenance bridgeProvenance, treeSHA string) error {
	if provenance.SchemaVersion != "1" || len(provenance.Command) == 0 || provenance.CheckoutSHA != treeSHA || provenance.VerifierVersion == "" {
		return fmt.Errorf("identity is incomplete or not bound to tree")
	}
	for _, arg := range provenance.Command {
		if arg == "" {
			return fmt.Errorf("command contains an empty argument")
		}
	}
	commandDigest, err := canonicalHashAnyValue(provenance.Command)
	if err != nil || commandDigest != provenance.CommandDigest {
		return fmt.Errorf("command digest does not match command")
	}
	if !sha256Pattern.MatchString(provenance.EnvironmentDigest) || !sha256Pattern.MatchString(provenance.OutputDigest) || !sha256Pattern.MatchString(provenance.LogDigest) {
		return fmt.Errorf("digests are incomplete")
	}
	if len(provenance.ToolVersions) == 0 {
		return fmt.Errorf("tool_versions are required")
	}
	for name, version := range provenance.ToolVersions {
		if name == "" || version == "" {
			return fmt.Errorf("tool_versions contain an empty entry")
		}
	}
	if provenance.CacheStatus != "hit" && provenance.CacheStatus != "miss" && provenance.CacheStatus != "not_applicable" {
		return fmt.Errorf("cache_status %q is unsupported", provenance.CacheStatus)
	}
	return nil
}

func canonicalHashAnyValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	canonical, err := CanonicalJSONValue(raw)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func bridgeProvenanceAsAny(provenance bridgeProvenance) map[string]any {
	tools := make(map[string]any, len(provenance.ToolVersions))
	for name, version := range provenance.ToolVersions {
		tools[name] = version
	}
	return map[string]any{
		"schema_version": provenance.SchemaVersion, "command": append([]string(nil), provenance.Command...),
		"command_digest": provenance.CommandDigest, "environment_digest": provenance.EnvironmentDigest,
		"output_digest": provenance.OutputDigest, "log_digest": provenance.LogDigest,
		"checkout_sha": provenance.CheckoutSHA, "verifier_version": provenance.VerifierVersion,
		"tool_versions": tools, "cache_status": provenance.CacheStatus,
	}
}

func sourceRefAsAny(ref SourceRef) map[string]any {
	value := map[string]any{"kind": ref.Kind, "id": ref.ID}
	if ref.ContentHash != "" {
		value["content_hash"] = ref.ContentHash
	}
	return value
}

func bridgeEnvelope(kind, id string, sourceRefs []SourceRef, authority, status, sourceID, sourceRole, capturedAt, freshnessReason string, payload any) ([]byte, error) {
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	contentHash, err := CanonicalContentHash(payloadRaw)
	if err != nil {
		return nil, err
	}
	envelope := map[string]any{
		"kind": kind, "version": "1", "id": id, "source_refs": sourceRefsAsAny(sourceRefs),
		"content_hash": contentHash, "captured_at": capturedAt, "authority_class": authority,
		"freshness": map[string]any{"mode": "source_bound", "valid_until": nil, "reason": freshnessReason},
		"status":    status, "source": map[string]any{"id": sourceID, "role": sourceRole}, "payload": payload,
	}
	return json.Marshal(envelope)
}
