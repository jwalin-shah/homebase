package epistemic

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"homebase/internal/journal"
	"homebase/internal/records"
)

var fixedNow = time.Date(2026, 9, 24, 22, 0, 0, 0, time.UTC)

func TestAuthenticatedEpistemicBundleCommitsAtomicallyAndReplays(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.journal")
	store, closeJournal := newTestStore(t, path)
	defer closeJournal()
	requestPublic, requestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiptPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, "research-ingestor", requestPublic, "epistemic-test-key-v1", receiptPrivate, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}
	raw := validSubmission(t)
	signature := signSubmission(t, requestPrivate, raw)
	first, err := service.Accept(raw, signature)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Accepted || first.Existing || first.Sequence == 0 {
		t.Fatalf("unexpected first outcome: %+v", first)
	}
	if got := len(store.List()); got != 5 {
		t.Fatalf("durable record count = %d, want 5", got)
	}
	if first.Receipt.AcceptingAuthority != "research-ingestor" || first.Receipt.ClaimStatuses["claim-corroborated"] != "CORROBORATED" || first.Receipt.ClaimStatuses["claim-negative-control"] != "REJECTED" {
		t.Fatalf("receipt lost authority or epistemic statuses: %+v", first.Receipt)
	}
	second, err := service.Accept(raw, signature)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Existing || second.Receipt.ReceiptID != first.Receipt.ReceiptID {
		t.Fatalf("replay not idempotent: %+v", second)
	}

	closeJournal()
	reopenedJournal, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := records.NewStore(reopenedJournal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewService(reopened, "research-ingestor", requestPublic, "epistemic-test-key-v1", receiptPrivate, func() time.Time { return fixedNow }); err != nil {
		t.Fatalf("replayed HomeBase receipt did not verify: %v", err)
	}
	if got := len(reopened.List()); got != 5 {
		t.Fatalf("replayed record count = %d, want 5", got)
	}
	_ = reopenedJournal.Close()
}

func TestEpistemicBoundaryRejectsTamperingAndFalseCorroboration(t *testing.T) {
	store, closeJournal := newTestStore(t, filepath.Join(t.TempDir(), "records.journal"))
	defer closeJournal()
	requestPublic, requestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiptPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, "research-ingestor", requestPublic, "epistemic-test-key-v1", receiptPrivate, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}
	raw := validSubmission(t)
	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-2] ^= 1
	if _, err := service.Accept(tampered, signSubmission(t, requestPrivate, raw)); err == nil {
		t.Fatal("tampered signed input was accepted")
	}
	if len(store.List()) != 0 {
		t.Fatal("failed authentication wrote records")
	}

	var request Request
	if err := json.Unmarshal(raw, &request); err != nil {
		t.Fatal(err)
	}
	request.Claims[0].EvidenceRefs = []string{"ev-one"}
	weak, _ := json.Marshal(request)
	if _, err := service.Accept(weak, signSubmission(t, requestPrivate, weak)); err == nil {
		t.Fatal("single lineage was accepted as corroborated")
	}
	if len(store.List()) != 0 {
		t.Fatal("invalid epistemic status wrote records")
	}

	request.Claims[0].EvidenceRefs = []string{"ev-one", "ev-two"}
	request.Evidence[1].LineageSHA256 = stringOf('8')
	forgedLineage, _ := json.Marshal(request)
	if _, err := service.Accept(forgedLineage, signSubmission(t, requestPrivate, forgedLineage)); err == nil {
		t.Fatal("caller-supplied lineage digest not bound to source URI")
	}
	if len(store.List()) != 0 {
		t.Fatal("forged lineage wrote records")
	}
}

func TestContestedClaimRequiresResolvedContradictionAndPreservesContentAvailability(t *testing.T) {
	store, closeJournal := newTestStore(t, filepath.Join(t.TempDir(), "records.journal"))
	defer closeJournal()
	requestPublic, requestPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_, receiptPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, "research-ingestor", requestPublic, "epistemic-test-key-v1", receiptPrivate, func() time.Time { return fixedNow })
	if err != nil {
		t.Fatal(err)
	}
	var request Request
	if err := json.Unmarshal(validSubmission(t), &request); err != nil {
		t.Fatal(err)
	}
	request.Evidence[0].ContentAvailable = true
	request.Claims[0].EpistemicStatus = "CONTESTED"
	request.Claims[0].ContradictionRefs = []string{"claim-negative-control"}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Accept(raw, signSubmission(t, requestPrivate, raw)); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	for _, record := range store.List() {
		if record.ID == "ev-one" {
			if err := json.Unmarshal(record.Payload, &payload); err != nil {
				t.Fatal(err)
			}
		}
	}
	if payload["source_content_available"] != true {
		t.Fatalf("evidence content availability was not retained: %#v", payload["source_content_available"])
	}
}

func newTestStore(t *testing.T, path string) (*records.Store, func()) {
	t.Helper()
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := records.NewStore(j)
	if err != nil {
		_ = j.Close()
		t.Fatal(err)
	}
	return store, func() { _ = j.Close() }
}

func validSubmission(t *testing.T) []byte {
	t.Helper()
	request := Request{
		Version: "homebase.epistemic_submission.v1", SubmissionID: "submission-test-1",
		InputSHA256: stringOf('1'), OutputSHA256: stringOf('2'), FreshnessMaxAgeSeconds: 365 * 24 * 3600,
		Runs: []ProviderResearchRun{
			{RunID: "run-hyperagent", Provider: "hyperagent", InputSHA256: stringOf('1'), OutputSHA256: stringOf('2'), SourceArtifactSHA256: stringOf('3'), StartedAt: "2026-09-24T20:59:00Z", CompletedAt: "2026-09-24T21:00:30Z", RetrievedAt: "2026-09-24T21:00:00Z"},
			{RunID: "run-perplexity", Provider: "perplexity", InputSHA256: stringOf('1'), OutputSHA256: stringOf('2'), SourceArtifactSHA256: stringOf('4'), StartedAt: "2026-09-24T21:00:30Z", CompletedAt: "2026-09-24T21:01:30Z", RetrievedAt: "2026-09-24T21:01:00Z"},
		},
		Evidence: []EvidenceItem{
			{EvidenceID: "ev-one", RunID: "run-hyperagent", SourceURI: "https://opentelemetry.io/docs/specs/otel/logs/data-model/", Publisher: "opentelemetry.io", ContentSHA256: stringOf('5'), LineageSHA256: lineageHash(t, "https://opentelemetry.io/docs/specs/otel/logs/data-model/"), SourceTimestamp: "2026-09-24T20:00:00Z", RetrievedAt: "2026-09-24T21:00:00Z", ContentAvailable: false},
			{EvidenceID: "ev-two", RunID: "run-perplexity", SourceURI: "https://opentelemetry.io/docs/specs/otel/logs/sdk/", Publisher: "opentelemetry.io", ContentSHA256: stringOf('7'), LineageSHA256: lineageHash(t, "https://opentelemetry.io/docs/specs/otel/logs/sdk/"), SourceTimestamp: "2026-09-24T20:00:00Z", RetrievedAt: "2026-09-24T21:01:00Z", ContentAvailable: false},
		},
		Claims: []Claim{
			{ClaimID: "claim-corroborated", Text: "Trace fields are derived from active context.", EpistemicStatus: "CORROBORATED", EvidenceRefs: []string{"ev-one", "ev-two"}},
			{ClaimID: "claim-negative-control", Text: "Every record requires trace fields.", EpistemicStatus: "REJECTED", EvidenceRefs: []string{}},
		},
	}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func signSubmission(t *testing.T, private ed25519.PrivateKey, raw []byte) []byte {
	t.Helper()
	canonical, err := records.CanonicalJSONValue(raw)
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.Sign(private, canonical)
}

func stringOf(r rune) string {
	value := make([]rune, 64)
	for i := range value {
		value[i] = r
	}
	return string(value)
}

func lineageHash(t *testing.T, uri string) string {
	t.Helper()
	digest, err := sourceLineageSHA256(uri)
	if err != nil {
		t.Fatal(err)
	}
	return digest
}
