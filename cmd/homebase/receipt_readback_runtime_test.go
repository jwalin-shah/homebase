package main_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"homebase/internal/journal"
	"homebase/internal/records"
)

// CP-038X proves the compiled cmd/homebase process serves the receipt read-back
// route with temp storage, ephemeral port, and generated Bridge keys only.
func TestCompiledReceiptReadbackRoute(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "homebase_records.journal")
	if err := seedBridgePrerequisites(t, journalPath); err != nil {
		t.Fatal(err)
	}

	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(workDir, "homebase")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/homebase: %v\n%s", err, out)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"HOMEBASE_RECORD_JOURNAL="+journalPath,
		"HOMEBASE_BRIDGE_PUBLIC_KEY_HEX="+hex.EncodeToString(public),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForServer(baseURL, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	receiptRaw := bridgeReceiptJSON(t)
	signBridge := func(raw []byte) string {
		canonical, err := records.CanonicalJSONValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	receiptID := "receipt:task-1:0123456789012345678901234567890123456789"

	appendReq, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/bridge", bytes.NewReader(receiptRaw))
	if err != nil {
		t.Fatal(err)
	}
	appendReq.Header.Set("Content-Type", "application/json")
	appendReq.Header.Set("X-Bridge-Verification-Signature", signBridge(receiptRaw))
	appendReq.Header.Set("Idempotency-Key", receiptID)
	appendResp, err := http.DefaultClient.Do(appendReq)
	if err != nil {
		t.Fatal(err)
	}
	appendBody, _ := io.ReadAll(appendResp.Body)
	_ = appendResp.Body.Close()
	if appendResp.StatusCode != http.StatusCreated {
		t.Fatalf("append status = %d, body = %s", appendResp.StatusCode, appendBody)
	}

	signRead := func(raw []byte) string {
		canonical, err := records.CanonicalJSONValue(raw)
		if err != nil {
			t.Fatal(err)
		}
		return hex.EncodeToString(ed25519.Sign(private, canonical))
	}
	readBody := mustJSON(t, map[string]string{"receipt_id": receiptID})

	readReq, err := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	if err != nil {
		t.Fatal(err)
	}
	readReq.Header.Set("Content-Type", "application/json")
	readReq.Header.Set("X-Bridge-Verification-Read-Signature", signRead(readBody))
	readResp, err := http.DefaultClient.Do(readReq)
	if err != nil {
		t.Fatal(err)
	}
	readBytes, _ := io.ReadAll(readResp.Body)
	_ = readResp.Body.Close()
	if readResp.StatusCode != http.StatusOK {
		t.Fatalf("read status = %d, body = %s", readResp.StatusCode, readBytes)
	}

	missingBody := mustJSON(t, map[string]string{"receipt_id": "receipt:task-9:0123456789012345678901234567890123456789"})
	missingReq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/receipts/read", bytes.NewReader(missingBody))
	missingReq.Header.Set("Content-Type", "application/json")
	missingReq.Header.Set("X-Bridge-Verification-Read-Signature", signRead(missingBody))
	missingResp, err := http.DefaultClient.Do(missingReq)
	if err != nil {
		t.Fatal(err)
	}
	missingBytes, _ := io.ReadAll(missingResp.Body)
	_ = missingResp.Body.Close()
	if missingResp.StatusCode != http.StatusNotFound {
		t.Fatalf("missing receipt status = %d, body = %s", missingResp.StatusCode, missingBytes)
	}

	wrongKindBody := mustJSON(t, map[string]string{"receipt_id": "receipt:wrong-kind:0123456789012345678901234567890123456789"})
	wrongKindReq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/receipts/read", bytes.NewReader(wrongKindBody))
	wrongKindReq.Header.Set("Content-Type", "application/json")
	wrongKindReq.Header.Set("X-Bridge-Verification-Read-Signature", signRead(wrongKindBody))
	wrongKindResp, err := http.DefaultClient.Do(wrongKindReq)
	if err != nil {
		t.Fatal(err)
	}
	wrongKindBytes, _ := io.ReadAll(wrongKindResp.Body)
	_ = wrongKindResp.Body.Close()
	if wrongKindResp.StatusCode != http.StatusNotFound {
		t.Fatalf("wrong-kind receipt status = %d, body = %s", wrongKindResp.StatusCode, wrongKindBytes)
	}

	badSigReq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	badSigReq.Header.Set("Content-Type", "application/json")
	badSigReq.Header.Set("X-Bridge-Verification-Read-Signature", hex.EncodeToString(ed25519.Sign(private, []byte("wrong"))))
	badSigResp, err := http.DefaultClient.Do(badSigReq)
	if err != nil {
		t.Fatal(err)
	}
	badSigBytes, _ := io.ReadAll(badSigResp.Body)
	_ = badSigResp.Body.Close()
	if badSigResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad read signature status = %d, body = %s", badSigResp.StatusCode, badSigBytes)
	}

	noAuthReq, _ := http.NewRequest(http.MethodPost, baseURL+"/api/v1/verifications/receipts/read", bytes.NewReader(readBody))
	noAuthReq.Header.Set("Content-Type", "application/json")
	noAuthResp, err := http.DefaultClient.Do(noAuthReq)
	if err != nil {
		t.Fatal(err)
	}
	noAuthBytes, _ := io.ReadAll(noAuthResp.Body)
	_ = noAuthResp.Body.Close()
	if noAuthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("missing auth status = %d, body = %s", noAuthResp.StatusCode, noAuthBytes)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()

	canonical := canonicalStoredBytes(t, journalPath, receiptID)
	if !bytes.Equal(readBytes, append(canonical, '\n')) {
		t.Fatalf("read response is not canonical stored bytes:\n got %s\n want %s", readBytes, append(canonical, '\n'))
	}
}

func canonicalStoredBytes(t *testing.T, journalPath, receiptID string) []byte {
	t.Helper()
	j, err := journal.OpenBinaryJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := records.NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := store.GetVerificationReceiptCanonical(receiptID)
	if err != nil {
		t.Fatal(err)
	}
	return canonical
}

func waitForServer(baseURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/v1/records")
		if err == nil {
			_ = resp.Body.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("server at %s did not become ready within %s", baseURL, timeout)
}

func seedBridgePrerequisites(t *testing.T, journalPath string) error {
	t.Helper()
	j, err := journal.OpenBinaryJournal(journalPath)
	if err != nil {
		return err
	}
	defer j.Close()
	store, err := records.NewStore(j)
	if err != nil {
		return err
	}
	for _, raw := range [][]byte{
		bridgeApprovalDecision(t),
		bridgeSpecification(t),
		bridgeVerificationContract(t, "contract-1"),
		bridgeGrant(t, "grant-1", "contract-1"),
		wrongKindContractRecord(t, "receipt:wrong-kind:0123456789012345678901234567890123456789"),
	} {
		if _, err := store.Append(raw); err != nil {
			return err
		}
	}
	return nil
}

func wrongKindContractRecord(t *testing.T, id string) []byte {
	t.Helper()
	document := decodeObject(t, bridgeVerificationContract(t, "contract-shadow"))
	document["id"] = id
	document["kind"] = "Contract"
	return mustJSON(t, document)
}

func bridgeReceiptJSON(t *testing.T) []byte {
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
		"checks":        durableChecks,
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
		"source_refs": []any{
			map[string]any{"kind": "decision", "id": decision["id"], "content_hash": decision["content_hash"]},
			map[string]any{"kind": "specification", "id": specificationID, "content_hash": specificationDigest},
		},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
		"status":          "approved", "source": map[string]any{"id": "homebase", "role": "homebase"}, "payload": payload,
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
		"authority_class": records.AuthorityAuthoritative,
		"freshness":       map[string]any{"mode": "time_bound", "valid_until": "2026-12-31T00:00:00Z"},
		"status":          "active", "source": map[string]any{"id": "bridge", "role": "bridge"}, "payload": payload,
	})
}

func bridgeApprovalDecision(t *testing.T) []byte {
	t.Helper()
	specification := decodeObject(t, bridgeSpecification(t))
	payload := map[string]any{
		"decision":   "approve specification spec:homebase:api-test:v1",
		"scope":      "running-machine contract admission",
		"decided_by": "captain",
		"specification_ref": map[string]any{
			"kind": "specification", "id": specification["id"], "content_hash": specification["content_hash"],
		},
	}
	return mustJSON(t, map[string]any{
		"kind": "Decision", "version": "1", "id": "decision:spec-api-test",
		"source_refs":  []any{map[string]any{"kind": "document", "id": "captain-approval"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil},
		"status":          "approved", "source": map[string]any{"id": "captain", "role": "captain"}, "payload": payload,
	})
}

func bridgeSpecification(t *testing.T) []byte {
	t.Helper()
	payload := map[string]any{
		"purpose":   "test approved specification",
		"scope":     map[string]any{"systems": []string{"HomeBase"}, "effects": []string{"admission"}},
		"non_goals": []string{"production certification"}, "requirements": []any{},
		"proof_obligations": []any{}, "golden_scenarios": []any{}, "context_sources": []any{}, "assumptions": []any{},
		"admission_policy": map[string]any{
			"requires_human_approval": true, "fail_closed_on_open_obligation": true, "worker_may_authorize": false,
		},
		"approval_ref":    map[string]any{"kind": "decision", "id": "decision:spec-api-test"},
		"revision_policy": "new ID and digest for every revision",
	}
	return mustJSON(t, map[string]any{
		"kind": "Specification", "version": "1", "id": "spec:homebase:api-test:v1",
		"source_refs":  []any{map[string]any{"kind": "document", "id": "api-test-spec"}},
		"content_hash": payloadHash(t, payload), "captured_at": "2026-07-28T00:00:00Z",
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil},
		"status":          "approved", "source": map[string]any{"id": "captain", "role": "captain"}, "payload": payload,
	})
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
