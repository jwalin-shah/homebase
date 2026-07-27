# HomeBase: Immutable Decision Ledger (Graph-Structured)

**Status:** Phase 0 Design Complete, Phase 1-5 Implementation Ready  
**Language:** Go 1.26 (primary) + Python (Neo4j seam)  
**Architecture:** Graph-structured execution (traces.com, April 2026 arXiv)  
**Authority:** SYSTEM-DESIGN.md (formal), HB-001 Specification (axiom-grounded)  
**Principles:** Never assume, verify, prove everything (CLAUDE.md)

---

## What is HomeBase

HomeBase is an **immutable decision ledger** with **graph-structured execution** (not agent loops).

### Why Graphs, Not Loops

**Loops (agent reasoning—hidden):**
- ✗ Agent secretly decides to "retry" or "escalate" inside its reasoning
- ✗ Invisible to you (can only read transcript and hope it explains itself)
- ✗ Unbounded retries possible
- ✗ No way to inspect decision logic before run starts
- ✗ Silent plan drift (plan changes as agent adapts mid-execution)

**Graphs (explicit states—visible):**
- ✓ Five named states defined in advance: PLAN → EXECUTE → RECOVER → ESCALATE → REPEAT → COMPLETE
- ✓ Every transition explicit (defined in SYSTEM-DESIGN.md before any code runs)
- ✓ Bounded recovery (Attempt 1, Attempt 2, then escalate—never "creative Attempt 3")
- ✓ Auditable (entire flow inspectable before execution)
- ✓ Immutable plan (cannot drift mid-execution)

### Three Architectural Commitments

**Commitment 1: Immutable Plan**
- Workflow locked at start (PLAN state)
- Cannot be silently revised during EXECUTE or RECOVER
- If revision needed → escalate to human (explicit, not hidden)

**Commitment 2: Separated Layers**
- PLAN: defines workflow (planning only)
- EXECUTE: records decision (execution only, never decides recovery)
- RECOVER: attempts fixes (recovery only, never executes new work)
- ESCALATE: hands to human (escalation only, never decides itself)
- Prevents tangled reasoning where planning/execution/recovery happen in one opaque function

**Commitment 3: Strict Escalation**
- Recovery has bounded attempts (exactly 2, defined in advance)
- No "keep trying different approaches" loop
- After Attempt 2 fails → escalate to human immediately
- No Attempt 3, no "creative workarounds"

### Six Provable Invariants

| Invariant | Statement | How Verified |
|-----------|-----------|--------------|
| **I1: Immutability** | Once written, decisions cannot be modified | JSONL append-only + fsync, no UPDATE/DELETE paths |
| **I2: Uniqueness** | Escalation approved exactly once (≤1) | Mutex per escalation ID + double-check pattern |
| **I3: Durability** | Writes survive process crashes | fsync() called after every write to disk |
| **I4: Integrity** | Invalid signatures rejected before use | Verify() called before any downstream operation |
| **I5: Non-Repudiation** | Bridge responses verified by signature | VerifyBridgeResponse() called before accepting |
| **I6: Axiom Grounding** | All decisions cite existing axioms | ValidateAxioms() rejects invalid citations |

---

## Architecture: Five Moves (Graph States)

```
PLAN (state S0)
  ├─ Input: Decision request
  ├─ Action: Validate preconditions, lock execution workflow
  ├─ Output: Locked plan (immutable artifact)
  └─ Transition to: EXECUTE

EXECUTE (state S1)
  ├─ Input: Locked plan + decision
  ├─ Action: Record to ledger, compute signature, index in Neo4j, log
  ├─ Output: Decision object with status (SUCCESS or FAILURE)
  ├─ Critically: Does NOT decide what to do on failure
  └─ Transition to: RECOVER (if failed) or REPEAT (if success)

RECOVER (state S2)
  ├─ Input: Failure from EXECUTE
  ├─ Protocol (strict):
  │  ├─ Attempt 1: Specific retry (e.g., recompute signature)
  │  ├─ Attempt 2: Different fix (e.g., rebuild Neo4j cache)
  │  └─ No Attempt 3: Proceed to ESCALATE if both fail
  ├─ Output: Success or explicit "exhausted recovery"
  └─ Transition to: REPEAT (if recovered) or ESCALATE (if exhausted)

ESCALATE (state S3)
  ├─ Input: Failure + full recovery history
  ├─ Action: Create escalation record, notify human, await decision
  ├─ Output: Human decision (approve, reject, investigate)
  └─ Transition to: REPEAT (if approved) or COMPLETE (if rejected)

REPEAT (state S4)
  ├─ Input: Status from EXECUTE/RECOVER/ESCALATE
  ├─ Action: Next decision in plan (or end if plan exhausted)
  └─ Transition to: EXECUTE (if more decisions) or COMPLETE (if done)

COMPLETE (state S5)
  ├─ Input: All decisions processed
  ├─ Action: Return audit trail
  └─ End state
```

---

## Type System: Compile-Time Invariant Enforcement

Using Go types to prove correctness at compile-time (before code runs).

```go
// Sealed types (can't be faked)
type SignatureType string        // Only Signer can create
type EscalationStatus string      // Only 5 valid values (PENDING, APPROVED, etc.)
type AxiomID string               // Only exists if in Neo4j

// Example: This won't compile
decision.Signature = "fake_sig"            // ✗ SignatureType is not string
escalation.Status = "WEIRD_STATE"          // ✗ not in const enum
decision.Axioms = []string{"AX-FAKE"}      // ✗ should be AxiomID, requires validation
```

**Benefit:** Compiler catches invariant violations before code even runs.

---

## Formal Specification

**See:** `SYSTEM-DESIGN.md` for complete formal specification including:
- State machine definition (5 states + transitions)
- Type system design (sealed types)
- Mathematical invariants (6 provable guarantees)
- Success criteria (testable conditions for each invariant)

---

## Five-Phase Implementation (Formal, Evidence-Based)

### Phase 0: Design (COMPLETE ✓)

**Deliverable:** SYSTEM-DESIGN.md (locked)

**Evidence:**
- ✓ Design document: SYSTEM-DESIGN.md
- ✓ Risk assessment: 5+ risks identified in design
- ✓ Architecture sketch: State machine + type system diagram

**Sign-offs:** Discovery Lead, Architect

**Gate:** Cannot start Phase 1 without Phase 0 signed

---

### Phase 1: Implementation (Tickets 202-205 Parallel)

**Ticket 202:** PLAN + EXECUTE states
- RecordDecision handler (PLAN)
- Execute sequence (EXECUTE)
- Type system (sealed Decision, Signature, etc.)
- Target: 45+ unit tests, 82%+ coverage

**Ticket 203:** RECOVER + ESCALATE states
- Recovery protocol (exactly 2 attempts)
- Escalation handler (create escalation record)
- Bridge signature verification
- Approval idempotency (mutex per escalation ID)
- Target: 38+ unit tests, 85%+ coverage

**Ticket 204:** Integration tests (all 5 graph moves)
- Happy path: PLAN → EXECUTE → REPEAT → COMPLETE
- Failure path: PLAN → EXECUTE → RECOVER → REPEAT
- Escalation path: PLAN → EXECUTE → RECOVER (both fail) → ESCALATE → human
- Target: 85+ integration tests, 91%+ coverage

**Ticket 205:** Observability (state transitions)
- Structured logging for all 5 states
- Correlation ID threading (end-to-end)
- Metrics collection (decisions recorded, escalated, approved)
- Target: 42+ logging tests, 88%+ coverage

**Evidence per ticket:** ./scripts/collect-phase-evidence.sh 1 [TICKET]

**Sign-offs:** Implementation Team, Architect (per ticket)

**Gate:** All 4 tickets must have Phase 1 evidence + signatures before Phase 2

---

### Phase 2: Unit Tests (Sequential Per Ticket)

**Each ticket:** Write unit tests for their Phase 1 implementation

**Target:** 80%+ coverage (overall average >85%)

**Evidence:** ./scripts/collect-phase-evidence.sh 2 [TICKET]

**Sign-offs:** Tester, Architect

**Gate:** Coverage <80% → Phase 3 blocked until fixed

---

### Phase 3: Integration Tests (All Tickets Together)

**Goal:** Verify all 4 tickets work together, full graph flow works end-to-end

**Tests:**
- Decision recorded → indexed → queryable
- Decision escalated → Bridge analyzes → human approves
- Concurrent decisions don't interfere (each has unique correlation ID)
- Graceful degradation (Neo4j down → skip indexing, continue)

**Evidence:** ./scripts/collect-phase-evidence.sh 3 "INTEGRATED"

**Sign-offs:** Tester, Architect

**Gate:** Integration tests failing → Phase 4 blocked

---

### Phase 4: Independent Audit

**Auditor stress-tests the graph structure:**

**Does SYSTEM-DESIGN.md match implementation?**
- Verify 5 states exist (PLAN, EXECUTE, RECOVER, ESCALATE, REPEAT, COMPLETE)
- Verify transitions follow state machine diagram
- Verify no invalid state transitions possible

**Are all 6 invariants actually enforced?**
- I1 (Immutability): Modify decision in file → rejected? ✓
- I2 (Uniqueness): 5 concurrent approvals → 1 succeeds + 4 get 409? ✓
- I3 (Durability): Kill process mid-write → restart → decision still there? ✓
- I4 (Integrity): Tamper with signature → rejected? ✓
- I5 (Non-Repudiation): Forge Bridge response → rejected? ✓
- I6 (Axiom Grounding): Cite non-existent axiom → rejected? ✓

**Is recovery actually bounded?**
- Feed failure that neither recovery attempt can fix
- Verify escalation after exactly 2 attempts (no Attempt 3)

**Evidence:** AUDIT-FINDINGS-[TICKET]-PHASE-4.md (per ticket)

**Target:** 0 CRITICAL, ≤3 HIGH findings (down from 31 in prior approach)

**Sign-offs:** Independent Auditor, Architect

**Gate:** CRITICAL findings → Phase 5 blocked, send back to Phase 1 for fixes

---

### Phase 5: Production Deployment

**Checks:**
- ✓ All CRITICAL findings fixed
- ✓ HIGH findings have mitigation plans
- ✓ Monitoring wired up (logs, metrics, alerts)
- ✓ Backup/restore tested
- ✓ Graceful shutdown implemented

**Deploy:**
- Stage environment (test against real Neo4j, real Bridge)
- Prod environment (increasing percentage of traffic)
- Monitor for 24 hours

**Evidence:** DEPLOYMENT-STATUS.md, metrics dashboard

**Sign-offs:** Operator, Captain

---

## Epistemic Gate (The Hallucination Firewall)

An agent may propose a claim from memory, but it **may not use that claim as an architectural premise** until the claim is verified against an authoritative source. 
The system enforces separation of: proposal → verification → decision → execution.

### 1. CLAIMS.yaml
Every consequential technical assertion must be recorded in `CLAIMS.yaml` with the following structure before architecture changes occur:
- `id`: Unique claim ID
- `text`: The exact claim
- `type`: Category of evidence (e.g., tool_capability, code_exists, property_proved, crash_durability)
- `consequence`: Impact level (architectural, destructive, security)
- `status`: proposed / verified / refuted
- `source`: Locator for the official documentation, code, or proof artifact
- `confidence`: Agent's confidence

### 2. Primary-Source Gate
For any toolchain, formal method, compiler, storage primitive, or protocol:
1. Retrieve official documentation or source.
2. Record the exact supported direction of transformation.
3. Distinguish between source language, target language, executable artifact, and proof artifact.
4. Only then recommend architecture.

### 3. Evidence-Language Validator
Prose language must never exceed the formal evidence type provided.
- Disallow `mathematically proved`, `guaranteed` unless `formal_verification == true`.
- Disallow `production safe`, `survives power loss` unless `operational_evidence == true`.
- "Generated code" does not imply "Verified code".

### 4. Independent Verifier Pass
Before committing destructive or high-impact architectural changes:
1. The implementer report must be checked by an independent verification invocation.
2. Artifact inspection must match the claim evidence.
3. The claim budget (max 0 unverified claims, max 3 inferred claims) must be strictly enforced.

---

## File Structure (Updated for Graph)

```
homebase/
├── SYSTEM-DESIGN.md              # ← Formal specification (Phase 0)
├── AGENTS.md                     # ← This file (how to implement)
├── CLAUDE.md                     # ← Project principles (mirrors global)
│
├── cmd/
│   └── homebase/
│       └── main.go               # Entry point (initializes all 5 states)
│
├── internal/
│   ├── graph/                    # ← NEW: Graph state machine
│   │   ├── state.go              # State interface + enum
│   │   ├── plan.go               # PLAN state
│   │   ├── execute.go            # EXECUTE state
│   │   ├── recover.go            # RECOVER state (bounded attempts)
│   │   ├── escalate.go           # ESCALATE state
│   │   └── graph_test.go         # State transition tests
│   │
│   ├── ledger/
│   │   ├── store.go              # JSONL append-only (I1: immutable)
│   │   └── ledger_test.go
│   │
│   ├── signing/
│   │   ├── signer.go             # Ed25519 signing
│   │   ├── verifier.go           # Verification (I4: integrity, I5: non-repudiation)
│   │   └── signing_test.go
│   │
│   ├── cache/
│   │   ├── neo4j.go              # Neo4j client (I6: axiom grounding)
│   │   └── cache_test.go
│   │
│   ├── validation/
│   │   ├── validator.go          # Validate axioms exist
│   │   └── validator_test.go
│   │
│   └── types/
│       ├── sealed.go             # SignatureType, AxiomID (compile-time invariants)
│       └── decision.go           # Immutable decision struct
│
├── api/
│   ├── handlers.go               # HTTP handlers (map to graph states)
│   ├── middleware.go             # Auth, rate limiting, correlation ID
│   └── handlers_test.go
│
├── scripts/
│   ├── setup-hooks.sh            # Install git hooks
│   ├── collect-phase-evidence.sh # Collect evidence per phase
│   └── common-bugs-check.sh      # Scan for known patterns
│
├── .githooks/
│   ├── pre-commit                # Check for common bugs
│   ├── commit-msg                # Validate ticket reference
│   └── phase-gate-local          # Check phase progression
│
├── tickets/
│   ├── TICKET-202-PHASE-*.md     # Bridge integration (202 per phase)
│   ├── TICKET-203-PHASE-*.md     # Neo4j querying (203 per phase)
│   ├── TICKET-204-PHASE-*.md     # Integration testing (204 per phase)
│   ├── TICKET-205-PHASE-*.md     # Observability (205 per phase)
│   │
│   ├── PHASE-ENFORCEMENT-FRAMEWORK.md
│   ├── COMMON-BUGS-CATALOG.md
│   ├── GROUNDED-REASONING-EVIDENCE.md
│   ├── ENFORCEMENT-HARNESS-LOCAL.md
│   │
│   ├── PHASE-SIGN-OFF-SHEET.md   # Sign-off tracking (5 roles × 6 phases)
│   ├── QUALITY-METRICS.md        # Dashboard (issues per phase, coverage, etc.)
│   └── AUDIT-FINDINGS-*.md       # Phase 4 findings (per ticket)
│
├── docs/
│   ├── ARCHITECTURE.md           # System overview
│   ├── API.md                    # Endpoint documentation
│   ├── DEPLOYMENT.md             # Ops guide
│   └── GRAPH-ENGINEERING.md      # Graph vs loops explained
│
├── go.mod
├── Makefile
└── README.md                     # Quick start
```

---

## Building & Testing

```bash
# Install git hooks (one-time)
./scripts/setup-hooks.sh

# Build
make build

# Test (all phases)
make test

# Format check
make fmt-check

# Full CI (what Phase 1-4 gates use)
make ci

# Collect evidence for current phase
./scripts/collect-phase-evidence.sh 1 202

# Check for common bugs
./scripts/common-bugs-check.sh

# Clean
make clean
```

---

## The Paradigm Shift: Verified Extraction (No More Raw Go for Core Logic)

**Crucial Lesson Learned:** We can no longer rely on writing raw Go and trying to verify it retroactively with Gobra. AI agents will hallucinate tautologies and bypass constraints (e.g., deadlocking under concurrency, spoofing Z3 proofs). 

To achieve the "best version" of this system, we use **Verified Extraction**:
1. **The Core Logic (State Machine & Locks):** Must be written in **Dafny**. The AI writes Dafny, the Z3 solver mathematically proves it, and the Dafny compiler auto-generates the verified Go code.
2. **The Ledger (Crash Safety):** Must use **Coq/Perennial/Goose**. The AI writes the storage logic in Coq to guarantee survival through arbitrary crashes, which is then extracted to Go.
3. **The Global Bounds:** Must be written in **Lean 4**. The Lean type system acts as the absolute, infallible referee for the system's global architecture.
4. **The Armor (HTTP, Routing, Structs):** We use standard Go, but bounded by strict Pydantic/Struct schemas and the Cryptographic Ledger.

*AI Agents are no longer allowed to hand-write Go code for the execution core. They must use the ITP (Interactive Theorem Prover) compilers.*

---

## Alignment with CLAUDE.md Principles

| Your Principle | How HomeBase Enforces It |
|---|---|
| **Never assume** | Graph makes every decision explicit (no hidden logic) |
| **Everything must be proved** | 6 provable invariants, formal state machine, type system |
| **Challenge everything** | Graph forces strict escalation (challenges agent retries) |
| **Minimum code, maximum understanding** | Type system + explicit states (easy to understand) |
| **Service capability gate** | 5 named states = "capability inventory" (what can happen) |

---

## Ready?

Once captain approves SYSTEM-DESIGN.md, implementation follows this AGENTS.md exactly.

All code is **shaped by the graph structure, not by agent reasoning.**
