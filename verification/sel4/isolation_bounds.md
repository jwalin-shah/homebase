# seL4 Mathematical Isolation Bounds for HomeBase EXECUTE State

## 1. Overview
To ensure AI agents cannot bypass the global bounds of the HomeBase immutable decision ledger, we integrate **seL4 microkernel capabilities** backed by **Isabelle/HOL proofs**. This creates a mathematical sandbox for the agent during the `EXECUTE` (S1) state, acting as a cryptographic dependency that guarantees isolation and bounded authority.

## 2. seL4 Capability-Based Security
seL4 enforces strict isolation using capability-based access control. Every resource access (memory, I/O, IPC) requires a verified capability token.
* **Isabelle/HOL Proofs**: seL4 provides machine-checked proofs that its C implementation correctly enforces high-level security properties (functional correctness, integrity, and confidentiality).
* **Authority Confinement**: The mathematical proofs guarantee that authority cannot be arbitrarily propagated. An isolated component cannot gain privileges beyond its initial configuration.
* **capDL (Capability Distribution Language)**: The static protection state of the system is modeled in capDL.

## 3. Mathematical Sandboxing in EXECUTE (State S1)
The `EXECUTE` state records the decision, computes signatures, and interacts with Neo4j. AI reasoning inside this state must be aggressively bounded:
* **Plan as capDL**: During the `PLAN` state (S0), the Locked Plan compiles down to a deterministic capDL specification. 
* **Execution Confinement**: When transitioning to `EXECUTE`, the AI agent operates strictly within the capability bounds defined by that capDL spec. It receives one-way IPC endpoints to the cryptographic ledger and signing nodes.
* **No Dynamic Retries**: Because the agent lacks capabilities to request new network sockets, spawn unlogged processes, or manipulate the state machine, "hidden retries" become mathematically impossible. It can only emit a success/failure signal back to the graph.

## 4. Mapping to Coverage Matrix & Invariants
By anchoring our `EXECUTE` state sandboxing in seL4 proofs, we map directly to HomeBase invariants:
* **I1 (Immutability)**: The AI agent's capability to the ledger is restricted to `Append` via an IPC channel. The Isabelle/HOL proof guarantees the agent cannot escalate this to a `Modify` or `Delete` capability.
* **I3 (Durability) & I4 (Integrity)**: The signer and ledger storage run in isolated seL4 partitions. A compromised or hallucinating AI agent in `EXECUTE` cannot read the Ed25519 signing keys or corrupt the fsync process.
* **Bounded Recovery (Strict Escalation)**: Without the capability to alter the capDL configuration, the AI agent is forced to yield control back to the `RECOVER` (S2) or `REPEAT` (S4) state upon failure. It cannot self-grant additional attempts.
