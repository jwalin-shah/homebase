# TICKET 201: Portfolio Redesign Implementation

**ID:** 201  
**Title:** Build Portfolio as Master Control Plane  
**Status:** Ready to Implement (Design-Backed)  
**Authority:** HB-001 Specification + DESIGN.md + Portfolio Redesign V2  
**Timeline:** 1 week (Phase 2)  
**Blocked by:** None  
**Blocks:** Tickets 202, 203  

**Design Reference:** `/Users/jwalinshah/projects/homebase/docs/DESIGN.md`
- Part 1: System Overview
- Part 2: Data Flow 1 (Decision Creation)
- Part 3: Portfolio API Endpoints (1-6)
- Part 4: Portfolio Tests (P1T1-P1T12)

---

## What

Implement Portfolio as the **master control plane** for ALL decisions. Every decision (architectural, implementation, policy) gets logged with axiom citations, evidence, and cryptographic signatures.

**Result:** 
- Immutable JSONL ledger (ACID)
- Neo4j cache (eventual consistency)
- 6 REST API endpoints
- Non-repudiation via Ed25519 signing

---

## From

- `/Users/jwalinshah/projects/homebase/HB-001-COMPLETE-SPECIFICATION.md` (Section 4.2, 4.3, Defenses 5, 10, 19, 21)
- `/Users/jwalinshah/projects/homebase/docs/DESIGN.md` (Part 2, 3, 4)
- `/Users/jwalinshah/projects/portfolio/PORTFOLIO_REDESIGN_V2.md`

---

## Tasks

### Task 1: Wayfinder Map Structure
**What:** Implement directory structure for active + completed maps

**Structure:**
```
portfolio/wayfinder/
├── active-maps/
│   ├── system-redesign-axiom-first-2026-07-25/
│   │   ├── map.md (plan + decisions + status)
│   │   ├── tickets/ (201, 202, 203 specs)
│   │   └── results/ (deliverables)
│   └── [other active maps]
├── completed-maps/ (archive reference)
└── template-map.md (for new maps)
```

**Success Criteria:**
- ☑ Directories created (active-maps, completed-maps)
- ☑ Existing wayfinder map moved to active-maps/
- ☑ Template created (template-map.md)
- ☑ Template includes: Destination, Decisions, Active Work, Success Criteria sections

---

### Task 2: Decision Ledger (JSONL Append-Only)
**What:** Implement immutable decision log at `portfolio/ledger.jsonl`

**Files to Create:**
- `internal/ledger/ledger.go` — append-only store, atomic writes
- `internal/ledger/schema.go` — JSON schema validation
- `internal/ledger/ledger_test.go` — tests (P1T1, P1T2, P1T9, P1T11, P1T12)

**Decision Schema (from DESIGN.md Part 3):**
```go
type Decision struct {
    ID                 string        `json:"id"`
    Decision           string        `json:"decision"`
    Axioms             []string      `json:"axioms"`              // Required
    Evidence           string        `json:"evidence"`
    DecidedBy          string        `json:"decided_by"`
    Approver           string        `json:"approver"`
    Status             string        `json:"status"`              // PENDING|APPROVED
    Tags               []string      `json:"tags"`
    RiskLevel          string        `json:"risk_level"`          // trivial|minor|major|critical
    AffectedSystems    []string      `json:"affected_systems"`
    RelatedDecisions   []string      `json:"related_decisions"`
    RecordedAt         time.Time     `json:"recorded_at"`
    Signature          string        `json:"signature"`           // Ed25519 hex
    LedgerLine         int           `json:"ledger_line"`         // For audit trail
}
```

**Key Properties:**
- Append-only (fsync after every write)
- Hash-chained (include hash of previous entry)
- ACID guarantee (atomicity + durability)
- Queryable by: ID, axiom, tag, date range, risk level

**Success Criteria:**
- ☑ Ledger file created, append-only enforced (verify overwrite fails)
- ☑ Schema validation works (5 invalid decisions rejected)
- ☑ Duplicate detection works (same ID twice rejected)
- ☑ Ledger survives crash (durability test)
- ☑ Hash chain valid (each entry includes previous hash)

---

### Task 3: Axiom Validation Gate + Neo4j Integration
**What:** Wire portfolio to Neo4j. Before approving decision, validate axiom citations.

**Files to Create:**
- `internal/cache/neo4j.go` — Neo4j client, Cypher queries
- `internal/cache/divergence.go` — divergence detection + cache rebuild

**Validation Logic (from DESIGN.md Part 2, Flow 1, Step 2):**
```
Before writing to ledger:
  FOR each axiom_id in decision.axioms:
    QUERY Neo4j: MATCH (a:Axiom {id: $axiom_id}) RETURN a.id
    IF NOT found:
      REJECT decision with 400 "axiom {id} not found"
    IF found:
      Continue to next axiom
  IF all axioms valid:
    Proceed to signing + ledger write
```

**Neo4j Queries to Implement (from DESIGN.md Part 3):**
```cypher
# Query 1: Check axiom exists
MATCH (a:Axiom {id: $axiom_id}) RETURN a.id

# Query 2: Query decisions by axiom
MATCH (a:Axiom {id: $axiom_id})<-[:CITES]-(d:Decision)
RETURN d ORDER BY d.recorded_at DESC LIMIT $limit

# Query 3: Query decisions by tag
MATCH (d:Decision) WHERE $tag IN d.tags
RETURN d ORDER BY d.recorded_at DESC LIMIT $limit

# Query 4: Divergence detection
MATCH (d:Decision) RETURN count(d) as neo4j_count
# Compare with: wc -l < ledger.jsonl
```

**Indexes to Create:**
```cypher
CREATE INDEX idx_axiom_id FOR (a:Axiom) ON (a.id)
CREATE INDEX idx_decision_tags FOR (d:Decision) ON (d.tags)
CREATE INDEX idx_decision_risk FOR (d:Decision) ON (d.risk_level)
```

**Success Criteria:**
- ☑ Neo4j connection works (health check passes)
- ☑ Axiom validation gate rejects non-existent axioms
- ☑ Cache built from ledger (background job)
- ☑ Divergence detection works (detect when ledger > cache)
- ☑ Cache rebuild works (Neo4j offline → rebuild from ledger)
- ☑ Query performance SLAs met (axiom query P99 < 100ms, tag query P99 < 500ms)

---

### Task 4: Signing + Non-Repudiation
**What:** Sign every decision with Ed25519, enable verification and non-repudiation.

**Files to Create:**
- `internal/signing/signer.go` — Ed25519 signature generation
- `internal/signing/verifier.go` — signature verification
- `internal/signing/signing_test.go` — tests (P1T8)

**Implementation:**
```
Before ledger write:
  1. Serialize decision to JSON
  2. Sign with private key: Ed25519Sign(decision_json, private_key)
  3. Store signature hex in decision.signature
  4. Append to ledger with signature
  
On verification:
  1. Retrieve decision from ledger
  2. Retrieve signature
  3. Verify: Ed25519Verify(decision_json, signature, public_key)
  4. If valid: decision authentic (non-repudiation proved)
  5. If invalid: decision tampered (reject)
```

**Success Criteria:**
- ☑ Private/public key pair generated (stored securely)
- ☑ All decisions signed before ledger write
- ☑ Signature verification endpoint works (POST /api/v1/decisions/{id}/verify)
- ☑ Tampered decisions detected (signature fails verification)
- ☑ Non-repudiation test passes (decision maker can't deny)

---

### Task 5: REST API Endpoints (6 total)
**What:** Implement Portfolio API endpoints (from DESIGN.md Part 3, Endpoints 1-6)

**Files to Create:**
- `api/handlers.go` — HTTP handlers
- `api/decision.proto` — protocol definition (optional, if using protobuf)

**Endpoints to Implement:**

1. **POST /api/v1/decisions** (Decision 1)
   - Validation → Signing → Ledger write
   - Return: 201 {id, recorded_at, signature, ledger_line}
   - Errors: 400 (validation), 409 (duplicate), 503 (Neo4j down but ledger works)

2. **GET /api/v1/decisions/{id}** (Decision 2)
   - Retrieve from cache (fast) or ledger (fallback)
   - Return: 200 {decision}
   - Errors: 404 (not found), 202 (cache stale, Neo4j down)

3. **GET /api/v1/decisions?axiom=AX-001&limit=100** (Decision 3)
   - Query by axiom (indexed, fast)
   - Also support: ?tag=auth&risk_level=critical, ?decided_by=captain
   - Return: 200 [decisions]
   - Latency SLA: P99 < 100ms (axiom), < 500ms (tag)

4. **POST /api/v1/decisions/{id}/verify** (Decision 4)
   - Verify signature, detect tampering
   - Return: 200 {valid: bool}
   - Errors: 400 (invalid signature)

5. **POST /api/v1/decisions/log** (Decision 5, for Bridge/Orbit)
   - Systems log decisions back to portfolio
   - Request: {system, action, ticket_id, result, axioms_checked, evidence}
   - Return: 201 {decision_id, logged_at, signature}

6. **GET /api/v1/health** (Decision 6)
   - System health status
   - Return: 200 {status: "full|degraded", ledger, cache, ...}

**Success Criteria:**
- ☑ All 6 endpoints implemented
- ☑ All endpoints have proper error handling
- ☑ Latency SLAs met (P99: axiom < 100ms, tag < 500ms)
- ☑ Fallback works (Neo4j down → queries use ledger)
- ☑ End-to-end test passes (record → query → verify)

---

### Task 6: Coordination Layer (Bridge/Orbit Integration)
**What:** Enable Bridge and Orbit to log decisions back to Portfolio

**Implementation:**
- Task 5 endpoint 5 (POST /api/v1/decisions/log) handles this
- Bridge calls this endpoint after spawn execution
- Orbit calls this endpoint after verification
- Axioms calls this endpoint when discovering new axioms

**Success Criteria:**
- ☑ Bridge can POST to /api/v1/decisions/log
- ☑ Decisions logged include: system, action, result, axioms_checked
- ☑ Orbit can POST test results back
- ☑ Axioms can POST discovery notifications back

---

## Acceptance Criteria

Portfolio redesign is DONE when:

- ☑ Wayfinder structure (active-maps, completed-maps, template)
- ☑ ledger.jsonl queryable (ID, axiom, tag, date, risk)
- ☑ Neo4j axiom validation gate working
- ☑ Signature verification endpoint working
- ☑ All 6 API endpoints implemented + documented
- ☑ Bridge can log decisions via POST /api/v1/decisions/log
- ☑ Orbit can log test results via same endpoint
- ☑ All existing decisions have axiom citations (audit trail)
- ☑ Portfolio queryable (HTTP API working)
- ☑ Documentation complete (API docs, deployment, troubleshooting)

**All P1 Tests Pass (12/12):**
- P1T1: Ledger append-only
- P1T2: Schema validation
- P1T3: Axiom validation gate
- P1T4: Duplicate detection
- P1T5: Query by ID
- P1T6: Query by axiom (indexed, P99 < 100ms)
- P1T7: Query by tag (indexed, P99 < 500ms)
- P1T8: Signature verification
- P1T9: Ledger durability (crash recovery)
- P1T10: Full decision lifecycle
- P1T11: Ledger crash survival
- P1T12: Neo4j divergence recovery

---

## Owned by

Portfolio team (or captain if solo)

---

## Next

Once 201 complete:
- Bridge can read portfolio decisions (ticket 202 depends on this)
- Axioms can query portfolio for patterns (ticket 203 depends on this)
- Escalation endpoints ready for bridge to use
