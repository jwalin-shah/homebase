package signing

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// KeyPair holds Ed25519 public and private keys
type KeyPair struct {
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}

// GenerateKeyPair generates a new Ed25519 key pair
func GenerateKeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// PublicKeyHex returns the public key as hex string
func (k *KeyPair) PublicKeyHex() string {
	return hex.EncodeToString(k.PublicKey)
}

// PrivateKeyHex returns the private key as hex string
func (k *KeyPair) PrivateKeyHex() string {
	return hex.EncodeToString(k.PrivateKey)
}

// LoadPublicKeyFromHex loads a public key from hex string
func LoadPublicKeyFromHex(hexStr string) (ed25519.PublicKey, error) {
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size: %d", len(keyBytes))
	}

	return ed25519.PublicKey(keyBytes), nil
}

// LoadPrivateKeyFromHex loads a private key from hex string
func LoadPrivateKeyFromHex(hexStr string) (ed25519.PrivateKey, error) {
	keyBytes, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex: %w", err)
	}

	if len(keyBytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size: %d", len(keyBytes))
	}

	return ed25519.PrivateKey(keyBytes), nil
}
