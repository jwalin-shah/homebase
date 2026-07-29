package api

import (
	"bytes"
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
