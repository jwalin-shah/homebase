package records

import (
	"homebase/internal/journal"
	"testing"
)

// newStoreWithHistoricalRecords seeds authoritative prerequisites through the
// journal replay path, then reopens the Store. Use this only for test setup
// representing already-committed history; never use it for the record whose
// new-submission admission or cross-record validation is under test.
func newStoreWithHistoricalRecords(t *testing.T, raws ...[]byte) (*Store, *journal.BinaryJournal, string) {
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
	store, err := NewStore(reopened)
	if err != nil {
		reopened.Close()
		t.Fatal(err)
	}
	return store, reopened, path
}
