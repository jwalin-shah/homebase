package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"homebase/internal/journal"
	"testing"
	"time"
)

func TestVerifierPolicyReplacementKeySucceedsInsideNewWindow(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 7, 28, 12, 30, 0, 0, time.UTC) }
	path := t.TempDir() + "/records.journal"
	seed, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range [][]byte{
		validApprovalDecision(t),
		validSpecification(t),
		validBridgeContract(t, "contract-1", productionVerifierID),
		validGrant(t, "grant-1", "contract-1", "idem-1"),
	} {
		encoded, err := journal.EncodeRecord(journal.RecordKindSharedRecord, raw)
		if err != nil {
			seed.Close()
			t.Fatal(err)
		}
		if _, err := seed.Append(encoded); err != nil {
			seed.Close()
			t.Fatal(err)
		}
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}

	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, authorities, err := NewStoreWithClockAndAuthorities(j, clock)
	if err != nil {
		t.Fatal(err)
	}

	oldPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	replacementPublic, replacementPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewVerifierPolicy([]VerifierAuthority{
		{
			VerifierID: productionVerifierID,
			KeyID:      "verifier-key-old",
			PublicKey:  oldPublic,
			ValidFrom:  time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC),
			ValidUntil: time.Date(2026, 7, 28, 11, 45, 0, 0, time.UTC),
		},
		{
			VerifierID: productionVerifierID,
			KeyID:      "verifier-key-replacement",
			PublicKey:  replacementPublic,
			ValidFrom:  time.Date(2026, 7, 28, 11, 45, 0, 0, time.UTC),
			ValidUntil: time.Date(2026, 7, 28, 13, 30, 0, 0, time.UTC),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	verifierAuthority, err := BindStoreVerifierPolicy(authorities.VerifierPolicy, policy)
	if err != nil {
		t.Fatal(err)
	}

	raw := productionBridgeSubmission(t, replacementPrivate, "verifier-key-replacement")
	result, err := store.AppendBridgeVerificationSubmissionAuthorized(raw, verifierAuthority)
	if err != nil {
		t.Fatalf("replacement verifier inside active window was rejected: %v", err)
	}
	if result.Existing || result.Receipt.Kind != "VerificationReceipt" {
		t.Fatalf("unexpected replacement-key admission result: %+v", result)
	}
}
