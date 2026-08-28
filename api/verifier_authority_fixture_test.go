package api

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"homebase/internal/journal"
	"homebase/internal/records"
	"testing"
	"time"
)

// newAPIStoreWithHistoricalRecordsAndClockAndAuthorities replays only
// already-committed authoritative prerequisites, then reopens the live Store
// with the deterministic clock and Store-bound capabilities required by an
// authenticated verifier-admission test. The verification receipt under test
// must still enter through the live authority-bearing handler.
func newAPIStoreWithHistoricalRecordsAndClockAndAuthorities(t *testing.T, clock func() time.Time, raws ...[]byte) (*records.Store, *journal.BinaryJournal, records.StoreAuthorities) {
	t.Helper()
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range raws {
		encoded, err := journal.EncodeRecord(journal.RecordKindSharedRecord, raw)
		if err != nil {
			j.Close()
			t.Fatal(err)
		}
		if _, err := j.Append(encoded); err != nil {
			j.Close()
			t.Fatal(err)
		}
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, authorities, err := records.NewStoreWithClockAndAuthorities(reopened, clock)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened, authorities
}

func bindAPIVerifierAuthorityForTest(t *testing.T, storeAuthority records.StoreAuthority, public ed25519.PublicKey, keyID string, validFrom, validUntil time.Time) records.StoreVerifierAuthority {
	t.Helper()
	policy, err := records.NewVerifierPolicy([]records.VerifierAuthority{{
		VerifierID: "bridge:verifier:v2",
		KeyID:      keyID,
		PublicKey:  public,
		ValidFrom:  validFrom,
		ValidUntil: validUntil,
	}})
	if err != nil {
		t.Fatal(err)
	}
	authority, err := records.BindStoreVerifierPolicy(storeAuthority, policy)
	if err != nil {
		t.Fatal(err)
	}
	return authority
}

func bridgeProductionVerificationContract(t *testing.T, id string) []byte {
	t.Helper()
	document := decodeObject(t, bridgeContract(t, id))
	payload := document["payload"].(map[string]any)
	payload["verifier_id"] = "bridge:verifier:v2"
	document["content_hash"] = payloadHash(t, payload)
	return mustJSON(t, document)
}

func productionBridgeReceiptForAPI(t *testing.T, private ed25519.PrivateKey, keyID string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(bridgeReceipt(t), &document); err != nil {
		t.Fatal(err)
	}
	document["verifier_id"] = "bridge:verifier:v2"
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
	contentHash, err := records.CanonicalContentHash(mustJSON(t, durablePayload))
	if err != nil {
		t.Fatal(err)
	}
	document["content_hash"] = contentHash
	message := []byte("running-machine-verifier-attestation:v1:" + document["id"].(string) + ":" + contentHash + "\x00" + document["worker_statement"].(string))
	document["attestation"] = map[string]any{
		"scheme":    "ed25519-v1",
		"key_id":    keyID,
		"signature": hex.EncodeToString(ed25519.Sign(private, message)),
	}
	return mustJSON(t, document)
}

func sha256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
