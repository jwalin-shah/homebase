package records

import (
	"fmt"
	"homebase/internal/journal"
	"time"
)

type storeAuthorityDomain uint8

const (
	promotionAuthorityDomain storeAuthorityDomain = iota + 1
	contractGrantAuthorityDomain
)

// StoreAuthority is an opaque, Store-bound capability for one specialized
// authoritative write domain. Its fields are deliberately unexported so a
// caller cannot construct or retarget a valid capability from identity/key
// metadata alone.
type StoreAuthority struct {
	store  *Store
	domain storeAuthorityDomain
}

// StoreAuthorities is returned only alongside privileged Store construction.
// Composition roots should inject each capability only into the subsystem
// that already authenticated that authority domain.
type StoreAuthorities struct {
	Promotion     StoreAuthority
	ContractGrant StoreAuthority
}

// NewStoreWithAuthorities constructs a Store and its non-forgeable in-process
// capabilities. Historical replay happens before capabilities are returned and
// does not require live authority.
func NewStoreWithAuthorities(j *journal.BinaryJournal) (*Store, StoreAuthorities, error) {
	store, err := NewStore(j)
	if err != nil {
		return nil, StoreAuthorities{}, err
	}
	return store, authoritiesFor(store), nil
}

// NewStoreWithClockAndAuthorities is the deterministic test form of
// NewStoreWithAuthorities.
func NewStoreWithClockAndAuthorities(j *journal.BinaryJournal, clock func() time.Time) (*Store, StoreAuthorities, error) {
	store, err := NewStoreWithClock(j, clock)
	if err != nil {
		return nil, StoreAuthorities{}, err
	}
	return store, authoritiesFor(store), nil
}

func authoritiesFor(store *Store) StoreAuthorities {
	return StoreAuthorities{
		Promotion:     StoreAuthority{store: store, domain: promotionAuthorityDomain},
		ContractGrant: StoreAuthority{store: store, domain: contractGrantAuthorityDomain},
	}
}

func (s *Store) requireStoreAuthority(authority StoreAuthority, domain storeAuthorityDomain) error {
	if authority.store != s || authority.domain != domain {
		return fmt.Errorf("%w: specialized Store authority", ErrAuthorityRequired)
	}
	return nil
}

// AppendPromotionCommitAuthorized is the capability-gated promotion commit
// boundary. Authority is checked before any payload parsing or journal
// mutation; the underlying implementation is intentionally package-private.
func (s *Store) AppendPromotionCommitAuthorized(authority StoreAuthority, decisionRaw, evidenceRaw, receiptRaw []byte) (PromotionCommitResult, error) {
	if err := s.requireStoreAuthority(authority, promotionAuthorityDomain); err != nil {
		return PromotionCommitResult{}, err
	}
	return s.appendPromotionCommit(decisionRaw, evidenceRaw, receiptRaw)
}

// AppendContractAndGrantAuthorized is the capability-gated Contract/Grant
// commit boundary. Authority is checked before any payload parsing or journal
// mutation; the underlying implementation is intentionally package-private.
func (s *Store) AppendContractAndGrantAuthorized(authority StoreAuthority, specificationRaw, contractRaw, grantRaw []byte) (ContractGrantCommitResult, error) {
	if err := s.requireStoreAuthority(authority, contractGrantAuthorityDomain); err != nil {
		return ContractGrantCommitResult{}, err
	}
	return s.appendContractAndGrant(specificationRaw, contractRaw, grantRaw)
}
