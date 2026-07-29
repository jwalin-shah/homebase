package records

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"homebase/internal/journal"
	"testing"
)

func TestEvidenceAppendIsDurableAndIdempotent(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	raw := validEvidence(t, "evidence-1")
	first, err := store.Append(raw)
	if err != nil {
		t.Fatal(err)
	}
	if first.Existing || first.Sequence != 1 {
		t.Fatalf("unexpected first append result: %+v", first)
	}
	second, err := store.Append(raw)
	if err != nil {
		t.Fatal(err)
	}
	if !second.Existing || second.Sequence != 0 {
		t.Fatalf("duplicate append was not an idempotent no-op: %+v", second)
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
		t.Fatal(err)
	}
	got, err := reopened.Get("evidence-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "Evidence" || got.Source.Role != "trajectory" {
		t.Fatalf("unexpected reopened record: %+v", got)
	}
	if len(reopened.List()) != 1 {
		t.Fatalf("expected one record after replay, got %d", len(reopened.List()))
	}
}

func TestAppendRejectsForgedAndStructurallyInvalidRecords(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(map[string]any){
		"forged content hash":     func(value map[string]any) { value["content_hash"] = hex.EncodeToString(make([]byte, 32)) },
		"unknown top-level field": func(value map[string]any) { value["surprise"] = true },
		"agent cannot issue decision": func(value map[string]any) {
			value["kind"] = "Decision"
			value["authority_class"] = AuthorityHumanDecision
			value["status"] = "approved"
			value["source"] = map[string]any{"id": "agent-1", "role": "agent"}
			value["payload"] = map[string]any{"decision": "approve", "scope": "task-1", "decided_by": "agent-1"}
			value["content_hash"] = payloadHash(t, value["payload"])
		},
		"trajectory cannot issue a bridge grant": func(value map[string]any) {
			value["kind"] = "CapabilityGrant"
			value["authority_class"] = AuthorityAuthoritative
			value["source"] = map[string]any{"id": "trajectory", "role": "trajectory"}
			value["payload"] = map[string]any{
				"grant_id": "grant-1", "contract_id": "contract-1", "task_id": "task-1", "worker_id": "worker-1",
				"allowed_paths": []string{"internal"}, "commands": []string{"go test ./..."},
				"issued_at": "2026-07-28T00:00:00Z", "expires_at": "2026-07-29T00:00:00Z",
				"context_hash": hex.EncodeToString(make([]byte, 32)), "idempotency_key": "idem-1", "effect_id": "effect-1",
			}
			value["content_hash"] = payloadHash(t, value["payload"])
		},
		"receipt verifier identity mismatch": func(value map[string]any) {
			value = replaceMap(value, decodeObject(t, validVerificationReceipt(t, "receipt-bad-id", "verifier-2", "verifier-1", hex.EncodeToString(make([]byte, 32)), hex.EncodeToString(make([]byte, 32)))))
		},
		"receipt tree digest mismatch": func(value map[string]any) {
			value = replaceMap(value, decodeObject(t, validVerificationReceipt(t, "receipt-bad-tree", "verifier-1", "verifier-1", hex.EncodeToString(make([]byte, 32)), hex.EncodeToString(make([]byte, 31))+"01")))
		},
	} {
		t.Run(name, func(t *testing.T) {
			value := decodeObject(t, validEvidence(t, "bad-"+name))
			mutate(value)
			_, err := store.Append(mustJSON(t, value))
			if !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("expected ErrInvalidRecord, got %v", err)
			}
		})
	}
	if len(store.List()) != 0 {
		t.Fatalf("invalid records changed the store")
	}
}

func TestCanonicalPayloadMatchesContractVector(t *testing.T) {
	payload := []byte(`{"subject_refs":[{"kind":"x","id":"s"}],"statement":"<&é😀"}`)
	canonical, err := canonicalObject(payload)
	if err != nil {
		t.Fatal(err)
	}
	const expected = `{"statement":"<&\u00e9\ud83d\ude00","subject_refs":[{"id":"s","kind":"x"}]}`
	if string(canonical) != expected {
		t.Fatalf("canonical JSON = %q, want %q", canonical, expected)
	}
	digest := sha256.Sum256(canonical)
	if got := hex.EncodeToString(digest[:]); got != "06cd25284a438c9dca385cb91e86efca0d2781272cf58b1c8f14ef28aa144a6f" {
		t.Fatalf("canonical digest = %s", got)
	}
}

func TestDuplicateIDWithDifferentRecordIsConflict(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validEvidence(t, "same-id")); err != nil {
		t.Fatal(err)
	}
	conflicting := decodeObject(t, validEvidence(t, "same-id"))
	conflicting["status"] = "challenged"
	_, err = store.Append(mustJSON(t, conflicting))
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	if len(store.List()) != 1 {
		t.Fatalf("conflicting record changed the store")
	}
}

func TestCrossRecordReferencesAndIdempotencyAreEnforced(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
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
	if _, err := store.Append(validObservation(t, "observation-1", "grant-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validGrant(t, "grant-2", "contract-1", "idem-1")); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("duplicate grant idempotency result = %v", err)
	}
	if _, err := store.Append(validObservation(t, "observation-missing", "grant-missing")); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("missing grant result = %v", err)
	}
}

func TestVerificationReceiptReferencesVerifiedRecords(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
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
	if _, err := store.Append(validObservation(t, "observation-1", "grant-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validProof(t, "proof-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Append(validVerificationReceiptWithRefs(t, "receipt-1", "verifier-1", "proof-1", "observation-1")); err != nil {
		t.Fatal(err)
	}
}

func TestVerificationReceiptRejectsMismatchedReferenceKindsAndLineage(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{validContract(t, "contract-1"), validGrant(t, "grant-1", "contract-1", "idem-1"), validObservation(t, "observation-1", "grant-1"), validProof(t, "proof-1")} {
		if _, err := store.Append(raw); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("reference kind cannot lie about target record", func(t *testing.T) {
		value := decodeObject(t, validVerificationReceiptWithRefs(t, "receipt-wrong-kind", "verifier-1", "proof-1", "observation-1"))
		payload := value["payload"].(map[string]any)
		payload["worker_claim_ref"] = map[string]any{"kind": "claim", "id": "contract-1"}
		value["content_hash"] = payloadHash(t, payload)
		if _, err := store.Append(mustJSON(t, value)); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("append error = %v, want ErrInvalidRecord", err)
		}
	})

	t.Run("check reference must be listed and point to proof or evidence", func(t *testing.T) {
		value := decodeObject(t, validVerificationReceiptWithRefs(t, "receipt-unlisted-check", "verifier-1", "proof-1", "observation-1"))
		payload := value["payload"].(map[string]any)
		checks := payload["checks"].([]any)
		checks[0].(map[string]any)["evidence_ref"] = map[string]any{"kind": "contract", "id": "contract-1"}
		value["content_hash"] = payloadHash(t, payload)
		if _, err := store.Append(mustJSON(t, value)); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("append error = %v, want ErrInvalidRecord", err)
		}
	})

	t.Run("receipt fields must match grant lineage", func(t *testing.T) {
		value := decodeObject(t, validVerificationReceiptWithRefs(t, "receipt-wrong-task", "verifier-1", "proof-1", "observation-1"))
		payload := value["payload"].(map[string]any)
		payload["task_id"] = "task-other"
		value["content_hash"] = payloadHash(t, payload)
		if _, err := store.Append(mustJSON(t, value)); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("append error = %v, want ErrInvalidRecord", err)
		}
	})
}

func TestSharedRecordDoesNotBecomeAttemptEvent(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	raw := validEvidence(t, "separation-1")
	payload, err := journal.EncodeRecord(journal.RecordKindSharedRecord, raw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(payload); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	store, err := NewStore(j2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get("separation-1"); err != nil {
		t.Fatal(err)
	}
}

func validEvidence(t *testing.T, id string) []byte {
	t.Helper()
	payload := map[string]any{
		"evidence_type":   "trajectory_observation",
		"subject_refs":    []any{map[string]any{"kind": "session", "id": "session-1"}},
		"observed_digest": hex.EncodeToString(make([]byte, 32)),
	}
	return mustJSON(t, map[string]any{
		"kind":            "Evidence",
		"version":         "1",
		"id":              id,
		"source_refs":     []any{map[string]any{"kind": "trajectory_result", "id": id}},
		"content_hash":    payloadHash(t, payload),
		"captured_at":     "2026-07-28T00:00:00Z",
		"authority_class": AuthorityUntrustedText,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil, "reason": "raw observation"},
		"status":          "observed",
		"source":          map[string]any{"id": "trajectory", "role": "trajectory"},
		"payload":         payload,
	})
}

func validVerificationReceipt(t *testing.T, id, payloadVerifierID, sourceID, treeDigest, subjectDigest string) []byte {
	t.Helper()
	payload := map[string]any{
		"task_id": "task-1", "contract_id": "contract-1", "grant_id": "grant-1", "worker_id": "worker-1",
		"verifier_id": payloadVerifierID, "tree_digest": treeDigest,
		"subject":          map[string]any{"kind": "git_tree", "name": "repo", "digest": map[string]any{"sha256": subjectDigest}},
		"checks":           []any{map[string]any{"name": "go-test", "result": "passed", "evidence_ref": map[string]any{"kind": "proof", "id": "proof-1"}}},
		"evidence_refs":    []any{map[string]any{"kind": "proof", "id": "proof-1"}},
		"worker_claim_ref": map[string]any{"kind": "observation", "id": "observation-1"},
		"verified_at":      "2026-07-28T00:00:00Z",
	}
	return mustJSON(t, map[string]any{
		"kind": "VerificationReceipt", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "proof", "id": "proof-1"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityVerifiedEvidence,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil}, "status": "verified",
		"source": map[string]any{"id": sourceID, "role": "verifier"}, "payload": payload,
	})
}

func validContract(t *testing.T, id string) []byte {
	t.Helper()
	payload := map[string]any{
		"task_id": "task-1", "repository": "homebase", "base_commit": "0123456789012345678901234567890123456789",
		"allowed_paths": []string{"internal/"}, "forbidden_paths": []string{"secrets/"},
		"context_hash": hex.EncodeToString(make([]byte, 32)), "context_valid_until": "2026-12-31T00:00:00Z",
		"idempotency_key": "idem-1", "worker_id": "worker-1", "verifier_id": "verifier-1",
		"acceptance": []string{"go test ./..."}, "publication": "prohibited",
	}
	return mustJSON(t, map[string]any{
		"kind": "Contract", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "decision", "id": "decision-1"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityHumanDecision, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
		"status": "approved", "source": map[string]any{"id": "homebase", "role": "homebase"}, "payload": payload,
	})
}

func validGrant(t *testing.T, id, contractID, idempotencyKey string) []byte {
	t.Helper()
	payload := map[string]any{
		"grant_id": id, "contract_id": contractID, "task_id": "task-1", "worker_id": "worker-1",
		"allowed_paths": []string{"internal/"}, "commands": []string{"go test ./..."},
		"issued_at": "2026-07-28T00:00:00Z", "expires_at": "2026-12-31T00:00:00Z",
		"context_hash": hex.EncodeToString(make([]byte, 32)), "idempotency_key": idempotencyKey, "effect_id": "effect-1",
	}
	return mustJSON(t, map[string]any{
		"kind": "CapabilityGrant", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "contract", "id": contractID}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityAuthoritative, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
		"status": "active", "source": map[string]any{"id": "bridge", "role": "bridge"}, "payload": payload,
	})
}

func validObservation(t *testing.T, id, grantID string) []byte {
	t.Helper()
	payload := map[string]any{
		"task_id": "task-1", "grant_id": grantID, "worker_id": "worker-1",
		"effect": map[string]any{"operation": "write", "path": "internal/file.go"},
	}
	return mustJSON(t, map[string]any{
		"kind": "Observation", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "grant", "id": grantID}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityWorkerObservation, "freshness": map[string]any{"mode": "immutable", "valid_until": nil},
		"status": "observed", "source": map[string]any{"id": "worker-1", "role": "worker"}, "payload": payload,
	})
}

func validProof(t *testing.T, id string) []byte {
	t.Helper()
	payload := map[string]any{
		"proof_type": "test", "proof_command": "go test ./...", "result": "passed",
		"subject_refs": []any{map[string]any{"kind": "observation", "id": "observation-1"}}, "verifier_id": "verifier-1",
	}
	return mustJSON(t, map[string]any{
		"kind": "Proof", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "observation", "id": "observation-1"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityVerifiedEvidence, "freshness": map[string]any{"mode": "immutable", "valid_until": nil},
		"status": "verified", "source": map[string]any{"id": "verifier-1", "role": "verifier"}, "payload": payload,
	})
}

func validVerificationReceiptWithRefs(t *testing.T, id, verifierID, proofID, observationID string) []byte {
	t.Helper()
	digest := hex.EncodeToString(make([]byte, 32))
	payload := map[string]any{
		"task_id": "task-1", "contract_id": "contract-1", "grant_id": "grant-1", "worker_id": "worker-1",
		"verifier_id": verifierID, "tree_digest": digest,
		"subject":       map[string]any{"kind": "git_tree", "name": "homebase", "digest": map[string]any{"sha256": digest}},
		"checks":        []any{map[string]any{"name": "go-test", "result": "passed", "evidence_ref": map[string]any{"kind": "proof", "id": proofID}}},
		"evidence_refs": []any{map[string]any{"kind": "proof", "id": proofID}}, "worker_claim_ref": map[string]any{"kind": "observation", "id": observationID},
		"verified_at": "2026-07-28T00:00:00Z",
	}
	return mustJSON(t, map[string]any{
		"kind": "VerificationReceipt", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "proof", "id": proofID}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": AuthorityVerifiedEvidence, "freshness": map[string]any{"mode": "immutable", "valid_until": nil},
		"status": "verified", "source": map[string]any{"id": verifierID, "role": "verifier"}, "payload": payload,
	})
}

func replaceMap(target, replacement map[string]any) map[string]any {
	for key := range target {
		delete(target, key)
	}
	for key, value := range replacement {
		target[key] = value
	}
	return target
}

func payloadHash(t *testing.T, payload any) string {
	t.Helper()
	encoded := mustJSON(t, payload)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
