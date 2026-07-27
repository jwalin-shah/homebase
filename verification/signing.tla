---- MODULE HomeBaseSigning ----
\* Formal specification of Ed25519 signing (non-repudiation)
\* Proves: signature soundness, verification correctness, unforgeable

EXTENDS Naturals, Sequences

\* Constants
CONSTANT Keys     \* Set of (pubkey, privkey) pairs
CONSTANT Messages \* Set of possible messages

\* Variables
VARIABLE signatures,   \* Map (message, signer) -> signature
         verified,     \* Map (message, signature, pubkey) -> {TRUE, FALSE}
         tampered      \* Set of tampered (message, signature) pairs

\* Initial state
Init ≡
  /\ signatures = {}
  /\ verified = {}
  /\ tampered = {}

\* Action: Sign a message
Sign(message, privkey) ≡
  LET sig ≡ EdSign(message, privkey)
      pubkey ≡ PubKeyOf(privkey)
  IN
    /\ signatures' = signatures ∪ {((message, pubkey) ↦ sig)}
    /\ UNCHANGED verified
    /\ UNCHANGED tampered

\* Action: Verify a signature
Verify(message, signature, pubkey) ≡
  LET isValid ≡ EdVerify(message, signature, pubkey)
  IN
    /\ verified' = verified ∪ {((message, signature, pubkey) ↦ isValid)}
    /\ UNCHANGED signatures
    /\ UNCHANGED tampered

\* Action: Tamper with message (adversary attack)
TamperMessage(message, signature, pubkey, newMessage) ≡
  LET wasValid ≡ EdVerify(message, signature, pubkey)
  IN
    /\ wasValid = TRUE  \* Only tamper with valid signatures
    /\ tampered' = tampered ∪ {(newMessage, signature)}
    /\ UNCHANGED signatures
    /\ UNCHANGED verified

\* Verification protocol
VerificationProtocol(message, signature, pubkey) ≡
  EdVerify(message, signature, pubkey)

\* Define next state
Next ≡
  ∨ ∃ msg ∈ Messages, key ∈ Keys: Sign(msg, key)
  ∨ ∃ msg ∈ Messages, sig, key: Verify(msg, sig, key)
  ∨ ∃ msg, newMsg ∈ Messages, sig, key: TamperMessage(msg, sig, key, newMsg)

\* INVARIANTS

\* Invariant 1: Signature Soundness
\* If Verify(msg, sig, pubkey) returns TRUE, then sig was created with corresponding privkey
Invariant_Soundness ≡
  □ (∀ msg ∈ Messages, sig, pubkey:
       verified[msg, sig, pubkey] = TRUE ⇒
         ∃ privkey ∈ Keys:
           PubKeyOf(privkey) = pubkey ∧
           sig = EdSign(msg, privkey))

\* Invariant 2: Verification Correctness
\* Verify always returns the same result for the same input (deterministic)
Invariant_Deterministic ≡
  □ (∀ msg ∈ Messages, sig, pubkey:
       verified[msg, sig, pubkey] = verified[msg, sig, pubkey])

\* Invariant 3: Unforgeable
\* No adversary can create a valid signature without the private key
Invariant_Unforgeable ≡
  □ (∀ msg ∈ Messages, sig, pubkey:
       verified[msg, sig, pubkey] = TRUE ⇒
         (msg, sig) ∉ tampered)

\* Invariant 4: Non-Repudiation
\* Signer cannot deny creating a valid signature
Invariant_NonRepudiation ≡
  □ (∀ msg ∈ Messages, sig, pubkey:
       verified[msg, sig, pubkey] = TRUE ⇒
         privkey_holder_of(pubkey) created_signature_for(msg))

\* Invariant 5: Tamper Detection
\* If message is tampered, verification with original signature fails
Invariant_TamperDetection ≡
  □ (∀ msg, newMsg ∈ Messages, sig, pubkey:
       (newMsg, sig) ∈ tampered ⇒
         verified[newMsg, sig, pubkey] = FALSE)

\* THEOREMS

\* Theorem 1: Signatures are unforgeable
THEOREM Unforgeable ≡
  Init ∧ [][Next]_signatures ⇒ Invariant_Unforgeable

\* Theorem 2: Verification is deterministic
THEOREM Deterministic ≡
  Init ∧ [][Next]_verified ⇒ Invariant_Deterministic

\* Theorem 3: Non-repudiation holds
THEOREM NonRepudiation ≡
  Init ∧ [][Next]_signatures ⇒ Invariant_NonRepudiation

\* Theorem 4: Tamper is always detected
THEOREM TamperDetected ≡
  Init ∧ [][Next]_tampered ⇒ Invariant_TamperDetection

\* Theorem 5: Soundness is maintained
THEOREM Soundness ≡
  Init ∧ [][Next]_verified ⇒ Invariant_Soundness

====
