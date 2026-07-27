# Phase Enforcement Framework

**Purpose:** Ensure every phase sign-off is mandatory, auditable, and measured. Prevent teams from skipping steps that led to the 31 Phase 4 issues.

**Authority:** Derived from 5-Phase Mandatory Pipeline + System Invariants

---

## Five Roles (Personas)

### 1. **Discovery Lead**
**Responsibility:** Define what we're building and why.

**In Scope:**
- Gather requirements from stakeholders
- Document use cases and constraints
- Identify risks and assumptions
- Write Phase 0 specification

**Out of Scope:**
- Implementation design (Architect owns)
- Code review (Implementation Team owns)
- Testing strategy (Tester owns)

**Key Questions They Ask:**
- "What problem are we solving?"
- "What constraints apply (compliance, performance, security)?"
- "Who are the users and what do they need?"
- "What could go wrong and how bad?"

**Sign-Off Gate:** Phase 0 specification approved = Discovery Lead signature + date

---

### 2. **Architect**
**Responsibility:** Translate requirements into design. Verify implementation matches design. Gate all phase transitions.

**In Scope:**
- Read Discovery's Phase 0 spec
- Design system architecture (modules, interfaces, data flow)
- Review Phase 1 implementation against spec
- Verify Phase 2 unit tests cover design invariants
- Verify Phase 3 integration tests verify end-to-end contract
- Sign off before Phase 4 audit starts

**Out of Scope:**
- Writing code (Implementation Team owns)
- Writing tests (Tester owns)
- Auditing findings (Independent Auditor owns)

**Key Questions They Ask:**
- "Does the implementation match the specification?"
- "Are all invariants enforced in code?"
- "What's the data flow end-to-end?"
- "How does this handle failure modes?"
- "Do we have evidence (tests) that this works?"

**Sign-Off Gate:** Architect signature required before starting Phase 2, 3, 4, 5

---

### 3. **Implementation Team**
**Responsibility:** Write code that matches the architecture.

**In Scope:**
- Implement features per Architect design
- Write code to pass unit tests
- Review tests for correctness
- Work with Tester on edge cases

**Out of Scope:**
- Deciding the architecture (Architect owns)
- Writing test strategy (Tester owns)
- Independent audit (Independent Auditor owns)

**Key Questions They Ask:**
- "Does this code do what the spec says?"
- "Are all error paths handled?"
- "Does this match the architecture design?"
- "Can we write a unit test for this?"

**Sign-Off Gate:** Implementation Team marks work "ready for test" when code compiles and passes linting

---

### 4. **Tester**
**Responsibility:** Verify behavior matches specification via automated tests.

**In Scope:**
- Write unit tests (Phase 2)
- Write integration tests (Phase 3)
- Test coverage analysis
- Edge case identification
- Test isolation verification (no flaky tests)

**Out of Scope:**
- Writing implementation code (Implementation Team owns)
- Architectural review (Architect owns)
- Independent audit findings (Independent Auditor owns)

**Key Questions They Ask:**
- "Can I write a test for this behavior?"
- "What edge cases might break this?"
- "Are tests isolated (not sharing state)?"
- "Do tests pass consistently (no flakes)?"
- "Do we have >80% code coverage?"

**Sign-Off Gate:** Tester signature when all tests pass, coverage >80%, zero flaky tests over 5 consecutive runs

---

### 5. **Independent Auditor**
**Responsibility:** Verify the entire system works as designed. Find what everyone else missed.

**In Scope:**
- Review specification (Phase 0)
- Review architecture design (Phase 1)
- Review unit test coverage (Phase 2)
- Review integration tests (Phase 3)
- Run full audit: code review, test gaps, security, performance, operational concerns
- Compare implementation vs specification

**Out of Scope:**
- Writing code (Implementation Team owns)
- Writing tests (Tester owns)
- Approving design (Architect owns)
- Fixing issues (Implementation Team owns)

**Key Questions They Ask:**
- "Does the code do what the spec says?"
- "What's missing from tests?"
- "What could break in production?"
- "Are invariants actually enforced?"
- "What happens when things fail?"
- "Is there a security flaw?"
- "Will this scale to year-2 requirements?"

**Sign-Off Gate:** Independent Auditor signature when:
- Zero CRITICAL findings
- ≤3 HIGH findings (with mitigation plan)
- ≤10 MEDIUM findings
- Issues document sent to Implementation Team
- Remediation plan approved

---

## Phase Sign-Off Sheet Template

**Ticket:** 202  
**Phase:** 1 (Implementation)  
**Start Date:** 2026-07-26  
**End Date:** 2026-07-27  

| Role | Responsibility | Approval? | Date | Notes |
|------|---|---|---|---|
| Discovery Lead | Spec written + approved | ✓ | 2026-07-26 | Spec complete, no open questions |
| Architect | Design reviewed, gate passed | ✓ | 2026-07-26 | Design matches HB-001 spec, 3 integration points verified |
| Implementation Team | Code ready for test | ✓ | 2026-07-27 | All functions implemented, passes linting, compiles |
| Tester | Tests pass, coverage >80% | ✓ | 2026-07-27 | 165 tests, 87% coverage, zero flakes over 5 runs |
| Architect | Phase 2 gate approved | ✓ | 2026-07-27 | Tests match spec, integration points verified |
| Independent Auditor | Audit complete | ✗ | [PENDING] | Waiting for Phase 4 start |

**Issues Found This Phase:**
- None (Phase 1)

**Blockers for Next Phase:**
- None

---

## Quality Metrics Dashboard

**Target:** Measure effectiveness of enforcement framework over time.

### Metrics to Track

1. **Issues Found Per Phase** (per ticket)
   ```
   Phase 0: 0-5 issues (discovery/requirement gaps)
   Phase 1: 0-3 issues (implementation bugs)
   Phase 2: 0-5 issues (test coverage gaps)
   Phase 3: 0-3 issues (integration flaws)
   Phase 4: ≤3 CRITICAL + ≤3 HIGH (target: down from 31 total)
   Phase 5: 0 issues (deployment is mechanical)
   ```

2. **Sign-Off Completeness**
   - Target: 100% of phases have all 5 roles signed off
   - Track: % of tickets where all roles signed

3. **Time Per Phase** (optional tracking, not a gate)
   - Phase 0: 2-4 hours (discovery)
   - Phase 1: 4-6 hours (implementation)
   - Phase 2: 2-3 hours (unit tests)
   - Phase 3: 3-4 hours (integration tests)
   - Phase 4: 4-6 hours (audit)
   - Phase 5: 1-2 hours (deployment)

4. **Flaky Test Rate**
   - Target: 0% (100% deterministic)
   - Track: % of tests that fail inconsistently

5. **Test Coverage**
   - Target: ≥80% per ticket
   - Track: coverage % trend

6. **Rework Rate**
   - Track: % of issues found in Phase 4 that could have been caught earlier
   - Goal: Shift left (find in Phase 1-3, not Phase 4)

---

## Enforcement Mechanism

### CORE RULE: No Partial Execution Allowed

**Rule 0: Complete Workflow or Nothing**

**If:** Team attempts Phase N+1 while Phase N is incomplete  
**Then:** BLOCKED. Cannot proceed.

**Why:** Every phase assumes the prior phase is complete.
- Phase 0 spec assumes Phase 1 will verify it
- Phase 1 implementation assumes Phase 2 will test it  
- Phase 2 tests assume Phase 3 will integrate
- Phase 3 integration assumes Phase 4 will audit
- If ANY step is partial/rushed, the entire chain breaks

**Examples of "Partial Work" (FORBIDDEN):**
```
FORBIDDEN: "Phase 1 is 80% done, let's start Phase 2"
FORBIDDEN: "This endpoint doesn't need tests, we'll add them later"
FORBIDDEN: "Health check is small, skip the full validation workflow"
FORBIDDEN: "Neo4j divergence is rare, we'll handle it in production"
FORBIDDEN: "We trust Bridge, skip signature verification"
```

**Enforcement:** Automated CI job blocks phase advance if prior phase signature is missing.

---

### Rule 1: No Phase Advance Without 100% Sign-Off
**If:** Phase N phase-lead signature is missing OR any role unsigned  
**Then:** Ticket BLOCKED from Phase N+1. Cannot merge.

**Implementation (Automated):**
```bash
# CI job: phase-gate-check.sh
# Runs before allowing PR merge to main

PHASE=$(git show HEAD:tickets/PHASE-SIGN-OFF-SHEET.md | grep "^## Ticket")
PHASE_NUM=$(echo $PHASE | grep -o "Phase [0-9]" | tail -1)

# Check if PHASE_NUM-1 has all signatures
PRIOR_PHASE=$((PHASE_NUM - 1))

SIGNATURES=$(grep "✓" PHASE-SIGN-OFF-SHEET.md | grep "Phase $PRIOR_PHASE")

if [ $(echo "$SIGNATURES" | wc -l) -lt 5 ]; then
  echo "BLOCKED: Phase $PRIOR_PHASE incomplete (need 5 signatures, found $(echo "$SIGNATURES" | wc -l))"
  echo "Cannot advance to Phase $PHASE_NUM"
  exit 1
fi

echo "✓ Phase $PRIOR_PHASE complete. Phase $PHASE_NUM approved."
exit 0
```

**Human-Enforced:**
- Pull request requires CODEOWNERS approval (Architect, Auditor)
- PHASE-SIGN-OFF-SHEET.md must show all signatures
- CI checks signature count; if <5 roles signed, PR fails
- No force-push allowed (prevents skipping CI checks)

---

### Rule 2: Independent Audit is Mandatory
**If:** Ticket in Phase 4 but Phase 1-3 not signed  
**Then:** Audit blocked until prior phases complete

**Implementation (Automated):**
```bash
# CI job: audit-gate-check.sh
# Blocks audit start if Phase 1-3 incomplete

TICKET=$(git show HEAD:tickets/TICKET-*.md | head -1 | grep -o "TICKET-[0-9]*")
SIGN_OFF="tickets/PHASE-SIGN-OFF-SHEET-$TICKET.md"

for PHASE in 1 2 3; do
  SIGNED=$(grep "Phase $PHASE" "$SIGN_OFF" | grep "✓" | wc -l)
  if [ $SIGNED -lt 5 ]; then
    echo "BLOCKED: Phase $PHASE incomplete (need 5 signatures, have $SIGNED)"
    echo "Cannot start Phase 4 audit"
    exit 1
  fi
done

echo "✓ Phases 1-3 complete. Phase 4 audit approved."
exit 0
```

**Human-Enforced:**
- Auditor cannot open Phase 4 findings until phase-gate-check succeeds
- PHASE-SIGN-OFF-SHEET.md is audit precondition
- Audit can't begin without written proof of completion

---

### Rule 3: All Findings Must Be Documented & Remediated
**If:** Auditor finds issue  
**Then:** Must be logged in AUDIT-FINDINGS-*.md, remediation tracked

**Implementation (Automated):**
```bash
# CI job: findings-enforcement.sh
# Blocks production advance if findings not remediated

FINDINGS="tickets/AUDIT-FINDINGS-*.md"

# Count unfixed findings by severity
CRITICAL=$(grep "Severity: CRITICAL" $FINDINGS | grep -v "Status: FIXED" | wc -l)
HIGH=$(grep "Severity: HIGH" $FINDINGS | grep -v "Status: FIXED" | wc -l)

if [ $CRITICAL -gt 0 ]; then
  echo "BLOCKED: $CRITICAL CRITICAL findings unfixed"
  echo "Fix all CRITICAL findings before Phase 5"
  exit 1
fi

if [ $HIGH -gt 3 ]; then
  echo "BLOCKED: $HIGH HIGH findings (max 3 allowed)"
  echo "Fix HIGH findings or get captain approval"
  exit 1
fi

echo "✓ All critical findings fixed. Phase 5 approved."
exit 0
```

**Human-Enforced:**
- AUDIT-FINDINGS-*.md must exist before production deploy
- Each finding must have Status: FIXED, Status: DEFERRED, or Status: ACCEPTED
- CRITICAL findings: must be fixed (0 tolerance)
- HIGH findings: max 3 allowed; must have mitigation plan

---

### Rule 4: Quality Gate Metrics Must Be Met
**If:** Metrics exceed targets (e.g., >5 issues in Phase 1, >1% flakiness)  
**Then:** Post-mortem required before Phase advance

**Implementation (Automated):**
```bash
# CI job: quality-gate-check.sh
# Verifies metrics are within targets

source tickets/QUALITY-METRICS.md

# Check issues per phase
PHASE_1_ISSUES=$(grep "Phase 1" QUALITY-METRICS.md | grep -o "issues=[0-9]*" | cut -d= -f2)
if [ $PHASE_1_ISSUES -gt 5 ]; then
  echo "BLOCKED: Phase 1 has $PHASE_1_ISSUES issues (max 5)"
  echo "Needs post-mortem before Phase 2 advance"
  exit 1
fi

# Check test coverage
COVERAGE=$(grep "Overall.*%" tickets/PHASE-2-RESULTS.md | grep -o "[0-9]*%")
if [ ${COVERAGE%\%} -lt 80 ]; then
  echo "BLOCKED: Coverage ${COVERAGE} < 80%"
  exit 1
fi

# Check flakiness
FLAKY=$(grep "Flaky Tests" QUALITY-METRICS.md | grep -o "[0-9]*")
if [ $FLAKY -gt 0 ]; then
  echo "BLOCKED: $FLAKY flaky tests (must be 0)"
  exit 1
fi

echo "✓ All quality gates passed."
exit 0
```

**Human-Enforced:**
- QUALITY-METRICS.md updated after each phase
- If any metric exceeds target, PR blocked
- Captain must explicitly approve override (with ticket number)

---

### Rule 5: All Common Bugs Must Be Checked
**If:** Ticket in Phase 1 without running common-bugs-check  
**Then:** PR fails; must run checks first

**Implementation (Automated):**
```bash
# CI job: common-bugs-check.sh
# Runs linters and checks for known bad patterns

set -e  # Fail on any error

echo "=== CHECKING FOR COMMON BUGS ==="

# Bug 1.1: Signature verification skipped
echo "Checking: Signature verification not skipped..."
grep -n "Signature.*=" *.go | while read line; do
  line_num=$(echo "$line" | cut -d: -f1)
  # Verify there's a Verify() call within 5 lines
  if ! sed -n "${line_num},$((line_num+5))p" *.go | grep -q "Verify\|validate"; then
    echo "FAIL: Signature assignment at line $line_num without verification"
    exit 1
  fi
done

# Bug 2.1: Unhandled errors
echo "Checking: No unhandled error returns..."
if grep -n "_ = " *.go | grep -v "for _" | grep -v range | grep -v "//.*intentional"; then
  echo "FAIL: Unhandled error found. Add // intentional comment if intentional"
  exit 1
fi

# Bug 3.1: Unvalidated input
echo "Checking: Query parameters validated..."
if grep -n "Query.Get" *.go | grep -v "Validate\|Length" | head -5; then
  echo "WARN: Query.Get without validation. Check if intentional."
fi

# Bug 4.1: State transitions enforced
echo "Checking: Status transitions guarded..."
if grep -n "Status = " *.go | grep -v "mu.Lock\|check\|guard\|if"; then
  echo "WARN: Status assignment without guard. Check if intentional."
fi

# Bug 6.1: Test isolation
echo "Checking: Tests use TempDir, not :memory:..."
if grep -n ":memory:" *_test.go; then
  echo "FAIL: Tests use shared :memory: database - breaks isolation"
  exit 1
fi

echo "✓ All common bug checks passed"
exit 0
```

**Human-Enforced:**
- PR cannot merge if common-bugs-check fails
- Checks are in `pre-commit` hook (local) + CI (remote)
- False positives allowed with `// intentional` comment
- Captain can override with approval

---

### Rule 6: Phases Must Be Sequential (No Parallelization Across Tickets)
**If:** Two tickets attempt Phase N simultaneously  
**Then:** One is allowed, other QUEUED until first completes Phase N+1

**Why:** Prevents process shortcuts ("we'll run phases in parallel to save time")

**Implementation (Automated):**
```bash
# CI job: phase-sequencing-check.sh
# Ensures phases are sequential

ACTIVE_TICKETS=$(git log --all --grep="Phase [0-9]" --oneline | grep -o "TICKET-[0-9]*" | sort | uniq)

for TICKET in $ACTIVE_TICKETS; do
  PHASE=$(grep "TICKET-$TICKET" DEPLOYMENT-STATUS.md | grep -o "Phase [0-9]" | tail -1)
  
  # Check if prior phase is complete for this ticket
  PHASE_NUM=${PHASE#Phase }
  PRIOR_PHASE=$((PHASE_NUM - 1))
  
  if [ $PRIOR_PHASE -gt 0 ]; then
    PRIOR_SIGNED=$(grep "TICKET-$TICKET" PHASE-SIGN-OFF-SHEET.md | grep "Phase $PRIOR_PHASE" | grep "✓" | wc -l)
    if [ $PRIOR_SIGNED -lt 5 ]; then
      echo "BLOCKED: TICKET-$TICKET cannot advance to Phase $PHASE_NUM"
      echo "Prior phase (Phase $PRIOR_PHASE) not complete"
      exit 1
    fi
  fi
done

echo "✓ All tickets on valid phase progression"
exit 0
```

---

### The Gate Matrix: What Blocks What

```
┌─────────────────────────────────────────────────────────────┐
│ PHASE GATE MATRIX: When Does CI Block?                     │
├─────────────────────────────────────────────────────────────┤
│ PHASE 0 → PHASE 1: Requires                                │
│   □ PHASE-SIGN-OFF-SHEET.md has Discovery Lead ✓          │
│   □ PHASE-SIGN-OFF-SHEET.md has Architect ✓               │
│   □ PHASE-0-CHECKLIST-SPECIFICATION.md complete           │
│   → If ANY missing: PR blocked                             │
│                                                             │
│ PHASE 1 → PHASE 2: Requires                                │
│   □ Phase 1 signatures (5/5 roles)                         │
│   □ common-bugs-check passes                               │
│   □ Code compiles without warnings                         │
│   □ PHASE-1-CHECKLIST-IMPLEMENTATION.md complete          │
│   → If ANY missing: PR blocked                             │
│                                                             │
│ PHASE 2 → PHASE 3: Requires                                │
│   □ Phase 2 signatures (5/5 roles)                         │
│   □ Test coverage ≥80%                                     │
│   □ Zero flaky tests (run 5x, all pass)                    │
│   □ PHASE-2-RESULTS.md with metrics                        │
│   → If ANY missing: PR blocked                             │
│                                                             │
│ PHASE 3 → PHASE 4: Requires                                │
│   □ Phase 3 signatures (5/5 roles)                         │
│   □ Integration tests passing                              │
│   □ End-to-end flow verified                               │
│   → If ANY missing: Audit cannot start                     │
│                                                             │
│ PHASE 4 → PHASE 5: Requires                                │
│   □ Phase 4 signatures (5/5 roles)                         │
│   □ CRITICAL findings = 0                                  │
│   □ HIGH findings ≤3 (with mitigation)                     │
│   □ All findings documented in AUDIT-FINDINGS-*.md         │
│   □ QUALITY-METRICS.md shows all targets met               │
│   → If ANY missing: Production deploy blocked              │
└─────────────────────────────────────────────────────────────┘
```

---

## Harness Implementation: CI/CD Gates

**The CI/CD harness actively enforces the workflow.** No manual override possible except by Captain with written approval.

### Phase Gate Job (runs on every PR)
```yaml
# .github/workflows/phase-gate.yml
name: Phase Gate Enforcement

on: [pull_request]

jobs:
  phase-gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Check prior phase complete
        run: ./scripts/phase-gate-check.sh
        
      - name: Check audit prerequisites
        run: ./scripts/audit-gate-check.sh
        
      - name: Common bugs scan
        run: ./scripts/common-bugs-check.sh
        
      - name: Quality metrics gate
        run: ./scripts/quality-gate-check.sh
        
      - name: Report status
        if: failure()
        run: |
          echo "❌ PHASE GATE FAILED"
          echo "This PR cannot merge until gates are satisfied"
          echo "See logs above for what's blocking"
          exit 1
```

### Sign-Off Enforcement (human + automated)
```yaml
# .github/workflows/sign-off-required.yml
name: Signature Enforcement

on: [pull_request]

jobs:
  require-signatures:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Verify signatures in PR
        run: |
          PHASE=$(git show HEAD:tickets/PHASE-SIGN-OFF-SHEET.md | grep "Phase")
          SIGNATURES=$(grep "✓" PHASE-SIGN-OFF-SHEET.md | wc -l)
          
          if [ $SIGNATURES -lt 5 ]; then
            echo "❌ Missing signatures: $SIGNATURES/5"
            echo "Required: Discovery Lead, Architect, Implementation, Tester, Auditor"
            exit 1
          fi
          
          echo "✓ All 5 signatures present"
```

### Cannot Bypass Rules
```yaml
# .github/workflows/protect-main.yml
# (Branch protection: requires all checks to pass)

name: Protect Main Branch

on: [pull_request]

jobs:
  protect:
    runs-on: ubuntu-latest
    steps:
      - name: Require status checks
        run: |
          # These checks MUST pass:
          # - phase-gate-check
          # - audit-gate-check
          # - common-bugs-check
          # - quality-gate-check
          # - signature-verification
          # 
          # No --force-push allowed
          # No direct commits to main allowed
          # All merges must go through PR + checks
```

---

**Summary: Three-Layer Enforcement**

| Layer | What | How | Enforcer |
|-------|------|-----|----------|
| **Rule** | No partial execution allowed | Hard rule in docs | Process |
| **Checklist** | Each phase verifies prior complete | PHASE-*-CHECKLIST.md | Humans |
| **Harness** | CI/CD gates block incomplete phases | .github/workflows/*.yml | Automation |

**No one can skip steps because:**
1. Documentation says you can't
2. Checklist requires verification
3. CI/CD job will reject the PR

**Captain override is possible but tracked:**
```bash
# Override syntax (requires captain approval in PR comment)
# @captain override-phase-gate TICKET-202 "approval reason"
# 
# This creates:
# - tickets/OVERRIDES.md (log of all overrides)
# - Alert to leadership (why was phase skipped?)
# - Post-mortem required within 1 week
```

---

## Example: Ticket 202 (Bridge Integration) Sign-Off Flow

```
2026-07-26 08:00 — Discovery Lead approves Phase 0 spec
                    "Bridge integration spec complete"
                    
2026-07-26 09:00 — Architect reviews spec + architecture
                    Signs off: "Design matches AX-SPAWN-001 spec"
                    Implementation can now start
                    
2026-07-26 18:00 — Implementation Team marks ready for test
                    "All 8 handlers implemented, passes lint"
                    
2026-07-27 08:00 — Tester runs test suite
                    165 tests pass, 87% coverage
                    Signs off: "Tests match spec, zero flakes"
                    
2026-07-27 09:00 — Architect reviews Phase 2 gate
                    "Tests verify all invariants, sign-off: Phase 3 clear"
                    
2026-07-27 14:00 — Integration tests pass
                    End-to-end Bridge flow verified
                    
2026-07-27 16:00 — Independent Auditor begins Phase 4
                    Stress tests, finds 12 issues (details in AUDIT-FINDINGS-202-PHASE-4.md)
                    
2026-07-27 20:00 — Implementation Team fixes all 12 issues
                    Re-runs full test suite: all pass
                    
2026-07-27 21:00 — Auditor verifies fixes: 0 CRITICAL, 0 HIGH remaining
                    Signs off: "Phase 4 audit complete, approved for Phase 5"
                    
2026-07-28 08:00 — Operator runs deployment checklist
                    Verifies: ledger backup, Neo4j health, monitoring in place
                    Deploys to staging
                    
2026-07-28 16:00 — Production deployment
                    Bridge integration live
```

---

## Next Steps

1. **Print Phase Sign-Off Sheet** → Paste into each ticket
2. **Distribute Persona Descriptions** → Each role understands their job
3. **Create Per-Phase Checklists** → Review guides for each role
4. **Wire Metrics Dashboard** → Update after each phase
5. **Captain Enforcement** → Review before any phase transition

Ready to build the checklists?
