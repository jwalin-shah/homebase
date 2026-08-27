package records

import (
	"errors"
	"homebase/internal/journal"
	"testing"
)

// TASK-021: generic Store.Append is not an authenticated authority boundary.
// Schema-valid records carrying authoritative classes must be rejected before
// any journal mutation and routed through a typed, authority-bearing path.
func TestRawAppendRejectsVerifiedEvidenceWithoutAuthority(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()

	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	before := len(store.List())

	if _, err := store.Append(validEvidence(t, "raw-authoritative-evidence")); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("raw authoritative append error = %v, want ErrAuthorityRequired", err)
	}
	if after := len(store.List()); after != before {
		t.Fatalf("rejected authoritative append changed store: before=%d after=%d", before, after)
	}
}
