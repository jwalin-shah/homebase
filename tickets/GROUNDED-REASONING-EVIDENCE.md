# Grounded Reasoning: Evidence-Based Phase Completion

**Principle:** No claim without proof. Every phase signs off with evidence collected, verified, and committed to git.

**Inspired by:** Sentient Arena's grounded reasoning approach—agents prove their work, not just claim it.

---

## The Problem (Without Evidence)

**Claim-based sign-off (broken):**
```
TICKET-202, Phase 1: ✓ COMPLETE
Signed by: Implementation Team
Date: 2026-07-27

What did they do? Unknown.
Did code actually compile? Unknown.
How many tests pass? Unknown.
Are there hidden bugs? Unknown.

Later, Phase 4 audit finds 12 issues.
Question: "Where was Phase 1 verification?"
Answer: "There wasn't any, just a signature."
```

**Evidence-based sign-off (fixed):**
```
TICKET-202, Phase 1: ✓ COMPLETE
Signed by: Implementation Team
Evidence: PHASE-1-EVIDENCE-202.md

Evidence includes:
  • Code compiles: ✓ (go build ./... exit code 0)
  • Lint passes: ✓ (golangci-lint 0 errors)
  • Tests pass: ✓ (165 tests, all passing)
  • Coverage: ✓ (87%, exceeds 80% target)
  • Errors handled: ✓ (grep scan, 0 violations)
  • Flakiness: ✓ (5 runs, all identical)

Later, Phase 4 audit finds issues? Can ask:
  "Why did Phase 1 evidence not catch this?"
  Answer: Verifiable (look at PHASE-1-EVIDENCE-202.md)
```

---

## Three Types of Evidence

### Type 1: Automated Evidence (Greppable, Measurable)

**What it is:** Output from automated checks that prove requirements were met.

**Examples:**
```bash
# Evidence: Code compiles
$ go build ./...
$ echo $?
0  # ← Evidence: exit code 0 means success

# Evidence: Tests pass
$ go test ./...
ok    homebase/api        0.523s
ok    homebase/internal   0.891s
# ← Evidence: "ok" status means all tests passed

# Evidence: Coverage
$ go test ./... -cover
coverage: 87.3% of statements
# ← Evidence: 87.3% ≥ 80% target

# Evidence: No unhandled errors
$ grep "_ = " *.go | wc -l
0  # ← Evidence: zero unhandled errors
```

**Stored in evidence file:**
```markdown
### Automated Evidence

- **Code compiles:** ✓ (go build ./... exit code 0)
- **Tests pass:** ✓ (165 tests passing)
- **Coverage:** ✓ (87.3% ≥ 80% target)
- **Errors handled:** ✓ (0 unhandled errors detected)
```

### Type 2: Human Evidence (Judgement Call, Documented)

**What it is:** Human review that verifies design/implementation matches spec.

**Examples:**
```markdown
### Human Review Evidence

**Architect reviewed implementation against spec:**
- ✓ All handlers from spec are implemented
- ✓ Data structures match design sketch
- ✓ Integration points have graceful fallbacks
- ✓ Error paths are guarded
- Notes: "Implementation matches spec. No deviations found."
  Reviewed by: alice@example.com
  Date: 2026-07-27
```

### Type 3: Structural Evidence (Design, Contracts)

**What it is:** Code patterns that prove invariants are enforced.

**Examples:**
```go
// Evidence: Signature verification enforced
type BridgeSignature string  // Sealed type prevents misuse

func (v *Verifier) Verify(response *BridgeResponse, sig BridgeSignature) error {
    // Implementation MUST verify sig
    return v.verify(...)
}

// ← This type system structure is evidence that signatures are validated
```

**Stored in evidence file:**
```markdown
### Structural Evidence

**Signature Verification:**
- Type-sealed BridgeSignature (prevents unsigned use)
- Verify() method required before accepting responses
- Handler calls verify() before recording
- ← Code structure proves invariant is enforced
```

---

## Evidence for Each Phase

### Phase 0: Specification Evidence

**What evidence proves Phase 0 is complete:**

```markdown
# Phase 0 Evidence Report

## Specification Completeness

✓ Spec file exists: TICKET-202-PHASE-0-SPECIFICATION.md
✓ Size: 2500 words (target: 200-500 words) [or too long, needs trimming]
✓ Required sections:
  - Problem statement: Present
  - Use cases (3+): 5 use cases documented
  - Constraints: Performance, scale, security documented
  - Risks (3+): 5 major risks identified with impact levels
  - Assumptions (5+): 8 assumptions documented
  
## Risk Assessment

Risks documented: 5 (target: ≥3)
- Risk 1: Bridge LLM analysis unreliable (Impact: HIGH)
- Risk 2: Neo4j diverges from ledger (Impact: HIGH)
- Risk 3: Ledger file corruption (Impact: CRITICAL)
- [...]

Mitigations identified: ✓ (each risk has mitigation strategy)

## Architecture Clarity

✓ High-level architecture sketched
✓ Data flow documented (decision → ledger → Neo4j → query)
✓ Integration points identified (Bridge, Neo4j, logging)
✓ Architect can implement from this spec: YES

## Sign-Off Gate

This evidence proves Phase 0 is complete.
Required before Phase 1 can start:

- [ ] Discovery Lead confirms spec is complete
- [ ] Architect confirms spec is buildable
- [ ] Both have reviewed evidence above
```

### Phase 1: Implementation Evidence

**What evidence proves Phase 1 is complete:**

```markdown
# Phase 1 Evidence Report

## Code Compilation

✓ go build ./... exit code: 0
✓ No compile errors
✓ Tested: 2026-07-27 14:23 UTC

## Code Quality

✓ gofmt check: PASS (0 formatting issues)
✓ golangci-lint: PASS (0 errors)
✓ Tested: 2026-07-27 14:24 UTC

## Error Handling

✗ Unhandled errors found: 0 (target: 0)
  Scan results: grep -n "_ = " *.go | wc -l = 0
✓ Matches Phase 1 checklist requirement

## Configuration

✓ No hardcoded localhost: PASS
✓ No hardcoded :7474 in production code: PASS
✓ No /tmp paths in production: PASS

## Integration Points

✓ Neo4j graceful degradation: PASS
  - Code checks if cache is nil, continues without it
✓ Bridge errors handled: PASS
  - BridgeCallback rejects with error if verification fails

## Security Verification

✓ Signatures verified before use: PASS
  - All Bridge responses have Verify() call
  - All decisions have signature check on read
✓ Input validation present: PASS
  - Query parameters validated (length, charset)
  - Correlation IDs validated before logging

## Sign-Off Gate

Evidence proves Phase 1 is complete.

- [ ] Implementation Team confirms code is complete and tested
- [ ] Architect confirms implementation matches specification
- [ ] Both have reviewed evidence above
```

### Phase 2: Unit Test Evidence

**What evidence proves Phase 2 is complete:**

```markdown
# Phase 2 Evidence Report

## Test Execution

✓ go test ./... PASSED
  - Total tests: 165
  - Passed: 165
  - Failed: 0
  - Exit code: 0
✓ Tested: 2026-07-27 15:45 UTC

## Code Coverage

✓ go test ./... -cover
  - Overall coverage: 87.3%
  - Target: ≥80%
  - Status: PASS

Coverage by package:
  - internal/ledger: 92% (↑)
  - internal/signing: 93% (↑)
  - api/handlers: 81% (↑)
  - internal/cache: 78% (↓ needs review)

## Flakiness Verification

✓ Determinism check (5 consecutive runs):
  Run 1: 165 PASS, 0 FAIL
  Run 2: 165 PASS, 0 FAIL
  Run 3: 165 PASS, 0 FAIL
  Run 4: 165 PASS, 0 FAIL
  Run 5: 165 PASS, 0 FAIL
  Result: 100% consistent (no flakes detected)

## Test Isolation

✓ No :memory: databases in tests: PASS
✓ Each test uses t.TempDir(): PASS
✓ Test IDs unique (no collisions): PASS
  grep "esc-" *_test.go | wc -l = 0 (hardcoded IDs)

## Critical Path Testing

✓ Signature round-trip tested: PASS
  - Sign → Verify succeeds
  - Sign → Tamper → Verify fails
  
✓ Escalation idempotency tested: PASS
  - Concurrent approvals: only 1 succeeds, 4 get 409
  
✓ Race conditions (go test -race): PASS
  no race conditions detected

## Sign-Off Gate

Evidence proves Phase 2 is complete.

- [ ] Tester confirms test coverage ≥80% and zero flakes
- [ ] Architect confirms tests verify all invariants
- [ ] Both have reviewed evidence above
```

### Phase 3: Integration Test Evidence

**What evidence proves Phase 3 is complete:**

```markdown
# Phase 3 Evidence Report

## End-to-End Flows Tested

✓ Decision Recording Flow
  - Create decision → Sign it → Store in ledger → Query by ID → Verify signature
  - Test: TestRecordDecisionEndToEnd (PASS)
  
✓ Escalation Flow
  - Create escalation → Bridge responds → Approve → Record approval
  - Test: TestEscalationFlow (PASS)
  
✓ Neo4j Integration
  - Record decisions → Index in Neo4j → Query by axiom → Get results
  - Test: TestNeo4jIntegration (PASS)
  
✓ Graceful Degradation
  - Neo4j down → Queries degrade to ledger scan
  - Test: TestNeo4jDown (PASS)

## Failure Mode Testing

✓ Ledger corruption recovery: PASS
  - Corrupt ledger file → Load detects → Reports error
  
✓ Signature tampering detection: PASS
  - Modify signature in file → Verify fails → Rejection
  
✓ Bridge timeout handling: PASS
  - Bridge unresponsive → Escalation queued → No cascade
  
✓ Concurrent operations: PASS
  - go test -race: no races detected
  - 5 concurrent approvals: 1 succeeds, 4 get 409

## Data Consistency Verification

✓ Ledger vs Neo4j consistency: PASS
  - 1000 decisions recorded
  - Ledger scan: 1000 decisions
  - Neo4j query: 1000 decisions
  - Divergence: 0

✓ Timestamp ordering: PASS
  - Decisions have monotonic timestamps
  - Replay attacks detected

## Integration Test Pass Rate

✓ go test -tags integration ./...
  - Total tests: 43
  - Passed: 43
  - Failed: 0
  - Flakiness (5 runs): 0% (100% consistent)

## Sign-Off Gate

Evidence proves Phase 3 is complete.

- [ ] Tester confirms end-to-end flows work
- [ ] Architect confirms integration points verified
- [ ] Both have reviewed evidence above
```

### Phase 4: Audit Evidence

**What evidence proves Phase 4 is complete:**

```markdown
# Phase 4 Evidence Report

## Audit Findings Documentation

✓ AUDIT-FINDINGS-202-PHASE-4.md exists
✓ All findings categorized by severity:
  - CRITICAL: 0 (↓ fixed from 2)
  - HIGH: 2 (↓ fixed from 6, only strategic ones remain)
  - MEDIUM: 4 (same)
  - LOW: 2 (same)

## Findings Resolution Status

✓ All CRITICAL findings FIXED:
  - Escalation double-approval race: FIXED (added mutex)
  - Health check ledger pollution: FIXED (removed append)
  
✓ HIGH findings with mitigation:
  - Correlation ID collision risk: MITIGATED (UnixNano, not Unix)
  - Input validation: MITIGATED (added validators)

✓ Remediation verified: PASS
  - Each fixed issue has test proving fix works
  - Re-audit confirms findings resolved

## Audit Checklist Completed

✓ Specification audit: PASS
  - Traced each use case end-to-end
  
✓ Invariant audit: PASS
  - Immutability: verified ✓
  - Idempotency: verified ✓
  - Durability: verified ✓
  - Integrity: verified ✓
  - Correlation tracking: verified ✓
  
✓ Security audit: PASS
  - Auth/authz: verified ✓
  - Input validation: verified ✓
  - Error handling: verified ✓
  - Secrets not logged: verified ✓
  
✓ Performance audit: PASS
  - Query limits: verified ✓
  - No unbounded operations: verified ✓
  - Resource usage stable: verified ✓

## Re-Audit Verification

✓ Phase 4 findings after fixes: 0 NEW findings
  → All original issues are fixed
  → No new issues introduced by fixes
  → Audit approved for production

## Sign-Off Gate

Evidence proves Phase 4 is complete.

- [ ] Independent Auditor confirms all findings resolved
- [ ] Architect confirms audit quality acceptable
- [ ] Both have reviewed evidence above
```

---

## Implementation: How to Use Grounded Reasoning

### For Each Phase:

**1. Collect Evidence Automatically**
```bash
./scripts/collect-phase-evidence.sh 1 202
# Creates: PHASE-1-EVIDENCE-202.md with automated checks
```

**2. Review Evidence Manually**
```bash
cat tickets/PHASE-1-EVIDENCE-202.md
# Architect/Tester verify evidence is sufficient
```

**3. Address Gaps**
```bash
# If evidence shows gaps:
#   - coverage < 80%? Add tests
#   - unhandled errors? Fix code
#   - hardcoded values? Externalize config
```

**4. Commit Evidence to Git**
```bash
git add tickets/PHASE-1-EVIDENCE-202.md
git commit -m "TICKET-202: Phase 1 evidence (DRAFT)"
# Evidence is now immutable
```

**5. Request Sign-Offs**
```
Now that evidence is committed:
- Implementation Team reviews evidence
- Architect reviews evidence  
- Tester reviews evidence
- All sign PHASE-SIGN-OFF-SHEET.md with evidence link

Sign-off means: "I reviewed the evidence and it proves Phase 1 is complete."
```

### Updated Sign-Off Sheet:

```markdown
| Phase | Role | Evidence | Status | Date | Notes |
|-------|------|----------|--------|------|-------|
| 1 | Implementation | PHASE-1-EVIDENCE-202.md | ✓ | 2026-07-27 | "Code compiles, 165 tests pass, 87% coverage" |
| 1 | Architect | PHASE-1-EVIDENCE-202.md | ✓ | 2026-07-27 | "Implementation matches spec, errors handled" |
| 1 | Tester | PHASE-1-EVIDENCE-202.md | ✓ | 2026-07-27 | "No flakiness, all invariants tested" |
```

**Sign-off now means:** "I have reviewed the evidence file linked above and verified that Phase N is complete."

---

## Why Grounded Reasoning Works

| Approach | Problem | How Grounded Reasoning Fixes It |
|----------|---------|----------------------------------|
| Claim-based ("Phase 1 done") | No proof, audit finds bugs later | Evidence shows exactly what was tested |
| Testing-based ("87% coverage") | No connection to spec | Evidence links coverage to spec requirements |
| Process-based ("Follow checklist") | Checklist can be skipped | Evidence forces verification before sign-off |
| Grounded reasoning | Immutable, verified, auditable proof | Evidence file committed to git, can't be faked |

**Key insight:** Evidence files are committed to git. They are:
- ✓ Immutable (can't be altered after commit)
- ✓ Verifiable (captain can re-run same checks)
- ✓ Historical (git log shows all evidence over time)
- ✓ Auditable (compliance can audit evidence trail)

---

## Measuring Improvement

**Before (claim-based):**
```
Phase 4 audit finds: 31 issues
Phase 0 verification: None
Phase 1 verification: Checklist only
Phase 2 verification: Test count only
Phase 3 verification: "It works"
```

**After (grounded reasoning):**
```
Phase 0 evidence: Spec doc + risk assessment (both committed)
Phase 1 evidence: Compilation + linting + error scan (committed)
Phase 2 evidence: Test results + coverage report (committed)
Phase 3 evidence: Integration test results (committed)
Phase 4 audit: Verifies evidence quality, finds issues EARLIER
```

**Result:** Issues found in Phase 1-3 when they're cheap to fix, not Phase 4 when they're expensive.

---

**Next:** Wire evidence collection into the enforcement hooks so they run automatically before sign-off.
