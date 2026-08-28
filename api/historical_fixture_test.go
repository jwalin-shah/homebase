package api

import (
	"homebase/internal/journal"
	"homebase/internal/records"
	"testing"
)

// newAPIStoreWithHistoricalRecords seeds authoritative prerequisites through
// journal replay and then reopens Store. Use it only for already-committed
// historical setup; never use it for the record whose authenticated admission
// or cross-record behavior is the target of the test.
func newAPIStoreWithHistoricalRecords(t *testing.T, raws ...[]byte) (*records.Store, *journal.BinaryJournal) {
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
	store, err := records.NewStore(reopened)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened
}

// newAPIStoreWithHistoricalRecordsAndAuthorities is the authority-bearing
// sibling for tests whose live target is a capability-gated Store mutation.
// Prerequisites are still replayed as historical state before Store authority
// is minted; unrelated read-only/verification fixtures should keep using the
// authority-free helper above.
func newAPIStoreWithHistoricalRecordsAndAuthorities(t *testing.T, raws ...[]byte) (*records.Store, *journal.BinaryJournal, records.StoreAuthorities) {
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
	store, authorities, err := records.NewStoreWithAuthorities(reopened)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened, authorities
}
