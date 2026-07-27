# TICKET 202: Bridge Redesign (AX-SPAWN-001 Implementation)

**ID:** 202  
**Title:** Rebuild Spawn System Following AX-SPAWN-001  
**Status:** Ready to Implement (Design-Backed)  
**Authority:** AX-SPAWN-001 axiom + DESIGN.md + HB-001 Specification  
**Timeline:** 1 week (Phase 3)  
**Blocked by:** Ticket 201 (portfolio must exist first)  
**Blocks:** Ticket 203  

**Design Reference:** `/Users/jwalinshah/projects/homebase/docs/DESIGN.md`
- Part 1: System Overview
- Part 2: Data Flow 2 (Bridge Spawn Decision)
- Part 3: Escalation API Endpoints (7-9)
- Part 4: Bridge Tests (B1T1-B1T12)

---

## What

Rebuild the bridge spawn system to satisfy **AX-SPAWN-001** equation:

```
∀worker_execution(W):
  preconditions_verified(W) ∧
  tool_calls_observable(W) ∧
  failures_explicit(W) ∧
  escalation_path_exists(W)
```

**Current problem:** Spawn workers timeout after 10 minutes with no explanation.

**Root cause:** None of the 4 conditions met.

**Solution:** Implement all 4 conditions.

**Result:** Workers complete OR escalate with explicit ask (no silent hangs).

---

## From

- `/Users/jwalinshah/projects/axioms/NEW_AXIOM_AX_SPAWN_001.md`
- `/Users/jwalinshah/projects/homebase/HB-001-COMPLETE-SPECIFICATION.md` (Section 3-7, Defenses 1-2, 8, 11, 15, 17)
- `/Users/jwalinshah/projects/homebase/docs/DESIGN.md` (Part 2 Flow 2, Part 3 Endpoints 7-9, Part 4 Tests B1T1-B1T12)

---

## Tasks

### Task 1: Precondition Verification (Condition 1 of AX-SPAWN-001)
**What:** Check sandbox rules + ticket verification commands BEFORE spawning worker.

**Files to Create:**
- `internal/spawn/preconditions.go` — precondition check logic
- `internal/spawn/preconditions_test.go` — tests (B1T1, B1T2)

**Implementation (from DESIGN.md Part 2, Flow 2, Step 2):**
```go
func CheckPreconditions(ticket Ticket, sandbox SandboxRules) error {
    for _, cmd := range ticket.VerificationCommands {
        toolName := cmd.Argv[0]  // e.g., "curl"
        
        // Does this tool need a permission?
        requiredPerms := GetToolRequiredPermissions(toolName)
        // e.g., {"network-outbound": "127.0.0.1:7474"}
        
        // Does sandbox grant these permissions?
        for perm, resource := range requiredPerms {
            if !sandbox.HasPermission(perm, resource) {
                return fmt.Errorf(
                    "precondition_failed: %s requires %s but sandbox denies it",
                    toolName, perm,
                )
            }
        }
    }
    return nil  // All preconditions met
}
```

**Flow:**
1. Bridge receives spawn request
2. Call CheckPreconditions(ticket, sandbox)
3. If error: REJECT immediately (return 400)
4. If success: Proceed to Task 2 (observability)

**Success Criteria:**
- ☑ Precondition check before spawn (not after)
- ☑ 100% of timeout scenarios prevented
- ☑ Mismatch detected + rejected before worker launches
- ☑ Error message includes missing permission + resource
- ☑ Tests B1T1 (pass) + B1T2 (fail) pass

---

### Task 2: Live Observability (Condition 2 of AX-SPAWN-001)
**What:** Watch worker execution in real-time (NOT polling manifest.json)

**Files to Create:**
- `internal/spawn/observability.go` — mintmux pane watching
- `internal/spawn/observability_test.go` — tests (B1T4)

**Implementation (from DESIGN.md Part 2, Flow 2, Step 4):**
```go
func WatchPane(paneID string, logWriter io.Writer) error {
    // Don't poll for manifest.json
    // Instead, watch mintmux pane in real-time
    
    // Open mintmux pane stream
    stream, err := mm.CapturePane(paneID)
    if err != nil {
        return err
    }
    defer stream.Close()
    
    // Read and log every line
    for {
        line, err := stream.ReadLine()
        if err == EOF {
            break
        }
        
        // Log tool call
        toolCall := ParseToolCall(line)  // Extract: tool, input, output
        LogToolCall(toolCall)
        
        // Also send to portfolio async (non-blocking)
        go portfolioClient.LogToolCall(toolCall)
    }
    
    return nil
}
```

**Key Requirement:** Watch pane, don't poll manifest. This ensures:
- Real-time observability
- No silent hangs (if process dies, stream closes)
- Every tool call captured

**Success Criteria:**
- ☑ Watch worker pane in real-time (not polling)
- ☑ All tool calls logged {tool, input, output, exit_code, timestamp}
- ☑ No silent failures (log stream shows everything)
- ☑ Events sent to portfolio asynchronously (non-blocking)
- ☑ Test B1T4 passes

---

### Task 3: Explicit Error Messages (Condition 3 of AX-SPAWN-001)
**What:** No timeouts or opaque errors. Every failure has a reason.

**Files to Create:**
- `internal/spawn/errors.go` — error types + enum
- `internal/spawn/errors_test.go` — tests (B1T3, B1T5)

**Error Types to Define:**
```go
type ErrorType string

const (
    ErrorDenied      ErrorType = "denied"          // Permission denied
    ErrorToolNotFound ErrorType = "tool_not_found"  // Tool not in PATH
    ErrorCrashed     ErrorType = "crashed"          // Process crashed
    ErrorTimeout     ErrorType = "timeout"          // Explicit timeout (not silent)
    ErrorOffline     ErrorType = "offline"          // Network unavailable
)

type WorkerError struct {
    ErrorType       ErrorType  `json:"error"`
    Reason          string     `json:"reason"`
    CommandFailed   string     `json:"command_that_failed"`
    PermissionNeeded string    `json:"permission_needed,omitempty"`
    Timestamp       time.Time  `json:"timestamp"`
}
```

**Implementation:**
```
On ANY failure:
  1. Capture error reason (from stderr, exit code, sandbox denial)
  2. Classify error (use ErrorType enum)
  3. Write manifest.json with error details
  4. Worker NEVER hangs (timeout becomes explicit error)

Example manifest on network denied:
{
  "status": "failed",
  "error": "denied",
  "reason": "network-outbound to 127.0.0.1:7474",
  "command_that_failed": "curl -s http://localhost:7474/...",
  "permission_needed": "network-outbound",
  "timestamp": "2026-07-25T10:00:05Z"
}
```

**Success Criteria:**
- ☑ All 5 error types defined (denied, tool_not_found, crashed, timeout, offline)
- ☑ Errors classified before writing manifest
- ☑ No "unknown error" or timeouts without reason
- ☑ Manifest includes permission_needed (for escalation)
- ☑ Tests B1T3 + B1T5 pass

---

### Task 4: Escalation Path (Condition 4 of AX-SPAWN-001)
**What:** Permission denied → ask captain to approve, then retry.

**Files to Create:**
- `internal/spawn/escalation.go` — escalation logic
- `internal/spawn/escalation_test.go` — tests (B1T6, B1T7, B1T8)

**Implementation (from DESIGN.md Part 2, Flow 2, Steps 6-9):**

```go
func HandleEscalation(workerErr WorkerError) error {
    // Step 1: Detect escalation need
    if workerErr.ErrorType != ErrorDenied {
        return workerErr  // Not an escalation, just fail
    }
    
    // Step 2: Create escalation request
    esc := &Escalation{
        PermissionNeeded: workerErr.PermissionNeeded,
        Resource: workerErr.Reason,
        CreatedAt: time.Now(),
        Status: "PENDING",
    }
    escalationID, err := portfolioClient.CreateEscalation(esc)
    if err != nil {
        return err
    }
    
    // Step 3: Wait for captain approval (with timeout)
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    
    status, err := portfolioClient.PollEscalationStatus(ctx, escalationID)
    if err != nil {
        return fmt.Errorf("escalation_timeout: %v", err)
    }
    
    if status == "DENIED" {
        return fmt.Errorf("escalation_denied")
    }
    
    if status != "APPROVED" {
        return fmt.Errorf("escalation_unknown_status: %s", status)
    }
    
    // Step 4: Update sandbox with new permission
    err = sandbox.AddPermission(workerErr.PermissionNeeded, workerErr.Reason)
    if err != nil {
        return fmt.Errorf("sandbox_update_failed: %v", err)
    }
    
    // Step 5: Retry spawn (recursive)
    return nil  // Caller will retry with updated sandbox
}
```

**Portfolio Integration (from DESIGN.md Part 3, Endpoints 7-9):**

Ticket 201 must provide these endpoints:
- `POST /api/v1/escalations` — create escalation request
- `GET /api/v1/escalations/{id}/status` — check approval status
- `POST /api/v1/escalations/{id}/approve` — captain approves

Bridge calls these endpoints when:
1. Worker fails with "denied" error → POST /api/escalations
2. Bridge polls: GET /api/escalations/{id}/status
3. Captain approves → portfolio updates status
4. Bridge retries with updated sandbox

**Success Criteria:**
- ☑ Escalation detected from "denied" errors
- ☑ Escalation request created (POST to portfolio)
- ☑ Bridge polls escalation status (with timeout)
- ☑ On approval: sandbox updated + worker retried
- ☑ On denial: worker fails with reason
- ☑ On timeout: explicit error (not hangs)
- ☑ Tests B1T6 + B1T7 + B1T8 pass

---

### Task 5: Axiom Compliance Check (Before Spawn)
**What:** Load relevant axioms before spawn, verify compliance with AX-SPAWN-001

**Files to Create:**
- `internal/spawn/axiom_check.go` — axiom verification
- `internal/spawn/axiom_check_test.go` — tests

**Implementation:**
```go
func CheckAxiomCompliance(ticket Ticket) error {
    // Query portfolio for axioms
    axioms, err := portfolioClient.GetAxioms([]string{
        "AX-SPAWN-001",
        "AX-SECURITY-004",
        "AX-SYSTEMS-012",
    })
    if err != nil {
        return err  // Portfolio down, but spawn can continue (degrades gracefully)
    }
    
    // Verify AX-SPAWN-001 conditions
    for _, axiom := range axioms {
        if axiom.ID == "AX-SPAWN-001" {
            // Check all 4 conditions will be met
            // (other tasks implement these, just log here)
        }
    }
    
    return nil
}
```

**Success Criteria:**
- ☑ Bridge queries portfolio for axioms before spawn
- ☑ Axioms logged in spawn decision (for portfolio)
- ☑ All 4 AX-SPAWN-001 conditions verified before spawn

---

### Task 6: Integration with Portfolio
**What:** Log all spawn decisions to portfolio

**Implementation:**
```
After spawn success/failure:
  Bridge calls: POST /api/v1/decisions/log
  Payload: {
    system: "bridge",
    action: "spawn-ticket-202",
    ticket_id: "ticket-202",
    result: "success|failed|escalated",
    axioms_checked: ["AX-SPAWN-001", "AX-SECURITY-004"],
    evidence: "worker output, exit code, manifest",
    timestamp: now()
  }
```

**Success Criteria:**
- ☑ Bridge logs every spawn attempt to portfolio
- ☑ Logs include axioms checked
- ☑ Logs include result (success/failed/escalated)

---

## Acceptance Criteria

Bridge redesign is DONE when:

**Condition 1 (Preconditions):**
- ☑ Precondition check before spawn (not after)
- ☑ Rejects on sandbox/command mismatch
- ☑ Error message includes missing permission

**Condition 2 (Observability):**
- ☑ Watch worker pane in real-time (not polling)
- ☑ All tool calls logged {tool, input, output, exit_code, timestamp}
- ☑ No silent failures (stream shows everything)

**Condition 3 (Explicit Errors):**
- ☑ All failures have explicit reasons (no timeouts)
- ☑ 5 error types defined (denied, tool_not_found, crashed, timeout, offline)
- ☑ Manifest includes error type + reason + permission_needed

**Condition 4 (Escalation):**
- ☑ Permission denied → creates escalation request
- ☑ Bridge polls portfolio for approval status
- ☑ On approval: sandbox updated + retry
- ☑ On denial: worker fails with reason
- ☑ Escalation timeout: explicit error (not silent)

**Integration:**
- ☑ Bridge queries axioms before spawn
- ☑ All spawn decisions logged to portfolio
- ☑ Every decision cites AX-SPAWN-001

**All B1 Tests Pass (12/12):**
- B1T1: Precondition check succeeds
- B1T2: Precondition check fails
- B1T3: Error type enum
- B1T4: Live observability
- B1T5: Explicit error messages
- B1T6: Escalation detection
- B1T7: Escalation flow (create → approve → retry)
- B1T8: Sandbox update after approval
- B1T9: Full AX-SPAWN-001 workflow
- B1T10: Timeout handling (explicit, not silent)
- B1T11: Escalation approval timeout
- B1T12: Worker crash recovery

---

## Owned by

Bridge team (or captain if solo)

---

## Next

Once 202 complete:
- New bridge ready for production
- Can spawn all future work via portfolio
- Old bridge retired
- Ticket 203 can proceed (axioms integration)
