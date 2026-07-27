# HomeBase Development Workflow v2.0

**Adopted:** 2026-07-26  
**Reason:** Post-mortem from Ticket 201 revealed critical process gaps  
**Status:** Mandatory for all future tickets

---

## Why This Matters: Lessons from 59 Years

HomeBase has been running for nearly 6 decades. We've made these mistakes before:

1. **Implementation drift** - Code diverges from design without validation
2. **Silent failures** - Error handling patterns that hide bugs (like `_, _` in Go)
3. **Incomplete integration testing** - Unit tests pass but workflows break
4. **Missing quality gates** - No mandatory checkpoints before shipping
5. **No independent review** - Issues caught only after deployment

**This workflow exists to prevent repeating these lessons.**

---

## Development Phases & Quality Gates

```
┌─────────────────────────────────────────────────────────┐
│ PHASE 0: SPECIFICATION (Design Review)                   │
│ ✓ Spec matches requirements                              │
│ ✓ All endpoints/components listed                        │
│ ✓ Data flows documented                                  │
│ ✓ Error scenarios identified                             │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ PHASE 1: IMPLEMENTATION (Code Review)                    │
│ ✓ Implementation matches spec                            │
│ ✓ All components built                                   │
│ ✓ No silent error patterns (e.g., _, _)                 │
│ ✓ Error handling explicit                                │
│ ✓ Logging adequate for debugging                         │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ PHASE 2: UNIT TESTS (Component Validation)               │
│ ✓ 80%+ code coverage                                     │
│ ✓ All tests passing                                      │
│ ✓ Error paths tested (not just happy path)              │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ PHASE 3: INTEGRATION TESTS (Workflow Validation)         │
│ ✓ End-to-end flows work                                 │
│ ✓ Test isolation verified (no state sharing)            │
│ ✓ Error scenarios tested                                │
│ ✓ All endpoints respond correctly                       │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ PHASE 4: INDEPENDENT REVIEW (Fresh Eyes)                │
│ ✓ Code review by someone not on team                    │
│ ✓ Security review (crypto, data integrity)              │
│ ✓ Specification compliance verified                     │
│ ✓ No hidden bugs or patterns                            │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│ PHASE 5: PRODUCTION READINESS (Go/No-Go)                │
│ ✓ All phases complete                                    │
│ ✓ Deployment procedures documented                      │
│ ✓ Recovery procedures documented                        │
│ ✓ Signed off by independent reviewer                    │
└─────────────────────────────────────────────────────────┘
```

---

## PHASE 0: Specification Review (Design Phase)

**Owner:** Architecture team  
**Blocker:** Cannot start implementation until spec is approved

### Specification Must Include

- [ ] **All endpoints listed** with HTTP method, path, request/response schema
- [ ] **All data flows documented** (create → read → update paths)
- [ ] **All error scenarios identified** (what happens when X fails?)
- [ ] **All components** that need to be built
- [ ] **Dependencies** between components
- [ ] **Performance requirements** (latency, throughput, scalability)
- [ ] **Recovery procedures** (how to recover from failures)

### Validation Checklist

```bash
# Before implementation starts:
✓ Specification is 100% complete (not "95% done, finish later")
✓ All endpoints documented (list them: count should match design)
✓ All error paths documented (not just happy path)
✓ Architecture team approved (independent sign-off)
✓ No "Phase 2" work (complete upfront)
```

**Lesson from Ticket 201:** Design said 10 endpoints, implementation only built 6. Specification was created but not validated before coding started.

---

## PHASE 1: Implementation (Code Review)

**Owner:** Developer  
**Blocker:** Code review must pass before integration testing

### Before Starting Implementation

1. **Read the specification again** - understand every endpoint and flow
2. **List all components** you need to build
3. **Create checklist** for yourself (helps catch incomplete work)

### During Implementation

- [ ] **No silent errors** - Never use `_, _` for error handling in critical paths
  ```go
  // WRONG:
  sig, _ := h.signer.SignJSON(&decision)
  
  // RIGHT:
  sig, err := h.signer.SignJSON(&decision)
  if err != nil {
      return fmt.Errorf("signing failed: %w", err)
  }
  ```

- [ ] **Explicit error handling** - Every operation that can fail has an error handler
- [ ] **Adequate logging** - Can trace a request through the system
- [ ] **Clear variable names** - Code explains itself
- [ ] **Consistent patterns** - Error responses formatted the same way everywhere

### Code Review (Peer)

```bash
Reviewer must verify:
✓ Does implementation match spec? (count endpoints, check schemas)
✓ Are all error paths handled?
✓ Is there adequate logging?
✓ No silent error patterns?
✓ Would I understand this code in 2 years?
```

**Lesson from Ticket 201:** Silent error `sig, _` pattern compiles and works for happy path but breaks in error case. Should have been caught in review.

---

## PHASE 2: Unit Tests (Component Validation)

**Owner:** Developer  
**Blocker:** All tests must pass before integration testing

### Coverage Requirements

- [ ] **80%+ code coverage** minimum
- [ ] **All error paths tested** (not just happy path)
- [ ] **Edge cases tested** (empty input, large input, concurrent access)
- [ ] **Integration between components tested** (not just individual functions)

### Test Isolation

- [ ] **No shared state between tests** (each test gets fresh resources)
- [ ] **Temporary storage per test** (not `:memory:` treated as real file)
- [ ] **Tests can run in any order** (no dependencies between tests)
- [ ] **Tests clean up after themselves** (no leftover files)

### Example: Ledger Tests

```go
// WRONG: Shared ledger across tests
var sharedLedger = ledger.NewStore(":memory:")

func TestAppend(t *testing.T) {
  decision := ...
  sharedLedger.Append(decision)  // Pollutes next test!
}

// RIGHT: Fresh ledger per test
func TestAppend(t *testing.T) {
  testLedger := ledger.NewStore(":memory:")
  testLedger.Load()
  decision := ...
  testLedger.Append(decision)  // Isolated!
}
```

**Lesson from Ticket 201:** Integration tests failed because ledger state shared across test cases. Proper test isolation would have caught this.

---

## PHASE 3: Integration Tests (Workflow Validation)

**Owner:** Developer + QA  
**Blocker:** All end-to-end flows must work before independent review

### End-to-End Workflows

For each API flow in the spec, test:

```
Flow 1: Record → Query → Verify
  POST /decisions → 201 Created with signature
  GET /decisions/{id} → 200 with decision
  POST /decisions/{id}/verify → {valid: true}

Flow 2: Error Handling
  POST /decisions (no axioms) → 400 Validation Failed
  POST /decisions (duplicate ID) → 409 Conflict
  POST /decisions (invalid schema) → 400 Bad Request

Flow 3: Graceful Degradation
  Record decision (Neo4j online) → SUCCESS
  Stop Neo4j
  Record decision (Neo4j offline) → SUCCESS (axiom check skipped)
  Verify query works

Flow 4: Persistence
  Record decision
  Stop server
  Start server
  Query decision → Decision still there
```

### Integration Test Checklist

```bash
✓ All endpoints respond to requests
✓ Response schemas match spec
✓ Error scenarios handled correctly
✓ No data loss in any flow
✓ Tests can run independently (no ordering)
✓ Tests clean up after themselves
```

**Lesson from Ticket 201:** Integration tests existed but were broken. Proper isolation would have revealed they weren't actually testing end-to-end flows.

---

## PHASE 4: Independent Review (Fresh Eyes)

**Owner:** Someone not on implementation team  
**Blocker:** Blocker findings must be fixed before production readiness

### Independent Reviewer Must Verify

- [ ] **Specification compliance** - Does implementation match spec?
  ```
  Spec says 10 endpoints? Count them: 6 implemented, 4 missing
  Spec says error handling? Check: silent errors found
  ```

- [ ] **Code quality** - Are there bugs or patterns that shouldn't be there?
  ```
  Silent error handling patterns
  Race conditions
  Unsafe serialization
  Incomplete error handling
  ```

- [ ] **Testing** - Do tests actually validate the system?
  ```
  Integration tests passing? Check: they're not isolated
  Unit tests covering errors? Check: happy path only
  ```

- [ ] **Security** - Cryptographic implementation sound?
  ```
  Ed25519 used correctly? Check
  Non-repudiation guaranteed? Check: if signing succeeds
  Hash chain safe? Check: serialization format
  ```

### Independent Reviewer Process

1. **Read specification** - understand what system should do
2. **Review code** - check against spec
3. **Review tests** - check they validate workflows
4. **Report findings** - critical, medium, low
5. **Verify fixes** - re-review before approval

**Lesson from Ticket 201:** No independent review until code review agent found 3 critical issues. Would have been caught earlier with this phase.

---

## PHASE 5: Production Readiness (Go/No-Go)

**Owner:** Captain (decision maker) + Independent reviewer  
**Blocker:** All critical findings must be resolved

### Production Readiness Checklist

```
CRITICAL (Blocker):
  ☑ All critical findings resolved
  ☑ All endpoints implemented
  ☑ All error paths handled
  ☑ No silent errors
  ☑ Integration tests passing

MEDIUM (Should have):
  ☑ Adequate logging
  ☑ Query parameters work
  ☑ Consistent error responses
  ☑ Safe serialization

NICE TO HAVE:
  ☑ Performance tested
  ☑ Load testing done
  ☑ Chaos testing done
  ☑ Security audit passed
```

### Deployment Readiness

- [ ] **Binary builds without warnings**
- [ ] **Deployment procedures documented** (how to run, flags, config)
- [ ] **Recovery procedures documented** (how to recover from failures)
- [ ] **Monitoring in place** (logs, metrics, health checks)

---

## Preventing Past Mistakes

### Lesson 1: Implementation Drift (Ticket 201)
**Problem:** Design said 10 endpoints, implementation only built 6  
**Solution:** Phase 0 sign-off + Phase 4 compliance check
```
Specification Review: ✓ All 10 endpoints required
Phase 4 Review: "Only 6 endpoints implemented. This blocks Phase 2. Fix before shipping."
```

### Lesson 2: Silent Errors (Ticket 201)
**Problem:** `sig, _ := h.signer.SignJSON()` ignores signing failures  
**Solution:** Code review + independent review catch pattern
```
Code Review: "Why is error ignored here? Fix before merge."
Phase 4 Review: "Silent error on line 265. This violates non-repudiation. Fix or don't ship."
```

### Lesson 3: Incomplete Testing (Ticket 201)
**Problem:** Unit tests pass but integration tests broken, workflows untested  
**Solution:** Phase 3 mandatory, integration tests required for end-to-end validation
```
Phase 3: "Integration tests must test Record → Query → Verify flow end-to-end."
Phase 4: "Tests have isolation issues. Fix test infrastructure before production."
```

### Lesson 4: No Checkpoints (Ticket 201)
**Problem:** Code shipped to "production readiness" without review  
**Solution:** Mandatory phases with gate at each step
```
Phase 0 → 1 → 2 → 3 → 4 → 5
Each phase has checkpoint. Cannot proceed without approval.
```

### Lesson 5: Learning From History (59 Years)
**Problem:** We keep making the same mistakes  
**Solution:** Institutionalize lessons in workflow
```
This workflow exists because we learned these lessons before.
Every developer reads this so we don't repeat past mistakes.
```

---

## Workflow Rules (Non-Negotiable)

1. **Specification must be 100% complete before coding starts**
   - No "finish it in Phase 2"
   - No "we'll add it later"
   - Complete → Review → Approve → Code

2. **No silent errors**
   - Every operation that can fail must handle the error
   - `_, _` pattern is forbidden in critical paths
   - Code review checks for this

3. **Integration tests are mandatory**
   - Unit tests prove components work
   - Integration tests prove workflows work
   - Cannot ship with only unit tests

4. **Independent review before production**
   - Someone not on the team reads your code
   - They verify against specification
   - They catch things you missed

5. **All phases must complete**
   - Spec → Code → Unit Tests → Integration Tests → Review → Ship
   - Cannot skip or rush any phase
   - Quality gates exist for reason

---

## How This Prevents Future Issues

**Scenario:** Ticket 202 (Bridge Integration)

```
Spec Phase:
  ✓ All endpoints listed (10 total)
  ✓ All error scenarios documented
  ✓ Data flows clear
  ✓ Ready for implementation

Code Phase:
  Reviewer: "Silent error handling? Line 142?"
  Developer: "Oh, you're right. Fixed."
  ✓ No bugs slip through

Integration Phase:
  Tester: "Workflow Record → Query → Verify passes? Test isolation ok?"
  ✓ Confident workflows work end-to-end

Review Phase:
  Independent Reviewer: "Do you have all 10 endpoints? Missing 4."
  Developer: "Missed those. Adding now."
  ✓ Complete before shipping

Production:
  ✓ Ship with confidence. All phases passed.
  ✓ No surprises. No critical bugs. No missing features.
```

---

## New Developer Onboarding

When a new developer joins:

1. Read this workflow
2. Understand the phases
3. Know the quality gates
4. Commit to following all 5 phases

**Key message:** "We do this because we learned these lessons. Don't skip phases. Don't ignore errors. Quality gates exist for reason."

---

## Ticket 201 Post-Mortem

What went wrong:
- ❌ No independent review until end
- ❌ Missing endpoints not caught
- ❌ Silent error pattern not reviewed
- ❌ Integration tests broken, not caught

What should have happened:
- ✅ Phase 0: Spec review (validate 10 endpoints listed)
- ✅ Phase 1: Code review (catch silent error, validate endpoints)
- ✅ Phase 2: Unit tests (80%+ coverage)
- ✅ Phase 3: Integration tests (proper isolation, end-to-end validation)
- ✅ Phase 4: Independent review (catch missing endpoints and bugs)
- ✅ Phase 5: Only then production ready

**Result:** Would have caught all 3 critical issues before claiming "production ready"

---

**Effective Date:** 2026-07-26  
**Mandatory For:** All future Ticket development  
**Exceptions:** None. No shortcuts.

This workflow is how we prevent repeating 59 years of lessons.
