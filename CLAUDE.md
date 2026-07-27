# HomeBase Project Instructions

**Derived from:** Global CLAUDE.md principles + SYSTEM-DESIGN.md  
**Authority:** Captain + Formal specification (SYSTEM-DESIGN.md)

---

## Principles (HomeBase-Specific)

### 1. Never Assume About Graph States

The entire system is a state machine with exactly 5 states. Never assume you can do something "inside" a state that belongs in another state.

**RED FLAGS:**
- ✗ "EXECUTE can decide to retry" (retry decisions belong in RECOVER)
- ✗ "RECOVER can update the execution plan" (plan updates belong in PLAN)
- ✗ "ESCALATE can decide to keep trying" (escalation is final)

**RIGHT:**
- ✓ Verify which state you're in before acting
- ✓ Verify your action belongs in that state
- ✓ If you need something from another state, call it explicitly (don't inline it)

---

### 2. Everything Must Be Proved (6 Invariants)

Every design decision is validated against one of the six invariants:

| Invariant | Test |
|-----------|------|
| **I1: Immutability** | Can this decision be changed after being written? If yes, violates I1 |
| **I2: Uniqueness** | Can this escalation be approved twice? If yes, violates I2 |
| **I3: Durability** | Does this write survive a process crash? If no, violates I3 |
| **I4: Integrity** | Could an invalid signature be accepted? If yes, violates I4 |
| **I5: Non-Repudiation** | Could a forged Bridge response be used? If yes, violates I5 |
| **I6: Axiom Grounding** | Could a fake axiom be cited? If yes, violates I6 |

**Before coding anything:** Ask which invariant it serves, or which it might break.

---

### 3. Challenge Agent Reasoning (Enforce Graph)

The biggest risk is drifting back toward "agent decides" behavior. Catch this:

**RED FLAGS (signs of agent reasoning creeping in):**
- ✗ "If X fails, the system should try Y" (hidden in error handling, no explicit protocol)
- ✗ "Recovery logic is flexible" (should be fixed 2-attempt protocol)
- ✗ "We'll retry if needed" (how many retries? Should be exactly 2)
- ✗ "Error handling depends on context" (should depend on protocol, not context)

**RIGHT:**
- ✓ "Recovery protocol: Attempt 1 (specific), Attempt 2 (specific), then escalate"
- ✓ "Retry logic is bounded (exactly N attempts)"
- ✓ "Escalation point is defined in advance"

---

### 4. Minimum Code, Maximum Structure

Graph structure does the work. Code should be thin.

**WRONG (too much code):**
```go
func handleDecision(d *Decision) {
    if err := recordToLedger(d); err != nil {
        if err := tryAlternateStorage(d); err != nil {
            if err := tryCompressionFix(d); err != nil {
                if err := tryRevalidation(d); err != nil {
                    escalateToHuman(d, err)
                }
            }
        }
    }
}
```

**RIGHT (structure does the work):**
```go
// PLAN state (locking workflow)
plan := planDecision(d)  // ["record", "sign", "index"]

// EXECUTE state (just does it)
result := executeSequence(plan, d)
if result.success {
    repeatNextDecision()
} else {
    enterRecoveryState(result.failure)
}

// RECOVER state (bounded protocol)
for attempt := 1; attempt <= 2; attempt++ {
    result := tryRecovery(attempt, d)
    if result.success { return repeatNextDecision() }
}

// After 2 attempts, escalate (no Attempt 3)
escalateToHuman(d, allAttemptHistory)
```

The **structure** makes the logic obvious. Code is almost commentary.

---

### 5. Service Capability Gate (5 States = Inventory)

Before adding anything, verify it fits one of the five states.

**The 5 states define what HomeBase CAN DO:**
1. **PLAN** — Lock workflow (validate, prepare)
2. **EXECUTE** — Record decision (write, sign, index)
3. **RECOVER** — Fix failures (exactly 2 attempts, defined)
4. **ESCALATE** — Ask human (not a decision point, just notify)
5. **REPEAT** — Continue (loop, or finish)

**If you need a new capability:**
- Does it fit in an existing state? ✓ Add it there
- Does it need a new state? ✗ Stop. Redesign is required. Talk to Captain.

---

## Development Workflow (Graph-Shaped)

### Phase 0: Design (Done)
- SYSTEM-DESIGN.md is locked
- No code yet
- Focus: Spec + risk assessment + architecture sketch

### Phase 1: Implementation (In Progress—4 Tickets)
- Ticket 202: Implement PLAN + EXECUTE states
- Ticket 203: Implement RECOVER + ESCALATE states
- Ticket 204: Test all 5 graph moves
- Ticket 205: Log all state transitions

**Development rule:** Implement states as separate, clearly named functions/types. Do NOT merge them.

```go
// RIGHT: Separate, clear
func planDecision(d *Decision) Plan { ... }
func executeDecision(plan Plan, d *Decision) Result { ... }
func recoverFromFailure(result Result) Action { ... }
func escalateToHuman(d *Decision, history []Attempt) { ... }

// WRONG: Merged, opaque
func handleDecision(d *Decision) { ... everything in one function ... }
```

### Phase 2: Unit Tests
- Each ticket tests their states
- Target: 80%+ coverage
- Focus: Verify each state does exactly what it claims

### Phase 3: Integration Tests
- All 4 tickets work together
- Focus: Verify state transitions are correct

### Phase 4: Audit
- Independent review of graph structure
- Stress-test all 6 invariants
- Target: 0 CRITICAL findings (down from 31)

### Phase 5: Production
- Deploy with monitoring
- Track metrics (escalations, recovery attempts, etc.)

---

## Code Review Checklist (Enforce Graph)

For every code change, ask:

**1. Which state is this for?**
- [ ] PLAN (planning/validation)
- [ ] EXECUTE (recording decision)
- [ ] RECOVER (fixing failures)
- [ ] ESCALATE (notifying human)
- [ ] REPEAT (looping/continuation)
- [ ] General (middleware, types, etc.)

**2. Does the code stay in its state?**
- [ ] EXECUTE doesn't decide recovery? ✓
- [ ] RECOVER doesn't execute new work? ✓
- [ ] ESCALATE doesn't decide for itself? ✓
- [ ] No hidden retries? ✓

**3. Which invariant(s) does this protect?**
- [ ] I1 (Immutability): Can this be modified after write?
- [ ] I2 (Uniqueness): Can this happen twice?
- [ ] I3 (Durability): Does this survive crash?
- [ ] I4 (Integrity): Could invalid data pass through?
- [ ] I5 (Non-Repudiation): Could a forged input be used?
- [ ] I6 (Axiom Grounding): Could invalid axiom be accepted?

**4. Is recovery bounded?**
- [ ] If this can fail, where does recovery happen?
- [ ] Recovery has exactly 2 defined attempts? ✓
- [ ] After 2 attempts, escalates (no Attempt 3)? ✓

**5. Is the plan locked?**
- [ ] Can mid-execution code change what happens next?
- [ ] Or is the sequence defined in advance (PLAN state)?

---

## Common Mistakes to Avoid

### Mistake 1: "Flexible Recovery"

❌ WRONG:
```go
if err := recordToLedger(d); err != nil {
    // "Try something reasonable"
    if retry(d) { ... }
    else if useCache(d) { ... }
    else if recompress(d) { ... }
    else escalate(d)
}
```

✓ RIGHT:
```go
// RECOVER protocol (defined in SYSTEM-DESIGN.md)
for attempt := 1; attempt <= 2; attempt++ {
    switch attempt {
    case 1:
        // Specific Attempt 1: recompute signature
        if recomputeSignature(d) { return success }
    case 2:
        // Specific Attempt 2: rebuild Neo4j cache
        if rebuildCache(d) { return success }
    }
}
escalateToHuman(d)  // No Attempt 3
```

---

### Mistake 2: Silent Plan Drift

❌ WRONG:
```go
// PLAN state locks: ["record", "sign", "index"]
// But EXECUTE decides to skip indexing on failure
if err := indexInNeo4j(d); err != nil {
    log.Warn("skipping index")
    // SILENTLY deviates from plan!
    return success
}
```

✓ RIGHT:
```go
// PLAN state locks: ["record", "sign", "index"]
// EXECUTE can't skip steps
if err := indexInNeo4j(d); err != nil {
    // Don't skip—report failure
    return failure(err)
    // Let RECOVER handle it (or escalate if unrecoverable)
}
```

---

### Mistake 3: Decisions Hidden in Error Handling

❌ WRONG:
```go
// Who decides to retry? The error handler. Invisible.
func (h *Handler) RecordDecision(w http.ResponseWriter, r *http.Request) {
    if err := h.ledger.Append(d); err != nil {
        if strings.Contains(err.Error(), "connection") {
            // Hidden decision to retry
            time.Sleep(100 * time.Millisecond)
            h.ledger.Append(d)  // Retry (how many times? unknown)
        }
    }
}
```

✓ RIGHT:
```go
// Explicit RECOVER state (defined protocol)
func (h *Handler) RecordDecision(w http.ResponseWriter, r *http.Request) {
    result := h.executeRecordDecision(d)  // EXECUTE
    if result.err != nil {
        action := h.recoverFromRecordFailure(result)  // RECOVER (bounded)
        if action.escalate {
            h.escalateToHuman(d, action.history)
        }
    }
}
```

---

### Mistake 4: Invariant Not Tested

❌ WRONG:
```
// Code assumes "decisions are immutable"
// But nothing verifies this
```

✓ RIGHT:
```go
// Test I1 (Immutability)
func TestImmutability(t *testing.T) {
    d1 := recordDecision(t, "original")
    
    // Try to modify in ledger file
    modifyLedgerFile(t, d1.ID, "modified")
    
    // Read it back
    d2, err := readDecision(t, d1.ID)
    
    // Must be rejected or unchanged
    if d2.Decision == "modified" {
        t.Fatal("Immutability violated: decision was modified")
    }
}
```

---

## Alignment with Global CLAUDE.md

| Global Principle | HomeBase Application |
|---|---|
| **Never assume** | Graph makes every state explicit (no hidden logic) |
| **Everything must be proved** | 6 invariants (I1-I6) + formal proofs |
| **Challenge everything** | Graph forces strict escalation (challenges "try again") |
| **Minimum code, maximum understanding** | Type system + state structure = clear code |
| **Service capability gate** | 5 states = "what HomeBase can do" |
| **Never auto-add co-author** | Don't silently change the plan mid-execution |

---

## Useful Commands

```bash
# Install git hooks (one-time)
./scripts/setup-hooks.sh

# Check for common bugs
./scripts/common-bugs-check.sh

# Collect evidence for a phase
./scripts/collect-phase-evidence.sh 1 202

# Verify phase gate (before pushing)
./.githooks/phase-gate-local --strict

# Build + test everything
make ci

# Run specific phase test
go test ./internal/graph -v

# Check test coverage
go test ./... -cover
```

---

## When in Doubt

1. **Read SYSTEM-DESIGN.md** (source of truth for the graph structure)
2. **Check which state you're in** (PLAN, EXECUTE, RECOVER, ESCALATE, REPEAT)
3. **Verify your action belongs in that state**
4. **Ask: which invariant does this protect?**
5. **If recovery, verify it's bounded (≤2 attempts)**

---

## Captain's Authority

- Design decisions: SYSTEM-DESIGN.md is locked (approved by Captain)
- Phase gates: Captain approves each phase sign-off
- Deviations: Captain only (explicit approval, tracked in tickets/OVERRIDES.md)

---

**Status:** DESIGN LOCKED (Phase 0)  
**Next:** Phase 1 Implementation (Tickets 202-205)  
**Timeline:** ~17-20 hours total work (vs 31 issues found in Phase 4 before)
