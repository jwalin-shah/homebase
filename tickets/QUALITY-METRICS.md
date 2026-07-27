# Quality Metrics Dashboard

**Purpose:** Measure effectiveness of Phase Enforcement Framework. Answer: "Are we shipping better software?"

**Audience:** Captain, Engineering Leadership, Process Improvement Team

**Update Frequency:** After each ticket completes Phase 4 audit

---

## Metric 1: Issues Found Per Phase

**Question:** Where are issues being caught?

**Why It Matters:** Shifting issues left (find in Phase 1-2, not Phase 4) means:
- Faster iteration (fix early, not late)
- Lower cost (1 hour fix in Phase 1 vs 4-hour fix in Phase 4)
- Better planning (Phase 4 audit catches only unknown unknowns)

**How to Measure:**

For each ticket, count issues found in each phase:

```
Ticket 202 (Bridge Integration):
  Phase 0: 0 issues (spec was clear)
  Phase 1: 0 issues (implementation was correct)
  Phase 2: 0 issues (tests caught everything)
  Phase 3: 2 issues (integration revealed problems)
  Phase 4: 12 issues (audit found race condition, validation gaps, etc.)
  ————————————————
  Total: 14 issues
  
Ticket 203 (Neo4j Querying):
  Phase 0: 1 issue (axiom query performance spec unclear)
  Phase 1: 2 issues (index creation missing)
  Phase 2: 0 issues
  Phase 3: 1 issue (pagination missing)
  Phase 4: 3 issues
  ————————————————
  Total: 7 issues
```

**Target (Year 1):**
```
Phase 0: 0-5 issues (design gaps)
Phase 1: 0-3 issues (implementation bugs)
Phase 2: 0-5 issues (test gaps)
Phase 3: 0-3 issues (integration problems)
Phase 4: ≤3 CRITICAL + ≤3 HIGH (unknown unknowns, adversarial finds)
—————————————————
Total: ≤20 issues per ticket
```

**Target (Year 2, after process optimization):**
```
Phase 0: 0-2 issues
Phase 1: 0-1 issues
Phase 2: 0-2 issues
Phase 3: 0-1 issues
Phase 4: ≤1 CRITICAL + ≤2 HIGH
—————————————————
Total: ≤8 issues per ticket
```

**Dashboard Entry:**

| Ticket | Phase 0 | Phase 1 | Phase 2 | Phase 3 | Phase 4 | Total | Trend |
|--------|---------|---------|---------|---------|---------|-------|-------|
| 202 (Bridge) | 0 | 0 | 0 | 2 | 12 | 14 | ↗ (HIGH, needs investigation) |
| 203 (Neo4j) | 1 | 2 | 0 | 1 | 3 | 7 | ↓ (Better than 202) |
| 204 (Testing) | ? | ? | ? | ? | ? | ? | TBD |
| 205 (Observability) | ? | ? | ? | ? | ? | ? | TBD |

---

## Metric 2: Issue Severity Distribution

**Question:** Are we finding critical issues before production?

**Why It Matters:** Severity is the real measure. 20 LOW issues < 1 CRITICAL issue.

**How to Measure:**

After Phase 4 audit, count by severity:

```
Ticket 202:
  CRITICAL: 2 (escalation race condition, health check corruption)
  HIGH: 6 (auth missing, rate limiting missing, validation gaps)
  MEDIUM: 4 (logging to stdout, correlation ID not validated, etc.)
  LOW: 2
  ————————
  Total: 14
  
Ticket 203:
  CRITICAL: 0 ✓
  HIGH: 2 (transactional consistency issue, hardcoded limit)
  MEDIUM: 3
  LOW: 2
  ————————
  Total: 7
```

**Target (Year 1):**
```
CRITICAL found: 0 (all critical issues prevented by design/review)
HIGH found: ≤3 per ticket
MEDIUM found: ≤10 per ticket
LOW found: Unrestricted
```

**Dashboard Entry:**

| Ticket | CRITICAL | HIGH | MEDIUM | LOW | Total | Pass? |
|--------|----------|------|--------|-----|-------|-------|
| 202 | 2 | 6 | 4 | 2 | 14 | ✗ (2 CRITICAL) |
| 203 | 0 | 2 | 3 | 2 | 7 | ✓ |
| 204 | ? | ? | ? | ? | ? | ? |
| 205 | ? | ? | ? | ? | ? | ? |

---

## Metric 3: Rework Rate (Phase 4 Escape Analysis)

**Question:** How many issues found in Phase 4 should have been caught in Phase 1-3?

**Why It Matters:** If 90% of Phase 4 issues are "obvious bugs that should be in Phase 1 checklist," the process needs adjustment.

**How to Measure:**

For each Phase 4 finding, categorize:

```
Category A: "This should have been caught in Phase 1" (checklist gap)
Category B: "This should have been caught in Phase 2" (test gap)
Category C: "This should have been caught in Phase 3" (integration gap)
Category D: "This is adversarial/unknown, hard to predict" (acceptable)

Ticket 202 Phase 4 issues (12 total):
  A (Phase 1 gap): 8 issues
    - rand.Read() error not checked → should be in Phase 1 checklist
    - Correlation ID not validated → should be in Phase 1 checklist
    - No auth middleware → should be in Phase 1 checklist
    - No rate limiting → should be in Phase 1 checklist
    - Health check pollutes ledger → should be in Phase 1 checklist
    - Hardcoded test IDs → should be in Phase 2 test checklist
    - Test isolation bug (shared :memory:) → should be in Phase 2 checklist
    - Race condition in approval → should be in Phase 2 (concurrent test)
    
  B (Phase 2 gap): 2 issues
    - Query result unbounded → should be in integration test
    - Transactional consistency issue → should be in integration test
    
  C (Phase 3 gap): 1 issue
    - Ledger vs Neo4j divergence → integration test should verify
    
  D (Adversarial/unknown): 1 issue
    - Correlation ID collision under extreme load → acceptable adversarial find
    
Rework Rate = 10/12 = 83% (Issues that should have been caught)
```

**Target (Year 1):**
```
Rework Rate: ≤50% (aim to catch in Phase 1-3, not Phase 4)
```

**Target (Year 2):**
```
Rework Rate: ≤20% (only adversarial/truly unknown issues escape)
```

**Dashboard Entry:**

| Ticket | Total Issues | Should Be Phase 1-3 | Adversarial Finds | Rework Rate | Target |
|--------|--------------|-----------------|-------------------|------------|--------|
| 202 | 14 | 10 | 1 | 83% | ≤50% |
| 203 | 7 | 4 | 1 | 57% | ≤50% |
| 204 | ? | ? | ? | ? | ≤50% |
| 205 | ? | ? | ? | ? | ≤50% |

---

## Metric 4: Test Coverage

**Question:** Do we have adequate test coverage to prevent bugs?

**Why It Matters:** Coverage >80% correlates with fewer Phase 4 issues.

**How to Measure:**

After Phase 2 completes, capture:

```bash
go test ./... -cover
# Output: coverage: 87.3% of statements

# By package:
internal/ledger: 92%
internal/cache: 78%
internal/validation: 85%
api: 81%
```

**Target (Year 1):**
```
Overall coverage: ≥80%
Critical paths (ledger, signing, escalation): ≥90%
Non-critical paths (logging, metrics): ≥70%
```

**Dashboard Entry:**

| Ticket | Overall | Ledger | Signing | Escalation | Validation | Cache | Pass? |
|--------|---------|--------|---------|------------|------------|-------|-------|
| 202 | 87% | 95% | 93% | 88% | 85% | 79% | ✓ |
| 203 | ? | ? | ? | ? | ? | ? | ? |
| 204 | ? | ? | ? | ? | ? | ? | ? |
| 205 | ? | ? | ? | ? | ? | ? | ? |

---

## Metric 5: Test Flakiness

**Question:** Can we trust the test results?

**Why It Matters:** Flaky tests hide real bugs. If tests pass/fail randomly, no one believes them.

**How to Measure:**

Run full test suite 5 times consecutively:

```bash
for i in {1..5}; do
  echo "Run $i:"
  go test ./... 2>&1 | grep -E "^(ok|FAIL)" | sort | uniq -c
done

# Expected: identical output all 5 runs
# If different: you have flaky tests
```

**Flake Report:**
```
Test: TestApproveEscalation_DoubleApproval
  Run 1: PASS
  Run 2: FAIL (race condition detected)
  Run 3: PASS
  Run 4: PASS
  Run 5: FAIL
  
Flakiness: 40% (flakes 2/5 runs)
Root cause: Shared :memory: ledger across tests (test isolation bug)
```

**Target (Year 1):**
```
Flaky test rate: 0% (100% deterministic)
```

**Dashboard Entry:**

| Ticket | Flaky Tests | Rate | Root Cause | Status |
|--------|------------|------|------------|--------|
| 202 | 3 | 3/165 = 1.8% | test isolation (:memory:), hardcoded test IDs | ✗ (should be 0%) |
| 203 | 0 | 0% | — | ✓ |
| 204 | ? | ? | ? | ? |
| 205 | ? | ? | ? | ? |

---

## Metric 6: Sign-Off Completeness

**Question:** Are all roles actually signing off, or are we skipping steps?

**Why It Matters:** If Architect isn't reviewing Phase 1, we lose gate. If Auditor skips certain checks, we miss bugs.

**How to Measure:**

For each ticket, check PHASE-SIGN-OFF-SHEET.md:

```
Ticket 202:
  Phase 0: Discovery Lead ✓ Architect ✓
  Phase 1: Implementation ✓ Architect ✓
  Phase 2: Tester ✓ Architect ✓
  Phase 3: Tester ✓ Architect ✓
  Phase 4: Auditor ✓ Architect ✓
  
Ticket 203:
  Phase 0: Discovery Lead ✓ Architect ✓
  Phase 1: Implementation ✓ Architect ✗ (Missing!)
  Phase 2: Tester ✓ Architect ✓
  ...
```

**Target (Year 1):**
```
All phases: 100% sign-off completeness
If any role missing: Block next phase, send back.
```

**Dashboard Entry:**

| Ticket | Phase 0 | Phase 1 | Phase 2 | Phase 3 | Phase 4 | Completeness |
|--------|---------|---------|---------|---------|---------|--------------|
| 202 | ✓ | ✓ | ✓ | ✓ | ✓ | 100% |
| 203 | ✓ | ✗ | ✓ | ✓ | ✓ | 80% |
| 204 | ? | ? | ? | ? | ? | ? |
| 205 | ? | ? | ? | ? | ? | ? |

---

## Master Dashboard (Aggregate)

**Updated:** Every Friday EOD

```
╔════════════════════════════════════════════════════════════════╗
║                  HomeBase Quality Metrics                      ║
║                   Period: 2026-07-20 to 2026-07-27             ║
╚════════════════════════════════════════════════════════════════╝

TICKETS IN FLIGHT:
  ✓ 202 (Bridge Integration) — Phase 4 complete, 14 issues found
  ✓ 203 (Neo4j Querying) — Phase 4 complete, 7 issues found
  ⧗ 204 (Integration Testing) — Phase 2 in progress
  ⧗ 205 (Observability) — Phase 1 in progress

KEY METRICS:

1. ISSUES FOUND PER PHASE
   Target: ≤20/ticket, shift left to Phase 1-3
   Status: 202=14, 203=7, 204=?, 205=?
   Trend: ↑ (Need to improve Phase 1 checklist)

2. SEVERITY DISTRIBUTION
   Target: 0 CRITICAL, ≤3 HIGH
   Status: 202=2 CRITICAL/6 HIGH (FAILS), 203=0/2 (PASS)
   Trend: ↗ (Too many critical escaping Phase 1)

3. REWORK RATE
   Target: ≤50% of Phase 4 issues should come from Phase 1-3 gaps
   Status: 202=83% (EXCEEDS TARGET), 203=57% (EXCEEDS TARGET)
   Trend: ↗ (Checklists need refinement)

4. TEST COVERAGE
   Target: ≥80%
   Status: 202=87% (PASS), 203=? (PENDING)
   Trend: ✓

5. FLAKINESS
   Target: 0% (100% deterministic)
   Status: 202=1.8% (FAILS), 203=0% (PASS)
   Trend: ↓ (Fixed test isolation, now tracking)

6. SIGN-OFF COMPLETENESS
   Target: 100% all phases signed
   Status: 202=100%, 203=80% (missing Architect Phase 1)
   Trend: → (Mostly compliant)

ACTION ITEMS:
  □ Captain: Review Phase 1 checklist — too many issues escaping
  □ Implementation: Run flake tests on remaining tickets before Phase 4
  □ Architect: Enforce sign-off on 203 Phase 1
  □ Auditor: Prepare for 204/205 Phase 4 audits

NEXT REVIEW: 2026-08-03 EOD
```

---

## How to Use This Dashboard

**Captain's View (Strategic):**
1. Check "Action Items" first
2. Look at "Severity Distribution" — any CRITICAL issues blocking production?
3. Look at "Rework Rate" — are we shifting issues left, or still finding in Phase 4?
4. Look at trends — improving or degrading?

**Process Improvement View (Tactical):**
1. If Rework Rate is high, which phase checklist needs work?
2. If Flakiness is high, which test suite needs isolation fixes?
3. If Coverage is low, which packages need more tests?
4. Compare Ticket 202 vs 203 — what did 203 do better?

**Architect's View (Per-Ticket):**
1. Before approving Phase advance, check "Sign-Off Completeness" for that ticket
2. Before audit starts, verify Rework Rate for this ticket (expected ≤50%)
3. After audit, add findings to this dashboard

---

## Long-Term Targets

**By 2026-12-31 (Year 1 end):**
```
Issues per ticket: ≤10 (down from 14)
CRITICAL found: 0 (all caught in Phase 1-2)
Rework rate: ≤30% (most issues caught early)
Test coverage: ≥85% overall
Flakiness: 0%
Sign-off: 100% compliance
```

**By 2027-06-30 (Year 2 mid):**
```
Issues per ticket: ≤5 (design excellence)
CRITICAL found: 0 (consistently)
Rework rate: ≤20% (only adversarial finds)
Test coverage: ≥90% overall
Flakiness: 0% maintained
Sign-off: 100% sustained
```

---

**Dashboard Maintainer:** Process Owner  
**Update Cadence:** After each ticket Phase 4 completion  
**Review Cadence:** Weekly with Captain  
**Archive:** Old dashboards at `metrics/archive/`
