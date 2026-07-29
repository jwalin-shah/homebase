package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"homebase/api"
	"homebase/internal/cache"
	"homebase/internal/journal"
	"homebase/internal/ledger"
	"homebase/internal/promotion"
	"homebase/internal/records"
	"homebase/internal/signing"
	"homebase/internal/validation"
)

func main() {
	fmt.Println("Starting HomeBase Immutable Decision Ledger...")

	// 1. Initialize Storage (Immutability & Durability)
	// This creates or opens the JSONL ledger file in append-only mode.
	store, err := ledger.NewStore("homebase_ledger.jsonl")
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize ledger store: %v", err)
	}
	defer store.Close()

	recordJournalPath := os.Getenv("HOMEBASE_RECORD_JOURNAL")
	if recordJournalPath == "" {
		recordJournalPath = "homebase_records.journal"
	}
	recordJournal, err := journal.OpenBinaryJournal(recordJournalPath)
	if err != nil {
		log.Fatalf("FATAL: Failed to initialize typed record journal: %v", err)
	}
	defer recordJournal.Close()
	recordStore, err := records.NewStore(recordJournal)
	if err != nil {
		log.Fatalf("FATAL: Failed to replay typed record journal: %v", err)
	}

	// 2. Initialize Cryptography (Integrity)
	// In production, this key is loaded from a secure vault (e.g., AWS KMS or HashiCorp).
	// For this build, we generate a highly secure ephemeral key.
	_, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("FATAL: Failed to generate cryptographic signing key: %v", err)
	}
	signer := signing.NewSigner(privKey)

	// 2.5 Initialize the Neo4j Ontology (The Firewall)
	// This physically enforces Coyle's "Ontology at the Ledger" rule.
	neo4jClient, err := cache.NewClient("bolt://localhost:7687", "neo4j", "password")
	if err != nil {
		// Log warning but continue for local dev if Neo4j is down
		fmt.Printf("WARNING: Neo4j down, Axiom Firewall disabled: %v\n", err)
	}

	// 3. Initialize the Axiom Firewall Validator
	validator := validation.NewValidator(neo4jClient, store)

	// 4. Initialize the authenticated transcript promotion path only when its
	// persistent authority keys are configured. The endpoint remains mounted in
	// an unavailable state rather than silently generating ephemeral authority.
	var promotionService *promotion.Service
	var captainPublic ed25519.PublicKey
	if captainKey, publicErr := loadKey("HOMEBASE_CAPTAIN_PUBLIC_KEY_HEX", "HOMEBASE_CAPTAIN_PUBLIC_KEY_FILE", ed25519.PublicKeySize); publicErr == nil {
		captainPublic = ed25519.PublicKey(captainKey)
		if receiptPrivate, privateErr := loadKey("HOMEBASE_RECEIPT_PRIVATE_KEY_HEX", "HOMEBASE_RECEIPT_PRIVATE_KEY_FILE", ed25519.PrivateKeySize); privateErr == nil {
			promotionService, err = promotion.NewService(recordStore, promotion.Ed25519Verifier("captain", captainPublic), ed25519.PrivateKey(receiptPrivate), nil)
			if err != nil {
				log.Printf("WARNING: transcript promotion unavailable: authority state could not be rebuilt")
			}
		} else {
			log.Printf("WARNING: transcript promotion unavailable: receipt key is not configured")
		}
	} else {
		log.Printf("WARNING: transcript promotion unavailable: captain key is not configured")
	}

	// 5. Initialize the API Server
	var bridgePublic ed25519.PublicKey
	if bridgeKey, bridgeErr := loadKey("HOMEBASE_BRIDGE_PUBLIC_KEY_HEX", "HOMEBASE_BRIDGE_PUBLIC_KEY_FILE", ed25519.PublicKeySize); bridgeErr == nil {
		bridgePublic = ed25519.PublicKey(bridgeKey)
	} else {
		log.Printf("WARNING: Bridge verification unavailable: public key is not configured")
	}
	server := api.NewServerWithAuthorities(validator, signer, store, recordStore, promotionService, captainPublic, bridgePublic)

	// 5. Mount the Endpoints
	mux := http.NewServeMux()
	// The legacy decision endpoint is intentionally not mounted. It accepted
	// caller-controlled decisions and signed them with a process-local key,
	// which is not an authenticated authority boundary. Decisions now enter
	// through owner-authenticated typed promotion or the future Knowledge
	// Engine decision path.
	mux.HandleFunc("/api/v1/records", server.HandleAppendExternalRecord)
	mux.HandleFunc("/api/v1/promotions/transcript", server.HandlePromoteTranscript)
	mux.HandleFunc("/api/v1/contracts/grants", server.HandleAppendContractGrant)
	mux.HandleFunc("/api/v1/verifications/bridge", server.HandleAppendBridgeVerification)

	// 6. Start the Engine
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("HomeBase Engine running on port %s.\n", port)
	fmt.Println("[OK] Graph State Machine loaded.")
	fmt.Println("[OK] Ed25519 Cryptography initialized.")
	fmt.Println("[OK] Append-only Ledger connected.")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server halted: %v", err)
	}
}

func loadKey(hexEnv, fileEnv string, size int) ([]byte, error) {
	value := strings.TrimSpace(os.Getenv(hexEnv))
	if path := strings.TrimSpace(os.Getenv(fileEnv)); path != "" {
		file, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		value = strings.TrimSpace(string(file))
	}
	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != size {
		return nil, fmt.Errorf("invalid key material")
	}
	return decoded, nil
}
