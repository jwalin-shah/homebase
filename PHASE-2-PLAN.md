# Phase 2: Integration Testing - Staging Environment

**Date Started:** 2026-07-26  
**Duration:** 2-3 days  
**Objective:** Validate API contracts and end-to-end flows against real systems

---

## Deployment Target

**Environment:** Staging  
**Components:**
- HomeBase binary (built from Ticket 201 implementation)
- Neo4j instance (staging cluster)
- Ledger storage (filesystem, `/data/ledger.jsonl`)
- Bridge/Orbit integration points

---

## Test Scenarios

### 2.1 API Contract Validation (Real Systems)

**Test Set 1: Decision Recording**
- [ ] POST /api/v1/decisions with valid decision → 201 Created
  - Verify: signature generated and stored
  - Verify: decision appears in ledger
  - Verify: Neo4j cache updated (if online)
- [ ] POST with invalid JSON → 400 Bad Request
- [ ] POST with missing axioms → 400 Validation Failed
- [ ] POST with axiom not in Neo4j corpus → 400 Validation Failed (with Neo4j online)
- [ ] POST with duplicate ID → 409 Conflict

**Test Set 2: Decision Retrieval**
- [ ] GET /api/v1/decisions/{id} with valid ID → 200 with decision
- [ ] GET with nonexistent ID → 404 Not Found
- [ ] GET /api/v1/decisions (list) → 200 with array of decisions
- [ ] GET with query params (axiom filter) → filtered results (requires Neo4j)

**Test Set 3: Signature Verification**
- [ ] POST /api/v1/decisions/{id}/verify with correct signature → {valid: true}
- [ ] POST with tampered signature → {valid: false}
- [ ] POST with wrong signature format → 400 Bad Request

**Test Set 4: Bridge/Orbit Logging**
- [ ] POST /api/v1/decisions/log (from Bridge) → 201 Created
- [ ] POST /api/v1/decisions/log (from Orbit) → 201 Created
- [ ] Logged decisions appear in ledger with system tags

**Test Set 5: Health & Status**
- [ ] GET /api/v1/health → {status: "full"}
- [ ] GET /api/v1/health (Neo4j offline) → {status: "degraded", message: "Neo4j unavailable"}
- [ ] Metrics endpoint (if implemented) → response time, decision count, etc.

---

### 2.2 End-to-End Flow Validation

**Flow 1: Record → Verify → Query**
```
1. POST decision (axiom AX-001, evidence: "test evidence")
2. Verify signature returned in 201 response
3. GET decision by ID
4. Verify decision content matches original
5. Verify signature verifies
```

**Flow 2: Neo4j Graceful Degradation**
```
1. Record decision (Neo4j online)
2. Verify axiom validation passes
3. Stop Neo4j
4. Record another decision (same axiom)
5. Verify axiom check skipped (logged as WARNING)
6. Verify decision still recorded
7. Restart Neo4j
8. Verify cache rebuilt from ledger
```

**Flow 3: Ledger Durability**
```
1. Record decision D1
2. Verify in ledger file (fsync persisted)
3. Stop HomeBase process
4. Restart HomeBase
5. Verify D1 still retrievable
6. Verify hash chain intact
```

**Flow 4: Duplicate ID Rejection**
```
1. Record decision with ID "dec-test-001"
2. Attempt to record another decision with same ID
3. Verify rejection (409 Conflict)
4. Verify first decision still in ledger
5. Verify second decision NOT in ledger
```

---

### 2.3 Edge Cases & Failure Scenarios

**Edge Case 1: Large Decision Body**
```
- Decision text: 100KB+ 
- Axioms: 50+ axioms
- Evidence: detailed markdown
- Verify: API handles without truncation or error
```

**Edge Case 2: Concurrent Requests**
```
- 10 concurrent POST requests (unique IDs)
- Verify: all 10 recorded successfully
- Verify: no duplicates, no lost data
- Verify: performance acceptable (<100ms per request)
```

**Edge Case 3: Corrupted Ledger Recovery**
```
- Ledger has N valid entries
- Corrupt line N (invalid JSON)
- Restart HomeBase
- Verify: system handles gracefully (logs error, continues)
- Verify: entries 1..N-1 still queryable
```

**Edge Case 4: Neo4j Connection Timeout**
```
- Configure Neo4j with 1-second timeout
- Make connection hang
- Verify: HomeBase continues (timeout caught)
- Verify: axiom check skipped gracefully
```

**Edge Case 5: Disk Full Scenario**
```
- Fill disk to 95%
- Attempt to record decision
- Verify: error caught and reported
- Verify: system doesn't crash
- Verify: clear error message
```

---

### 2.4 Performance Baseline

**Metrics to Record:**
- Decision record latency (p50, p95, p99)
- Signature verification latency
- Ledger append throughput (decisions/second)
- Memory usage (baseline, peak, under load)
- Neo4j query latency for axiom validation
- Cache rebuild performance (time to rebuild from N entries)

**Load Profile:**
```
Scenario 1: Steady State
- 100 decisions recorded over 1 hour
- Expected: <50ms per decision, memory stable

Scenario 2: Burst
- 1000 decisions in 1 minute (10 concurrent connections)
- Expected: <200ms p95, memory spike <500MB

Scenario 3: Large Queries
- List all decisions (10K entries)
- Expected: <500ms response, memory manageable
```

---

## Testing Procedure

### Pre-Test Setup
```bash
1. Deploy HomeBase binary to staging
2. Generate signing keypair (save securely)
3. Ensure Neo4j staging cluster healthy
4. Prepare empty ledger file
5. Run health check: GET /api/v1/health
```

### During Testing
```bash
1. Run Test Set 1 (API contracts) - 30 min
2. Run Test Set 2 (retrieval) - 15 min
3. Run Test Set 3 (verification) - 15 min
4. Run Test Set 4 (Bridge/Orbit) - 20 min
5. Run Test Set 5 (health) - 10 min
6. Run Flow 1-4 (end-to-end) - 45 min
7. Run Edge Cases 1-5 - 60 min
8. Run Performance Baseline - 45 min
```

### Post-Test Analysis
```
- Collect metrics and logs
- Identify any failures or performance issues
- Compare to acceptance criteria
- Prepare report for next phase decision
```

---

## Acceptance Criteria

**Go/No-Go Decision:**

✅ **GO to Phase 3** if:
- [ ] All API endpoints return correct status codes
- [ ] End-to-end flows complete without errors
- [ ] Performance: decision latency <100ms p95
- [ ] Graceful degradation works (Neo4j unavailable)
- [ ] Ledger durability verified (survives restart)
- [ ] Concurrent requests handled correctly
- [ ] No data loss under any tested scenario

❌ **NO-GO to Phase 3** if:
- [ ] Any critical failure (data loss, crash, corruption)
- [ ] Performance unacceptable (>500ms p95)
- [ ] Graceful degradation fails
- [ ] Security validation fails (signature verification broken)

---

## Tickets & Dependencies

**Blocking:**
- Ticket 201: Implementation COMPLETE ✓
- Neo4j staging cluster: Available
- Bridge/Orbit staging endpoints: Available

**Unblocking:**
- Phase 3 work (Tickets 202-203): Blocked until Phase 2 passes

---

## Resources

**Code:**
- Binary: `/Users/jwalinshah/projects/homebase/cmd/homebase/main.go`
- API: `/Users/jwalinshah/projects/homebase/api/`
- Tests: `17 unit tests passing`

**Configuration:**
- Neo4j URI: `neo4j://staging.internal:7474` (set via flag)
- Ledger path: `/data/ledger.jsonl` (configurable)
- Listen address: `:8080` (configurable)

**Contacts:**
- Captain: jwalinshah13@gmail.com
- Neo4j Admin: (staging cluster)
- Bridge/Orbit Integration: (team contact)

---

**Status:** Ready to Deploy  
**Next Milestone:** Phase 2 Results Report
