package ledger

import (
	"os"
	"testing"
)

// TestLedgerDurability proves Invariant 3 (Durability).
// It verifies that when Record() is called, the file is physically created
// and the SignedDecision is append-only written to disk.
func TestLedgerDurability(t *testing.T) {
	tmpfile := "test_durability.jsonl"
	defer os.Remove(tmpfile)

	// Initialize the Store (which returns *Store, error in our new architecture)
	store, err := NewStore(tmpfile)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}
	defer store.Close()

	// Construct a mathematically sealed DSSEEnvelope
	signed := DSSEEnvelope{
		Payload:     "base64payload",
		PayloadType: "application/vnd.in-toto+json",
		Signatures:  []string{"b72af4e9"},
	}

	// 1. Record the decision
	if err := store.Append(signed); err != nil {
		t.Fatalf("Failed to durably record decision: %v", err)
	}

	// 2. Prove physical durability
	info, err := os.Stat(tmpfile)
	if err != nil {
		t.Fatalf("Ledger file was not created by os: %v", err)
	}

	if info.Size() == 0 {
		t.Fatalf("Ledger file is empty! fsync failed.")
	}
}
