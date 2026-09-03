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
	"strings"
	"testing"
	"time"
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
	if _, err := recordStore.Append(bridgeApprovalDecision(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeSpecification(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeVerificationContract(t, "contract-1")); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeGrant(t, "grant-1", "contract-1")); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	responsePublic, responsePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	_ = responsePublic
	server := NewServerWithAuthoritiesAndAdmissionResponse(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore, nil, nil, public, responsePrivate)
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
	request.Header.Set("Idempotency-Key", "receipt:task-1:0123456789012345678901234567890123456789")
	response := httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("first Bridge append status = %d, body = %s", response.Code, response.Body.String())
	}
	var firstSubmission struct {
		ReceiptID   string `json:"receipt_id"`
		Existing    bool   `json:"existing"`
		Sequence    uint64 `json:"sequence"`
		RecordCount int    `json:"record_count"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &firstSubmission); err != nil {
		t.Fatalf("decode first Bridge append response: %v", err)
	}
	if got := len(recordStore.List()); got != 7 {
		t.Fatalf("record count after Bridge append = %d, want 7", got)
	}

	// Treat the first response as lost. The client may safely replay the exact
	// signed receipt because HomeBase's journal identity is the receipt ID.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	request.Header.Set("X-Bridge-Verification-Signature", sign(raw))
	request.Header.Set("Idempotency-Key", "receipt:task-1:0123456789012345678901234567890123456789")
	response = httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate Bridge append status = %d, body = %s", response.Code, response.Body.String())
	}
	var replaySubmission struct {
		ReceiptID   string `json:"receipt_id"`
		Existing    bool   `json:"existing"`
		Sequence    uint64 `json:"sequence"`
		RecordCount int    `json:"record_count"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &replaySubmission); err != nil {
		t.Fatalf("decode replay Bridge append response: %v", err)
	}
	if !replaySubmission.Existing || replaySubmission.ReceiptID != firstSubmission.ReceiptID || replaySubmission.Sequence != firstSubmission.Sequence || replaySubmission.RecordCount != firstSubmission.RecordCount {
		t.Fatalf("replay response = %+v, first response = %+v", replaySubmission, firstSubmission)
	}
	if got := len(recordStore.List()); got != 7 {
		t.Fatalf("record count after lost-response replay = %d, want unchanged 7", got)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	request.Header.Set("X-Bridge-Verification-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	request.Header.Set("Idempotency-Key", "receipt:task-1:0123456789012345678901234567890123456789")
	response = httptest.NewRecorder()
	server.HandleAppendBridgeVerification(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("tampered signature status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestHandleReadVerificationReceiptReturnsCanonicalStoredReceipt(t *testing.T) {
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
	if _, err := recordStore.Append(bridgeApprovalDecision(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeSpecification(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeVerificationContract(t, "contract-1")); err != nil {
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
	signReceipt := func(value []byte) string {
		canonical, err := records.CanonicalJSONValue(value)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	appendRequest := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/bridge", bytes.NewReader(raw))
	appendRequest.Header.Set("X-Bridge-Verification-Signature", signReceipt(raw))
	appendRequest.Header.Set("Idempotency-Key", "receipt:task-1:0123456789012345678901234567890123456789")
	appendResponse := httptest.NewRecorder()
	server.HandleAppendBridgeVerification(appendResponse, appendRequest)
	if appendResponse.Code != http.StatusCreated {
		t.Fatalf("append status = %d, body = %s", appendResponse.Code, appendResponse.Body.String())
	}

	receiptID := "receipt:task-1:0123456789012345678901234567890123456789"
	readBody := mustJSON(t, map[string]string{"receipt_id": receiptID})
	signRead := func(raw []byte) string {
		canonical, err := records.CanonicalJSONValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	readRequest := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	readRequest.Header.Set("X-Bridge-Verification-Read-Signature", signRead(readBody))
	readResponse := httptest.NewRecorder()
	server.HandleReadVerificationReceipt(readResponse, readRequest)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", readResponse.Code, readResponse.Body.String())
	}
	canonical, err := recordStore.GetVerificationReceiptCanonical(receiptID)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(readResponse.Body.Bytes(), append(canonical, '\n')) {
		t.Fatalf("read response is not canonical stored bytes:\n got %s\n want %s", readResponse.Body.Bytes(), append(canonical, '\n'))
	}
	readAgain := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	readAgain.Header.Set("X-Bridge-Verification-Read-Signature", signRead(readBody))
	readAgainResponse := httptest.NewRecorder()
	server.HandleReadVerificationReceipt(readAgainResponse, readAgain)
	if !bytes.Equal(readResponse.Body.Bytes(), readAgainResponse.Body.Bytes()) {
		t.Fatal("repeated read returned different bytes")
	}

	missingBody := mustJSON(t, map[string]string{"receipt_id": "receipt:task-9:0123456789012345678901234567890123456789"})
	missingRequest := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/receipts/read", bytes.NewReader(missingBody))
	missingRequest.Header.Set("X-Bridge-Verification-Read-Signature", signRead(missingBody))
	missingResponse := httptest.NewRecorder()
	server.HandleReadVerificationReceipt(missingResponse, missingRequest)
	if missingResponse.Code != http.StatusNotFound {
		t.Fatalf("missing receipt status = %d, body = %s", missingResponse.Code, missingResponse.Body.String())
	}

	badIDBody := mustJSON(t, map[string]string{"receipt_id": "not-a-receipt"})
	badIDRequest := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/receipts/read", bytes.NewReader(badIDBody))
	badIDRequest.Header.Set("X-Bridge-Verification-Read-Signature", signRead(badIDBody))
	badIDResponse := httptest.NewRecorder()
	server.HandleReadVerificationReceipt(badIDResponse, badIDRequest)
	if badIDResponse.Code != http.StatusBadRequest {
		t.Fatalf("malformed receipt_id status = %d, body = %s", badIDResponse.Code, badIDResponse.Body.String())
	}

	badSigRequest := httptest.NewRequest(http.MethodPost, "/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	badSigRequest.Header.Set("X-Bridge-Verification-Read-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	badSigResponse := httptest.NewRecorder()
	server.HandleReadVerificationReceipt(badSigResponse, badSigRequest)
	if badSigResponse.Code != http.StatusUnauthorized {
		t.Fatalf("bad read signature status = %d, body = %s", badSigResponse.Code, badSigResponse.Body.String())
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
	if _, err := recordStore.Append(bridgeApprovalDecision(t)); err != nil {
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithAuthorities(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore, nil, public, nil)
	bundle := mustJSON(t, map[string]any{
		"specification": json.RawMessage(bridgeSpecification(t)),
		"contract":      json.RawMessage(bridgeContract(t, "contract-1")),
		"grant":         json.RawMessage(bridgeGrant(t, "grant-1", "contract-1")),
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
	if got := len(recordStore.List()); got != 4 {
		t.Fatalf("record count after Specification/Contract/Grant append = %d, want 4", got)
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

func TestHandleAppendSpecificationDecisionAuthenticatesAndCommitsAtomically(t *testing.T) {
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
		"specification": json.RawMessage(bridgeSpecification(t)),
		"decision":      json.RawMessage(bridgeApprovalDecision(t)),
	})
	sign := func(value []byte) string {
		canonical, err := records.CanonicalJSONValue(value)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}

	// Neither AppendExternal nor any other unauthenticated route can create
	// this pair: it does not exist in the store before the owner-signed call.
	if _, err := recordStore.Get("spec:homebase:api-test:v1"); err == nil {
		t.Fatal("specification unexpectedly pre-exists before authenticated append")
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Specification-Signature", sign(bundle))
	response := httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("first Specification/Decision status = %d, body = %s", response.Code, response.Body.String())
	}
	var first struct {
		SpecificationID string `json:"specification_id"`
		DecisionID      string `json:"decision_id"`
		Existing        bool   `json:"existing"`
		Sequence        uint64 `json:"sequence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &first); err != nil {
		t.Fatalf("decode first Specification/Decision response: %v", err)
	}
	if first.Existing || first.SpecificationID != "spec:homebase:api-test:v1" || first.DecisionID != "decision:spec-api-test" {
		t.Fatalf("unexpected first response: %+v", first)
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after Specification/Decision append = %d, want 2", got)
	}

	// Duplicate-identical resubmission (a lost response) is idempotent.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Specification-Signature", sign(bundle))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("duplicate Specification/Decision status = %d, body = %s", response.Code, response.Body.String())
	}
	var second struct {
		Existing bool   `json:"existing"`
		Sequence uint64 `json:"sequence"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode duplicate Specification/Decision response: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate response = %+v, want existing at sequence %d", second, first.Sequence)
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after duplicate append = %d, want unchanged 2", got)
	}

	// A conflicting bundle for the same Specification ID is rejected and
	// leaves the journal/store unchanged.
	conflicting := decodeObject(t, bridgeApprovalDecision(t))
	conflicting["payload"].(map[string]any)["decision"] = "approve specification spec:homebase:api-test:v1 (revised)"
	conflicting["content_hash"] = payloadHash(t, conflicting["payload"])
	conflictBundle := mustJSON(t, map[string]any{
		"specification": json.RawMessage(bridgeSpecification(t)),
		"decision":      json.RawMessage(mustJSON(t, conflicting)),
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(conflictBundle))
	request.Header.Set("X-HomeBase-Specification-Signature", sign(conflictBundle))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("conflicting Specification/Decision status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after rejected conflict = %d, want unchanged 2", got)
	}

	// A bad signature is rejected before anything is persisted.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(bundle))
	request.Header.Set("X-HomeBase-Specification-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("bad Specification authority signature status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after bad signature = %d, want unchanged 2", got)
	}

	// A missing signature header is rejected the same way.
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(bundle))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing signature status = %d, body = %s", response.Code, response.Body.String())
	}

	// A wrong-authority signer (not the enrolled captain/contract key) is
	// rejected identically to a corrupted signature.
	_, otherPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	otherSpecification := decodeObject(t, bridgeSpecification(t))
	otherSpecification["id"] = "spec:homebase:other-test:v1"
	otherDecision := decodeObject(t, bridgeApprovalDecision(t))
	otherDecision["id"] = "decision:spec-other-test"
	otherDecision["payload"].(map[string]any)["specification_ref"].(map[string]any)["id"] = "spec:homebase:other-test:v1"
	otherDecision["content_hash"] = payloadHash(t, otherDecision["payload"])
	otherBundle := mustJSON(t, map[string]any{
		"specification": json.RawMessage(mustJSON(t, otherSpecification)),
		"decision":      json.RawMessage(mustJSON(t, otherDecision)),
	})
	canonicalOther, err := records.CanonicalJSONValue(otherBundle)
	if err != nil {
		t.Fatal(err)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(otherBundle))
	request.Header.Set("X-HomeBase-Specification-Signature", hex.EncodeToString(ed25519.Sign(otherPrivate, canonicalOther)))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("wrong-authority signer status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 2 {
		t.Fatalf("record count after wrong-authority signer = %d, want unchanged 2", got)
	}
}

func TestHandleAppendSpecificationDecisionRejectsWrongStatusAndMissingReference(t *testing.T) {
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
	sign := func(value []byte) string {
		canonical, err := records.CanonicalJSONValue(value)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}

	// A proposed (not yet approved) Specification is rejected.
	proposed := decodeObject(t, bridgeSpecification(t))
	proposed["status"] = "proposed"
	proposed["authority_class"] = records.AuthorityAgentProposal
	proposed["source"] = map[string]any{"id": "knowledge-engine", "role": "knowledge_engine"}
	delete(proposed["payload"].(map[string]any), "approval_ref")
	proposed["content_hash"] = payloadHash(t, proposed["payload"])
	proposedBundle := mustJSON(t, map[string]any{
		"specification": json.RawMessage(mustJSON(t, proposed)),
		"decision":      json.RawMessage(bridgeApprovalDecision(t)),
	})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(proposedBundle))
	request.Header.Set("X-HomeBase-Specification-Signature", sign(proposedBundle))
	response := httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("proposed specification status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 0 {
		t.Fatalf("rejected proposed specification left %d records, want 0", got)
	}

	// A Decision missing specification_ref is rejected.
	decision := decodeObject(t, bridgeApprovalDecision(t))
	delete(decision["payload"].(map[string]any), "specification_ref")
	decision["content_hash"] = payloadHash(t, decision["payload"])
	missingRefBundle := mustJSON(t, map[string]any{
		"specification": json.RawMessage(bridgeSpecification(t)),
		"decision":      json.RawMessage(mustJSON(t, decision)),
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/specifications/decisions", bytes.NewReader(missingRefBundle))
	request.Header.Set("X-HomeBase-Specification-Signature", sign(missingRefBundle))
	response = httptest.NewRecorder()
	server.HandleAppendSpecificationDecision(response, request)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing specification_ref status = %d, body = %s", response.Code, response.Body.String())
	}
	if got := len(recordStore.List()); got != 0 {
		t.Fatalf("rejected missing-reference bundle left %d records, want 0", got)
	}
}

func TestHandleCheckContractGrantRequiresBridgeSignatureAndMatchingScope(t *testing.T) {
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
	if _, err := recordStore.Append(bridgeApprovalDecision(t)); err != nil {
		t.Fatal(err)
	}
	if _, err := recordStore.Append(bridgeSpecification(t)); err != nil {
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
	_, responsePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := NewServerWithAuthoritiesAndAdmissionResponse(validation.NewValidator(nil, ledgerStore), nil, ledgerStore, recordStore, nil, nil, public, responsePrivate)
	specification := decodeObject(t, bridgeSpecification(t))
	check := mustJSON(t, map[string]any{
		"contract_id": "contract-1", "grant_id": "grant-1", "task_id": "task-1", "worker_id": "worker-1",
		"repository": "homebase", "base_commit": "0123456789012345678901234567890123456789",
		"allowed_paths": []string{"internal/"}, "forbidden_paths": []string{"secrets/"},
		"acceptance": []string{"go test ./..."}, "commands": []string{"go test ./..."},
		"context_hash":     hex.EncodeToString(make([]byte, 32)),
		"specification_id": specification["id"], "specification_digest": specification["content_hash"],
		"verifier_id": "verifier-1", "idempotency_key": "idem-1",
	})
	sign := func(raw []byte) string {
		canonical, err := records.CanonicalJSONValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants/check", bytes.NewReader(check))
	request.Header.Set("X-Bridge-Contract-Check-Signature", sign(check))
	response := httptest.NewRecorder()
	server.HandleCheckContractGrant(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("valid check status = %d, body = %s", response.Code, response.Body.String())
	}

	mismatchedTree := decodeObject(t, check)
	mismatchedTree["base_commit"] = "ffffffffffffffffffffffffffffffffffffffff"
	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants/check", bytes.NewReader(mustJSON(t, mismatchedTree)))
	request.Header.Set("X-Bridge-Contract-Check-Signature", sign(mustJSON(t, mismatchedTree)))
	response = httptest.NewRecorder()
	server.HandleCheckContractGrant(response, request)
	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("mismatched base tree status = %d, body = %s", response.Code, response.Body.String())
	}

	wrongScope := mustJSON(t, map[string]any{
		"contract_id": "contract-1", "grant_id": "grant-1", "task_id": "other-task", "worker_id": "worker-1",
		"repository": "homebase", "base_commit": "0123456789012345678901234567890123456789",
		"allowed_paths": []string{"internal/"}, "forbidden_paths": []string{"secrets/"},
		"acceptance": []string{"go test ./..."}, "commands": []string{"go test ./..."},
		"context_hash":     hex.EncodeToString(make([]byte, 32)),
		"specification_id": specification["id"], "specification_digest": specification["content_hash"],
		"verifier_id": "verifier-1", "idempotency_key": "idem-1",
	})
	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants/check", bytes.NewReader(wrongScope))
	request.Header.Set("X-Bridge-Contract-Check-Signature", sign(wrongScope))
	response = httptest.NewRecorder()
	server.HandleCheckContractGrant(response, request)
	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("wrong scope status = %d, body = %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants/check", bytes.NewReader(check))
	request.Header.Set("X-Bridge-Contract-Check-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	response = httptest.NewRecorder()
	server.HandleCheckContractGrant(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("bad signature status = %d, body = %s", response.Code, response.Body.String())
	}

	server.now = func() time.Time { return time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) }
	request = httptest.NewRequest(http.MethodPost, "/api/v1/contracts/grants/check", bytes.NewReader(check))
	request.Header.Set("X-Bridge-Contract-Check-Signature", sign(check))
	response = httptest.NewRecorder()
	server.HandleCheckContractGrant(response, request)
	if response.Code != http.StatusPreconditionFailed {
		t.Fatalf("expired authority status = %d, body = %s", response.Code, response.Body.String())
	}
}

func bridgeReceipt(t *testing.T) []byte {
	t.Helper()
	treeSHA := "0123456789012345678901234567890123456789"
	digest := sha256.Sum256([]byte("git-tree-sha:v1:" + treeSHA))
	treeDigest := hex.EncodeToString(digest[:])
	command := []string{"go", "test", "./..."}
	commandDigest, err := canonicalHash(t, command)
	if err != nil {
		t.Fatal(err)
	}
	environmentDigest, err := canonicalHash(t, []string{"PATH=/usr/bin"})
	if err != nil {
		t.Fatal(err)
	}
	proofDigest := sha256.Sum256([]byte("proof output"))
	provenance := map[string]any{
		"schema_version": "1", "command": command, "command_digest": commandDigest,
		"environment_digest": environmentDigest, "output_digest": hex.EncodeToString(proofDigest[:]),
		"log_digest": hex.EncodeToString(proofDigest[:]), "checkout_sha": treeSHA,
		"verifier_version": "bridge-verifier/v2", "tool_versions": map[string]string{"bridge-verifier": "bridge-verifier/v2"}, "cache_status": "miss",
	}
	durableChecks := []any{map[string]any{
		"name": "go test ./...", "result": "passed", "proof_command": "go test ./...", "provenance": provenance,
		"evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"},
	}}
	payload := map[string]any{
		"task_id": "task-1", "contract_id": "contract-1", "grant_id": "grant-1",
		"worker_id": "worker-1", "verifier_id": "bridge:verifier", "tree_digest": treeDigest,
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
		"worker_id": "worker-1", "verifier_id": "bridge:verifier", "tree_sha": treeSHA, "tree_digest": treeDigest,
		"subject":       payload["subject"],
		"checks":        []any{map[string]any{"name": "go test ./...", "result": "passed", "proof_command": "go test ./...", "provenance": provenance, "evidence_ref": map[string]any{"kind": "proof", "id": "proof:bridge:0"}}},
		"evidence_refs": payload["evidence_refs"], "worker_claim_ref": payload["worker_claim_ref"],
		"verified_at": "2026-07-28T12:00:00Z", "worker_statement": "worker completed the contract",
	})
}

func bridgeContract(t *testing.T, id string) []byte {
	t.Helper()
	specification := decodeObject(t, bridgeSpecification(t))
	decision := decodeObject(t, bridgeApprovalDecision(t))
	specificationID := specification["id"].(string)
	specificationDigest := specification["content_hash"].(string)
	payload := map[string]any{
		"task_id": "task-1", "repository": "homebase", "base_commit": "0123456789012345678901234567890123456789",
		"allowed_paths": []string{"internal/"}, "forbidden_paths": []string{"secrets/"},
		"context_hash": hex.EncodeToString(make([]byte, 32)), "context_valid_until": "2026-12-31T00:00:00Z",
		"idempotency_key": "idem-1", "worker_id": "worker-1", "verifier_id": "verifier-1",
		"acceptance": []string{"go test ./..."}, "publication": "prohibited",
		"specification_id": specificationID, "specification_digest": specificationDigest,
	}
	return mustJSON(t, map[string]any{
		"kind": "Contract", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "decision", "id": decision["id"], "content_hash": decision["content_hash"]}, map[string]any{"kind": "specification", "id": specificationID, "content_hash": specificationDigest}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
		"status": "approved", "source": map[string]any{"id": "homebase", "role": "homebase"}, "payload": payload,
	})
}

func bridgeVerificationContract(t *testing.T, id string) []byte {
	t.Helper()
	document := decodeObject(t, bridgeContract(t, id))
	payload := document["payload"].(map[string]any)
	payload["verifier_id"] = "bridge:verifier"
	document["content_hash"] = payloadHash(t, payload)
	return mustJSON(t, document)
}

func canonicalHash(t *testing.T, value any) (string, error) {
	t.Helper()
	canonical, err := records.CanonicalJSONValue(mustJSON(t, value))
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func bridgeGrant(t *testing.T, id, contractID string) []byte {
	t.Helper()
	specification := decodeObject(t, bridgeSpecification(t))
	specificationID := specification["id"].(string)
	specificationDigest := specification["content_hash"].(string)
	payload := map[string]any{
		"grant_id": id, "contract_id": contractID, "task_id": "task-1", "worker_id": "worker-1",
		"allowed_paths": []string{"internal/"}, "commands": []string{"go test ./..."},
		"issued_at": "2026-07-28T00:00:00Z", "expires_at": "2026-12-31T00:00:00Z",
		"context_hash": hex.EncodeToString(make([]byte, 32)), "idempotency_key": "idem-1", "effect_id": "effect-1",
		"specification_id": specificationID, "specification_digest": specificationDigest,
	}
	return mustJSON(t, map[string]any{
		"kind": "CapabilityGrant", "version": "1", "id": id,
		"source_refs":  []any{map[string]any{"kind": "contract", "id": contractID}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityAuthoritative, "freshness": map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
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

func bridgeApprovalDecision(t *testing.T) []byte {
	t.Helper()
	specification := decodeObject(t, bridgeSpecification(t))
	payload := map[string]any{
		"decision":   "approve specification spec:homebase:api-test:v1",
		"scope":      "running-machine contract admission",
		"decided_by": "captain",
		"specification_ref": map[string]any{
			"kind":         "specification",
			"id":           specification["id"],
			"content_hash": specification["content_hash"],
		},
	}
	return mustJSON(t, map[string]any{
		"kind": "Decision", "version": "1", "id": "decision:spec-api-test",
		"source_refs":  []any{map[string]any{"kind": "document", "id": "captain-approval"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil}, "status": "approved",
		"source": map[string]any{"id": "captain", "role": "captain"}, "payload": payload,
	})
}

func bridgeSpecification(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"purpose":   "test approved specification",
		"scope":     map[string]any{"systems": []string{"HomeBase"}, "effects": []string{"admission"}},
		"non_goals": []string{"production certification"}, "requirements": []any{},
		"proof_obligations": []any{}, "golden_scenarios": []any{}, "context_sources": []any{}, "assumptions": []any{},
		"admission_policy": map[string]any{"requires_human_approval": true, "fail_closed_on_open_obligation": true, "worker_may_authorize": false},
		"approval_ref":     map[string]any{"kind": "decision", "id": "decision:spec-api-test"},
		"revision_policy":  "new ID and digest for every revision",
	}
	return mustJSON(t, map[string]any{
		"kind": "Specification", "version": "1", "id": "spec:homebase:api-test:v1",
		"source_refs":  []any{map[string]any{"kind": "document", "id": "api-test-spec"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil}, "status": "approved",
		"source": map[string]any{"id": "captain", "role": "captain"}, "payload": payload,
	})
}

func TestHandleStatusReportsReadinessWithoutSecrets(t *testing.T) {
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

	request := httptest.NewRequest(http.MethodGet, "/v1/status", nil)
	response := httptest.NewRecorder()
	server.HandleStatus(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("GET /v1/status = %d, want 200; body=%s", response.Code, response.Body.String())
	}
	if ct := response.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode status body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}
	if body["ledger_ready"] != true {
		t.Fatalf("ledger_ready = %v, want true", body["ledger_ready"])
	}
	if body["record_store_ready"] != true {
		t.Fatalf("record_store_ready = %v, want true", body["record_store_ready"])
	}

	forbidden := []string{"key", "priv", "secret", "journal", "receipt.priv", "bridge.pub"}
	lowerBody := strings.ToLower(response.Body.String())
	for _, needle := range forbidden {
		if strings.Contains(lowerBody, needle) {
			t.Fatalf("status response leaked forbidden term %q: %s", needle, response.Body.String())
		}
	}
}

func TestHandleStatusRejectsNonGET(t *testing.T) {
	ledgerStore, err := ledger.NewStore(t.TempDir() + "/legacy.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer ledgerStore.Close()
	server := NewServer(validation.NewValidator(nil, ledgerStore), nil, ledgerStore)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		request := httptest.NewRequest(method, "/v1/status", nil)
		response := httptest.NewRecorder()
		server.HandleStatus(response, request)
		if response.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s /v1/status = %d, want 405", method, response.Code)
		}
	}
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
