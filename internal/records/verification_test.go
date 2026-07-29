package records

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"homebase/internal/journal"
	"testing"
)

func bridgeSubmission(t *testing.T, contractID, grantID, treeSHA string) []byte {
	t.Helper()
	treeDigest := digestBridgeTree(treeSHA)
	checks := []any{map[string]any{
		"name": "go test ./...", "result": "passed", "proof_command": "go test ./...",
		"evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"},
	}}
	durableChecks := []any{map[string]any{
		"name": "go test ./...", "result": "passed",
		"evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"},
	}}
	payload := map[string]any{
		"task_id": "task-1", "contract_id": contractID, "grant_id": grantID,
		"worker_id": "worker-1", "verifier_id": "verifier-1", "tree_digest": treeDigest,
		"subject":          map[string]any{"kind": "git_tree", "name": treeSHA, "digest": map[string]any{"sha256": treeDigest}},
		"checks":           durableChecks,
		"evidence_refs":    []any{map[string]any{"kind": "proof", "id": "proof:bridge:0"}},
		"worker_claim_ref": map[string]any{"kind": "claim", "id": "claim:bridge:run-1"},
		"verified_at":      "2026-07-28T12:00:00Z",
	}
	contentHash, err := CanonicalContentHash(mustJSONBridge(t, payload))
	if err != nil {
		t.Fatal(err)
	}
	return mustJSONBridge(t, map[string]any{
		"kind": "VerificationReceipt", "version": "1", "id": "receipt:task-1:" + treeSHA,
		"content_hash": contentHash, "task_id": "task-1", "contract_id": contractID, "grant_id": grantID,
		"worker_id": "worker-1", "verifier_id": "verifier-1", "tree_sha": treeSHA, "tree_digest": treeDigest,
		"subject": payload["subject"], "checks": checks, "evidence_refs": payload["evidence_refs"],
		"worker_claim_ref": payload["worker_claim_ref"], "verified_at": "2026-07-28T12:00:00Z",
		"worker_statement": "worker completed the contract",
	})
}

func digestBridgeTree(treeSHA string) string {
	digest := sha256.Sum256([]byte("git-tree-sha:v1:" + treeSHA))
	return hex.EncodeToString(digest[:])
}

func TestBridgeVerificationSubmissionIsAtomicIdempotentAndRebuildable(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validContract(t, "contract-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validGrant(t, "grant-1", "contract-1", "idem-1")); err != nil {
		t.Fatal(err)
	}

	raw := bridgeSubmission(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789")
	first, err := store.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		t.Fatalf("first Bridge submission: %v", err)
	}
	if first.Existing || first.Sequence != 3 || first.Receipt.Kind != "VerificationReceipt" || len(first.Records) != 3 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := store.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		t.Fatalf("duplicate Bridge submission: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate Bridge submission was not idempotent: %+v", second)
	}
	if got := len(store.List()); got != 5 {
		t.Fatalf("record count after atomic submission = %d, want 5", got)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	reopened, err := NewStore(j2)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if _, err := reopened.Get(first.Receipt.ID); err != nil {
		t.Fatalf("replayed receipt missing: %v", err)
	}
	if got := len(reopened.List()); got != 5 {
		t.Fatalf("replayed record count = %d, want 5", got)
	}
}

func TestBridgeVerificationSubmissionCannotMintContractOrGrant(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendBridgeVerificationSubmission(bridgeSubmission(t, "missing-contract", "missing-grant", "0123456789012345678901234567890123456789")); err == nil {
		t.Fatal("Bridge submission minted missing Contract/CapabilityGrant")
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("failed submission changed store: %d records", got)
	}
}

func TestStoreReplayRejectsUnknownJournalKind(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := journal.EncodeRecord("UnknownAuthorityKind", []byte(`{"not":"a record"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(payload); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedJournal, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedJournal.Close()
	if _, err := NewStore(reopenedJournal); err == nil {
		t.Fatal("replay accepted an unknown journal kind")
	}
}

func TestStoreStopsWritesAfterUncertainJournalCommit(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	store.poisoned = fmt.Errorf("%w: injected", ErrJournalUncertain)
	if _, err := store.Append(validExternalEvidenceForPoisonTest(t)); !errors.Is(err, ErrJournalUncertain) {
		t.Fatalf("poisoned store append error = %v, want ErrJournalUncertain", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("poisoned store changed records: %d", got)
	}
}

func validExternalEvidenceForPoisonTest(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"evidence_type": "test", "subject_refs": []any{map[string]any{"kind": "session", "id": "session-1"}},
		"observed_digest": hex.EncodeToString(make([]byte, 32)),
	}
	return mustJSONBridge(t, map[string]any{
		"kind": "Evidence", "version": "1", "id": "poison-evidence",
		"source_refs":  []any{map[string]any{"kind": "trajectory_result", "id": "poison-evidence"}},
		"content_hash": payloadHashBridge(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityUntrustedText, "freshness": map[string]any{"mode": "immutable", "valid_until": nil},
		"status": "observed", "source": map[string]any{"id": "trajectory", "role": "trajectory"}, "payload": payload,
	})
}

func payloadHashBridge(t *testing.T, payload any) string {
	t.Helper()
	digest := sha256.Sum256(mustJSONBridge(t, payload))
	return hex.EncodeToString(digest[:])
}

func mustJSONBridge(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
