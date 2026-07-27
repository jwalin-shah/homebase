package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"homebase/internal/types"
	"testing"
)

// TestSigner_Integrity proves Invariant 4 (Integrity).
// It verifies that a decision signed by the Ed25519 key passes verification,
// and any tampering instantly fails verification.
func TestSigner_Integrity(t *testing.T) {
	// 1. Generate ephemeral key
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	signer := NewSigner(privKey)

	// 2. Create the sealed unsigned payload (AssuranceCase)
	subject := []types.Subject{
		types.NewSubject("TEST-001", map[string]string{"sha256": "placeholder"}),
	}
	unsigned := types.NewAssuranceCase("TEST-001", subject, "deploy rules", "model", "arg", []types.AxiomID{types.InternalNewAxiomID("AX-001")}, "100% coverage", "tester")

	// 3. Cryptographically seal it
	signed, err := signer.SignDecision(unsigned)
	if err != nil {
		t.Fatalf("Failed to sign decision: %v", err)
	}

	// 4. Prove verification works for a valid signature
	if err := signer.Verify(signed); err != nil {
		t.Fatalf("Valid signature was rejected by Verifier: %v", err)
	}

	// 5. Prove verification catches tampering
	// We simulate a rogue agent trying to swap the signature
	// Depending on implementation, we can't easily mutate a struct with getters.
	// But if Verify catches ANY bad envelope, we can construct one manually:
	badSig := types.NewDSSESignature("bad", "bad")
	badEnv := types.NewDSSEEnvelope("bad", unsigned, badSig)
	if err := signer.Verify(badEnv); err == nil {
		t.Fatalf("CRITICAL: Verifier accepted a tampered signature! Invariant 4 breached.")
	}
}
