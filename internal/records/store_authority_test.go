package records

import (
	"errors"
	"homebase/internal/journal"
	"testing"
)

func TestSpecializedStoreAuthorityRejectsMissingCrossStoreAndWrongDomainBeforeParse(t *testing.T) {
	storeA, authoritiesA, journalA := newStoreWithAuthoritiesForTest(t)
	defer journalA.Close()
	_, authoritiesB, journalB := newStoreWithAuthoritiesForTest(t)
	defer journalB.Close()

	malformed := []byte(`{"not":"a record"}`)

	promotionCases := map[string]StoreAuthority{
		"missing":      {},
		"cross_store":  authoritiesB.Promotion,
		"wrong_domain": authoritiesA.ContractGrant,
	}
	for name, authority := range promotionCases {
		t.Run("promotion_"+name, func(t *testing.T) {
			before := len(storeA.List())
			if _, err := storeA.AppendPromotionCommitAuthorized(authority, malformed, malformed, malformed); !errors.Is(err, ErrAuthorityRequired) {
				t.Fatalf("error = %v, want ErrAuthorityRequired", err)
			}
			if got := len(storeA.List()); got != before {
				t.Fatalf("rejected promotion changed record count from %d to %d", before, got)
			}
		})
	}

	contractCases := map[string]StoreAuthority{
		"missing":      {},
		"cross_store":  authoritiesB.ContractGrant,
		"wrong_domain": authoritiesA.Promotion,
	}
	for name, authority := range contractCases {
		t.Run("contract_grant_"+name, func(t *testing.T) {
			before := len(storeA.List())
			if _, err := storeA.AppendContractAndGrantAuthorized(authority, malformed, malformed, malformed); !errors.Is(err, ErrAuthorityRequired) {
				t.Fatalf("error = %v, want ErrAuthorityRequired", err)
			}
			if got := len(storeA.List()); got != before {
				t.Fatalf("rejected contract/grant changed record count from %d to %d", before, got)
			}
		})
	}

	if _, err := storeA.AppendPromotionCommitAuthorized(authoritiesA.Promotion, malformed, malformed, malformed); errors.Is(err, ErrAuthorityRequired) || !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("matching promotion authority error = %v, want downstream ErrInvalidRecord", err)
	}
	if _, err := storeA.AppendContractAndGrantAuthorized(authoritiesA.ContractGrant, malformed, malformed, malformed); errors.Is(err, ErrAuthorityRequired) || !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("matching contract/grant authority error = %v, want downstream ErrInvalidRecord", err)
	}
}

func TestAuthorizedContractGrantPreservesSuccessIdempotencyAndReplay(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	approval := validApprovalDecision(t)
	encoded, err := journal.EncodeRecord(journal.RecordKindSharedRecord, approval)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(encoded); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j, err = journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, authorities, err := NewStoreWithAuthorities(j)
	if err != nil {
		t.Fatal(err)
	}
	specification := validSpecification(t)
	contract := validContract(t, "contract-authorized")
	grant := validGrant(t, "grant-authorized", "contract-authorized", "idem-authorized")
	first, err := store.AppendContractAndGrantAuthorized(authorities.ContractGrant, specification, contract, grant)
	if err != nil {
		t.Fatalf("authorized contract/grant commit: %v", err)
	}
	second, err := store.AppendContractAndGrantAuthorized(authorities.ContractGrant, specification, contract, grant)
	if err != nil {
		t.Fatalf("authorized idempotent replay: %v", err)
	}
	if second.Existing != true || second.Sequence != first.Sequence {
		t.Fatalf("authorized duplicate not idempotent: first=%+v second=%+v", first, second)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	replayedJournal, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer replayedJournal.Close()
	replayed, err := NewStore(replayedJournal)
	if err != nil {
		t.Fatalf("historical replay required live authority: %v", err)
	}
	if _, err := replayed.Get("contract-authorized"); err != nil {
		t.Fatalf("replayed contract missing: %v", err)
	}
	if _, err := replayed.Get("grant-authorized"); err != nil {
		t.Fatalf("replayed grant missing: %v", err)
	}
}

func newStoreWithAuthoritiesForTest(t *testing.T) (*Store, StoreAuthorities, *journal.BinaryJournal) {
	t.Helper()
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	store, authorities, err := NewStoreWithAuthorities(j)
	if err != nil {
		j.Close()
		t.Fatal(err)
	}
	return store, authorities, j
}
