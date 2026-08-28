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
		return verifyProductionBridgeReceiptAttestation(receipt, authority.PublicKey, authority.KeyID)
	}
	return fmt.Errorf("%w: verifier key %q is not enrolled", ErrAuthorityRequired, receipt.Attestation.KeyID)
}

// VerifyBridgeReceiptWithPolicy verifies both enrolled verifier identity and
// its time-scoped authority at the current admission boundary.
func VerifyBridgeReceiptWithPolicy(raw []byte, policy *VerifierPolicy, admissionTime time.Time) error {
	return policy.verify(raw, admissionTime)
}

// AppendBridgeVerificationSubmission intentionally has no live authority
// input. It remains fail-closed so a direct Store caller cannot bypass verifier
// enrollment. Historical replay uses replayVerificationCommit instead.
func (s *Store) AppendBridgeVerificationSubmission(raw []byte) (VerificationCommitResult, error) {
	return VerificationCommitResult{}, fmt.Errorf("%w: enrolled verifier policy", ErrAuthorityRequired)
}

// AppendBridgeVerificationSubmissionWithPolicy is the live Store admission
// seam. Policy verification happens before the underlying implementation can
// inspect duplicate state, references, or mutate the journal.
func (s *Store) AppendBridgeVerificationSubmissionWithPolicy(raw []byte, policy *VerifierPolicy) (VerificationCommitResult, error) {
	if err := policy.verify(raw, s.now().UTC()); err != nil {
		return VerificationCommitResult{}, err
	}
	return s.appendBridgeVerificationSubmission(raw)
}
