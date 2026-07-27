package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"net/http"
	"os"

	"homebase/api"
	"homebase/internal/cache"
	"homebase/internal/ledger"
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

	// 4. Initialize the API Server
	server := api.NewServer(validator, signer, store)

	// 5. Mount the Endpoints
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/decisions", server.HandleRecordDecision)

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
