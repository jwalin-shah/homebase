package records

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"testing"
)

// productionBridgeSubmissionForTree is the production-verifier sibling of
// productionBridgeSubmission for tests that need more than one candidate Git
// tree while preserving the same signed verifier-policy admission boundary.
func productionBridgeSubmissionForTree(t *testing.T, private ed25519.PrivateKey, keyID, treeSHA string) []byte {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal(bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", treeSHA), &document); err != nil {
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
		"scheme": attestationScheme,
		"key_id": keyID,
		"signature": hex.EncodeToString(ed25519.Sign(private, []byte(
			"running-machine-verifier-attestation:v1:"+document["id"].(string)+":"+contentHash+"\x00"+document["worker_statement"].(string),
		))),
	}
	return mustJSONBridge(t, document)
}
