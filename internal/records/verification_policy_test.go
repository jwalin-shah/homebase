package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"homebase/internal/journal"
	"testing"
)

// TASK-021 requires legacy Bridge verifier receipts to remain replayable as
// historical evidence while being rejected at every new-submission boundary.
// This test is intentionally red until VerifyBridgeReceiptAttestation stops
// treating bridge:verifier as sufficient authority for a new receipt.
func TestNewBridgeVerificationRejectsLegacyVerifierAtAttestationBoundary(t *testing.T) {
	raw := bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789")
	if err := VerifyBridgeReceiptAttestation(raw, nil, ""); err == nil {
		t.Fatal("new verification admission accepted unsigned legacy verifier identity")
	}
}

// The Store is a durable authority boundary, not merely an HTTP persistence
// helper. A caller that reaches the Store directly must not bypass the same
// verifier policy enforced by the HTTP ingress.
func TestNewBridgeVerificationRejectsLegacyVerifierAtStoreBoundary(t *testing.T) {
	store, j := newStoreWithHistoricalVerificationAuthority(t, legacyVerifierID)
	defer j.Close()

	before := len(store.List())
	raw := bridgeSubmissionWithProvenance(t, "contract-1", "grant-1", "0123456789012345678901234567890123456789")
	if _, err := store.AppendBridgeVerificationSubmission(raw); err == nil {
		t.Fatal("Store accepted a new unsigned legacy verifier receipt")
	}
	if after := len(store.List()); after != before {
		t.Fatalf("rejected legacy submission changed store: before=%d after=%d", before, after)
	}
}

// A production-looking receipt is not independently verified merely because
// it carries an attestation object. The Store must have an explicitly enrolled
// verifier authority (or receive a capability proving that check) before it
// can admit the receipt. Today direct Store callers can bypass that policy.
func TestNewBridgeVerificationStoreRequiresEnrolledProductionVerifier(t *testing.T) {
	store, j := newStoreWithHistoricalVerificationAuthority(t, productionVerifierID)
	defer j.Close()

	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	before := len(store.List())
	raw := productionBridgeSubmission(t, private, "unenrolled-verifier-key")
	if _, err := store.AppendBridgeVerificationSubmission(raw); err == nil {
		t.Fatal("Store accepted a production verifier receipt without any enrolled verifier policy")
	}
	if after := len(store.List()); after != before {
		t.Fatalf("rejected production submission changed store: before=%d after=%d", before, after)
	}
}

// A1 verifier-policy tests need historical authoritative prerequisites, not a
// second new-submission escape hatch. Seed those records directly into the
// journal and reopen the Store so future A0 hardening of generic Store.Append
// cannot mask verifier-policy failures during test setup. This also exercises
// the intended invariant that historical authoritative records remain readable
// after new-submission admission is tightened.
func newStoreWithHistoricalVerificationAuthority(t *testing.T, verifierID string) (*Store, *journal.BinaryJournal) {
	t.Helper()
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}

	records := [][]byte{
		validApprovalDecision(t),
		validSpecification(t),
		validBridgeContract(t, "contract-1", verifierID),
		validGrant(t, "grant-1", "contract-1", "idem-1"),
	}
	for _, raw := range records {
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
	store, err := NewStore(reopened)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened
}
