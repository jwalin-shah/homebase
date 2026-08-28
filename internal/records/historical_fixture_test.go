package records

import (
	"homebase/internal/journal"
	"testing"
	"time"
)

// newStoreWithHistoricalRecords seeds authoritative prerequisites through the
// journal replay path, then reopens the Store. Use this only for test setup
// representing already-committed history; never use it for the record whose
// new-submission admission or cross-record validation is under test.
func newStoreWithHistoricalRecords(t *testing.T, raws ...[]byte) (*Store, *journal.BinaryJournal, string) {
	t.Helper()
	return newStoreWithHistoricalRecordsAndClock(t, nil, raws...)
}

// newStoreWithHistoricalRecordsAndClock is the clocked form of
// newStoreWithHistoricalRecords. It exists for tests whose live target seam
// depends on Store time while their authoritative prerequisites are historical.
func newStoreWithHistoricalRecordsAndClock(t *testing.T, clock func() time.Time, raws ...[]byte) (*Store, *journal.BinaryJournal, string) {
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
	var store *Store
	if clock == nil {
		store, err = NewStore(reopened)
	} else {
		store, err = NewStoreWithClock(reopened, clock)
	}
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened, path
}
