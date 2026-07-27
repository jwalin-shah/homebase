# FoundationDB & Trillian: Consensus Epistemology Mapping for HomeBase

**Date:** 2026-07-27  
**Status:** Consensus Epistemology & Formal Mapping

## 1. Foundational Distributed State Models

### FoundationDB: Strict Serializability via Simulation Testing
FoundationDB achieves strict serializability (linearizability + serializability) by leveraging **Deterministic Simulation Testing**.
- **The Simulator Boundary:** Replaces non-determinism (network, clocks, threads) with deterministic shims. It injects chaos (partitions, disk failures, node crashes) into the system in a single-threaded loop, ensuring invariants hold across billions of simulated states.
- **Concurrency Control:** Utilizes Optimistic Concurrency Control (OCC) and Multi-Version Concurrency Control (MVCC), processing transactions based on strict monotonically increasing Log Sequence Numbers (commit versions).
- **HomeBase Relevance:** This forms the epistemological basis for HomeBase's **Crash Recovery and Consistency Model** tests (`P1T11`, `P1T12`).

### Trillian: Verifiable Append-Only Logs via TLA+ & Merkle Trees
Trillian implements a verifiable, append-only ledger using **Merkle Trees**.
- **Cryptographic Proofs:** Generates Inclusion Proofs (a record exists) and Consistency Proofs (Tree N+1 is an append-only extension of Tree N).
- **TLA+ Formalization:** TLA+ is used to formally model state transitions of the Merkle Tree updates. It proves path-dependence correctness—ensuring no historical records can ever be altered without changing the Signed Tree Head (STH).
- **HomeBase Relevance:** Trillian's TLA+ bounds correspond directly to HomeBase's **Ledger Append-Only** (`P1T1`) and **Signature Verification/Non-Repudiation** tests (`P1T8`, `P1T9`).

## 2. Horizontal Scaling & Crash-Safe Guarantees

HomeBase's crash-safe ledger scales horizontally without data loss by mapping to these models:
- **Append-Only Immutability (Trillian):** By mathematically proving immutability (as modeled in Trillian's TLA+ proofs), HomeBase can distribute the ledger across nodes. Nodes simply share the Tree Head Hash. Consistency proofs ensure that horizontal replication is just a sequence of append operations, eliminating race conditions during scale-out.
- **Deterministic Crash Recovery (FoundationDB):** FoundationDB's strict serializability under simulation proves that even if nodes fail mid-write, the system recovers deterministically. HomeBase maps this to our `P1T11` (Ledger crash survival), guaranteeing that an `fsync`'d JSONL entry either wholly exists in the hash chain or doesn't, preventing split-brain data loss across horizontally scaled instances.

## 3. Mapping to the HomeBase Coverage Matrix

The formal proofs and simulation boundaries map directly to our **HB-001 Section 5 Coverage Matrix (Defense 16)** in `docs/PROOF_PLAN.md`:

| Formal Bound / Proof Model | HomeBase Coverage Matrix Test | Defense Category |
|----------------------------|-------------------------------|------------------|
| **Trillian TLA+ Append-Only** | **P1T1:** Ledger append-only | Defense 1, 5 (Immutability, Consistency) |
| **Trillian Merkle Consistency** | **P1T9:** Ledger durability & sync | Defense 9 (Offline/Sync) |
| **Trillian STH / Signature** | **P1T8:** Signature Verification | Defense 19, 22 (Non-Repudiation) |
| **FDB Simulation: Node Crash** | **P1T11:** Ledger crash survival | Defense 5, 21 (Consistency, Crash) |
| **FDB Simulation: Partitions** | **P1T12:** Neo4j divergence/recovery | Defense 17 (Graceful Degradation) |
| **FDB OCC & Linearizability** | **P1T4:** Duplicate detection | Defense 3 (Integrity) |

### TLA+ / Simulation Boundary Constraints
The TLA+ specifications in `verification/` currently face syntactical and logical boundaries (as noted in `VERIFICATION_REPORT_FINAL.md`). By anchoring our epistemological requirements on FoundationDB's simulation and Trillian's Merkle tree models, we establish that **Coverage Gate 16** requires functional integration and chaos tests (`P1T11`, `P1T12`) to act as the empirical safety nets where formal TLA+ bounds currently fall short.
