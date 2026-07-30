package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"homebase/internal/journal"
	"testing"
	"time"
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
		"worker_id": "worker-1", "verifier_id": legacyVerifierID, "tree_digest": treeDigest,
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
		"worker_id": "worker-1", "verifier_id": legacyVerifierID, "tree_sha": treeSHA, "tree_digest": treeDigest,
		"subject": payload["subject"], "checks": checks, "evidence_refs": payload["evidence_refs"],
		"worker_claim_ref": payload["worker_claim_ref"], "verified_at": "2026-07-28T12:00:00Z",
		"worker_statement": "worker completed the contract",
	})
}

func digestBridgeTree(treeSHA string) string {
	digest := sha256.Sum256([]byte("git-tree-sha:v1:" + treeSHA))
	return hex.EncodeToString(digest[:])
}

func bridgeSubmissionWithProvenance(t *testing.T, contractID, grantID, treeSHA string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(bridgeSubmission(t, contractID, grantID, treeSHA), &document); err != nil {
		t.Fatal(err)
	}
	command := []string{"go", "test", "./..."}
	commandDigest, err := canonicalHashAnyValue(command)
	if err != nil {
		t.Fatal(err)
	}
	environmentDigest, err := canonicalHashAnyValue([]string{"GOFLAGS=-mod=mod", "PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	outputDigest := sha256.Sum256([]byte("proof output"))
	provenance := map[string]any{
		"schema_version": "1", "command": command, "command_digest": commandDigest,
		"environment_digest": environmentDigest, "output_digest": hex.EncodeToString(outputDigest[:]),
		"log_digest": hex.EncodeToString(outputDigest[:]), "checkout_sha": treeSHA,
		"verifier_version": "bridge-verifier/v2", "tool_versions": map[string]string{"bridge-verifier": "bridge-verifier/v2"}, "cache_status": "miss",
	}
	checks := document["checks"].([]any)
	checks[0].(map[string]any)["provenance"] = provenance
	durablePayload := map[string]any{
		"task_id": document["task_id"], "contract_id": document["contract_id"], "grant_id": document["grant_id"],
		"worker_id": document["worker_id"], "verifier_id": document["verifier_id"], "tree_digest": document["tree_digest"],
		"subject": document["subject"], "checks": []any{map[string]any{
			"name": checks[0].(map[string]any)["name"], "result": checks[0].(map[string]any)["result"],
			"proof_command": checks[0].(map[string]any)["proof_command"], "provenance": provenance,
			"evidence_ref": checks[0].(map[string]any)["evidence_ref"],
		}},
		"evidence_refs": document["evidence_refs"], "worker_claim_ref": document["worker_claim_ref"], "verified_at": document["verified_at"],
	}
	contentHash, err := canonicalHashValue(durablePayload)
	if err != nil {
		t.Fatal(err)
	}
	document["content_hash"] = contentHash
	return mustJSONBridge(t, document)
}

func TestBridgeReceiptProvenanceIsAuthenticated(t *testing.T) {
	tree := "0123456789012345678901234567890123456789"
	if _, err := decodeBridgeReceipt(bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", tree)); err != nil {
		t.Fatalf("decode provenance receipt: %v", err)
	}
}

func TestProductionBridgeReceiptRequiresValidVerifierAttestation(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw := productionBridgeSubmission(t, private, "verifier-key-1")
	if err := VerifyBridgeReceiptAttestation(raw, public, "verifier-key-1"); err != nil {
		t.Fatalf("VerifyBridgeReceiptAttestation: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	document["worker_statement"] = "tampered"
	if err := VerifyBridgeReceiptAttestation(mustJSONBridge(t, document), public, "verifier-key-1"); err == nil {
		t.Fatal("VerifyBridgeReceiptAttestation accepted a tampered signed receipt")
	}
}

func TestProductionBridgeReceiptRejectsMissingAndUnknownVerifierAuthority(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	raw := productionBridgeSubmission(t, private, "verifier-key-1")
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	delete(document, "attestation")
	if err := VerifyBridgeReceiptAttestation(mustJSONBridge(t, document), private.Public().(ed25519.PublicKey), "verifier-key-1"); err == nil {
		t.Fatal("VerifyBridgeReceiptAttestation accepted an unsigned production receipt")
	}

	document["verifier_id"] = "attacker-controlled-verifier"
	if err := VerifyBridgeReceiptAttestation(mustJSONBridge(t, document), private.Public().(ed25519.PublicKey), "verifier-key-1"); err == nil {
		t.Fatal("VerifyBridgeReceiptAttestation accepted an unknown verifier identity")
	}
}

func TestProductionBridgeReceiptRetainsAttestationAcrossReplay(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	appendApprovedSpecification(t, store)
	if _, err := store.Append(validBridgeContract(t, "contract-1", productionVerifierID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validGrant(t, "grant-1", "contract-1", "idem-1")); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	keyID := "verifier-key-1"
	raw := productionBridgeSubmission(t, private, keyID)
	if err := VerifyBridgeReceiptAttestation(raw, public, keyID); err != nil {
		t.Fatal(err)
	}
	first, err := store.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		t.Fatalf("append production receipt: %v", err)
	}
	fields, err := objectFields(first.Receipt.Payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["attestation"]; !ok {
		t.Fatal("durable production receipt dropped attestation")
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
		t.Fatalf("reopen production receipt: %v", err)
	}
	replayed, err := reopened.Get(first.Receipt.ID)
	if err != nil {
		t.Fatal(err)
	}
	replayedFields, err := objectFields(replayed.Payload)
	if err != nil {
		t.Fatal(err)
	}
	attestation, ok := replayedFields["attestation"]
	if !ok || string(attestation) == "null" {
		t.Fatal("replayed production receipt lost attestation")
	}
}

func productionBridgeSubmission(t *testing.T, private ed25519.PrivateKey, keyID string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789"), &document); err != nil {
		t.Fatal(err)
	}
	document["verifier_id"] = productionVerifierID
	checks := document["checks"].([]any)
	check := checks[0].(map[string]any)
	durablePayload := map[string]any{
		"task_id": document["task_id"], "contract_id": document["contract_id"], "grant_id": document["grant_id"],
		"worker_id": document["worker_id"], "verifier_id": document["verifier_id"], "tree_digest": document["tree_digest"],
		"subject": document["subject"], "checks": []any{map[string]any{
			"name": check["name"], "result": check["result"], "proof_command": check["proof_command"],
			"provenance": check["provenance"], "evidence_ref": check["evidence_ref"],
		}},
		"evidence_refs": document["evidence_refs"], "worker_claim_ref": document["worker_claim_ref"], "verified_at": document["verified_at"],
	}
	contentHash, err := canonicalHashValue(durablePayload)
	if err != nil {
		t.Fatal(err)
	}
	document["content_hash"] = contentHash
	document["attestation"] = map[string]any{
		"scheme": attestationScheme, "key_id": keyID,
		"signature": hex.EncodeToString(ed25519.Sign(private, []byte("running-machine-verifier-attestation:v1:"+document["id"].(string)+":"+contentHash+"\x00"+document["worker_statement"].(string)))),
	}
	return mustJSONBridge(t, document)
}

func validBridgeContract(t *testing.T, id, verifierID string) []byte {
	t.Helper()
	document := decodeObject(t, validContract(t, id))
	payload := document["payload"].(map[string]any)
	payload["verifier_id"] = verifierID
	document["content_hash"] = payloadHashBridge(t, payload)
	return mustJSONBridge(t, document)
}

func TestBridgeReceiptRejectsWorkerAsVerifier(t *testing.T) {
	var document map[string]any
	if err := json.Unmarshal(bridgeSubmission(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789"), &document); err != nil {
		t.Fatal(err)
	}
	document["worker_id"] = "verifier-1"
	if _, err := decodeBridgeReceipt(mustJSONBridge(t, document)); err == nil {
		t.Fatal("decode accepted the worker identity as verifier")
	}
}

func TestBridgeVerificationSubmissionPersistsProvenanceReceipt(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	appendApprovedSpecification(t, store)
	if _, err := store.Append(validBridgeContract(t, "contract-1", legacyVerifierID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validGrant(t, "grant-1", "contract-1", "idem-1")); err != nil {
		t.Fatal(err)
	}
	result, err := store.AppendBridgeVerificationSubmission(bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789"))
	if err != nil {
		t.Fatalf("append provenance receipt: %v", err)
	}
	if result.Receipt.Kind != "VerificationReceipt" || len(result.Records) != 3 {
		t.Fatalf("unexpected provenance submission result: %+v", result)
	}
	fields, err := objectFields(result.Receipt.Payload)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := arrayValues(fields["checks"])
	if err != nil || len(checks) != 1 {
		t.Fatalf("durable checks = %v, err=%v", checks, err)
	}
	check, err := objectFields(checks[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := check["provenance"]; !ok {
		t.Fatal("durable receipt dropped provenance")
	}
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
	appendApprovedSpecification(t, store)
	if _, err := store.Append(validBridgeContract(t, "contract-1", legacyVerifierID)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validGrant(t, "grant-1", "contract-1", "idem-1")); err != nil {
		t.Fatal(err)
	}

	raw := bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789")
	first, err := store.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		t.Fatalf("first Bridge submission: %v", err)
	}
	if first.Existing || first.Sequence != 5 || first.Receipt.Kind != "VerificationReceipt" || len(first.Records) != 3 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := store.AppendBridgeVerificationSubmission(raw)
	if err != nil {
		t.Fatalf("duplicate Bridge submission: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate Bridge submission was not idempotent: %+v", second)
	}
	if got := len(store.List()); got != 7 {
		t.Fatalf("record count after atomic submission = %d, want 7", got)
	}
	if _, err := store.AppendBridgeVerificationSubmission(bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "abcdefabcdefabcdefabcdefabcdefabcdefabcd")); err == nil {
		t.Fatal("second tree for the same admitted task was accepted as another terminal receipt")
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
	if got := len(reopened.List()); got != 7 {
		t.Fatalf("replayed record count = %d, want 7", got)
	}
}

func TestBridgeVerificationRejectsExpiredAuthorityAtSubmission(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	clock := func() time.Time { return time.Date(2026, 7, 28, 14, 0, 0, 0, time.UTC) }
	store, err := NewStoreWithClock(j, clock)
	if err != nil {
		t.Fatal(err)
	}
	appendApprovedSpecification(t, store)
	if _, err := store.Append(validBridgeContract(t, "contract-1", legacyVerifierID)); err != nil {
		t.Fatal(err)
	}
	expiredGrant := decodeObject(t, validGrant(t, "grant-1", "contract-1", "idem-1"))
	grantPayload := expiredGrant["payload"].(map[string]any)
	grantPayload["expires_at"] = "2026-07-28T13:00:00Z"
	expiredGrant["content_hash"] = payloadHashBridge(t, grantPayload)
	grantFreshness := expiredGrant["freshness"].(map[string]any)
	grantFreshness["valid_until"] = "2026-07-28T13:00:00Z"
	if _, err := store.Append(mustJSONBridge(t, expiredGrant)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendBridgeVerificationSubmission(bridgeSubmission(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789")); err == nil {
		t.Fatal("expired authority was accepted at submission")
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
