package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
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

	// 2.5 Initialize the Neo4j Ontology (The Firewall), OPTIONALLY.
	// Local-only builds may run without Neo4j: the validator degrades gracefully
	// when its cache client is nil, so the Axiom Firewall is simply skipped.
	// Set NEO4J_URI to enable the firewall; otherwise it is disabled with a
	// WARNING. This keeps local-only operation free of a mandatory Neo4j
	// password while preserving the firewall for deployments that do set it.
	neo4jURI := strings.TrimSpace(os.Getenv("NEO4J_URI"))
	var neo4jClient *cache.Client
	if neo4jURI != "" {
		neo4jPassword := strings.TrimSpace(os.Getenv("NEO4J_PASSWORD"))
		if neo4jPassword == "" {
			log.Fatal("FATAL: NEO4J_URI is set but NEO4J_PASSWORD is required; refusing to disable the Axiom Firewall")
		}
		var err error
		neo4jClient, err = cache.NewClient(neo4jURI, "neo4j", neo4jPassword)
		if err != nil {
			log.Fatalf("FATAL: Neo4j/Axiom Firewall unavailable: %v", err)
		}
	} else {
		fmt.Println("WARNING: NEO4J_URI not set; running WITHOUT the Axiom Firewall (local-only mode)")
	}

	// 3. Initialize the Axiom Firewall Validator (nil cache = firewall disabled)
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
	var admissionPrivate ed25519.PrivateKey
	if admissionKey, admissionErr := loadKey("HOMEBASE_ADMISSION_PRIVATE_KEY_HEX", "HOMEBASE_ADMISSION_PRIVATE_KEY_FILE", ed25519.PrivateKeySize); admissionErr == nil {
		admissionPrivate = ed25519.PrivateKey(admissionKey)
	} else {
		log.Printf("WARNING: Bridge admission responses unavailable: HomeBase response signing key is not configured")
	}
	var verifierPublic ed25519.PublicKey
	verifierKeyID := strings.TrimSpace(os.Getenv("HOMEBASE_VERIFIER_KEY_ID"))
	if verifierKeyID == "" {
		log.Printf("WARNING: production verifier receipts unavailable: HOMEBASE_VERIFIER_KEY_ID is not configured")
	} else if verifierKey, verifierErr := loadKey("HOMEBASE_VERIFIER_PUBLIC_KEY_HEX", "HOMEBASE_VERIFIER_PUBLIC_KEY_FILE", ed25519.PublicKeySize); verifierErr == nil {
		verifierPublic = ed25519.PublicKey(verifierKey)
	} else {
		log.Printf("WARNING: production verifier receipts unavailable: %v", verifierErr)
	}
	server := api.NewServerWithAuthoritiesAndAdmissionResponseAndVerifier(validator, signer, store, recordStore, promotionService, captainPublic, bridgePublic, admissionPrivate, verifierPublic, verifierKeyID)

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
	mux.HandleFunc("/api/v1/specifications/decisions", server.HandleAppendSpecificationDecision)
	mux.HandleFunc("/api/v1/contracts/grants/check", server.HandleCheckContractGrant)
	mux.HandleFunc("/api/v1/verifications/bridge", server.HandleAppendBridgeVerification)
	mux.HandleFunc("/api/v1/verifications/receipts/read", server.HandleReadVerificationReceipt)
	// LaunchAgent health contract (DAEMON_HEALTH_URL): unauthenticated,
	// non-secret readiness only. See api.Server.HandleStatus.
	mux.HandleFunc("/v1/status", server.HandleStatus)

	// 6. Start the Engine
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("HomeBase Engine running on port %s.\n", port)
	fmt.Println("[OK] Graph State Machine loaded.")
	fmt.Println("[OK] Ed25519 Cryptography initialized.")
	fmt.Println("[OK] Append-only Ledger connected.")

	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		log.Fatalf("Server failed to bind: %v", err)
	}
	defer listener.Close()
	fmt.Printf("HomeBase listening on %s\n", listener.Addr().String())
	if err := http.Serve(listener, mux); err != nil {
		log.Fatalf("Server halted: %v", err)
	}
}

func loadKey(hexEnv, fileEnv string, size int) ([]byte, error) {
	value := strings.TrimSpace(os.Getenv(hexEnv))
	if path := strings.TrimSpace(os.Getenv(fileEnv)); path != "" {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("key file must not be group/world-readable")
		}
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
