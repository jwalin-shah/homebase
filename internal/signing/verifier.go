package signing

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Verifier handles Ed25519 signature verification
type Verifier struct {
	publicKey ed25519.PublicKey
}

// NewVerifier creates a new verifier with the given public key
func NewVerifier(publicKey ed25519.PublicKey) *Verifier {
	return &Verifier{
		publicKey: publicKey,
	}
}

// Verify checks if a signature is valid for the given data
func (v *Verifier) Verify(data []byte, signatureHex string) error {
	if v.publicKey == nil {
		return fmt.Errorf("public key not initialized")
	}

	// Decode signature from hex
	signature, err := hex.DecodeString(signatureHex)
	if err != nil {
		return fmt.Errorf("failed to decode signature: %w", err)
	}

	// Verify signature
	if !ed25519.Verify(v.publicKey, data, signature) {
		return fmt.Errorf("signature verification failed (data may have been tampered)")
	}

	return nil
}

// VerifyJSON verifies a signature for a JSON object
func (v *Verifier) VerifyJSON(obj interface{}, signatureHex string) error {
	// Marshal to JSON (must match original marshal)
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal to JSON: %w", err)
	}

	return v.Verify(jsonBytes, signatureHex)
}

// VerifyDecision verifies a signature for a decision object
func (v *Verifier) VerifyDecision(decision interface{}, signatureHex string) error {
	return v.VerifyJSON(decision, signatureHex)
}
