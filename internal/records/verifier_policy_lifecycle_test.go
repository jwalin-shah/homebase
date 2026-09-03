package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"homebase/internal/journal"
	"testing"
	"time"
)

func TestStoreVerifierAuthorityAcceptsActiveEnrolledKey(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store, j, authorities, _ := newAuthorizedVerificationStore(t, now)
	defer j.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority := bindVerifierAuthorityForTest(t, authorities.VerifierPolicy, []VerifierAuthority{{
		VerifierID: productionVerifierID, KeyID: "active-key", PublicKey: public,
		ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}})
	raw := productionBridgeSubmission(t, private, "active-key")
	if err := authority.Verify(raw, now); err != nil {
		t.Fatalf("active verifier preflight: %v", err)
	}
	if _, err := store.AppendBridgeVerificationSubmissionAuthorized(raw, authority); err != nil {
		t.Fatalf("active verifier append: %v", err)
	}
}

func TestStoreVerifierAuthorityRejectsExpiredAndUnenrolledKeysWithoutMutation(t *testing.T) {
	now := time.Date(2026, 8, 2, 12, 0, 0, 0, time.UTC)
	store, j, authorities, _ := newAuthorizedVerificationStore(t, now)
	defer j.Close()
	oldPublic, oldPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	newPublic, newPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority := bindVerifierAuthorityForTest(t, authorities.VerifierPolicy, []VerifierAuthority{{
		VerifierID: productionVerifierID, KeyID: "old-key", PublicKey: oldPublic,
		ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}, {
		VerifierID: productionVerifierID, KeyID: "new-key", PublicKey: newPublic,
		ValidFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
	}})
	before := len(store.List())
	if _, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, oldPrivate, "old-key"), authority); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("expired key error = %v, want ErrAuthorityRequired", err)
	}
	if after := len(store.List()); after != before {
		t.Fatalf("expired key changed store: before=%d after=%d", before, after)
	}
	_, attackerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, attackerPrivate, "attacker-key"), authority); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("unenrolled key error = %v, want ErrAuthorityRequired", err)
	}
	if after := len(store.List()); after != before {
		t.Fatalf("unenrolled key changed store: before=%d after=%d", before, after)
	}
	// The replacement key is enrolled and active, but productionBridgeSubmission
	// uses a fixed 2026-07-28 verified_at, so it must still fail the replacement
	// key's 2026-08-01 validity window rather than accepting a backdated receipt.
	if _, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, newPrivate, "new-key"), authority); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("replacement key with pre-window receipt error = %v, want ErrAuthorityRequired", err)
	}
}

func TestStoreVerifierAuthorityRejectsCompromisedKeyEvenForBackdatedReceipt(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	store, j, authorities, _ := newAuthorizedVerificationStore(t, now)
	defer j.Close()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	compromisedAt := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	authority := bindVerifierAuthorityForTest(t, authorities.VerifierPolicy, []VerifierAuthority{{
		VerifierID: productionVerifierID, KeyID: "compromised-key", PublicKey: public,
		ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), CompromisedAt: &compromisedAt,
	}})
	before := len(store.List())
	// The receipt's fixed verified_at is 2026-07-28, before CompromisedAt. New
	// admission must still reject it because possession of a compromised key
	// cannot prove when the signature was actually created.
	if _, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, private, "compromised-key"), authority); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("compromised key error = %v, want ErrAuthorityRequired", err)
	}
	if after := len(store.List()); after != before {
		t.Fatalf("compromised key changed store: before=%d after=%d", before, after)
	}
}

func TestRetiredVerifierReceiptReplaysWithoutCurrentPolicy(t *testing.T) {
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	store, j, authorities, path := newAuthorizedVerificationStore(t, now)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	authority := bindVerifierAuthorityForTest(t, authorities.VerifierPolicy, []VerifierAuthority{{
		VerifierID: productionVerifierID, KeyID: "retired-key", PublicKey: public,
		ValidFrom: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), ValidUntil: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
	}})
	first, err := store.AppendBridgeVerificationSubmissionAuthorized(productionBridgeSubmission(t, private, "retired-key"), authority)
	if err != nil {
		t.Fatalf("append before retirement: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedJournal, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopenedJournal.Close()
	reopened, err := NewStore(reopenedJournal)
	if err != nil {
		t.Fatalf("policy-free historical replay: %v", err)
	}
	if _, err := reopened.Get(first.Receipt.ID); err != nil {
		t.Fatalf("retired verifier receipt missing after replay: %v", err)
	}
}

func bindVerifierAuthorityForTest(t *testing.T, storeAuthority StoreAuthority, verifierAuthorities []VerifierAuthority) StoreVerifierAuthority {
	t.Helper()
	policy, err := NewVerifierPolicy(verifierAuthorities)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := BindStoreVerifierPolicy(storeAuthority, policy)
	if err != nil {
		t.Fatal(err)
	}
	return authority
}

func newAuthorizedVerificationStore(t *testing.T, now time.Time) (*Store, *journal.BinaryJournal, StoreAuthorities, string) {
	t.Helper()
	_, seededJournal, path := newStoreWithHistoricalRecords(t,
		validApprovalDecision(t),
		validSpecification(t),
		validBridgeContract(t, "contract-1", productionVerifierID),
		validGrant(t, "grant-1", "contract-1", "idem-1"),
	)
	if err := seededJournal.Close(); err != nil {
		t.Fatal(err)
	}
	reopenedJournal, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, authorities, err := NewStoreWithClockAndAuthorities(reopenedJournal, func() time.Time { return now })
	if err != nil {
		reopenedJournal.Close()
		t.Fatal(err)
	}
	return store, reopenedJournal, authorities, path
}
