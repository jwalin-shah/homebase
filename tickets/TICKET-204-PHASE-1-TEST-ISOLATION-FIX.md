# TICKET 204: Integration Testing - Phase 1 Implementation

**Date:** 2026-07-26  
**Phase:** 1 (Implementation)  
**Status:** IN PROGRESS  
**Focus:** Fix test isolation (critical blocker)

---

## Task 1: Fix setupTestHandler() for Test Isolation

**Current Problem:**
```go
func setupTestHandler(t *testing.T) *Handler {
  ledgerStore := ledger.NewStore(":memory:")  // ❌ WRONG
  // All tests share same :memory: store
}
```

**Solution:**
```go
func setupTestHandler(t *testing.T) *Handler {
  tmpDir := t.TempDir()  // ✅ CORRECT
  ledgerPath := filepath.Join(tmpDir, "ledger.jsonl")
  ledgerStore := ledger.NewStore(ledgerPath)
  // Each test gets isolated temp directory
  // Cleaned up automatically by testing framework
}
```

**Implementation:**

1. Update `handlers_test.go` setupTestHandler()
2. Replace `:memory:` with `t.TempDir()`
3. Create unique ledger file per test
4. All tests now fully isolated

**Expected Result:**
- Each test gets fresh ledger
- No state sharing between tests
- Tests can run in parallel
- Zero flaky tests

---

## Task 2: Write End-to-End Workflow Tests

**Test 1: Record → Query → Verify Flow**
```go
func TestRecordQueryVerifyFlow(t *testing.T) {
  handler := setupTestHandler(t)
  
  // 1. Create decision
  decision := &ledger.Decision{...}
  handler.ledger.Append(decision)
  
  // 2. Query decision
  retrieved, err := handler.ledger.Get(decision.ID)
  
  // 3. Verify all fields preserved
  assert decision == retrieved
  assert signature valid
  assert axioms intact
}
```

**Test 2: Graceful Degradation (Neo4j Missing)**
```go
func TestGracefulDegradationWithoutNeoJ(t *testing.T) {
  handler := setupTestHandler(t)
  handler.validator.cacheClient = nil  // Simulate Neo4j unavailable
  
  // Create decision should still work
  decision := &ledger.Decision{...}
  err := handler.ledger.Append(decision)
  assert err == nil  // Should succeed
  
  // Axiom check skipped but logged as warning
}
```

**Test 3: Error Recovery**
```go
func TestErrorRecovery(t *testing.T) {
  handler := setupTestHandler(t)
  
  // Simulate ledger write failure
  // (close file descriptor, etc)
  
  // Operation should fail with clear error
  err := handler.ledger.Append(decision)
  assert err != nil
  assert "write failed" in err message
  
  // No partial state left behind
  count := handler.ledger.List(100)
  assert count == 0
}
```

**Test 4: Concurrent Operations**
```go
func TestConcurrentOperations(t *testing.T) {
  handler := setupTestHandler(t)
  
  // Spawn 10 goroutines
  // Each creates decision with unique ID
  // All complete without collision
  
  // Verify all 10 decisions recorded
  decisions := handler.ledger.List(100)
  assert len(decisions) == 10
  
  // All recoverable
  for _, d := range decisions {
    retrieved, err := handler.ledger.Get(d.ID)
    assert err == nil
  }
}
```

**Test 5: Bridge Integration End-to-End**
```go
func TestBridgeIntegrationEndToEnd(t *testing.T) {
  handler := setupTestHandler(t)
  
  // 1. Create decision
  decision := createTestDecision()
  handler.ledger.Append(decision)
  
  // 2. Create escalation
  escalation := createTestEscalation(decision.ID)
  err := handler.ledger.Append(escalation)
  assert err == nil
  
  // 3. Approve escalation
  approval := createTestApproval(escalation.ID)
  err := handler.ledger.Append(approval)
  assert err == nil
  
  // 4. Bridge callback
  callback := createTestCallback(escalation.ID)
  err := handler.ledger.Append(callback)
  assert err == nil
  
  // 5. Verify all linked correctly
  allDecisions := handler.ledger.List(100)
  assert verifyChain(allDecisions)
}
```

---

## Task 3: Performance Baseline Testing

**Baseline Metrics to Establish:**

```go
func BenchmarkDecisionCreate(b *testing.B) {
  handler := setupTestHandler(b)
  
  for i := 0; i < b.N; i++ {
    decision := &ledger.Decision{...}
    handler.ledger.Append(decision)
  }
  
  // Report: ns/op, should be < 5ms
}

func BenchmarkQuery(b *testing.B) {
  handler := setupTestHandler(b)
  
  for i := 0; i < b.N; i++ {
    handler.ledger.Get(testDecisionID)
  }
  
  // Report: ns/op, should be < 5ms
}

func BenchmarkSignature(b *testing.B) {
  handler := setupTestHandler(b)
  
  for i := 0; i < b.N; i++ {
    handler.signer.SignJSON(&decision)
  }
  
  // Report: ns/op, should be < 2ms
}
```

---

## Task 4: Durability Validation

**Test Crash Recovery:**
```go
func TestCrashRecovery(t *testing.T) {
  tmpDir := t.TempDir()
  ledgerPath := filepath.Join(tmpDir, "ledger.jsonl")
  
  // Process 1: Create and persist decision
  {
    handler := setupTestHandler(t)
    decision := createTestDecision()
    handler.ledger.Append(decision)
    // Process exits (simulated crash)
  }
  
  // Process 2: Restart and verify
  {
    handler := setupTestHandler(t)
    retrieved, err := handler.ledger.Get(decision.ID)
    assert err == nil
    assert retrieved == decision
    assert signature valid
    assert hash chain intact
  }
}
```

**Test Concurrent Write Safety:**
```go
func TestConcurrentWriteSafety(t *testing.T) {
  handler := setupTestHandler(t)
  
  // 20 goroutines writing simultaneously
  done := make(chan error, 20)
  
  for i := 0; i < 20; i++ {
    go func(id int) {
      decision := createDecision(id)
      done <- handler.ledger.Append(decision)
    }(i)
  }
  
  // Collect results
  for i := 0; i < 20; i++ {
    err := <-done
    assert err == nil  // All succeed
  }
  
  // Verify all recorded
  decisions := handler.ledger.List(100)
  assert len(decisions) == 20
  
  // Verify hash chain valid
  assert verifyHashChain(decisions)
}
```

---

## Implementation Checklist

- [ ] Task 1: Update setupTestHandler() (30 min)
- [ ] Task 2: Write 5 workflow tests (2 hours)
- [ ] Task 3: Performance baselines (1 hour)
- [ ] Task 4: Durability validation (1 hour)
- [ ] Run full test suite: 100+ iterations, zero flakes (2 hours)
- [ ] Code review (1 hour)
- [ ] Finalize Phase 1 (30 min)

**Estimated Phase 1 Time:** 8 hours (1 full day)

---

## Success Criteria

- [ ] All tests pass in isolation
- [ ] All tests pass in parallel
- [ ] Zero flaky tests (100 runs)
- [ ] No state sharing between tests
- [ ] Performance baselines established (all < target)
- [ ] Crash recovery verified
- [ ] Concurrent write safety proven
- [ ] Ledger hash chain always valid

---

**Phase 1 Status:** STARTING NOW

Next: Proceed to Task 1 (fix setupTestHandler)
