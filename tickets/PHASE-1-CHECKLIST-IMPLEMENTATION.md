# Phase 1 Checklist: Implementation

**Goal:** Verify code matches specification. Prevent incomplete implementations from reaching testing.

**Owner:** Implementation Team (builds), Architect (gates Phase 2)

**Blocking:** No Phase 2 (Unit Tests) starts until this checklist is 100% complete and Architect signs off.

---

## Implementation Team Checklist

### Item 0: Verify Phase 0 Is Complete (Not Partial)

**BLOCKING ITEM: Must check before starting any Phase 1 work**

- [ ] Phase 0 specification exists and is complete (2-5 pages, not stub)
- [ ] Discovery Lead has signed off (signature in PHASE-SIGN-OFF-SHEET.md)
- [ ] Architect has reviewed and signed off (signature in PHASE-SIGN-OFF-SHEET.md)
- [ ] All 5 required roles have signed (Discovery Lead, Architect, Implementation, Tester, Auditor placeholders)
- [ ] No "TODO" or "incomplete" markers in Phase 0 spec
- [ ] Risk assessment is complete (not "we'll handle this in Phase 1")
- [ ] Architecture sketch is clear (can you draw the data flow in 30 mins?)

**If ANY of above is unchecked:** DO NOT START Phase 1 work. Send Phase 0 back for completion.

**Why this matters:** Phase 1 assumes Phase 0 is done. If spec is incomplete, implementation will be incomplete. If risks aren't identified, Phase 1 will discover them (wasting time). If architecture is unclear, you'll rewrite code mid-Phase 1.

**Captain Override:** If Phase 0 needs work but is "mostly done," do NOT proceed with Phase 1 in parallel. Complete Phase 0 first. (This violates the "no partial" rule and will cost time, not save it.)

---

### Code Completeness
- [ ] All functions from Phase 0 spec are implemented
- [ ] All handlers/endpoints defined in spec exist
- [ ] All data structures (ledger format, decision schema) are in place
- [ ] All error paths are handled (not just happy path)

**Verification:**
```bash
# Does code match spec?
# Count: handlers in spec → handlers in code (should be ≥ spec)
# Count: data structures in spec → data structures in code (should be ≥ spec)
# Check: every code path has error handling (look for unhandled `err`)
```

### Code Quality
- [ ] Code compiles without errors: `go build ./...`
- [ ] Code passes linting: `golangci-lint run ./...` (or equivalent)
- [ ] No shadowed variables or unused imports
- [ ] No `TODO` comments without a ticket number
- [ ] Variable names are clear (no single-letter vars except loop counters)

**Red Flags:**
```
❌ "It compiles but golangci-lint has warnings"
❌ "We have a TODO but we'll fix it later"
❌ "The error path isn't fully tested, but we'll get to it"
❌ "One handler is missing but it's simple"
```

### Error Handling (CRITICAL)
- [ ] Every function that can fail has an error return
- [ ] Every error return is checked (no `_ = function()` that should check err)
- [ ] Error messages are specific (not just "error" or "failed")
- [ ] Errors propagate up (or are explicitly handled/logged)
- [ ] No silent failures (e.g., `rand.Read()` error not checked)

**Verify:**
```bash
# Search for unchecked errors
grep -n "_ =" *.go | grep -v "for _" | grep -v "range"
# Search for error returns not on same line
grep -n "err :=" *.go | head -20  # spot check each one
```

### External Dependencies
- [ ] All imports are from standard library or `go.mod` approved packages
- [ ] No circular dependencies between modules
- [ ] Dependencies match `go.mod` file
- [ ] Version pins are reasonable (not ancient, not bleeding edge)

**Verify:**
```bash
go mod verify
go mod graph | grep -c "^"  # count dependencies
```

### Integration Points
- [ ] All integration points from spec are implemented (e.g., Neo4j calls, ledger persistence)
- [ ] Integration points have graceful fallbacks (e.g., Neo4j down → graceful degradation)
- [ ] External services are optional or have timeout guards
- [ ] No hardcoded IPs/ports (should be config)

**Red Flags:**
```
❌ "Neo4j integration is stubbed out, we'll do it in Phase 2"
❌ "We're calling external API without a timeout"
❌ "If the service is down, the entire system fails"
❌ "Localhost:7474 is hardcoded in the code"
```

### Configuration & Deployment
- [ ] Configuration is externalized (not hardcoded)
- [ ] Command-line flags or env vars for: ledger path, Neo4j address, listen port
- [ ] Default config is sensible (e.g., `LEDGER_PATH=./ledger.jsonl`)
- [ ] Dockerfile/deployment config exists (even if just stub)

**Verify:**
```bash
grep -r "localhost" *.go | grep -v "127.0.0.1" | grep -v test
# Should have 0 matches (except in tests)
```

---

## Architect Checklist (Phase 1 Review)

**Goal:** Ensure implementation matches architecture design from Phase 0 spec.

### Specification Adherence
- [ ] I've re-read the Phase 0 specification (5 mins)
- [ ] Every requirement in spec is implemented in code
- [ ] Every data structure defined in spec is present
- [ ] Every integration point from spec is in place

**Questions to Ask:**
```
"Did we implement what we said we'd implement?" → Should see one-to-one mapping
"Are there extra features we added?" → Unexpected = red flag (scope creep)
"Are there features from spec that are missing?" → Blocker for Phase 2
```

### Architecture Integrity
- [ ] Module structure matches the design sketch from Phase 0
- [ ] Interfaces are well-defined (not everything passed as interface{})
- [ ] Data flow is clear (where does data come from, how does it flow, where does it go?)
- [ ] No circular dependencies between modules
- [ ] No god objects (modules doing too many things)

**Red Flags:**
```
❌ "Everything is in main.go"
❌ "The handler does database, signing, API response, and logging"
❌ "I can't trace data flow without reading 500 lines"
```

### Critical Invariants Checked
- [ ] **Invariant 1 (Immutability):** Once written to ledger, can data be changed? (Should: NO)
- [ ] **Invariant 2 (Uniqueness):** Can escalation be approved twice? (Should: NO, guarded by mutex/check)
- [ ] **Invariant 3 (Durability):** Is data persisted (fsync) or just buffered? (Should: persisted)
- [ ] **Invariant 4 (Integrity):** Are signatures verified on read? (Should: YES)
- [ ] **Invariant 5 (Correlation):** Does every request have correlation ID? (Should: YES, generated if missing)

**Verify:**
```go
// Invariant 1: Check ledger append is "add only"
// ledger.Append() should not have Update() or Delete()

// Invariant 2: Check escalation approval
// ApproveEscalation() should check if already approved before creating another approval

// Invariant 3: Check persistence
// File write should call fsync() or equivalent

// Invariant 4: Check signature
// Decision read should verify signature before returning

// Invariant 5: Check correlation ID
// getCorrelationID() should generate if missing, not return empty
```

### Error Paths Verified
- [ ] I've read the error handling code (not just tests)
- [ ] Error messages are informative (not generic "failed")
- [ ] Errors propagate correctly (caller can see what went wrong)
- [ ] No panic() calls in non-test code
- [ ] No infinite retry loops (errors that will never succeed)

**Questions:**
```
"If Neo4j is down, what happens?" → Code should handle gracefully
"If ledger is read-only, what's the error?" → Should be clear
"If signature is invalid, what happens?" → Should reject cleanly
```

### Code Review Focus Areas

**Search these patterns:**
```bash
# Unhandled errors
grep -n "_ = " *.go | grep -v "for _" | grep -v test

# Global variables (risky for concurrency)
grep -n "^var " *.go

# Hardcoded values that should be config
grep -n "localhost\|:7474\|:9200" *.go | grep -v test

# Unbounded loops
grep -n "for {" *.go | grep -v "break\|return"

# Race condition smell (multiple goroutines accessing same map)
grep -n "go " *.go  # Check for goroutines + shared state
```

### Performance Assumptions
- [ ] Spec says latency requirement (e.g., <100ms)
- [ ] I've spot-checked 1-2 critical paths for performance issues
- [ ] No obvious O(N^2) algorithms in hot paths
- [ ] Database queries have indexes (or will be added in Phase 3)
- [ ] No unbounded result sets (queries should have limits)

**Red Flag:**
```
❌ "Querying all decisions in a loop to find one by ID"
❌ "No database indices"
❌ "Ledger is read into memory every request"
```

---

## Joint Sign-Off (Implementation Team + Architect)

Before Phase 1 is complete:

### Implementation Team
- [ ] Code is complete per spec
- [ ] Code compiles without warnings
- [ ] Linting passes
- [ ] Error handling is explicit throughout
- [ ] Ready for unit testing

**Signature:** _____________________ **Date:** _________

### Architect
- [ ] Code matches specification
- [ ] Architecture integrity is good (no circular deps, clear data flow)
- [ ] Critical invariants are enforced in code (immutability, uniqueness, durability, integrity, correlation)
- [ ] Error paths are handled correctly
- [ ] Performance is acceptable for spec requirements
- [ ] **Phase 2 (Unit Tests) can proceed**

**Signature:** _____________________ **Date:** _________

---

## Common Phase 1 Failures (That Phase 4 Catches)

**❌ "Error checking on rand.Read() was skipped"**  
→ Result: Correlation IDs lose uniqueness component, become predictable

**❌ "Escalation approval checked but race condition between check and create"**  
→ Result: Escalation approved twice, duplicate Bridge calls

**❌ "Health check appends test decision to ledger"**  
→ Result: Ledger fills with thousands of noise entries, audit trail corrupted

**❌ "Input validation missing from query parameters"**  
→ Result: DoS possible with huge axiom IDs or malicious input

**❌ "No rate limiting on expensive operations"**  
→ Result: RebuildCache called repeatedly, resource exhaustion

---

## Success Criteria

Phase 1 is DONE when:

1. ✓ All handlers/functions from spec are implemented
2. ✓ Code compiles without warnings
3. ✓ Linting passes (`golangci-lint`, `gofmt`)
4. ✓ Error handling is explicit (no unhandled errors)
5. ✓ External dependencies have graceful fallbacks
6. ✓ Configuration is externalized (not hardcoded)
7. ✓ Critical invariants are enforced in code
8. ✓ Implementation Team signed off
9. ✓ Architect reviewed and signed off
10. ✓ Ready to move to Phase 2 (Unit Tests)

---

**Phase 1 typically takes 4-6 hours.**  
If it's taking much longer, ask: are we over-engineering, or is the spec unclear?
