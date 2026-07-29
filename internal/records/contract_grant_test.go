package records

import (
	"testing"

	"homebase/internal/journal"
)

func TestContractGrantCommitIsAtomicIdempotentAndRebuildable(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	contract := validContract(t, "contract-1")
	grant := validGrant(t, "grant-1", "contract-1", "idem-1")
	first, err := store.AppendContractAndGrant(contract, grant)
	if err != nil {
		t.Fatalf("first contract/grant commit: %v", err)
	}
	if first.Existing || first.Sequence != 1 || first.Contract.ID != "contract-1" || first.Grant.ID != "grant-1" {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := store.AppendContractAndGrant(contract, grant)
	if err != nil {
		t.Fatalf("duplicate contract/grant commit: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate contract/grant commit was not idempotent: %+v", second)
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("record count = %d, want 2", got)
	}
	if _, err := store.AppendContractAndGrant(contract, validGrant(t, "grant-2", "contract-1", "idem-2")); err == nil {
		t.Fatal("conflicting grant replaced the committed pair")
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("conflicting pair changed record count to %d", got)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	reopened, err := NewStore(j2)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if got := len(reopened.List()); got != 2 {
		t.Fatalf("replayed record count = %d, want 2", got)
	}
	if _, err := reopened.Get("contract-1"); err != nil {
		t.Fatalf("replayed contract missing: %v", err)
	}
	if _, err := reopened.Get("grant-1"); err != nil {
		t.Fatalf("replayed grant missing: %v", err)
	}
}

func TestContractGrantCommitRejectsMissingContractLineageAtomically(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	grant := validGrant(t, "grant-1", "missing-contract", "idem-1")
	if _, err := store.AppendContractAndGrant(validContract(t, "contract-1"), grant); err == nil {
		t.Fatal("contract/grant commit accepted mismatched grant lineage")
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("failed contract/grant commit changed store: %d", got)
	}
}
