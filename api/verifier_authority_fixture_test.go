package api

import (
	"crypto/ed25519"
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
