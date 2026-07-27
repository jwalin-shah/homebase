# TICKET 202: Bridge Redesign (AX-SPAWN-001 Implementation)

**ID:** 202  
**Title:** Rebuild Spawn System Following AX-SPAWN-001  
**Status:** Ready to Implement  
**Authority:** AX-SPAWN-001 axiom + Bridge Redesign Spec + HB-001 Specification  
**Timeline:** 1 week (Phase 3)  
**Blocked by:** Ticket 201 (portfolio must exist)  
**Blocks:** Ticket 203 (once new bridge works, it spawns other work)

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

Current problem: spawn workers timeout after 10 minutes with no explanation.

Root cause: None of the 4 conditions are met.

Solution: Implement all 4 conditions.

---

## From

- `/Users/jwalinshah/projects/axioms/NEW_AXIOM_AX_SPAWN_001.md`
- `/Users/jwalinshah/projects/bridge/REDESIGN_SPEC_AX_SPAWN_001.md`
- `/Users/jwalinshah/projects/homebase/HB-001-COMPLETE-SPECIFICATION.md`
- `/Users/jwalinshah/projects/portfolio/EXECUTION_PLAN.md`

---

## Tasks

### Task 1: Precondition Verification (Before Spawning)

**What:** Check sandbox rules + ticket verification commands BEFORE spawning worker.

**How:**
1. Read `sandbox.sb` (seccomp/sandbox rules)
2. Read ticket's `verification_commands` array
3. For each command: does it require tool T with permission P?
4. Is T available in sandbox? Is P granted?
5. If any check fails → reject spawn BEFORE worker runs

**Example:**
```
Ticket asks: run `curl -s http://localhost:7474/...`
Sandbox has: (deny network)
Result: Mismatch detected BEFORE spawning
→ Fail with error: "denied: network-outbound required"
```

**Code location:** `cmd/bridge/spawn.go:checkPreconditions()`

**Success:** 100% of timeout scenarios prevented (preconditions checked first).

---

### Task 2: Live Observability (During Execution)

**What:** Watch worker execution in real-time, log all tool calls.

**How:**
1. Don't poll for `manifest.json`
2. Use mintmux to watch worker pane LIVE
3. Log every tool call: name, input, output, exit code, timestamp
4. Stream events to portfolio ledger (asynchronously)

**Code location:** `internal/spawn/observability.go:watchPane()`

**Success:** All tool calls logged, no silent failures.

---

### Task 3: Explicit Error Messages (On Failure)

**What:** No timeouts or opaque errors. Every failure has a reason.

**How:**
1. Define error types: `denied`, `tool_not_found`, `crashed`, `timeout`, `offline`
2. When anything fails, write manifest with error type + reason
3. Worker never hangs (timeout is explicit error, not silent)

**Example:**
```json
{
  "status": "failed",
  "error": "denied",
  "reason": "network-outbound to 127.0.0.1:7474",
  "command_that_failed": "curl -s http://localhost:7474/...",
  "timestamp": "2026-07-25T10:00:05Z"
}
```

**Code location:** `internal/spawn/errors.go:ErrorType` enum

**Success:** All failures have explicit reasons (no timeouts, no "unknown error").

---

### Task 4: Escalation Path (On Permission Denied)

**What:** Tool denied → don't hang. Ask captain to approve permission.

**How:**
1. Tool interception layer catches denials (from sandbox)
2. Write manifest: `"needs_escalation: network-outbound"`
3. Bridge reads manifest, shows captain: "Worker needs network to 127.0.0.1:7474 — approve?"
4. Captain approves via portfolio (adds permission)
5. Bridge updates sandbox + retries worker
6. If approved: worker continues. If denied: fail with reason.

**Code location:** `internal/spawn/escalation.go:handleEscalation()`

**Success:** Permission denials escalate with specific asks (no silent hangs).

---

## Acceptance Criteria

Bridge redesign is DONE when:

- ☑ Precondition check prevents timeout scenarios
  * Check sandbox + command compatibility
  * Reject spawn if mismatch
- ☑ Live pane watching shows worker progress
  * Mintmux integration works
  * Tool calls logged in real-time
- ☑ All failures have explicit reasons
  * No timeouts (all errors are explicit)
  * Error types: denied, tool_not_found, crashed, timeout
- ☑ Permission denials escalate automatically
  * Manifest has `"needs_escalation"` field
  * Bridge shows captain specific ask
  * Captain approves via portfolio
  * Retry works after approval
- ☑ All decisions logged to portfolio
  * Bridge logs every spawn decision
  * Every decision cites AX-SPAWN-001
- ☑ End-to-end test
  * Spawn worker with network requirement
  * Precondition check catches it
  * Escalation asks captain
  * Captain approves
  * Worker retries + completes

---

## Owned by

Bridge team (or captain if solo)

---

## Next

Once new bridge works:
- Portfolio can spawn ticket 203 (axioms integration)
- All future work spawned via new bridge
- Old bridge retired
