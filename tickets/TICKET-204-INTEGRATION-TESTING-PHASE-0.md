# TICKET 204: Integration Testing - Phase 0 Specification

**Date:** 2026-07-26  
**Phase:** 0 (Specification Review)  
**Status:** IN PROGRESS  
**Decision ID:** DEC-2026-07-26-003-INTEGRATION-TESTING  
**Depends On:** Ticket 201 (Core System) ✅ COMPLETE

---

## Overview

Ticket 204 fixes integration test infrastructure and implements comprehensive end-to-end workflow validation across all HomeBase components.

---

## Current Issues (From Ticket 201)

❌ **Test Isolation Broken:**
- Tests share state
- `:memory:` ledger treated as real file
- Causes test flakiness
- Blocks Phase 2 validation

❌ **Workflows Not Validated:**
- Record → Query → Verify flow untested
- Graceful degradation scenarios untested
- Error recovery untested
- Neo4j failure scenarios untested

---

## Phase 0 Requirements

### 1. Fix Test Isolation

**Problem:** Tests use `:memory:` as file path, not actual in-memory DB

**Solution:**
```go
// Create unique temp file per test
tmpFile := filepath.Join(t.TempDir(), "ledger.jsonl")
ledgerStore := ledger.NewStore(tmpFile)
// Clean isolation: each test gets fresh file
```

**Implementation:**
- Modify setupTestHandler() to use TempDir()
- Each test gets unique temp file
- Tests can run in parallel
- No state sharing between tests

---

### 2. End-to-End Workflow Tests

**Test 1: Record → Query → Verify**
```
1. Create decision with axioms
2. Query decision by ID
3. Verify signature
4. Verify all fields preserved
5. Clean up temp file
```

**Test 2: Graceful Degradation**
```
1. Start without Neo4j (simulate unavailable)
2. Create decision (should succeed)
3. Verify axiom check skipped with warning
4. Verify decision still logged
```

**Test 3: Error Recovery**
```
1. Create decision
2. Simulate ledger write failure
3. Verify error returned (not silent)
4. Verify no partial state
```

**Test 4: Concurrent Operations**
```
1. Spawn 10 goroutines
2. Each creates decision
3. All complete without collision
4. All decisions recoverable
```

---

### 3. Bridge Workflow Tests (NEW)

**Test 5: Complete Bridge Flow**
```
1. Create decision
2. Create escalation
3. Approve escalation
4. Bridge sends callback
5. Verify all linked correctly
```

**Test 6: Bridge Error Paths**
```
1. Invalid signature
2. Timeout scenarios
3. Network failures
4. All handled gracefully
```

---

### 4. Performance Baseline

**Metrics to establish:**
- Decision create latency (p50, p99)
- Query latency (p50, p99)
- Signature verification time
- Escalation creation time
- Bridge callback processing time

**Target:**
- Create: < 5ms
- Query: < 5ms
- Verify: < 5ms
- Escalate: < 5ms
- Callback: < 5ms

---

### 5. Durability Validation

**Test 7: Crash Recovery**
```
1. Create decision
2. Kill process (simulated)
3. Restart
4. Query decision
5. Verify all fields present
6. Verify ledger consistent
```

**Test 8: Concurrent Write Safety**
```
1. Multiple processes write simultaneously
2. All decisions recorded
3. No corruption
4. No lost data
5. Hash chain valid
```

---

## Test Structure

### File Organization
```
api/
  handlers_test.go        (unit tests - existing, fixed)
  integration_test.go     (end-to-end workflows)
  performance_test.go     (latency baselines)
  durability_test.go      (crash/recovery)
```

### Test Isolation Pattern
```go
func TestSomething(t *testing.T) {
  // Fresh temp directory per test
  tmpDir := t.TempDir()
  ledgerPath := filepath.Join(tmpDir, "ledger.jsonl")
  
  // Create handler with isolated ledger
  handler := setupTestHandler(t, ledgerPath)
  
  // Test proceeds with clean state
  // TempDir() cleans up automatically
}
```

---

## Error Scenarios

| Scenario | Expected | Verified |
|----------|----------|----------|
| Ledger write fails | 500 error, no partial state | ✓ |
| Decision duplicate | 409 conflict | ✓ |
| Neo4j unavailable | Warning, continues | ✓ |
| Signature invalid | 401 Unauthorized | ✓ |
| Network failure | Retry/timeout | ✓ |

---

## Acceptance Criteria

- [ ] Test isolation fixed (TempDir per test)
- [ ] 8+ end-to-end workflow tests
- [ ] All workflows PASS in isolation
- [ ] Performance baseline established
- [ ] Durability verified (crash recovery)
- [ ] Concurrent write safety proven
- [ ] Zero flaky tests (100 runs without failure)
- [ ] No manual cleanup required

---

## Dependencies

- Ticket 201 (Core ledger, signing, validation) ✅
- Ticket 202 (Bridge integration) ✅
- Standard library (testing, os, path/filepath)

---

## Implementation Strategy

**Phase 1 (Code):**
1. Fix setupTestHandler() for test isolation
2. Write Record → Query → Verify test
3. Write graceful degradation test
4. Write error recovery test
5. Write concurrent operations test
6. Write Bridge workflow tests
7. Establish performance baseline
8. Verify durability

**Phase 2 (Unit Tests):**
- Test each workflow component in isolation
- 80%+ coverage of test code itself

**Phase 3 (Integration Tests):**
- Run all 8 workflows together
- Verify no interference

**Phase 4 (Independent Review):**
- Fresh reviewer checks test quality
- Verifies coverage
- Checks for edge cases

---

## Timeline

- Phase 0: Specification (THIS WEEK) ✅ IN PROGRESS
- Phase 1: Fix isolation + implement tests (3-4 DAYS)
- Phase 2: Unit tests (1 DAY)
- Phase 3: Integration testing (1 DAY)
- Phase 4: Independent review (1 DAY)
- Phase 5: Production ready (NEXT WEEK)

---

**Phase 0 Status:** READY FOR ARCHITECTURE TEAM REVIEW
