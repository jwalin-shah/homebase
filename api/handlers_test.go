package api

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"homebase/internal/journal"
	"homebase/internal/ledger"
	"homebase/internal/records"
	"homebase/internal/validation"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleAppendExternalRecordUsesTypedDurableBoundary(t *testing.T) {
	ledgerStore, err := ledger.NewStore(t.TempDir() + "/legacy.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer ledgerStore.Close()
	recordJournal, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer recordJournal.Close()
	recordStore, err := records.NewStore(recordJournal)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithRecords(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore)

	raw := validExternalEvidence(t, "http-evidence-1")
	request := httptest.NewRequest(http.MethodPost, "/api/v1/records", bytes.NewReader(raw))
	response := httptest.NewRecorder()
	server.HandleAppendExternalRecord(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("first append status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/records", bytes.NewReader(raw))
	response = httptest.NewRecorder()
	server.HandleAppendExternalRecord(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate append status = %d, body = %s", response.Code, response.Body.String())
	}

	decision := decodeObject(t, raw)
	decision["kind"] = "Decision"
	decision["version"] = "1"
	decision["authority_class"] = records.AuthorityHumanDecision
	decision["status"] = "approved"
	decision["source"] = map[string]any{"id": "captain", "role": "captain"}
	decision["payload"] = map[string]any{"decision": "approve", "scope": "task-1", "decided_by": "captain"}
	decision["content_hash"] = payloadHash(t, decision["payload"])
	decision["id"] = "http-decision-1"
	request = httptest.NewRequest(http.MethodPost, "/api/v1/records", bytes.NewReader(mustJSON(t, decision)))
	response = httptest.NewRecorder()
	server.HandleAppendExternalRecord(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("authoritative append status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandleAppendBridgeVerificationAuthenticatesAndCommitsAtomically(t *testing.T) {
	ledgerStore, err := ledger.NewStore(t.TempDir() + "/legacy.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer ledgerStore.Close()
	recordJournal, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer recordJournal.Close()
	recordStore, err := records.NewStore(recordJournal)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeContract(t, "contract-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeGrant(t, "grant-1", "contract-1")); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithBridge(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore, public)
	raw := bridgeReceipt(t)
	sign := func(value []byte) string {
		canonical, err := records.CanonicalJSONValue(value)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	request.Header.Set("X-Bridge-Verification-Signature", sign(raw))
	response := httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("first Bridge append status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 5 {
		t.Fatalf("record count after Bridge append = %d, want 5", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	request.Header.Set("X-Bridge-Verification-Signature", sign(raw))
	response = httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate Bridge append status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	request.Header.Set("X-Bridge-Verification-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	response = httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("tampered signature status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandleAppendContractGrantAuthenticatesAndCommitsAtomically(t *testing.T) {
	ledgerStore, err := ledger.NewStore(t.TempDir() + "/legacy.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer ledgerStore.Close()
	recordJournal, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer recordJournal.Close()
	recordStore, err := records.NewStore(recordJournal)
	if err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithAuthorities(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore, nil, public, nil)
	bundle := mustJSON(t, map[string]any{
		"contract": json.RawMessage(bridgeContract(t, "contract-1")),
		"grant":    json.RawMessage(bridgeGrant(t, "grant-1", "contract-1")),
	})
	sign := func(value []byte) string {
		canonical, err := records.CanonicalJSONValue(value)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Contract-Signature", sign(bundle))
	response := httptest.NewRecorder()
	server.HandleAppendContractGrant(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("first Contract/Grant status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after Contract/Grant append = %d, want 2", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Contract-Signature", sign(bundle))
	response = httptest.NewRecorder()
	server.HandleAppendContractGrant(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate Contract/Grant status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Contract-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	response = httptest.NewRecorder()
	server.HandleAppendContractGrant(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("bad Contract authority signature status = %d, body = %s", response.Code, response.Body.String())
	}
}

func bridgeReceipt(t *testing.T) []byte {
	t.Helper()
	treeSHA := "0123456789012345678901234567890123456789"
	digest := sha256.Sum256([]byte("git-tree-sha:v1:" + treeSHA))
	treeDigest := hex.EncodeToString(digest[:])
	durableChecks := []any{map[string]any{
		"name": "go test ./...", "result": "passed",
		"evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"},
	}}
	payload := map[string]any{
		"task_id": "task-1", "contract_id": "contract-1", "grant_id": "grant-1",
		"worker_id": "worker-1", "verifier_id": "verifier-1", "tree_digest": treeDigest,
		"subject":          map[string]any{"kind": "git_tree", "name": treeSHA, "digest": map[string]any{"sha256": treeDigest}},
		"checks":           durableChecks,
		"evidence_refs":    []any{map[string]any{"kind": "proof", "id": "proof:bridge:0"}},
		"worker_claim_ref": map[string]any{"kind": "claim", "id": "claim:bridge:run-1"},
		"verified_at":      "2026-07-28T12:00:00Z",
	}
	contentHash, err := records.CanonicalContentHash(mustJSON(t, payload))
	if err != nil {
		t.Fatal(err)
	}
	return mustJSON(t, map[string]any{
		"kind": "VerificationReceipt", "version": "1", "id": "receipt:task-1:" + treeSHA,
		"content_hash": contentHash, "task_id": "task-1", "contract_id": "contract-1", "grant_id": "grant-1",
		"worker_id": "worker-1", "verifier_id": "verifier-1", "tree_sha": treeSHA, "tree_digest": treeDigest,
		"subject":       payload["subject"],
		"checks":        []any{map[string]any{"name": "go test ./...", "result": "passed", "proof_command": "go test ./...", "evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"}}},
		"evidence_refs": payload["evidence_refs"], "worker_claim_ref": payload["worker_claim_ref"],
		"verified_at": "2026-07-28T12:00:00Z", "worker_statement": "worker completed the contract",
	})
}

func bridgeContract(t *testing.T, id string) []byte {
	t.Helper()
	payload := map[string]any{
		"task_id": "task-1", "repository": "homebase", "base_commit": "0123456789012345678901234567890123456789",
		"allowed_paths": []string{"internal/"}, "forbidden_paths": []string{"secrets/"},
		"context_hash": hex.EncodeToString(make([]byte, 32)), "context_valid_until": "2026-07-29T00:00:00Z",
		"idempotency_key": "idem-1", "worker_id": "worker-1", "verifier_id": "verifier-1",
		"acceptance": []string{"go test ./..."}, "publication": "prohibited",
	}
	return mustJSON(t, map[string]any{
		"kind": "Contract", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "decision", "id": "decision-1"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-07-29T00:00:00Z"},
		"status": "approved", "source": map[string]any{"id": "homebase", "role": "homebase"}, "payload": payload,
	})
}

func bridgeGrant(t *testing.T, id, contractID string) []byte {
	t.Helper()
	payload := map[string]any{
		"grant_id": id, "contract_id": contractID, "task_id": "task-1", "worker_id": "worker-1",
		"allowed_paths": []string{"internal/"}, "commands": []string{"go test ./..."},
		"issued_at": "2026-07-28T00:00:00Z", "expires_at": "2026-07-29T00:00:00Z",
		"context_hash": hex.EncodeToString(make([]byte, 32)), "idempotency_key": "idem-1", "effect_id": "effect-1",
	}
	return mustJSON(t, map[string]any{
		"kind": "CapabilityGrant", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "contract", "id": contractID}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityAuthoritative, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-07-29T00:00:00Z"},
		"status": "active", "source": map[string]any{"id": "bridge", "role": "bridge"}, "payload": payload,
	})
}

func validExternalEvidence(t *testing.T, id string) []byte {
	t.Helper()
	payload := map[string]any{
		"evidence_type":   "trajectory_observation",
		"subject_refs":    []any{map[string]any{"kind": "session", "id": "session-1"}},
		"observed_digest": hex.EncodeToString(make([]byte, 32)),
	}
	return mustJSON(t, map[string]any{
		"kind": "Evidence", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "trajectory_result", "id": id}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityUntrustedText,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil, "reason": "raw observation"},
		"status":          "observed", "source": map[string]any{"id": "trajectory", "role": "trajectory"}, "payload": payload,
	})
}

func payloadHash(t *testing.T, payload any) string {
	t.Helper()
	digest := sha256.Sum256(mustJSON(t, payload))
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
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
