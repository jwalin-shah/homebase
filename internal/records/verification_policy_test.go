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
// second new-submission escape hatch. Reuse the generic replay helper so A0
// fixture migration follows one invariant across the records package.
func newStoreWithHistoricalVerificationAuthority(t *testing.T, verifierID string) (*Store, *journal.BinaryJournal) {
	t.Helper()
	store, j, _ := newStoreWithHistoricalRecords(t,
		validApprovalDecision(t),
		validSpecification(t),
		validBridgeContract(t, "contract-1", verifierID),
		validGrant(t, "grant-1", "contract-1", "idem-1"),
	)
	return store, j
}
