# Cedar Formal Proofs Integration for HomeBase Authorization

## 1. Overview: AWS Cedar Verification
AWS Cedar is an authorization policy language built with high-assurance mathematical proofs. The engine's core was initially modeled in **Dafny** and mathematically proven using the Z3 solver. Its open-source specification recently transitioned to **Lean 4** to handle complex meta-theoretic properties robustly. This aligns precisely with HomeBase's architectural commitment: using Lean 4 for global architectural bounds and Dafny for execution core logic.

## 2. Coverage Matrix Integration
To adopt Cedar’s formal model into HomeBase's graph-structured ledger, we will augment our Coverage Matrix with:
*   **Core Logic Proofs (Dafny):** Integrating Dafny state machine constraints for the authorization pathways.
*   **Global Authorization Bounds (Lean 4):** Importing Cedar's Lean 4 specifications as the source of truth for permission boundaries.
*   **Differential Random Testing (DRT):** Generating millions of random requests to compare the extracted Go implementation against the Lean model, ensuring execution fidelity.

## 3. Cryptographic Dependency Importing
Cedar's Dafny and Lean proofs will be imported as cryptographic dependencies:
*   The source proofs (Dafny/Lean) will be treated as immutable materials, hashed, and signed (Ed25519).
*   During HomeBase’s `EXECUTE` state (enforcing Invariant I4: Integrity), the `verify()` handler will validate the signatures of the policy proofs before evaluating any access decisions.
*   This prevents tampered or unverified authorization models from influencing the decision ledger.

## 4. Mapping to in-toto Assurance Case Schema
We will secure the formal proofs using the in-toto supply chain framework:
*   **Materials:** Cedar's Lean 4 and Dafny proof files (pinned by strict git commit SHAs).
*   **Steps:** 
    1. `proof_verification`: ITP compilers (Lean/Dafny) run to mathematically verify the models.
    2. `code_extraction`: Verified logic is formally extracted into Go.
    3. `differential_testing`: DRT is executed between the Go runtime and the Lean model.
*   **Products:** The extracted and signed Go authorization modules.
*   **Attestations:** Cryptographic signatures produced during proof extraction, securely recorded in the HomeBase append-only JSONL ledger.
