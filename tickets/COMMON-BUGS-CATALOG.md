# Common Bugs Catalog

**Purpose:** Track concrete bugs found in HomeBase Phase 4 audit + general anti-patterns. Focus on easily verifiable checks, not aspirational quality statements.

**Authority:** Derived from Phase 4 findings (12 bugs) + historical software defects

---

## Category 1: Cryptography Failures

### Bug 1.1: Signature Verification Skipped Entirely

**Seen in HomeBase?** YES (Bridge responses)

**Anti-Pattern:**
```go
// WRONG: Signature field accepted without verification
func (h *Handler) BridgeCallback(w http.ResponseWriter, r *http.Request) {
    var req BridgeCallbackRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // Signature is read but never validated
    escalation.BridgeSignature = req.Signature
    h.ledger.Append(escalation)  // ← Recorded without verification
    
    w.WriteHeader(http.StatusOK)
}
```

**Why it's bad:** Attacker forges any response. Non-repudiation is broken. Audit trail is falsified.

**Verification Strategy:**
```bash
# Find all signature fields created/accepted
grep -n "Signature.*=" *.go | head -20

# For each assignment, verify there's a corresponding Verify() call
# PASS: if every `.Signature = ` has a prior Verify() or ValidateSignature()
# FAIL: if any signature is used without verification
```

**Test:**
```go
func TestBridgeSignatureVerification(t *testing.T) {
    // Try to send callback with forged signature
    forgedResponse := BridgeResponse{
        EscalationID: "esc-123",
        Analysis: "FAKE ANALYSIS",
        Signature: "forged_sig_xyz",  // Not signed by Bridge
    }
    
    resp := postJSON(t, "/api/v1/bridge/callback", forgedResponse)
    
    // PASS: 401 Unauthorized or 400 Bad Request (signature invalid)
    // FAIL: 200 OK (signature accepted without verification)
    if resp.StatusCode == 200 {
        t.Fatal("Forged signature accepted - verification not working")
    }
}
```

**Fix:**
```go
func (h *Handler) BridgeCallback(w http.ResponseWriter, r *http.Request) {
    var req BridgeCallbackRequest
    json.NewDecoder(r.Body).Decode(&req)
    
    // REQUIRED: Verify signature before accepting
    if err := h.verifier.VerifyBridgeResponse(req.Response, req.Signature); err != nil {
        w.WriteHeader(http.StatusUnauthorized)
        json.NewEncoder(w).Encode(map[string]string{"error": "invalid signature"})
        return
    }
    
    escalation.BridgeSignature = req.Signature
    h.ledger.Append(escalation)  // ← Only recorded after verification
    w.WriteHeader(http.StatusOK)
}
```

---

### Bug 1.2: Signature Verification Circular Reference

**Seen in HomeBase?** YES (Decision.Signature field included in hash)

**Anti-Pattern:**
```go
// WRONG: Sign data with Signature=""
decision.Signature = ""
sig := Sign(marshal(decision))  // Signs: {"id":"...", "signature":""}

// Later, set signature
decision.Signature = sig  // Now: {"id":"...", "signature":"abc123..."}

// Try to verify
Verify(marshal(decision), sig)  // Verifies: {"id":"...", "signature":"abc123..."}
// ← Mismatch! Originally signed without signature field, now verifying with it
```

**Why it's bad:** Signatures always fail to verify. Verification logic is completely broken.

**Verification Strategy:**
```bash
# Find all Sign() calls and Verify() calls
grep -n "Sign\|Verify" *.go

# For each pair, check: what fields are included in marshaling?
# PASS: Sign and Verify marshal identical data
# FAIL: Sign includes field F1, Verify includes field F2, F1 ≠ F2
```

**Test:**
```go
func TestSignatureRoundTrip(t *testing.T) {
    decision := Decision{
        ID: "dec-001",
        Decision: "Use Redis",
        Axioms: []string{"AX-PERF"},
    }
    
    // Sign it
    sig, _ := signer.Sign(&decision)
    
    // Verify it (most basic test - does signing work?)
    if err := verifier.Verify(&decision, sig); err != nil {
        t.Fatalf("Verify(Sign(d)) failed: %v - circular reference bug", err)
    }
    
    // Modify one field, should fail
    decision.Decision = "Use Memcached"
    if err := verifier.Verify(&decision, sig); err == nil {
        t.Fatalf("Verify accepted modified data - not detecting tampering")
    }
}
```

**Fix:**
```go
// Define unsigned decision (without signature)
type UnsignedDecision struct {
    ID       string
    Decision string
    Axioms   []string
    // Signature field NOT included
}

// Sign only includes unsigned fields
func Sign(d *UnsignedDecision) (string, error) {
    jsonBytes := marshal(d)  // Only marshals UnsignedDecision
    return hash_and_sign(jsonBytes), nil
}

// Verify also only checks unsigned fields
func Verify(d *UnsignedDecision, sig string) error {
    jsonBytes := marshal(d)  // Same structure
    return hash_and_verify(jsonBytes, sig)
}
```

---

### Bug 1.3: Hash Chain Excludes Security-Relevant Fields

**Seen in HomeBase?** YES (Signature excluded from hash)

**Anti-Pattern:**
```go
// WRONG: Hash doesn't include signature
func computeHash(d *Decision) string {
    // Signature field excluded
    content := fmt.Sprintf("%s:%s:%v:%s", d.ID, d.Decision, d.Axioms, d.RecordedAt)
    return sha256(content)
}

// Attacker can modify signature in ledger file:
// Before: {"signature": "real_sig_abc123", "hash": "def456"}
// After:  {"signature": "fake_sig_xyz789", "hash": "def456"}  ← hash unchanged!
```

**Why it's bad:** Signature tampering is undetectable. Hash chain doesn't catch it.

**Verification Strategy:**
```bash
# Find hash computation logic
grep -n "computeHash\|sha256\|Hash.*=" *.go

# Verify: all security-relevant fields (ID, data, signature) are included
# PASS: hash includes ID, Decision, Axioms, Signature, RecordedAt
# FAIL: hash excludes any of these
```

**Test:**
```go
func TestHashIncludesAllSecurityFields(t *testing.T) {
    d := Decision{ID: "dec-001", Decision: "...", Signature: "sig_abc"}
    hash1 := computeHash(d)
    
    // If we change signature, hash must change
    d.Signature = "sig_xyz"
    hash2 := computeHash(d)
    
    // PASS: hash1 ≠ hash2 (signature change detected)
    // FAIL: hash1 == hash2 (signature not in hash)
    if hash1 == hash2 {
        t.Fatal("Hash doesn't include Signature field - tampering undetectable")
    }
}
```

**Fix:**
```go
func computeHash(d *Decision) string {
    // Include ALL fields (including signature)
    content := fmt.Sprintf("%s:%s:%v:%s:%s", 
        d.ID, d.Decision, d.Axioms, d.RecordedAt, d.Signature)
    return sha256(content)
}
```

---

## Category 2: Error Handling Failures

### Bug 2.1: Silent Error Handling (Unhandled Error Returns)

**Seen in HomeBase?** YES (rand.Read(), signature checks)

**Anti-Pattern:**
```go
// WRONG: Error return ignored
randomBytes := make([]byte, 8)
rand.Read(randomBytes)  // ← Error not checked; might be partially filled if entropy exhausted

// WRONG: Function can fail silently
if err := file.Sync(); err != nil {
    // Silently ignored or logged without context
}
```

**Why it's bad:** Errors go unnoticed. Entropy-exhausted correlation IDs become predictable. Data not durably written.

**Verification Strategy:**
```bash
# Find unhandled error returns
grep -n "_ = " *.go | grep -v "for _" | grep -v range

# For each match, verify:
# - It's intentional (commented with reason)
# - Or it's a genuine bug
# PASS: every error return is explicitly handled
# FAIL: any error is silently ignored
```

**Test:**
```go
func TestRandReadErrorHandled(t *testing.T) {
    // Can't easily test entropy exhaustion, but can verify:
    // If rand.Read returned error, generateCorrelationID should fail
    
    // In production: correlation ID should never be predictable
    id1 := generateCorrelationID()
    id2 := generateCorrelationID()
    
    // PASS: id1 ≠ id2 (different correlation IDs)
    // FAIL: id1 == id2 or pattern detected (predictable = entropy issue)
    if id1 == id2 {
        t.Fatal("Correlation IDs not unique - rand.Read error not handled?")
    }
}
```

**Fix:**
```go
func generateCorrelationID() (string, error) {
    randomBytes := make([]byte, 8)
    if _, err := rand.Read(randomBytes); err != nil {
        return "", fmt.Errorf("entropy source exhausted: %w", err)
    }
    
    return fmt.Sprintf("corr-%d-%s", 
        time.Now().UnixNano(), 
        hex.EncodeToString(randomBytes)), nil
}
```

---

### Bug 2.2: No Graceful Degradation for Critical Resources

**Seen in HomeBase?** YES (Ledger failures)

**Anti-Pattern:**
```go
// WRONG: System fails if ledger is down
func (h *Handler) RecordDecision(...) {
    if err := h.ledger.Append(&decision); err != nil {
        w.WriteHeader(http.StatusInternalServerError)  // ← System down, no fallback
        return
    }
}

// Neo4j has fallback (query directly from ledger), ledger has none
```

**Why it's bad:** Single point of failure. If ledger is read-only or full, everything stops.

**Verification Strategy:**
```bash
# Find all error returns to user (StatusInternalServerError)
grep -n "StatusInternalServerError\|500" *.go

# For each, check if there's a fallback:
# PASS: Critical resource failure has fallback (cache, queue, degraded mode)
# FAIL: Critical resource failure has no fallback
```

**Test:**
```go
func TestRecordDecisionWithReadOnlyLedger(t *testing.T) {
    // Make ledger read-only
    os.Chmod(ledgerPath, 0444)
    defer os.Chmod(ledgerPath, 0644)
    
    // Try to record decision
    resp := postJSON(t, "/api/v1/decisions", decision)
    
    // PASS: 503 Service Unavailable with retry hint, or queued for later
    // FAIL: 500 Internal Server Error with no guidance
    if resp.StatusCode == 500 {
        body := resp.Body // should have clear error message or retry info
        if !strings.Contains(body, "retry") && !strings.Contains(body, "queue") {
            t.Fatal("Ledger failure has no graceful fallback")
        }
    }
}
```

**Fix:**
```go
func (h *Handler) RecordDecision(...) {
    if err := h.ledger.Append(&decision); err != nil {
        // Check if it's a permission/disk error
        if isPermanentError(err) {
            // Can't recover, must fail
            w.WriteHeader(http.StatusServiceUnavailable)
            // But queue for retry when ledger is available
            h.writeQueue.Add(&decision)  // Fallback queue
            return
        }
        
        w.WriteHeader(http.StatusInternalServerError)
        return
    }
}
```

---

## Category 3: Input Validation Failures

### Bug 3.1: Unvalidated External Input

**Seen in HomeBase?** YES (Query parameters, correlation ID)

**Anti-Pattern:**
```go
// WRONG: No length check on user input
axiom := r.URL.Query().Get("axiom")
results, err := queryAxioms(axiom)  // axiom could be 10MB string

// WRONG: No character set validation
correlationID := r.Header.Get("X-Correlation-ID")
// Could contain: newlines, special chars for log injection
log.Printf("correlation_id=%s event=request", correlationID)
// If correlationID = "a\nAlert: system hacked!", logs are corrupted
```

**Why it's bad:** DoS via oversized input. Log injection. Injection attacks.

**Verification Strategy:**
```bash
# Find all user input sources
grep -n "Query.Get\|Header.Get\|body.Decode" *.go

# For each, verify validation before use
# PASS: every input has length check, charset check, or type validation
# FAIL: any input used without validation
```

**Test:**
```go
func TestOversizedQueryParameter(t *testing.T) {
    // Try to query with 10MB axiom parameter
    oversized := strings.Repeat("A", 10*1024*1024)
    
    resp := makeRequest(t, "GET", "/api/v1/decisions?axiom="+oversized)
    
    // PASS: 400 Bad Request (input too large)
    // FAIL: 200 OK or 500 (oversized input processed)
    if resp.StatusCode == 200 || resp.StatusCode == 500 {
        t.Fatal("No validation on oversized input - DoS possible")
    }
}

func TestLogInjectionViaCorrelationID(t *testing.T) {
    // Try correlation ID with newlines
    injected := "req-001\nAlert: system compromised"
    
    resp := makeRequest(t, "POST", "/api/v1/decisions", 
        map[string]string{"X-Correlation-ID": injected})
    
    // Read logs
    logs := readLogs()
    
    // PASS: newlines escaped or stripped
    // FAIL: newlines in logs (log injection)
    if strings.Contains(logs, "Alert: system compromised") {
        t.Fatal("Correlation ID not validated - log injection possible")
    }
}
```

**Fix:**
```go
const MaxAxiomLength = 255

func (h *Handler) QueryDecisionsByAxiom(w http.ResponseWriter, r *http.Request) {
    axiom := r.URL.Query().Get("axiom")
    
    // Validation before use
    if len(axiom) == 0 {
        http.Error(w, "axiom required", http.StatusBadRequest)
        return
    }
    if len(axiom) > MaxAxiomLength {
        http.Error(w, "axiom too long", http.StatusBadRequest)
        return
    }
    if !isValidAxiomFormat(axiom) {
        http.Error(w, "axiom invalid characters", http.StatusBadRequest)
        return
    }
    
    // Only now use axiom
    results, _ := h.cache.QueryDecisionsByAxiom(axiom)
    json.NewEncoder(w).Encode(results)
}

func validateCorrelationID(id string) bool {
    if len(id) == 0 || len(id) > 256 {
        return false
    }
    // Only alphanumeric, hyphen, underscore
    return regexp.MustCompile(`^[a-zA-Z0-9\-_]+$`).MatchString(id)
}
```

---

### Bug 3.2: Unbounded Queries/Data

**Seen in HomeBase?** YES (Query results without LIMIT)

**Anti-Pattern:**
```go
// WRONG: Query returns all results
func (c *Client) QueryDecisionsByAxiom(axiom string) ([]Decision, error) {
    results := cypher(`
        MATCH (d:Decision) -[:CITES]-> (a:Axiom)
        WHERE a.id = $axiom
        RETURN d
    `, axiom)
    
    return results  // Could be millions of rows!
}
```

**Why it's bad:** Memory exhaustion. Timeout. DoS.

**Verification Strategy:**
```bash
# Find all database queries
grep -n "MATCH\|SELECT\|cypher\|query" *.go

# Verify all have LIMIT
# PASS: Every query has LIMIT clause
# FAIL: Any query without LIMIT
```

**Test:**
```go
func TestQueryResultsHaveLimit(t *testing.T) {
    // Create Neo4j with many decisions citing one axiom
    for i := 0; i < 100000; i++ {
        addDecision(t, fmt.Sprintf("dec-%d", i), axiom="AX-TEST")
    }
    
    // Query by axiom
    results, _ := queryByAxiom("AX-TEST")
    
    // PASS: results ≤ 10000 (LIMIT enforced)
    // FAIL: results > 10000 (unbounded)
    if len(results) > 10000 {
        t.Fatal("Query unbounded - memory exhaustion possible")
    }
}
```

**Fix:**
```go
const MaxQueryResults = 10000

func (c *Client) QueryDecisionsByAxiom(ctx context.Context, axiom string) ([]Decision, int, error) {
    results := cypher(`
        MATCH (d:Decision) -[:CITES]-> (a:Axiom)
        WHERE a.id = $axiom
        RETURN d
        LIMIT $limit
    `, map[string]interface{}{
        "axiom": axiom,
        "limit": MaxQueryResults,
    })
    
    return results, len(results), nil  // Return actual count
}
```

---

## Category 4: State Machine Failures

### Bug 4.1: State Transitions Not Enforced

**Seen in HomeBase?** YES (Escalation status)

**Anti-Pattern:**
```go
// WRONG: Status can be set to any value
escalation.Status = "PENDING"
escalation.Status = "APPROVED"
escalation.Status = "EXPIRED"  // ← Valid?
escalation.Status = "INVALID_STATUS"  // ← Also accepted?

// No guard checks
if someCondition {
    escalation.Status = "WEIRD_STATE"  // ← Anything goes
}
```

**Why it's bad:** Invalid state transitions. Audit trail has contradictory states.

**Verification Strategy:**
```bash
# Find all status assignments
grep -n "\.Status = \|status =" *.go

# For each, verify it's guarded by legitimate transition check
# PASS: Every status change has explicit guard ("must be PENDING to approve", etc.)
# FAIL: Any status change without guard
```

**Test:**
```go
func TestInvalidStatusTransitions(t *testing.T) {
    e := Escalation{Status: "PENDING"}
    
    // Try invalid transition
    e.Status = "COMPLETED"  // ← No such status in FSM
    
    if err := e.Validate(); err == nil {
        t.Fatal("Invalid status accepted - FSM not enforced")
    }
}
```

**Fix:**
```go
// Define valid status values
type EscalationStatus string

const (
    StatusPending   EscalationStatus = "PENDING"
    StatusApproved  EscalationStatus = "APPROVED"
    StatusRejected  EscalationStatus = "REJECTED"
    StatusExpired   EscalationStatus = "EXPIRED"
)

// Only allow valid statuses (compile-time check)
type Escalation struct {
    Status EscalationStatus  // Not string
}

// Transitions guarded by FSM
func (e *Escalation) Approve() error {
    if e.Status != StatusPending {
        return fmt.Errorf("can only approve PENDING, got %s", e.Status)
    }
    e.Status = StatusApproved
    return nil
}

func (e *Escalation) Reject() error {
    if e.Status != StatusPending {
        return fmt.Errorf("can only reject PENDING, got %s", e.Status)
    }
    e.Status = StatusRejected
    return nil
}
```

---

### Bug 4.2: Expiration Never Enforced

**Seen in HomeBase?** YES (ExpiresAt timestamp set but not checked)

**Anti-Pattern:**
```go
// WRONG: Expiration time set but never used
escalation.ExpiresAt = now.Add(24 * time.Hour)

// Later, no code checks if escalation is expired
// Approval recorded even if ExpiresAt is in the past
```

**Why it's bad:** Stale approvals recorded. Business logic broken.

**Verification Strategy:**
```bash
# Find all ExpiresAt assignments
grep -n "ExpiresAt" *.go

# Verify there's corresponding expiration check
# PASS: every ExpiresAt has guard checking `now < ExpiresAt`
# FAIL: any ExpiresAt without corresponding check
```

**Test:**
```go
func TestApprovalAfterExpiration(t *testing.T) {
    // Create escalation that expires in 1 minute
    e := Escalation{
        ID: "esc-123",
        ExpiresAt: time.Now().Add(1 * time.Minute),
    }
    
    // Wait 2 minutes
    time.Sleep(2 * time.Minute)
    
    // Try to approve
    err := e.Approve()
    
    // PASS: error "escalation expired"
    // FAIL: approval succeeds (expiration not checked)
    if err == nil {
        t.Fatal("Approval allowed after expiration - no expiration check")
    }
}
```

**Fix:**
```go
func (e *Escalation) Approve() error {
    // Check expiration FIRST
    if time.Now().After(e.ExpiresAt) {
        e.Status = StatusExpired
        return fmt.Errorf("escalation has expired")
    }
    
    if e.Status != StatusPending {
        return fmt.Errorf("can only approve PENDING")
    }
    
    e.Status = StatusApproved
    return nil
}
```

---

## Category 5: Concurrency Failures

### Bug 5.1: Race Conditions (Concurrent Access Without Synchronization)

**Seen in HomeBase?** YES (Double-approval race)

**Anti-Pattern:**
```go
// WRONG: Check then act without lock
if escalation.Status == "PENDING" {  // ← Check
    // Between check and act, another goroutine could also pass the check
    escalation.Status = "APPROVED"    // ← Act (race condition window)
}
```

**Why it's bad:** Multiple approvals recorded for single escalation. Idempotency violated.

**Verification Strategy:**
```bash
# Run tests with race detector
go test -race ./...

# Any race detected = bug
# PASS: no races detected
# FAIL: any race (even if tests pass)
```

**Test:**
```go
func TestConcurrentApprovals(t *testing.T) {
    e := Escalation{ID: "esc-123", Status: "PENDING"}
    
    // Launch 5 concurrent approval attempts
    var wg sync.WaitGroup
    results := make([]error, 5)
    
    for i := 0; i < 5; i++ {
        wg.Add(1)
        go func(idx int) {
            results[idx] = e.Approve()
            wg.Done()
        }(i)
    }
    
    wg.Wait()
    
    // PASS: exactly 1 approval succeeded, 4 got conflict error
    successCount := 0
    for _, err := range results {
        if err == nil {
            successCount++
        }
    }
    
    if successCount != 1 {
        t.Fatalf("Race condition: %d approvals succeeded (expected 1)", successCount)
    }
}
```

**Fix:**
```go
func (e *Escalation) Approve() error {
    // Lock BEFORE checking status
    e.mu.Lock()
    defer e.mu.Unlock()
    
    // Now check and act atomically
    if e.Status != StatusPending {
        return fmt.Errorf("already resolved: %s", e.Status)
    }
    
    e.Status = StatusApproved
    return nil
}
```

---

## Category 6: Test Isolation Failures

### Bug 6.1: Shared State Across Tests (Test Pollution)

**Seen in HomeBase?** YES (`:memory:` ledger shared across tests)

**Anti-Pattern:**
```go
// WRONG: Shared ledger across tests
func setupTestHandler() *Handler {
    ledger := NewStore(":memory:")  // ← Reused for all tests!
    return &Handler{ledger: ledger}
}

// Test 1: creates decision DEC-001
// Test 2: also creates DEC-001 → conflict
// Test 3: assumes empty ledger → fails if Test 1 ran first
```

**Why it's bad:** Tests pass or fail depending on execution order. Non-deterministic failures.

**Verification Strategy:**
```bash
# Run tests multiple times
for i in {1..5}; do go test ./... -count=1; done

# If tests pass all 5 times: ✓ probably no flakes
# If tests fail inconsistently: ✗ test pollution or race
```

**Test:**
```go
func TestIsolation(t *testing.T) {
    // Each test should have clean slate
    handler := setupTestHandler()
    
    // Test 1
    t.Run("Test1", func(t *testing.T) {
        // Create decision in handler's ledger
    })
    
    // Test 2 (should NOT see Test 1's data)
    t.Run("Test2", func(t *testing.T) {
        // New decision with same ID as Test 1
        // Should not conflict if isolated
    })
}
```

**Fix:**
```go
func setupTestHandler(t *testing.T) *Handler {
    // Each test gets unique temp directory
    tmpDir := t.TempDir()
    ledgerPath := filepath.Join(tmpDir, "ledger.jsonl")
    
    ledger := NewStore(ledgerPath)
    return &Handler{ledger: ledger}
}
```

---

## Summary: Quick Checklist

For each bug category, this is the verification checklist:

| Category | Bug | Grep Pattern | Expected Result | Fail Indicator |
|----------|-----|--------------|-----------------|----------------|
| **Crypto** | Verify skipped | `Signature.*=.*$ ` without Verify | Verify found | No Verify call |
| **Crypto** | Circular ref | Sign/Verify marshal different | Same object | Different fields |
| **Crypto** | Hash excludes field | `computeHash` | All fields included | Field missing |
| **Error** | Unhandled | `_ =` | Only intentional | Any unhandled error |
| **Error** | No graceful degrade | StatusInternalServerError | Has fallback | No fallback |
| **Input** | Unvalidated | `Query.Get\|Header.Get` | All have validation | No validation |
| **Input** | Unbounded query | `SELECT\|MATCH` | Has LIMIT | No LIMIT |
| **State** | Transitions free-form | `Status =` | Guard checks | Free assignment |
| **State** | Expiration ignored | `ExpiresAt` | Has check | No check |
| **Race** | Concurrent access | `go\|chan` | Run with -race | Race detected |
| **Test** | Shared state | `:memory:\|global` | TempDir per test | Shared resource |

---

**Captain: This is the "remove the bad" approach.** For each ticket Phase 1, we don't ask "is the code good?" We ask "does it have any of these 11 bugs?" If yes, fail Phase 1. If no, pass.

Ready to wire this into Phase 1 checklist?
