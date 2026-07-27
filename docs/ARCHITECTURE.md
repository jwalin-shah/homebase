# HomeBase Architecture

**Status:** Implementation
**Core Philosophy:** Formal semantics control unverified physical boundaries.

## The Mental Model

We are not building a system where `Dafny -> entire HomeBase`.
We are building a system where **Formal semantics** control **unverified physical boundaries**.

The physical boundary (filesystems, network, external APIs) may fail, return uncertainty, or behave unexpectedly. HomeBase’s job is to ensure that those observations cannot cause the system to violate its declared state-machine rules.

### The System Split by Assurance Method

There is no single proof technology that covers all domains equally well. HomeBase uses the right assurance tool for the right layer:

| Layer | Implementation | Evidence Method |
|-------|----------------|-----------------|
| **Requirements & Hazards** | YAML/typed assurance model | Review and traceability |
| **Domain Reducer (Core)** | Dafny | Formal Verification (Z3) |
| **Generated Runtime** | Go generated from Dafny | Reproducible generation & tests |
| **Journal (Storage)** | SQLite / restricted Go | Crash tests & replay validation |
| **Repository Adapter** | Handwritten Go | Contract, concurrency & replay tests |
| **Effect Policy** | Dafny | Lifecycle Proofs |
| **External Effect Adapter** | Handwritten Go | Capability contracts & fault tests |
| **Whole-System Integration**| Go | End-to-End & failure-injection testing |

## What Dafny Proves (And What It Doesn't)

**Good Dafny Territory (Semantic Decisions):**
- Command authorization
- Lifecycle transitions
- Recovery limits and bounds
- Effect identity rules and retry policies
- Claim epochs and duplicate handling
- Invariants over pure state

**Poor Dafny Territory (Physical Boundaries):**
- Opening files, partial writes, fsync
- HTTP clients, sockets, TLS
- Process crashes, goroutines
- Remote API semantics and adapter error classification

*Dafny verifies that the state-transition logic is mathematically sound. It does not prove that the underlying disk persisted data after a power loss, or that a network provider honored an idempotency key.*

## The Practical Strategy

1. **External World** → produces observations
2. **Handwritten Boundary Adapters** → convert to typed commands
3. **Dafny-Generated Decision Core** → mathematically authorizes events/intents
4. **Durable Repository** → physically commits events
5. **Handwritten Executors** → perform external effects safely based on core's intent

**Rule:** Boundary adapters are allowed to observe and execute. They are *never* allowed to decide policy.

## Adapter Capability Model

Adapters must explicitly declare their capabilities. The verified policy chooses safe behavior based on those capabilities.

```yaml
capabilities:
  idempotency:
    supported: true
    semantics: stable_key_deduplication
  result_query:
    supported: false
  transactionality: none
  retry_safety: conditional
```

An adapter without idempotency support must be handled with a non-retryable terminal failure model, as dictated by the Dafny effect lifecycle.

## Storage: Journal vs. DB

HomeBase focuses its proof obligations on the **repository protocol** and state reconstruction, rather than re-verifying a storage engine. We leverage mature storage engines (e.g., SQLite, Pebble) for the physical journal, relying on their operational evidence, and focus HomeBase's testing on crash consistency (partial write resilience) and deterministic state reconstruction from the ledger.
