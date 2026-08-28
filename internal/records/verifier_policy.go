package records

import (
	"crypto/ed25519"
	"fmt"
	"strings"
	"time"
)

// VerifierAuthority is one enrolled production verifier key and its admission
// window. Public keys are copied into VerifierPolicy at construction time.
type VerifierAuthority struct {
	VerifierID    string
	KeyID         string
	PublicKey     ed25519.PublicKey
	ValidFrom     time.Time
	ValidUntil    time.Time
	CompromisedAt *time.Time
}

// VerifierPolicy is an immutable trust set for new Bridge verification
// submissions. Historical journal replay never consults this policy.
type VerifierPolicy struct {
	authorities []VerifierAuthority
}

// StoreVerifierAuthority is an opaque, Store-bound verifier capability. The
// policy is bound only by a privileged StoreAuthority minted with the Store;
// mutation callers cannot supply or replace verifier trust per write.
type StoreVerifierAuthority struct {
	store  *Store
	policy *VerifierPolicy
}

func NewVerifierPolicy(authorities []VerifierAuthority) (*VerifierPolicy, error) {
	if len(authorities) == 0 {
		return nil, fmt.Errorf("%w: verifier policy requires at least one authority", ErrAuthorityRequired)
	}
	copied := make([]VerifierAuthority, 0, len(authorities))
	seen := make(map[string]struct{}, len(authorities))
	for _, authority := range authorities {
		authority.VerifierID = strings.TrimSpace(authority.VerifierID)
		authority.KeyID = strings.TrimSpace(authority.KeyID)
		if authority.VerifierID != productionVerifierID {
			return nil, fmt.Errorf("%w: verifier identity %q is not production authority", ErrAuthorityRequired, authority.VerifierID)
		}
		if authority.KeyID == "" || len(authority.PublicKey) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("%w: verifier authority key is incomplete", ErrAuthorityRequired)
		}
		if authority.ValidFrom.IsZero() || authority.ValidUntil.IsZero() || !authority.ValidFrom.Before(authority.ValidUntil) {
			return nil, fmt.Errorf("%w: verifier authority validity window is invalid", ErrAuthorityRequired)
		}
		key := authority.VerifierID + "\x00" + authority.KeyID
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("%w: duplicate verifier authority %s/%s", ErrAuthorityRequired, authority.VerifierID, authority.KeyID)
		}
		seen[key] = struct{}{}
		authority.PublicKey = append(ed25519.PublicKey(nil), authority.PublicKey...)
		if authority.CompromisedAt != nil {
			when := authority.CompromisedAt.UTC()
			authority.CompromisedAt = &when
		}
		authority.ValidFrom = authority.ValidFrom.UTC()
		authority.ValidUntil = authority.ValidUntil.UTC()
		copied = append(copied, authority)
	}
	return &VerifierPolicy{authorities: copied}, nil
}

// BindStoreVerifierPolicy converts the Store's privileged verifier-policy
// authority into an opaque capability containing one immutable policy. Only a
// capability minted with the target Store can bind that Store's live verifier
// trust; a caller holding only *Store cannot self-enroll a key.
func BindStoreVerifierPolicy(authority StoreAuthority, policy *VerifierPolicy) (StoreVerifierAuthority, error) {
	if authority.store == nil || authority.domain != verifierPolicyAuthorityDomain {
		return StoreVerifierAuthority{}, fmt.Errorf("%w: verifier policy Store authority", ErrAuthorityRequired)
	}
	if policy == nil {
		return StoreVerifierAuthority{}, fmt.Errorf("%w: enrolled verifier policy", ErrAuthorityRequired)
	}
	return StoreVerifierAuthority{store: authority.store, policy: policy}, nil
}

func (p *VerifierPolicy) verify(raw []byte, admissionTime time.Time) error {
	if p == nil {
		return fmt.Errorf("%w: enrolled verifier policy", ErrAuthorityRequired)
	}
	receipt, err := decodeBridgeReceipt(raw)
	if err != nil {
		return err
	}
	if receipt.VerifierID != productionVerifierID {
		return fmt.Errorf("%w: verifier identity %q is not admitted for new submissions", ErrAuthorityRequired, receipt.VerifierID)
	}
	if receipt.Attestation == nil {
		return invalid("production verification receipt is missing verifier attestation")
	}
	verifiedAt, err := time.Parse("2006-01-02T15:04:05Z", receipt.VerifiedAt)
	if err != nil {
		return invalid("VerificationReceipt verified_at: %v", err)
	}
	admissionTime = admissionTime.UTC()
	for _, authority := range p.authorities {
		if authority.VerifierID != receipt.VerifierID || authority.KeyID != receipt.Attestation.KeyID {
			continue
		}
		// Once a key is marked compromised, possession of it cannot prove when a
		// signature was actually created. Reject every new admission, including
		// receipts whose verified_at was backdated before compromise.
		if authority.CompromisedAt != nil {
			return fmt.Errorf("%w: verifier authority %s is compromised", ErrAuthorityRequired, authority.KeyID)
		}
		if verifiedAt.Before(authority.ValidFrom) || !verifiedAt.Before(authority.ValidUntil) {
			return fmt.Errorf("%w: receipt verified_at is outside verifier authority window", ErrAuthorityRequired)
		}
		if admissionTime.Before(authority.ValidFrom) || !admissionTime.Before(authority.ValidUntil) {
			return fmt.Errorf("%w: verifier authority is not active at admission", ErrAuthorityRequired)
		}
		return VerifyBridgeReceiptAttestation(raw, authority.PublicKey, authority.KeyID)
	}
	return fmt.Errorf("%w: verifier key %q is not enrolled", ErrAuthorityRequired, receipt.Attestation.KeyID)
}

// VerifyBridgeReceiptWithPolicy verifies both enrolled verifier identity and
// its time-scoped authority at the current admission boundary.
func VerifyBridgeReceiptWithPolicy(raw []byte, policy *VerifierPolicy, admissionTime time.Time) error {
	return policy.verify(raw, admissionTime)
}

// Verify checks a receipt against the immutable policy bound to this opaque
// Store verifier capability. It is useful for an API preflight that must use
// exactly the same trust policy as the durable Store admission boundary.
func (authority StoreVerifierAuthority) Verify(raw []byte, admissionTime time.Time) error {
	if authority.store == nil || authority.policy == nil {
		return fmt.Errorf("%w: Store-bound verifier policy", ErrAuthorityRequired)
	}
	return authority.policy.verify(raw, admissionTime)
}

// AppendBridgeVerificationSubmission intentionally has no live authority
// input. It remains fail-closed so a direct Store caller cannot bypass verifier
// enrollment. Historical replay uses replayVerificationCommit instead.
func (s *Store) AppendBridgeVerificationSubmission(raw []byte) (VerificationCommitResult, error) {
	return VerificationCommitResult{}, fmt.Errorf("%w: Store-bound verifier authority", ErrAuthorityRequired)
}

// AppendBridgeVerificationSubmissionAuthorized is the live Store admission
// seam. The opaque authority is Store-bound and carries a policy that could
// only be bound using the verifier-policy capability minted with this Store.
// Verification happens before duplicate/conflict/reference/journal work.
func (s *Store) AppendBridgeVerificationSubmissionAuthorized(raw []byte, authority StoreVerifierAuthority) (VerificationCommitResult, error) {
	if authority.store != s || authority.policy == nil {
		return VerificationCommitResult{}, fmt.Errorf("%w: Store-bound verifier authority", ErrAuthorityRequired)
	}
	if err := authority.policy.verify(raw, s.now().UTC()); err != nil {
		return VerificationCommitResult{}, err
	}
	return s.appendBridgeVerificationSubmission(raw)
}
