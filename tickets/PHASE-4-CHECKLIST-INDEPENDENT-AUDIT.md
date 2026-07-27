# Phase 4 Checklist: Independent Audit

**Goal:** Find what all previous phases missed. Challenge assumptions. Stress-test invariants.

**Owner:** Independent Auditor (conducts review), Architect (gates Phase 5)

**Authority:** Auditor is adversarial reviewer, assumes implementation has bugs until proven otherwise.

**Blocking:** No Phase 5 (Production Readiness) starts until audit is complete and findings are ≤ acceptance threshold.

---

## Independent Auditor Checklist

### Pre-Audit Verification (Mandatory)

Before starting audit, verify prior phases actually completed:

- [ ] Phase 0 (Specification) is complete and signed off
- [ ] Phase 1 (Implementation) is complete and signed off
- [ ] Phase 2 (Unit Tests) is complete and signed off
- [ ] Phase 3 (Integration Tests) is complete and signed off
- [ ] PHASE-SIGN-OFF-SHEET.md shows all roles signed each phase
- [ ] Zero blockers noted from previous phases

**If any phase is incomplete:** STOP. Send back to respective team with specific gaps.

### Specification Audit (Re-read Phase 0)

- [ ] Read entire Phase 0 specification
- [ ] For each use case, trace implementation end-to-end
  - Does the code actually implement the use case?
  - Does it handle the error path?
  - Are there edge cases not covered?

**Verification Method:**
```
For each use case in spec:
  1. Identify which code path(s) implement it
  2. Read that code start-to-finish
  3. Ask: "Does this code do what spec says?"
  4. Ask: "What could go wrong here?"
  5. Check: are there tests for this?
```

### Critical Invariants Audit (Deep Dive)

These should have been checked in Phase 1, but verify they actually work:

#### Invariant 1: Immutability
**Claim:** Once written to ledger, decisions cannot be changed.

- [ ] Read ledger.Append() - is it "add only" with no Update()?
- [ ] Check: can any code path call DELETE on ledger?
- [ ] Verify: ledger file permissions don't allow truncation
- [ ] Test: try to modify a decision in ledger, verify rejection

**If failed:**
```
CRITICAL: Invariant violated. Audit trail can be altered retroactively.
Impact: Compliance failure, non-repudiation broken.
```

#### Invariant 2: Escalation Idempotency (≤1 approval)
**Claim:** Escalation can be approved at most once.

- [ ] Read ApproveEscalation handler
- [ ] Check: does it verify escalation status before approving?
- [ ] Check: is there mutual exclusion (mutex/lock) preventing race?
- [ ] Stress test: simulate 5 concurrent approvals on same escalation
  - Should result in: 1 success (201/200) + 4 conflicts (409)
  - Not: 5 successes

**If failed:**
```
CRITICAL: Escalation can be approved multiple times.
Impact: Duplicate Bridge calls, decision analysis run multiple times, data corruption.
```

#### Invariant 3: Durability (fsync)
**Claim:** Data written to ledger persists on disk, survives process crash.

- [ ] Check: ledger.Append() calls fsync() or equivalent?
- [ ] Check: writes are synchronous (not buffered/async)?
- [ ] Verify: no writes cached in memory without disk sync
- [ ] Test: write decision, kill process, restart, verify decision still there

**If failed:**
```
CRITICAL: Data loss on process crash.
Impact: Decisions vanish, audit trail incomplete, compliance failure.
```

#### Invariant 4: Integrity (Signature Verification)
**Claim:** Decisions are signed, signatures are verified on read.

- [ ] Check: every decision write includes signature?
- [ ] Check: decision read verifies signature before returning?
- [ ] Test: modify decision in ledger file, verify rejection on read
- [ ] Test: load decision with wrong signature, verify error

**If failed:**
```
CRITICAL: Tampered data accepted.
Impact: Audit trail can be forged, non-repudiation fails.
```

#### Invariant 5: Correlation ID Tracking
**Claim:** Every request has a correlation ID that flows through entire system.

- [ ] Check: getCorrelationID() generates if missing?
- [ ] Check: correlation ID is validated (length, charset)?
- [ ] Check: correlation ID is logged on every operation?
- [ ] Trace: one request end-to-end, verify ID appears in all logs

**If failed:**
```
HIGH: Requests not traceable through system.
Impact: Debugging production issues becomes impossible.
```

### Code Security Audit

#### Authentication & Authorization
- [ ] Every endpoint requires authentication? (API key, Bearer token, or public with rate limiting)
- [ ] Public endpoints have rate limiting to prevent DoS?
- [ ] Sensitive operations (approve, delete, rebuild cache) require specific permissions?
- [ ] Can unauthenticated user query all decisions?

**Stress test:**
```bash
# Try to approve escalation without auth
curl -X POST http://localhost:8080/api/v1/escalations/esc-001/approve \
  -H "Content-Type: application/json" \
  -d '{"approved": true}'
# Should get 401 Unauthorized, not 200 OK
```

**If failed:**
```
CRITICAL: Unauthenticated access to sensitive operations.
Impact: Data breach, decisions can be modified by anyone.
```

#### Input Validation
- [ ] Query parameters validated (length, charset, SQL injection resistance)?
- [ ] Request body validated (schema, type, size)?
- [ ] File paths don't allow path traversal (../../etc/passwd)?
- [ ] Numeric inputs don't overflow (timestamp overflows)?

**Stress test:**
```bash
# Try oversized input
curl -X GET "http://localhost:8080/api/v1/decisions?axiom=$(python3 -c 'print("A"*10000)')"
# Should get 400 Bad Request, not 500 or crash

# Try special characters
curl -X GET "http://localhost:8080/api/v1/decisions?axiom=AX-INJECT%0A%0AAlert%20hack"
# Should reject, not log-inject
```

**If failed:**
```
HIGH: Injection attacks possible.
Impact: Log injection, DoS, unauthorized access.
```

#### Secrets & Logging
- [ ] No secrets in logs (API keys, tokens, private data)?
- [ ] Sensitive data (evidence, Bridge analysis) marked for redaction?
- [ ] Logs don't leak internal structure (database names, versions)?
- [ ] Log output goes to stderr, not stdout?

**Verify:**
```bash
grep -i "password\|token\|secret\|key" *.log | head
# Should be empty or redacted
```

**If failed:**
```
HIGH: Credential leakage.
Impact: Secrets exposed in logs, potential compromise.
```

### Data Consistency Audit

#### Ledger vs Cache (Neo4j)
- [ ] Ledger is source of truth?
- [ ] Neo4j cache can be rebuilt from ledger?
- [ ] Cache rebuild is consistent (count before clear = count after rebuild)?
- [ ] Queries against cache return same results as ledger?

**Stress test:**
```bash
# Record 10 decisions with axiom AX-TEST
# Query ledger for AX-TEST decisions: should be 10
# Query Neo4j for AX-TEST decisions: should be 10
# Rebuild cache
# Query Neo4j again: should still be 10
```

**If failed:**
```
CRITICAL: Ledger and cache diverge.
Impact: Queries return incomplete/inconsistent results.
```

#### Timestamp Ordering
- [ ] Decisions recorded with monotonically increasing timestamps?
- [ ] Escalations recorded in order they were created?
- [ ] Approvals recorded after escalations?
- [ ] Can we reconstruct decision timeline?

**Verify:**
```bash
# Read ledger, extract timestamps, verify sorted
jq -r '.recorded_at' ledger.jsonl | sort -c
# Should succeed (0 exit code)
```

**If failed:**
```
HIGH: Timeline reconstruction impossible.
Impact: Audit trail events out of order, misleading.
```

### Performance & Scale Audit

#### Query Performance
- [ ] Queries on large datasets (100K+ decisions) complete in <1 second?
- [ ] Query results have limits to prevent memory exhaustion?
- [ ] Indices exist on frequently queried fields?
- [ ] Database not scanning full table for every query?

**Stress test:**
```bash
# Add 100K decisions to Neo4j
# Query for popular axiom (cited by 50K decisions)
# Measure latency: should be <1s
# Measure result size: should be capped (not 50K rows)
```

**If failed:**
```
HIGH: Performance degrades with scale.
Impact: Year-2 targets (100K decisions/day) will cause timeouts.
```

#### Resource Usage
- [ ] Memory usage stable under load (no memory leaks)?
- [ ] Goroutines don't multiply infinitely (no goroutine leaks)?
- [ ] Database connections are pooled/limited (not unlimited)?
- [ ] Ledger file doesn't grow unbounded?

**Stress test:**
```bash
# Record 1000 decisions rapidly
# Check memory: should not grow 10x
# Check goroutines: should return to baseline
# Check connections: should be ≤ pool size
```

**If failed:**
```
HIGH: Resource exhaustion under load.
Impact: System crashes after days/weeks of normal usage.
```

### Failure Mode Analysis

For each integration point, ask: "What if it fails?"

#### If Neo4j is down
- [ ] System still works?
- [ ] Decisions can still be recorded?
- [ ] Queries degrade gracefully (e.g., linear scan or error message)?
- [ ] No cascade (Neo4j down doesn't take down entire system)?

**Verify:**
```bash
# Stop Neo4j
# Try to record decision: should succeed
# Try to query by axiom: should fail gracefully (not panic)
# Restart Neo4j, rebuild cache: should work
```

#### If Bridge is down
- [ ] Escalations can be created (marked PENDING)?
- [ ] System doesn't block waiting for Bridge?
- [ ] Retry mechanism exists for failed calls?
- [ ] No infinite retry loops?

**Verify:**
```bash
# Mock Bridge to timeout
# Create escalation: should return 202 ACCEPTED
# Check status: should be PENDING_BRIDGE_RESPONSE
# Escalation should be retryable later
```

#### If Ledger file is read-only
- [ ] System detects the issue?
- [ ] Error is clear (not silently failing)?
- [ ] Operations fail fast (not after minutes of retrying)?

**Verify:**
```bash
# Make ledger file read-only: chmod 444 ledger.jsonl
# Try to record decision: should get error
# Error should say "permission denied" or "read-only"
```

### Test Coverage Audit

- [ ] Unit test coverage >80% (already verified in Phase 2)?
- [ ] Critical paths have integration tests?
- [ ] Error paths are tested (not just happy path)?
- [ ] Edge cases tested (empty inputs, large inputs, concurrent operations)?
- [ ] Tests are deterministic (no flakes)?

**Stress test:**
```bash
# Run full test suite 5 times
# All tests pass every time? (0 flakes)
# If any flake: block Phase 5
```

### Operational Readiness

- [ ] Deployment procedure documented?
- [ ] Monitoring/alerting configured?
- [ ] Runbooks exist for common issues (Neo4j down, ledger corrupt)?
- [ ] Backup/restore procedure tested?
- [ ] Graceful shutdown implemented (close connections, flush buffers)?

**Verify:**
```bash
# Read deployment docs: can someone who's never deployed this do it?
# Check monitoring: are CRITICAL errors alerting?
# Test backup/restore: can we recover from ledger corruption?
```

---

## Audit Findings Document

For each finding, create entry in `AUDIT-FINDINGS-[TICKET]-PHASE-4.md`:

```markdown
## Finding [N]: [Title]

**Severity:** [CRITICAL/HIGH/MEDIUM/LOW]

**Category:** [Security/Performance/Correctness/Operational]

**Description:** [What's wrong]

**Failure Scenario:** [Concrete example: "If admin tries to..., then system..."]

**Impact:** [What breaks: compliance, security, availability, correctness]

**Current Code:** [Line number and snippet]

**Root Cause:** [Why does this exist? Design flaw? Oversight? Missed in Phase 1?]

**Fix:** [How to remediate]

**Verification:** [How to test that fix works]

**Notes:** [Any caveats or follow-ups]
```

---

## Acceptance Criteria

Audit is COMPLETE and Phase 5 can proceed when:

| Severity | Max Findings | Status |
|----------|---|---|
| CRITICAL | 0 | ✓ Required |
| HIGH | ≤3 | ✓ Required (with mitigation plan) |
| MEDIUM | ≤10 | ✓ Required |
| LOW | Unlimited | ✓ Can defer to Phase 5+ |

**Example acceptable audit result:**
```
CRITICAL: 0 ✓
HIGH: 2 (both have mitigation) ✓
MEDIUM: 5 ✓
LOW: 8 (defer to next iteration)

Verdict: APPROVED FOR PHASE 5
```

**Example unacceptable result:**
```
CRITICAL: 1 (escalation approved twice)
HIGH: 4

Verdict: BLOCKED - Send back to Phase 1 for fixes
```

---

## Auditor Sign-Off

**Audit Start Date:** ___________  
**Audit End Date:** ___________

### Findings Summary

| Severity | Count | Status |
|----------|---|---|
| CRITICAL | ___ | [ ] Acceptable (0) [ ] Unacceptable (>0) |
| HIGH | ___ | [ ] Acceptable (≤3) [ ] Unacceptable (>3) |
| MEDIUM | ___ | [ ] Acceptable (≤10) [ ] Unacceptable (>10) |
| LOW | ___ | [ ] Deferrable |

**Overall Verdict:**  
[ ] APPROVED - Phase 5 can proceed  
[ ] APPROVED WITH CONDITIONS - Phase 5 can proceed after mitigation plan  
[ ] BLOCKED - Send back to Phase 1 for fixes

**Auditor Signature:** _____________________ **Date:** _________

**Architect Approval:** _____________________ **Date:** _________

---

## Common Phase 4 Findings (From Previous Audits)

**CRITICAL:**
- [ ] Escalation can be approved multiple times (race condition)
- [ ] Decisions can be modified after creation (immutability broken)
- [ ] Tampered data accepted (signature verification skipped)
- [ ] Data loss on crash (no fsync)
- [ ] Unauthorized access to sensitive operations (no auth)

**HIGH:**
- [ ] Correlation ID not validated (injection attacks)
- [ ] Input not validated (DoS via oversized input)
- [ ] External service timeout not set (hangs indefinitely)
- [ ] Secrets in logs (credential leakage)
- [ ] Queries unbounded (100K rows returned, memory exhaustion)

**MEDIUM:**
- [ ] Health check pollutes ledger (noise in audit trail)
- [ ] Hardcoded config (can't deploy to new environment)
- [ ] Test flakes (non-deterministic failures)
- [ ] Goroutine leaks (resource exhaustion over time)
- [ ] Cache diverges from ledger (inconsistent queries)

---

**Phase 4 typically takes 4-6 hours per ticket.**  
If audit takes <2 hours, you probably missed something.
