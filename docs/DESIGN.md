# HomeBase: Upfront Design Document

**Date:** 2026-07-25  
**Authority:** HB-001 Specification + Captain Approved  
**Purpose:** Define interfaces, data flows, tests BEFORE implementation  
**Status:** Ready for Review  

---

# PART 1: SYSTEM OVERVIEW

## Three-Ticket Architecture

```
┌─────────────────────────────────────────────────┐
│         Ticket 201: Portfolio                   │
│  (Master Control Plane for ALL Decisions)       │
│                                                 │
│  • Immutable JSONL ledger                       │
│  • Decision storage + querying                  │
│  • Axiom validation gate                        │
│  • Coordination endpoints                       │
└──────────────────┬──────────────────────────────┘
                   │ creates: ledger.jsonl
                   │ exposes: 6 API endpoints
                   │ requires: Neo4j connection
                   │
┌──────────────────▼──────────────────────────────┐
│       Ticket 202: Bridge Redesign               │
│    (AX-SPAWN-001: 4 Conditions)                 │
│                                                 │
│  • Precondition verification                   │
│  • Live observability (mintmux)                │
│  • Explicit error handling                     │
│  • Escalation path                             │
└──────────────────┬──────────────────────────────┘
                   │ uses: portfolio API
                   │ logs to: portfolio endpoints
                   │ queries: axioms from Neo4j
                   │
┌──────────────────▼──────────────────────────────┐
│     Ticket 203: Axioms Integration              │
│  (Knowledge Engine + Feedback Loop)             │
│                                                 │
│  • Portfolio queries axioms                    │
│  • Bridge checks axiom compliance              │
│  • Orbit verifies axiom satisfaction           │
│  • Ledger patterns → new axioms                │
└─────────────────────────────────────────────────┘
```

---

# PART 2: DATA FLOWS

## Flow 1: Decision Creation → Ledger → Cache

```
Step 1: Decision Created
  WHO: Portfolio user (captain)
  WHERE: POST /api/v1/decisions
  INPUT: {
    id: "dec-20260725-001",
    decision: "Allow network access to Neo4j",
    axioms: ["AX-SECURITY-004"],
    evidence: "Need axiom corpus",
    decided_by: "captain",
    approver: "captain",
    tags: ["security"],
    risk_level: "critical",
    affected_systems: ["bridge"],
    related_decisions: []
  }

Step 2: Validation Gate (Portfolio Task 3)
  ACTION: Validate decision before ledger
  CHECK 1: JSON schema validation (required fields)
  CHECK 2: Axiom existence (Neo4j query: axiom_id exists?)
    QUERY: MATCH (a:Axiom {id: $axiom_id}) RETURN a.id
    RESULT: If NOT found → return 400 "axiom not found"
  CHECK 3: No duplicate decision (ledger scan by ID)

Step 3: Sign Decision (HB-001 Defense 19)
  ACTION: Ed25519 signature
  INPUT: {decision_json, private_key}
  OUTPUT: signature (hex-encoded Ed25519 sig)
  APPEND: decision.signature = signature

Step 4: Write to Ledger (JSONL append-only)
  FILE: portfolio/ledger.jsonl
  ACTION: append one line (JSON)
  GUARANTEE: ACID (atomic write, durable)
  VERIFY: fsync + hash chain

Step 5: Return Success (ACID guaranteed)
  RESPONSE: 201 Created {
    id: "dec-20260725-001",
    recorded_at: "2026-07-25T10:00:00Z",
    signature: "ed25519_hex_string",
    ledger_line: 42  // line number for audit
  }

Step 6: [Async] Update Neo4j Cache
  ACTION: Background job, non-blocking
  QUERY: CREATE (d:Decision {id, axioms, tags, ...})
  CREATE (d)-[:CITES]->(a:Axiom {id: axiom_id})
  TTL: Must complete within 5 seconds (or mark cache stale)
  FAILURE: If Neo4j down, ledger already persisted (no data loss)

Result:
  ✓ Decision in ledger (ACID)
  ✓ Decision in cache (eventually consistent)
  ✓ Both queryable
  ✓ Signature verified
```

---

## Flow 2: Bridge Spawn Decision → Escalation → Approval → Retry

```
Step 1: Bridge Spawn Triggered
  WHO: Bridge (via CLI or API)
  INPUT: ticket {
    id: "ticket-202",
    verification_commands: [
      {argv: ["curl", "-s", "http://localhost:7474/..."]}
    ]
  }

Step 2: Precondition Check (Ticket 202 Task 1)
  ACTION: Verify sandbox + command compatibility BEFORE spawn
  CHECK: For each verification_command:
    1. Parse command (argv[0] = tool name)
    2. Query sandbox.sb: does tool T have permission P?
    3. If mismatch → REJECT BEFORE spawn
  EXAMPLE:
    Command: curl to http://localhost:7474
    Sandbox: (deny network) ← has no network rule
    Result: REJECT with "denied: network-outbound to 127.0.0.1:7474"
  RESPONSE: 400 BadRequest {
    error: "precondition_failed",
    reason: "network-outbound required but not granted",
    requested_permission: "network-outbound",
    command: "curl -s http://localhost:7474/..."
  }

Step 3: Query Portfolio + Axioms (Ticket 203 Task 2)
  ACTION: Load axioms relevant to spawn before executing
  QUERY 1: GET /api/v1/decisions?axiom=AX-SPAWN-001
    RESULT: Load AX-SPAWN-001 conditions
  QUERY 2: GET /api/v1/decisions?axiom=AX-SECURITY-004
    RESULT: Load security axioms
  VERIFY: Spawn matches axiom requirements

Step 4: Spawn Worker (with Live Observability)
  ACTION: Launch worker via mintmux
  WATCH: Don't poll manifest.json, watch pane in real-time
  LOG: Every tool call: {tool, input, output, exit_code, timestamp}
  STREAM: Send events to portfolio async (don't block worker)

Step 5: Worker Succeeds (Happy Path)
  RESULT: manifest.json written by worker
  {
    "status": "completed",
    "exit_code": 0,
    "commands_executed": 1,
    "timestamp": "2026-07-25T10:05:00Z"
  }
  ACTION: Bridge logs success to portfolio

Step 6: Worker Fails (Denied Permission)
  RESULT: Sandbox denies network call
  manifest.json written by tool interceptor:
  {
    "status": "failed",
    "error": "denied",
    "reason": "network-outbound to 127.0.0.1:7474",
    "command_that_failed": "curl -s http://localhost:7474/...",
    "needs_escalation": true,
    "permission_needed": "network-outbound",
    "timestamp": "2026-07-25T10:00:05Z"
  }

Step 7: Escalation Request (Ticket 202 Task 4)
  WHO: Bridge detects "needs_escalation" in manifest
  ACTION: POST /api/v1/escalations
  {
    id: "esc-20260725-001",
    worker_id: "w-12345",
    ticket_id: "ticket-202",
    permission_needed: "network-outbound",
    resource: "127.0.0.1:7474",
    reason: "Need to query Neo4j axiom corpus",
    created_at: "2026-07-25T10:00:05Z",
    status: "PENDING"
  }
  RESPONSE: 201 {
    escalation_id: "esc-20260725-001",
    url: "/api/v1/escalations/esc-20260725-001"
  }

Step 8: Captain Approves (Captain Action)
  WHO: Captain reads escalation request
  ACTION: POST /api/v1/escalations/esc-20260725-001/approve
  {
    approved_by: "captain",
    reason: "Need axiom corpus for decisions",
    sandbox_update: "add network-outbound to 127.0.0.1:7474"
  }
  RESPONSE: 200 {
    escalation_id: "esc-20260725-001",
    status: "APPROVED",
    approved_at: "2026-07-25T10:01:00Z"
  }
  ACTION: Portfolio logs this decision (with axiom citations)

Step 9: Bridge Retries (Ticket 202 Task 4)
  WHO: Bridge polls: GET /api/v1/escalations/esc-20260725-001/status
  RESPONSE: {status: "APPROVED"}
  ACTION: Update sandbox.sb with new permission
  ACTION: Spawn worker again with updated sandbox
  RESULT: Worker succeeds ✓

Step 10: Success Logged to Portfolio
  WHO: Bridge logs success
  ACTION: POST /api/v1/decisions/log
  {
    system: "bridge",
    action: "spawn-ticket-202",
    ticket_id: "ticket-202",
    result: "success",
    axioms_checked: ["AX-SPAWN-001", "AX-SECURITY-004"],
    escalations: ["esc-20260725-001"],
    worker_output: "... tool output ...",
    timestamp: "2026-07-25T10:05:00Z"
  }
  RESPONSE: 201 {decision_id: "dec-20260725-002"}
```

---

## Flow 3: Axiom Extraction → Ingestion → Use in Next Decision

```
Step 1: Periodic Ledger Scan (Ticket 203 Task 4)
  TRIGGER: Weekly (or on-demand)
  ACTION: Query ledger for patterns
  QUERY: Scan last 100 decisions
  FIND: Decisions with {tags: "auth", risk_level: "critical"}
  COUNT: 12 similar decisions found
  THRESHOLD: >10 similar → pattern detected ✓

Step 2: Formalize Pattern as Axiom
  ACTION: Create axiom equation
  PATTERN: "All critical auth decisions require escalation"
  FORMALIZE: ∀d ∈ decisions: tag[auth] ∧ risk[critical] → escalation_required(d)
  AXIOM: {
    id: "AX-AUTH-CRITICAL-001",
    equation: "∀d: tag[auth] ∧ risk[critical] → escalation_required(d)",
    domain: "authentication",
    discovery_source: "portfolio-ledger-analysis",
    discovery_date: "2026-07-25",
    discovery_context: "12 critical auth decisions observed",
    verdict: "PROPOSED"  // Not VERIFIED yet
  }

Step 3: Validate New Axiom (Ticket 203 Task 4)
  ACTION: Check if new axiom cites existing axioms
  QUERY: Neo4j for AX-ESCALATION-009, AX-SECURITY-004
  VERIFY: New axiom doesn't contradict existing axioms
  RESULT: Valid ✓

Step 4: Ingest to Neo4j (Ticket 203 Task 4)
  ACTION: POST /api/v1/axioms/ingest
  {
    id: "AX-AUTH-CRITICAL-001",
    equation: "∀d: tag[auth] ∧ risk[critical] → escalation_required(d)",
    domain: "authentication",
    discovery_source: "portfolio-ledger-analysis",
    verdict: "PROPOSED"
  }
  BACKEND: 
    CREATE (a:Axiom {
      id: "AX-AUTH-CRITICAL-001",
      equation: "...",
      verdict: "PROPOSED"
    })
    CREATE (hb:System {name: "HomeBase"})-[:DISCOVERED]->(a)
  RESPONSE: 201 {
    axiom_id: "AX-AUTH-CRITICAL-001",
    status: "PROPOSED",
    queryable_at: "2026-07-25T10:10:00Z"
  }

Step 5: Next Decision Uses New Axiom
  SCENARIO: New auth decision created
  ACTION: POST /api/v1/decisions
  {
    decision: "Allow SSH key upload",
    tags: ["auth"],
    risk_level: "critical",
    axioms: ["AX-AUTH-CRITICAL-001"]  ← NEW AXIOM!
  }
  VALIDATION:
    1. Check AX-AUTH-CRITICAL-001 exists (yes, just ingested)
    2. Verify: tag[auth] ∧ risk[critical] → escalation_required(d)
    3. Escalation automatically required
  RESULT: Decision stored + escalation flow triggered
  BENEFIT: System learned from pattern, enforced it on next decision
```

---

# PART 3: INTERFACES (APIs)

## Portfolio API Endpoints

### **1. Record Decision**
```
POST /api/v1/decisions
Authorization: Bearer {token}

Request:
{
  "id": "dec-20260725-001",           # UUID or auto-generated
  "decision": "Allow network access to Neo4j",
  "axioms": ["AX-SECURITY-004"],      # Required (must exist in Neo4j)
  "evidence": "Need axiom corpus for decisions",
  "decided_by": "captain",
  "approver": "captain",              # Optional (approval is separate)
  "tags": ["security"],               # For filtering
  "risk_level": "critical",           # trivial|minor|major|critical
  "affected_systems": ["bridge"],     # Which systems impacted
  "related_decisions": []             # Linked decision IDs
}

Response: 201 Created
{
  "id": "dec-20260725-001",
  "recorded_at": "2026-07-25T10:00:00Z",
  "signature": "ed25519_hex_...",
  "ledger_line": 42,
  "status": "APPROVED"
}

Errors:
- 400: Validation failed (missing axiom, invalid schema)
- 409: Duplicate decision ID
- 503: Neo4j down (but ledger still works)
```

### **2. Retrieve Decision**
```
GET /api/v1/decisions/{id}

Response: 200 OK
{
  "id": "dec-20260725-001",
  "decision": "...",
  "axioms": [...],
  "recorded_at": "2026-07-25T10:00:00Z",
  "signature": "...",
  "status": "APPROVED",
  "source": "ledger"  # or "cache" if from Neo4j
}

Errors:
- 404: Decision not found
- 202: Found in cache but stale (>1h old, Neo4j down)
```

### **3. Query Decisions**
```
GET /api/v1/decisions?axiom=AX-SECURITY-004&limit=100
GET /api/v1/decisions?tag=auth&risk_level=critical
GET /api/v1/decisions?decided_by=captain&date_after=2026-07-25

Response: 200 OK
[
  {id, decision, axioms, recorded_at, ...},
  ...
]

Latency SLAs:
- By axiom (indexed): P99 < 100ms
- By tag (indexed): P99 < 500ms
- By date range: P99 < 1s
- Falls back to ledger scan if Neo4j down
```

### **4. Verify Signature**
```
POST /api/v1/decisions/{id}/verify

Request:
{
  "signature": "ed25519_hex_..."
}

Response: 200 OK
{
  "valid": true,
  "decision_id": "dec-20260725-001",
  "decision_maker": "captain",
  "verified_at": "2026-07-25T10:00:00Z"
}

Errors:
- 400: Signature invalid (tampering detected)
```

### **5. Log System Decision (for Bridge/Orbit/Axioms)**
```
POST /api/v1/decisions/log
Authorization: Bearer {system-token}

Request:
{
  "system": "bridge",              # Which system is logging
  "action": "spawn-ticket-202",    # What action
  "ticket_id": "ticket-202",       # What it's logging about
  "result": "success",             # success|failed|escalated
  "axioms_checked": ["AX-SPAWN-001"],  # Which axioms were checked
  "evidence": "worker completed, exit code 0",
  "timestamp": "2026-07-25T10:05:00Z"
}

Response: 201 Created
{
  "decision_id": "dec-20260725-002",
  "logged_at": "2026-07-25T10:05:00Z",
  "signature": "..."
}

Purpose: Bridge/Orbit log back to portfolio what happened
```

### **6. Health Check**
```
GET /api/v1/health

Response: 200 OK
{
  "status": "full",  # or "degraded"
  "ledger": "healthy",
  "cache": "healthy",  # or "unavailable"
  "signature_key": "loaded",
  "degraded_reason": null
}

Degraded when:
- Neo4j offline (ledger still works, cache stale)
- Signature key missing (can still record, can't sign)
```

---

## Escalation API Endpoints

### **7. Create Escalation Request**
```
POST /api/v1/escalations
Authorization: Bearer {bridge-token}

Request:
{
  "worker_id": "w-12345",
  "ticket_id": "ticket-202",
  "permission_needed": "network-outbound",
  "resource": "127.0.0.1:7474",
  "reason": "Need to query Neo4j axiom corpus"
}

Response: 201 Created
{
  "escalation_id": "esc-20260725-001",
  "status": "PENDING",
  "created_at": "2026-07-25T10:00:05Z",
  "url": "/api/v1/escalations/esc-20260725-001"
}
```

### **8. Get Escalation Status**
```
GET /api/v1/escalations/{escalation_id}

Response: 200 OK
{
  "escalation_id": "esc-20260725-001",
  "status": "PENDING",  # or APPROVED|DENIED
  "permission_needed": "network-outbound",
  "resource": "127.0.0.1:7474",
  "created_at": "2026-07-25T10:00:05Z",
  "approved_at": null
}
```

### **9. Approve Escalation**
```
POST /api/v1/escalations/{escalation_id}/approve
Authorization: Bearer {captain-token}

Request:
{
  "approved_by": "captain",
  "reason": "Need axiom corpus for decisions"
}

Response: 200 OK
{
  "escalation_id": "esc-20260725-001",
  "status": "APPROVED",
  "approved_at": "2026-07-25T10:01:00Z"
}

Side effect: Portfolio logs decision (with axiom citation)
Side effect: Bridge can now retry with updated sandbox
```

### **10. Axiom Ingestion**
```
POST /api/v1/axioms/ingest
Authorization: Bearer {axioms-token}

Request:
{
  "id": "AX-AUTH-CRITICAL-001",
  "equation": "∀d: tag[auth] ∧ risk[critical] → escalation_required(d)",
  "domain": "authentication",
  "discovery_source": "portfolio-ledger-analysis",
  "verdict": "PROPOSED"
}

Response: 201 Created
{
  "axiom_id": "AX-AUTH-CRITICAL-001",
  "status": "PROPOSED",
  "queryable_at": "2026-07-25T10:10:00Z"
}

Backend: Neo4j CREATE + link to HomeBase system
```

---

# PART 4: TESTS & PROOFS

## Tests Mapped to Tickets

### **Ticket 201: Portfolio Tests**

| Test | Type | What | How | Pass Criteria |
|------|------|------|-----|---------------|
| **P1T1** | Unit | Ledger append-only | Write decision, try to overwrite, verify fails | Overwrite rejected (file unchanged) |
| **P1T2** | Unit | JSONL schema validation | Send 5 invalid decisions (missing field, wrong type, etc) | All 5 rejected before ledger write |
| **P1T3** | Unit | Axiom validation gate | Cite non-existent axiom AX-FAKE-999 | Decision rejected with 400 error |
| **P1T4** | Unit | Duplicate detection | Create same decision twice | Second rejected as duplicate |
| **P1T5** | Integration | Record + Query by ID | Record decision, GET /decisions/{id} | Returns exact decision |
| **P1T6** | Integration | Query by axiom (indexed) | Record 100 decisions with tags, query by axiom | Returns all matching, latency P99 < 100ms |
| **P1T7** | Integration | Query by tag (indexed) | Query by tag="auth", risk="critical" | Returns matching decisions, P99 < 500ms |
| **P1T8** | Integration | Signature verification | Sign decision, verify signature, modify content, verify fails | Tampered signature rejected |
| **P1T9** | Integration | Ledger durability | Write decision, kill process mid-write, restart | Decision recovered (no corruption) |
| **P1T10** | E2E | Full decision lifecycle | Record → validate → sign → ledger → cache → query | All steps succeed, decision queryable |
| **P1T11** | Chaos | Ledger survives crash | Write, fsync, kill power sim, restart | Decision persisted, ledger consistent |
| **P1T12** | Chaos | Neo4j divergence | Kill Neo4j, record 10 decisions, restart | Ledger has 10, cache rebuilds from ledger, all 10 queryable |

**Total P1 Tests: 12 (11 functional + 1 chaos)**

### **Ticket 202: Bridge Tests**

| Test | Type | What | How | Pass Criteria |
|------|------|------|-----|---------------|
| **B1T1** | Unit | Precondition check succeeds | Sandbox allows network, command needs network | Spawn allowed |
| **B1T2** | Unit | Precondition check fails | Sandbox denies network, command needs network | Spawn rejected (error message includes missing permission) |
| **B1T3** | Unit | Error type enum | Test all 5 error types (denied, not_found, crashed, timeout, offline) | All types defined + used |
| **B1T4** | Integration | Live observability | Spawn worker, watch pane in real-time, log tool calls | All tool calls logged (not polling manifest) |
| **B1T5** | Integration | Explicit error messages | Worker fails with permission denied | Manifest has error type + reason (not timeout) |
| **B1T6** | Integration | Escalation detection | Manifest has "needs_escalation" | Bridge detects + creates escalation request |
| **B1T7** | Integration | Escalation flow | Create escalation → approve → bridge retries | Worker succeeds after retry |
| **B1T8** | Integration | Sandbox update | Captain approves permission → bridge updates sandbox.sb | Sandbox has new rule, worker succeeds |
| **B1T9** | E2E | Full AX-SPAWN-001 workflow | Spawn → precondition check → execute → escalate → approve → retry | Worker completes, decision logged to portfolio |
| **B1T10** | Chaos | Timeout explicit (not silent) | Tool takes 10min, circuit breaker timeout 30s | Circuit opens after 30s (not silent hang) |
| **B1T11** | Chaos | Escalation approval timeout | Escalation created, no approval for 5min | Bridge fails with reason (not hangs forever) |
| **B1T12** | Chaos | Worker crash recovery | Worker crashes mid-execution | Bridge detects, cleans up, writes error to manifest |

**Total B1 Tests: 12 (11 functional + 1 chaos)**

### **Ticket 203: Axioms Tests**

| Test | Type | What | How | Pass Criteria |
|------|------|------|-----|---------------|
| **A1T1** | Unit | Axiom query by domain | Query Neo4j: axioms with domain="authentication" | Returns AX-SECURITY-*, AX-AUTH-* |
| **A1T2** | Unit | Pattern detection | Scan 100 decisions, find >10 with {tag: "auth", risk: "critical"} | Pattern detected, threshold crossed |
| **A1T3** | Unit | Axiom formalization | Convert pattern to equation | Equation is valid logic (∀ with ∧/∨) |
| **A1T4** | Integration | Axiom ingestion | POST /api/axioms/ingest, verify Neo4j has it | Axiom queryable, marked PROPOSED |
| **A1T5** | Integration | Portfolio queries axioms | Create decision with tag="auth", portfolio queries relevant axioms | Loads AX-SECURITY-* + AX-AUTH-* |
| **A1T6** | Integration | Bridge checks axioms before spawn | Bridge loads AX-SPAWN-001 before spawn | Checks all 4 conditions (preconditions, observability, errors, escalation) |
| **A1T7** | Integration | New axiom used in next decision | Extract axiom AX-AUTH-CRITICAL-001, create new auth decision, cite it | New axiom available, citable, validated |
| **A1T8** | E2E | Full knowledge loop | Scan ledger → extract pattern → formalize → ingest → use in decision | Loop closes, new axiom benefits next decision |
| **A1T9** | Chaos | Neo4j down during extraction | Start extraction, kill Neo4j, restart | Extraction resilient, axiom ingested on reconnect |
| **A1T10** | Chaos | Axiom contradiction detected | Try to ingest axiom that contradicts existing (logic error) | Ingestion rejected, error logged |

**Total A1 Tests: 10 (7 functional + 3 chaos)**

---

## Test Execution Gates

### **Before Ticket 201 Can Ship:**
```
✓ All P1 tests pass (12/12)
✓ Ledger append-only verified
✓ All decisions have axiom citations
✓ Neo4j divergence recovered
✓ Signature verification works
```

### **Before Ticket 202 Can Ship:**
```
✓ All B1 tests pass (12/12)
✓ Precondition checks 100% accurate
✓ No silent timeouts (all errors explicit)
✓ Escalation flow works end-to-end
✓ 4 AX-SPAWN-001 conditions met
✓ Bridge logs all decisions to portfolio
```

### **Before Ticket 203 Can Ship:**
```
✓ All A1 tests pass (10/10)
✓ Portfolio queries axioms correctly
✓ Bridge checks axiom compliance
✓ New axioms discoverable + usable
✓ Knowledge loop closes (pattern → axiom → decision)
```

### **Overall System Gate (All 3 Tickets):**
```
✓ 34 total tests pass
✓ Coverage > 90% on critical paths
✓ 5 chaos tests pass (partition, clock skew, concurrent, timeout, crash)
✓ All 23 HB-001 defenses verified in tests
✓ Decision latency SLAs met (P99: Tier 1 < 100ms, Tier 2 < 500ms, Tier 3 < 5s)
✓ No data loss scenario
✓ Offline mode tested
✓ Graceful degradation tested
✓ Circuit breaker state machine verified
```

---

# PART 5: ERROR HANDLING & RECOVERY

## Error Scenarios

| Scenario | Trigger | Handling | Recovery |
|----------|---------|----------|----------|
| **Neo4j Down** | Neo4j unavailable | Ledger still works (ACID), cache unavailable | Queries fall back to ledger (slower), API returns 206 Partial |
| **Ledger Full** | Disk space exhausted | Async writes queue to memory (bounded) | Alert ops, fallback to memory buffer (degraded mode) |
| **Signature Key Missing** | Private key not found | Can't sign new decisions | Return 503, escalate to ops |
| **Axiom Not Found** | Cite non-existent axiom | Reject decision before ledger | Return 400, show missing axiom ID |
| **Escalation Timeout** | Captain doesn't approve | Worker retries N times, then escalates to ops | Manifest has "escalation_timeout" reason |
| **Worker Crash** | Worker process dies | Bridge detects via stale manifest | Writes error: "crashed", cleans up worktree |
| **Concurrent Write** | Two decisions same ID | Ledger append-only prevents duplicate | Second write rejected (conflict detection) |
| **Sandbox Update Fails** | Can't write sandbox.sb | Escalation stuck in APPROVED state | Bridge detects, alerts ops, retry with backoff |
| **Axiom Ingestion Fails** | Neo4j write error | Mark axiom as FAILED, don't ingest | Retry later, ledger still valid |
| **Cache Divergence** | Ledger and Neo4j out of sync | Detect via row count mismatch | Rebuild cache from ledger (background job) |

---

# PART 6: TICKET MAPPING

## What Each Ticket Implements

### **Ticket 201: Portfolio Redesign**

Implements from HB-001:
- Section 4.2: Decision type (with all fields)
- Section 4.3: API endpoints (6 endpoints)
- Defense 5: Hybrid ACID/BASE (ledger ACID, cache eventual)
- Defense 10: Validation layer
- Defense 19: Non-repudiation (Ed25519 signing)
- Defense 21: Consistency model (hybrid)

Creates:
- `portfolio/ledger.jsonl` (immutable, append-only)
- `internal/ledger/ledger.go` (JSONL store)
- `internal/signing/signer.go` (Ed25519)
- `internal/cache/neo4j.go` (Neo4j client)
- `api/handlers.go` (6 endpoints)
- Tests: P1T1-P1T12

Tests:
- Append-only ledger
- Axiom validation gate
- Signature verification
- Query by axiom/tag/date
- Neo4j divergence recovery
- Ledger durability (crash recovery)

---

### **Ticket 202: Bridge Redesign**

Implements from HB-001:
- Axiom AX-SPAWN-001 (4 conditions)
- Section 4.2: Decision type (logging decisions)
- Defense 1: Coupling (message queue)
- Defense 2: Cascading (circuit breaker)
- Defense 8: Tool changes (contract versioning)
- Defense 11: Error recovery (explicit errors)
- Defense 15: Isolation (circuit breaker)
- Defense 17: Graceful degradation (continue on tool failure)

Creates:
- `internal/spawn/preconditions.go` (condition 1)
- `internal/spawn/observability.go` (condition 2)
- `internal/spawn/errors.go` (condition 3)
- `internal/spawn/escalation.go` (condition 4)
- `cmd/bridge/spawn.go` (AX-SPAWN-001 orchestration)
- Tests: B1T1-B1T12

Tests:
- Precondition verification
- Live observability (no polling)
- Explicit error messages
- Escalation flow
- Circuit breaker state machine
- Timeout handling (explicit, not silent)

---

### **Ticket 203: Axioms Integration**

Implements from HB-001:
- Defense 12: Knowledge retention (extraction loop)
- Defense 6: Scope creep (decision classification)
- Section 6: Axiom ingestion process
- Section 6.2: Discovery → formalize → ingest

Creates:
- `cmd/axiom-extractor/main.go` (pattern detection)
- `internal/axioms/extractor.go` (formalization)
- `api/axioms_handler.go` (ingestion endpoint)
- Neo4j integration (queries + writes)
- Tests: A1T1-A1T10

Tests:
- Axiom query by domain
- Pattern detection (>10 similar decisions)
- Axiom formalization
- Ingestion to Neo4j
- Portfolio queries axioms
- Bridge checks axioms before spawn
- New axiom used in next decision
- Full knowledge loop (pattern → axiom → decision)

---

# CONCLUSION

**This design provides:**

✅ Clear data flows (201 → 202 → 203)  
✅ Defined interfaces (10 API endpoints)  
✅ Testable requirements (34 tests mapped to tickets)  
✅ Error handling (10 scenarios + recovery paths)  
✅ Alignment with HB-001 (all 23 defenses covered)  

**Tickets can now be rewritten with these details:**
- Exact APIs to implement
- Exact tests to pass
- Exact error scenarios to handle
- Exact data flows to support

Ready to rewrite tickets with this design, captain?
