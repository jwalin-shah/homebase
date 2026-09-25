// Package epistemic accepts authenticated, provenance-bound research bundles.
// Acceptance certifies intake and lineage checks, not truth of source claims.
package epistemic

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strings"
	"time"

	"homebase/internal/records"
)

var (
	ErrInvalid         = errors.New("invalid epistemic submission")
	ErrUnauthenticated = errors.New("epistemic submission authentication failed")
)

type ProviderResearchRun struct {
	RunID                string `json:"run_id"`
	Provider             string `json:"provider"`
	ProviderAccountRef   string `json:"provider_account_ref,omitempty"`
	ExecutionRoute       string `json:"execution_route,omitempty"`
	InputSHA256          string `json:"input_sha256"`
	OutputSHA256         string `json:"output_sha256"`
	SourceArtifactSHA256 string `json:"source_artifact_sha256"`
	StartedAt            string `json:"started_at"`
	CompletedAt          string `json:"completed_at"`
	RetrievedAt          string `json:"retrieved_at"`
}

type EvidenceItem struct {
	EvidenceID       string `json:"evidence_id"`
	RunID            string `json:"run_id"`
	SourceURI        string `json:"source_uri"`
	Publisher        string `json:"publisher_identity"`
	ContentSHA256    string `json:"content_sha256"`
	LineageSHA256    string `json:"lineage_sha256"`
	SourceTimestamp  string `json:"source_timestamp,omitempty"`
	RetrievedAt      string `json:"retrieved_at"`
	ContentAvailable bool   `json:"content_available"`
}

type Claim struct {
	ClaimID           string   `json:"claim_id"`
	Text              string   `json:"normalized_claim"`
	EpistemicStatus   string   `json:"epistemic_status"`
	EvidenceRefs      []string `json:"evidence_refs"`
	ContradictionRefs []string `json:"contradiction_refs"`
}

type Request struct {
	Version                string                `json:"version"`
	SubmissionID           string                `json:"submission_id"`
	InputSHA256            string                `json:"input_sha256"`
	OutputSHA256           string                `json:"output_sha256"`
	FreshnessMaxAgeSeconds int64                 `json:"freshness_max_age_seconds"`
	Runs                   []ProviderResearchRun `json:"provider_research_runs"`
	Evidence               []EvidenceItem        `json:"evidence_items"`
	Claims                 []Claim               `json:"claims"`
}

type AcceptanceReceipt struct {
	Version             string            `json:"version"`
	Accepted            bool              `json:"accepted"`
	ReceiptID           string            `json:"receipt_id"`
	SubmissionID        string            `json:"submission_id"`
	SubmissionSHA256    string            `json:"submission_sha256"`
	AcceptedAt          string            `json:"accepted_at"`
	AcceptingAuthority  string            `json:"accepting_authority"`
	ClaimsAssertedTrue  bool              `json:"claims_asserted_true"`
	VerifierKeyID       string            `json:"verifier_key_id"`
	InputSHA256         string            `json:"input_sha256"`
	OutputSHA256        string            `json:"output_sha256"`
	RunIDs              []string          `json:"provider_research_run_ids"`
	EvidenceIDs         []string          `json:"evidence_ids"`
	ClaimStatuses       map[string]string `json:"claim_statuses"`
	SourceLineageSHA256 []string          `json:"source_lineage_sha256"`
	Signature           string            `json:"signature"`
}

type Outcome struct {
	Accepted  bool              `json:"accepted"`
	Existing  bool              `json:"existing"`
	Sequence  uint64            `json:"sequence,omitempty"`
	Receipt   AcceptanceReceipt `json:"receipt"`
	RecordIDs []string          `json:"record_ids"`
}

type Service struct {
	store      *records.Store
	principal  string
	requestKey ed25519.PublicKey
	receiptID  string
	receiptKey ed25519.PrivateKey
	now        func() time.Time
}

func NewService(store *records.Store, principal string, requestPublic ed25519.PublicKey, receiptKeyID string, receiptPrivate ed25519.PrivateKey, now func() time.Time) (*Service, error) {
	if store == nil || strings.TrimSpace(principal) == "" || len(requestPublic) != ed25519.PublicKeySize || strings.TrimSpace(receiptKeyID) == "" || len(receiptPrivate) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("%w: store, request public key, receipt key, and key ids are required", ErrInvalid)
	}
	if now == nil {
		now = time.Now
	}
	service := &Service{store: store, principal: strings.TrimSpace(principal), requestKey: bytes.Clone(requestPublic), receiptID: strings.TrimSpace(receiptKeyID), receiptKey: bytes.Clone(receiptPrivate), now: now}
	if err := service.verifyDurableReceipts(); err != nil {
		return nil, fmt.Errorf("%w: durable receipt verification failed: %v", ErrInvalid, err)
	}
	return service, nil
}

func (s *Service) verifyDurableReceipts() error {
	publicKey := s.receiptKey.Public().(ed25519.PublicKey)
	for _, record := range s.store.List() {
		if record.Kind != "Proof" {
			continue
		}
		var payload struct {
			Receipt json.RawMessage `json:"epistemic_acceptance_receipt"`
		}
		if err := json.Unmarshal(record.Payload, &payload); err != nil || len(payload.Receipt) == 0 {
			continue
		}
		var receipt AcceptanceReceipt
		if err := json.Unmarshal(payload.Receipt, &receipt); err != nil {
			return fmt.Errorf("receipt %s is malformed", record.ID)
		}
		if receipt.VerifierKeyID != s.receiptID {
			return fmt.Errorf("receipt %s uses unconfigured verifier key id", record.ID)
		}
		signature, err := hex.DecodeString(receipt.Signature)
		if err != nil || len(signature) != ed25519.SignatureSize {
			return fmt.Errorf("receipt %s has invalid signature encoding", record.ID)
		}
		receipt.Signature = ""
		unsigned, _ := json.Marshal(receipt)
		canonical, err := records.CanonicalJSONValue(unsigned)
		if err != nil || !ed25519.Verify(publicKey, canonical, signature) {
			return fmt.Errorf("receipt %s signature mismatch", record.ID)
		}
	}
	return nil
}

func (s *Service) Accept(raw, signature []byte) (Outcome, error) {
	if _, err := records.DecodeStrictJSON(raw); err != nil {
		return Outcome{}, fmt.Errorf("%w: JSON: %v", ErrInvalid, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var request Request
	if err := decoder.Decode(&request); err != nil {
		return Outcome{}, fmt.Errorf("%w: %v", ErrInvalid, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Outcome{}, fmt.Errorf("%w: expected exactly one JSON request", ErrInvalid)
	}
	canonical, err := records.CanonicalJSONValue(raw)
	if err != nil || !ed25519.Verify(s.requestKey, canonical, signature) {
		return Outcome{}, ErrUnauthenticated
	}
	if err := validateRequest(request, s.now().UTC()); err != nil {
		return Outcome{}, err
	}
	receipt := s.buildReceipt(request)
	requestDigest := sha256.Sum256(canonical)
	receipt.SubmissionSHA256 = hex.EncodeToString(requestDigest[:])
	receipt.Signature = ""
	unsigned, _ := json.Marshal(receipt)
	unsignedCanonical, err := records.CanonicalJSONValue(unsigned)
	if err != nil {
		return Outcome{}, err
	}
	receipt.Signature = hex.EncodeToString(ed25519.Sign(s.receiptKey, unsignedCanonical))
	receiptRaw, _ := json.Marshal(receipt)
	receiptCanonical, err := records.CanonicalJSONValue(receiptRaw)
	if err != nil {
		return Outcome{}, err
	}
	recordsRaw, recordIDs, err := s.recordsFor(request, receipt, receiptCanonical)
	if err != nil {
		return Outcome{}, err
	}
	commit, err := s.store.AppendEpistemicAcceptanceCommit(request.SubmissionID, receipt.SubmissionSHA256, recordsRaw, receiptCanonical)
	if err != nil {
		return Outcome{}, fmt.Errorf("epistemic commit: %w", err)
	}
	if commit.Existing {
		if err := json.Unmarshal(commit.Receipt, &receipt); err != nil {
			return Outcome{}, fmt.Errorf("decode existing epistemic receipt: %w", err)
		}
	}
	return Outcome{Accepted: true, Existing: commit.Existing, Sequence: commit.Sequence, Receipt: receipt, RecordIDs: recordIDs}, nil
}

func (s *Service) buildReceipt(request Request) AcceptanceReceipt {
	runIDs := make([]string, 0, len(request.Runs))
	lineages := make([]string, 0, len(request.Evidence))
	evidenceIDs := make([]string, 0, len(request.Evidence))
	statuses := make(map[string]string, len(request.Claims))
	for _, run := range request.Runs {
		runIDs = append(runIDs, run.RunID)
	}
	for _, evidence := range request.Evidence {
		lineages = append(lineages, evidence.LineageSHA256)
		evidenceIDs = append(evidenceIDs, evidence.EvidenceID)
	}
	for _, claim := range request.Claims {
		statuses[claim.ClaimID] = claim.EpistemicStatus
	}
	sort.Strings(runIDs)
	sort.Strings(evidenceIDs)
	sort.Strings(lineages)
	return AcceptanceReceipt{
		Version: "homebase.epistemic_acceptance.v1", Accepted: true, ReceiptID: "epistemic:" + request.SubmissionID,
		SubmissionID: request.SubmissionID, AcceptedAt: s.now().UTC().Truncate(time.Second).Format(time.RFC3339),
		AcceptingAuthority: s.principal, VerifierKeyID: s.receiptID, InputSHA256: request.InputSHA256,
		ClaimsAssertedTrue: false,
		OutputSHA256:       request.OutputSHA256, RunIDs: runIDs, EvidenceIDs: evidenceIDs,
		ClaimStatuses: statuses, SourceLineageSHA256: lineages,
	}
}

func validateRequest(request Request, now time.Time) error {
	if request.Version != "homebase.epistemic_submission.v1" || strings.TrimSpace(request.SubmissionID) == "" || !validHash(request.InputSHA256) || !validHash(request.OutputSHA256) {
		return fmt.Errorf("%w: version, submission id, and input/output hashes are required", ErrInvalid)
	}
	if request.FreshnessMaxAgeSeconds < 1 || request.FreshnessMaxAgeSeconds > int64((10*365*24*time.Hour).Seconds()) || len(request.Runs) == 0 || len(request.Evidence) == 0 || len(request.Claims) == 0 {
		return fmt.Errorf("%w: bounded freshness and non-empty runs, evidence, and claims are required", ErrInvalid)
	}
	runs := make(map[string]ProviderResearchRun, len(request.Runs))
	for _, run := range request.Runs {
		if strings.TrimSpace(run.RunID) == "" || strings.TrimSpace(run.Provider) == "" || !validHash(run.InputSHA256) || !validHash(run.OutputSHA256) || !validHash(run.SourceArtifactSHA256) || !validTime(run.StartedAt, now) || !validTime(run.CompletedAt, now) || !validTime(run.RetrievedAt, now) {
			return fmt.Errorf("%w: malformed provider research run", ErrInvalid)
		}
		started, _ := time.Parse(time.RFC3339, run.StartedAt)
		completed, _ := time.Parse(time.RFC3339, run.CompletedAt)
		retrieved, _ := time.Parse(time.RFC3339, run.RetrievedAt)
		if completed.Before(started) || retrieved.Before(started) || retrieved.After(completed) {
			return fmt.Errorf("%w: research run timestamps are out of order", ErrInvalid)
		}
		if _, exists := runs[run.RunID]; exists {
			return fmt.Errorf("%w: duplicate run id", ErrInvalid)
		}
		runs[run.RunID] = run
	}
	evidence := make(map[string]EvidenceItem, len(request.Evidence))
	lineagesByEvidence := make(map[string]string, len(request.Evidence))
	for _, item := range request.Evidence {
		parsedURI, uriErr := url.ParseRequestURI(item.SourceURI)
		run, runOK := runs[item.RunID]
		if strings.TrimSpace(item.EvidenceID) == "" || !runOK || uriErr != nil || parsedURI.Scheme != "https" || parsedURI.Host == "" || strings.TrimSpace(item.Publisher) == "" || !validHash(item.ContentSHA256) || !validHash(item.LineageSHA256) || !validTime(item.RetrievedAt, now) {
			return fmt.Errorf("%w: malformed evidence item", ErrInvalid)
		}
		lineage, err := sourceLineageSHA256(item.SourceURI)
		if err != nil || lineage != strings.ToLower(item.LineageSHA256) {
			return fmt.Errorf("%w: source lineage digest does not match canonical source URI", ErrInvalid)
		}
		started, _ := time.Parse(time.RFC3339, run.StartedAt)
		completed, _ := time.Parse(time.RFC3339, run.CompletedAt)
		retrieved, _ := time.Parse(time.RFC3339, item.RetrievedAt)
		if retrieved.Before(started) || retrieved.After(completed) {
			return fmt.Errorf("%w: evidence retrieval falls outside its provider run", ErrInvalid)
		}
		if _, exists := evidence[item.EvidenceID]; exists {
			return fmt.Errorf("%w: duplicate evidence id", ErrInvalid)
		}
		if item.SourceTimestamp != "" && !validTime(item.SourceTimestamp, now) {
			return fmt.Errorf("%w: invalid source timestamp", ErrInvalid)
		}
		// Bind run-level hashes to the top-level submission manifest.
		if run.InputSHA256 != request.InputSHA256 {
			return fmt.Errorf("%w: run input hash does not match submission", ErrInvalid)
		}
		evidence[item.EvidenceID] = item
		lineagesByEvidence[item.EvidenceID] = item.LineageSHA256
	}
	claimIDs := make(map[string]bool, len(request.Claims))
	for _, claim := range request.Claims {
		if strings.TrimSpace(claim.ClaimID) == "" || strings.TrimSpace(claim.Text) == "" || claimIDs[claim.ClaimID] || !oneOf(claim.EpistemicStatus, "CORROBORATED", "CONTESTED", "STALE", "UNVERIFIED", "REJECTED") {
			return fmt.Errorf("%w: malformed claim", ErrInvalid)
		}
		claimIDs[claim.ClaimID] = true
	}
	for _, claim := range request.Claims {
		lineages := make(map[string]bool)
		stale := false
		unknownFreshness := false
		for _, ref := range claim.EvidenceRefs {
			item, ok := evidence[ref]
			if !ok {
				return fmt.Errorf("%w: claim references missing evidence %q", ErrInvalid, ref)
			}
			lineages[lineagesByEvidence[ref]] = true
			if item.SourceTimestamp != "" {
				t, _ := time.Parse(time.RFC3339, item.SourceTimestamp)
				stale = stale || now.Sub(t) > time.Duration(request.FreshnessMaxAgeSeconds)*time.Second
				unknownFreshness = unknownFreshness || t.After(now)
			} else {
				unknownFreshness = true
			}
		}
		for _, ref := range claim.ContradictionRefs {
			if !claimIDs[ref] || ref == claim.ClaimID {
				return fmt.Errorf("%w: contradiction reference %q must resolve to another included claim", ErrInvalid, ref)
			}
		}
		switch claim.EpistemicStatus {
		case "CORROBORATED":
			if len(lineages) < 2 || stale || unknownFreshness || len(claim.ContradictionRefs) > 0 {
				return fmt.Errorf("%w: corroborated claim requires two fresh independent source lineages", ErrInvalid)
			}
		case "CONTESTED":
			if len(claim.ContradictionRefs) == 0 && !isReferencedAsContradicted(claim.ClaimID, request.Claims) {
				return fmt.Errorf("%w: contested claim requires a resolved contradiction reference", ErrInvalid)
			}
		case "STALE":
			if !stale {
				return fmt.Errorf("%w: stale claim lacks source older than freshness bound", ErrInvalid)
			}
		case "REJECTED":
			if len(claim.EvidenceRefs) != 0 && len(claim.ContradictionRefs) == 0 {
				return fmt.Errorf("%w: rejected sourced claim requires contradiction references", ErrInvalid)
			}
		case "UNVERIFIED":
			// Preserve unverified status even when some source evidence is present.
		}
	}
	return nil
}

func (s *Service) recordsFor(request Request, receipt AcceptanceReceipt, receiptCanonical []byte) ([][]byte, []string, error) {
	runs := make(map[string]ProviderResearchRun, len(request.Runs))
	for _, run := range request.Runs {
		runs[run.RunID] = run
	}
	evidenceRecords := make(map[string]records.Record, len(request.Evidence))
	all := make([]records.Record, 0, len(request.Evidence)+len(request.Claims)+1)
	for _, item := range request.Evidence {
		run := runs[item.RunID]
		captured := normalizeTime(item.RetrievedAt)
		payload := map[string]any{
			"evidence_type": "provider_research_source", "subject_refs": []records.SourceRef{{Kind: "provider_research_run", ID: run.RunID, ContentHash: run.OutputSHA256}},
			"observed_digest": item.ContentSHA256, "provider_research_run": run,
			"source_uri": item.SourceURI, "publisher_identity": item.Publisher,
			"retrieved_at": item.RetrievedAt, "source_lineage_sha256": item.LineageSHA256,
			"source_content_available": item.ContentAvailable,
			"freshness_status":         evidenceFreshness(item, request.FreshnessMaxAgeSeconds, s.now().UTC()),
			"input_sha256":             request.InputSHA256, "output_sha256": request.OutputSHA256,
		}
		if item.SourceTimestamp != "" {
			payload["source_timestamp"] = item.SourceTimestamp
		}
		_, record, err := makeRecord("Evidence", item.EvidenceID, payload, []records.SourceRef{{Kind: "provider_research_run", ID: run.RunID, ContentHash: run.OutputSHA256}}, records.AuthorityUntrustedText, "observed", records.Source{ID: s.principal, Role: "knowledge_engine"}, records.Freshness{Mode: "source_bound", ValidUntil: nil, Reason: "provider-source-freshness-is-preserved"}, captured)
		if err != nil {
			return nil, nil, err
		}
		evidenceRecords[item.EvidenceID] = record
		all = append(all, record)
	}
	claimRecords := make(map[string]records.Record, len(request.Claims))
	for _, claim := range request.Claims {
		refs := make([]records.SourceRef, 0, len(claim.EvidenceRefs))
		subjects := make([]records.SourceRef, 0, len(claim.EvidenceRefs))
		lineages := make([]string, 0, len(claim.EvidenceRefs))
		for _, id := range claim.EvidenceRefs {
			evidence := evidenceRecords[id]
			refs = append(refs, records.SourceRef{Kind: "evidence", ID: evidence.ID, ContentHash: evidence.ContentHash})
			subjects = append(subjects, records.SourceRef{Kind: "evidence", ID: evidence.ID, ContentHash: evidence.ContentHash})
			for _, item := range request.Evidence {
				if item.EvidenceID == id {
					lineages = append(lineages, item.LineageSHA256)
				}
			}
		}
		if len(refs) == 0 {
			refs = append(refs, records.SourceRef{Kind: "provider_research_run", ID: request.Runs[0].RunID, ContentHash: request.Runs[0].OutputSHA256})
			subjects = append(subjects, refs[0])
		}
		payload := map[string]any{"statement": claim.Text, "subject_refs": subjects, "epistemic_status": claim.EpistemicStatus, "source_lineage_refs": uniqueSorted(lineages), "promotion_submission_id": request.SubmissionID}
		if len(claim.ContradictionRefs) > 0 {
			payload["contradiction_refs"] = uniqueSorted(claim.ContradictionRefs)
		}
		_, record, err := makeRecord("Claim", claim.ClaimID, payload, refs, records.AuthorityUntrustedText, "observed", records.Source{ID: s.principal, Role: "knowledge_engine"}, records.Freshness{Mode: "source_bound", ValidUntil: nil, Reason: claim.EpistemicStatus}, normalizeTime(s.now().UTC().Format(time.RFC3339)))
		if err != nil {
			return nil, nil, err
		}
		claimRecords[claim.ClaimID] = record
		all = append(all, record)
	}
	proofRefs := make([]records.SourceRef, 0, len(evidenceRecords)+len(claimRecords))
	for _, record := range evidenceRecords {
		proofRefs = append(proofRefs, records.SourceRef{Kind: "evidence", ID: record.ID, ContentHash: record.ContentHash})
	}
	for _, record := range claimRecords {
		proofRefs = append(proofRefs, records.SourceRef{Kind: "claim", ID: record.ID, ContentHash: record.ContentHash})
	}
	sort.Slice(proofRefs, func(i, j int) bool {
		if proofRefs[i].Kind == proofRefs[j].Kind {
			return proofRefs[i].ID < proofRefs[j].ID
		}
		return proofRefs[i].Kind < proofRefs[j].Kind
	})
	proofPayload := map[string]any{"proof_type": "epistemic_acceptance", "proof_command": "validate-provider-research-provenance-v1", "result": "passed", "subject_refs": proofRefs, "verifier_id": s.receiptID, "epistemic_acceptance_receipt": json.RawMessage(receiptCanonical)}
	_, proof, err := makeRecord("Proof", receipt.ReceiptID, proofPayload, proofRefs, records.AuthorityVerifiedEvidence, "verified", records.Source{ID: s.receiptID, Role: "verifier"}, records.Freshness{Mode: "immutable", ValidUntil: nil}, normalizeTime(receipt.AcceptedAt))
	if err != nil {
		return nil, nil, err
	}
	all = append(all, proof)
	rawRecords := make([][]byte, 0, len(all))
	ids := make([]string, 0, len(all))
	for _, record := range all {
		raw, _ := json.Marshal(record)
		rawRecords = append(rawRecords, raw)
		ids = append(ids, record.ID)
	}
	sort.Strings(ids)
	return rawRecords, ids, nil
}

func makeRecord(kind, id string, payload any, refs []records.SourceRef, authority, status string, source records.Source, freshness records.Freshness, captured string) ([]byte, records.Record, error) {
	return recordFromPayload(kind, id, payload, refs, authority, status, source, freshness, captured)
}

func recordFromPayload(kind, id string, payload any, refs []records.SourceRef, authority, status string, source records.Source, freshness records.Freshness, captured string) ([]byte, records.Record, error) {
	payloadRaw, err := json.Marshal(payload)
	if err != nil {
		return nil, records.Record{}, err
	}
	canonical, err := records.CanonicalJSONValue(payloadRaw)
	if err != nil {
		return nil, records.Record{}, err
	}
	digest := sha256.Sum256(canonical)
	record := records.Record{Kind: kind, Version: "1", ID: id, SourceRefs: refs, ContentHash: hex.EncodeToString(digest[:]), CapturedAt: normalizeTime(captured), AuthorityClass: authority, Freshness: freshness, Status: status, Source: source, Payload: canonical}
	raw, err := json.Marshal(record)
	return raw, record, err
}

func evidenceFreshness(item EvidenceItem, maxAge int64, now time.Time) string {
	if item.SourceTimestamp == "" {
		return "UNKNOWN"
	}
	sourceTime, err := time.Parse(time.RFC3339, item.SourceTimestamp)
	if err != nil || sourceTime.After(now) {
		return "UNKNOWN"
	}
	if now.Sub(sourceTime) > time.Duration(maxAge)*time.Second {
		return "STALE"
	}
	return "FRESH"
}

// sourceLineageSHA256 mirrors LifeOps' canonical URL lineage rule: lowercase
// scheme/host, trim trailing path slashes, discard utm_* query parameters,
// preserve the remaining query-pair order, and omit fragments.
func sourceLineageSHA256(raw string) (string, error) {
	u, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", fmt.Errorf("invalid source URI")
	}
	queryParts := make([]string, 0)
	for _, part := range strings.Split(u.RawQuery, "&") {
		if part == "" {
			continue
		}
		pair := strings.SplitN(part, "=", 2)
		key, err := url.QueryUnescape(pair[0])
		if err != nil {
			return "", err
		}
		value := ""
		if len(pair) == 2 {
			value, err = url.QueryUnescape(pair[1])
			if err != nil {
				return "", err
			}
		}
		if strings.HasPrefix(strings.ToLower(key), "utm_") {
			continue
		}
		queryParts = append(queryParts, url.QueryEscape(key)+"="+url.QueryEscape(value))
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	canonical := strings.ToLower(u.Scheme) + "://" + strings.ToLower(u.Host) + path
	if len(queryParts) > 0 {
		canonical += "?" + strings.Join(queryParts, "&")
	}
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:]), nil
}

func isReferencedAsContradicted(claimID string, claims []Claim) bool {
	for _, claim := range claims {
		for _, ref := range claim.ContradictionRefs {
			if ref == claimID {
				return true
			}
		}
	}
	return false
}

func validTime(value string, now time.Time) bool {
	t, err := time.Parse(time.RFC3339, value)
	return err == nil && !t.After(now.Add(5*time.Minute))
}

func normalizeTime(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t = time.Now().UTC()
	}
	return t.UTC().Truncate(time.Second).Format(time.RFC3339)
}

func validHash(value string) bool {
	if len(value) != 64 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}

func oneOf(value string, values ...string) bool {
	for _, candidate := range values {
		if value == candidate {
			return true
		}
	}
	return false
}
func uniqueSorted(values []string) []string {
	set := make(map[string]bool)
	for _, value := range values {
		set[value] = true
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
