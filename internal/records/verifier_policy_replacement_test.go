package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"homebase/internal/journal"
	"strings"
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

func TestAuthorizedVerificationStillRejectsExpiredGrant(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 7, 28, 14, 0, 0, 0, time.UTC) }
	expiredGrant := decodeObject(t, validGrant(t, "grant-1", "contract-1", "idem-1"))
	grantPayload := expiredGrant["payload"].(map[string]any)
	grantPayload["expires_at"] = "2026-07-28T13:00:00Z"
	expiredGrant["content_hash"] = payloadHashBridge(t, grantPayload)
	grantFreshness := expiredGrant["freshness"].(map[string]any)
	grantFreshness["valid_until"] = "2026-07-28T13:00:00Z"

	store, j, authority, private := newAuthorizedVerificationTestStore(t, clock,
		validApprovalDecision(t),
		validSpecification(t),
		validBridgeContract(t, "contract-1", productionVerifierID),
		mustJSONBridge(t, expiredGrant),
	)
	defer j.Close()
	before := len(store.List())
	_, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, private, "verifier-key-active"), authority)
	if err == nil || !strings.Contains(err.Error(), "CapabilityGrant expired before verification submission") {
		t.Fatalf("authorized expired-grant submission error = %v", err)
	}
	if got := len(store.List()); got != before {
		t.Fatalf("expired-grant rejection mutated store: before=%d after=%d", before, got)
	}
}

func TestAuthorizedVerificationCannotMintMissingContractOrGrant(t *testing.T) {
	clock := func() time.Time { return time.Date(2026, 7, 28, 12, 30, 0, 0, time.UTC) }
	store, j, authority, private := newAuthorizedVerificationTestStore(t, clock)
	defer j.Close()
	_, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, private, "verifier-key-active"), authority)
	if err == nil {
		t.Fatal("authorized Bridge submission minted missing Contract/CapabilityGrant")
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("missing-authority rejection changed store: %d records", got)
	}
}

func newAuthorizedVerificationTestStore(t *testing.T, clock func() time.Time, raws ...[]byte) (*Store, *journal.BinaryJournal, StoreVerifierAuthority, ed25519.PrivateKey) {
	t.Helper()
	path := t.TempDir() + "/records.journal"
	seed, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range raws {
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
	store, authorities, err := NewStoreWithClockAndAuthorities(j, clock)
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	policy, err := NewVerifierPolicy([]VerifierAuthority{{
		VerifierID: productionVerifierID,
		KeyID:      "verifier-key-active",
		PublicKey:  public,
		ValidFrom:  time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC),
		ValidUntil: time.Date(2026, 7, 28, 15, 0, 0, 0, time.UTC),
	}})
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	authority, err := BindStoreVerifierPolicy(authorities.VerifierPolicy, policy)
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	return store, j, authority, private
}
