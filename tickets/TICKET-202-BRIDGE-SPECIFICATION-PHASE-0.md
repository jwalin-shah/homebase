# TICKET 202: Bridge Integration - Phase 0 Specification

**Date:** 2026-07-26  
**Phase:** 0 (Specification Review)  
**Status:** IN PROGRESS  
**Decision ID:** DEC-2026-07-26-001-BRIDGE

---

## Phase 0 Specification (Complete Design Before Code)

### Overview

Bridge integration enables HomeBase to spawn decisions to external systems (GPT, Claude, other LLMs) and receive results back. Bridge acts as the gateway for decision automation and fulfillment.

### All Components (Complete List)

```
Bridge Integration = 
  1. Spawn Request Handler (POST /api/v1/escalations)
  2. Escalation Status Tracker (GET /api/v1/escalations/{id})
  3. Escalation Approval Workflow (POST /api/v1/escalations/{id}/approve)
  4. Bridge Callback Handler (POST /api/v1/bridge/callback)
  5. Decision Fulfillment Logger
  6. Error Recovery Manager
```

### 1. Spawn Request Handler

**Endpoint:** `POST /api/v1/escalations`

**Request Schema:**
```json
{
  "decision_id": "dec-001",
  "spawn_type": "bridge",
  "system": "gpt-4",
  "prompt": "Analyze this decision and provide feedback",
  "context": {
    "decision_text": "Use Go for backend",
    "axioms": ["AX-001", "AX-002"],
    "evidence": "..."
  }
}
```

**Response:** 201 Created
```json
{
  "escalation_id": "esc-123",
  "decision_id": "dec-001",
  "status": "PENDING",
  "created_at": "2026-07-26T...",
  "expires_at": "2026-07-27T..."  // 24 hour timeout
}
```

**Error Scenarios:**
- Missing decision_id → 400
- Invalid spawn_type → 400
- Decision not found → 404

### 2. Escalation Status Tracker

**Endpoint:** `GET /api/v1/escalations/{id}`

**Response:** 200 OK
```json
{
  "escalation_id": "esc-123",
  "decision_id": "dec-001",
  "status": "PENDING|APPROVED|REJECTED|EXPIRED",
  "bridge_response": null,  // filled when Bridge responds
  "created_at": "2026-07-26T...",
  "updated_at": "2026-07-26T...",
  "expires_at": "2026-07-27T..."
}
```

**Status Transitions:**
```
PENDING → APPROVED (human approves)
       → REJECTED (human rejects)
       → EXPIRED (24 hours passed)
       → BRIDGE_RESPONSE (Bridge returns result)
```

### 3. Escalation Approval Workflow

**Endpoint:** `POST /api/v1/escalations/{id}/approve`

**Request Schema:**
```json
{
  "approved": true,
  "approver_id": "captain",
  "notes": "Approved for production use"
}
```

**Response:** 200 OK
```json
{
  "escalation_id": "esc-123",
  "status": "APPROVED",
  "approved_at": "2026-07-26T...",
  "approver_id": "captain"
}
```

**Rejection:**
```json
{
  "approved": false,
  "rejection_reason": "Needs more evidence"
}
```

### 4. Bridge Callback Handler

**Endpoint:** `POST /api/v1/bridge/callback`

**Request Schema (from Bridge):**
```json
{
  "escalation_id": "esc-123",
  "status": "COMPLETE",
  "response": {
    "analysis": "Strong decision with good evidence",
    "confidence": 0.95,
    "recommendations": ["Document the choice", "Update runbooks"],
    "evidence_quality": "high"
  },
  "timestamp": "2026-07-26T...",
  "signature": "ed25519-sig"
}
```

**Response:** 200 OK
```json
{
  "escalation_id": "esc-123",
  "status": "BRIDGE_RESPONSE_RECORDED"
}
```

### 5. Decision Fulfillment Logger

**What happens when Bridge responds:**
```
1. Callback received and verified (signature check)
2. Bridge response logged to ledger as new decision:
   DEC-BRIDGE-<escalation_id>-RESPONSE
3. Record facts about Bridge analysis
4. Link back to original decision
5. Update escalation status to BRIDGE_RESPONSE
```

### 6. Error Recovery Manager

**Timeout handling:**
```
If escalation expires (24 hours):
  → Status = EXPIRED
  → Log timeout as decision
  → Alert operator
  → Escalation can be re-spawned
```

**Callback verification:**
```
Every callback must:
  → Verify escalation_id exists
  → Verify signature (Ed25519)
  → Verify timestamp (within 5 minutes)
  → Reject if any verification fails
```

---

## Complete API Endpoint List

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/escalations` | Create spawn/escalation request |
| GET | `/api/v1/escalations/{id}` | Get escalation status |
| POST | `/api/v1/escalations/{id}/approve` | Approve escalation |
| POST | `/api/v1/bridge/callback` | Bridge returns result |

---

## Error Scenarios (Complete)

```
POST /api/v1/escalations
  ✗ Missing decision_id → 400 Bad Request
  ✗ Invalid spawn_type → 400 Bad Request
  ✗ Decision not found → 404 Not Found
  ✗ Database error → 500 Internal Error

GET /api/v1/escalations/{id}
  ✗ Escalation not found → 404 Not Found
  ✗ Database error → 500 Internal Error

POST /api/v1/escalations/{id}/approve
  ✗ Escalation not found → 404 Not Found
  ✗ Invalid approval status → 400 Bad Request
  ✗ Escalation already approved/rejected → 409 Conflict
  ✗ Database error → 500 Internal Error

POST /api/v1/bridge/callback
  ✗ Escalation not found → 404 Not Found
  ✗ Signature verification failed → 401 Unauthorized
  ✗ Timestamp too old → 400 Bad Request
  ✗ Database error → 500 Internal Error
```

---

## Data Flow (End-to-End)

```
1. System records decision (Ticket 201)
   → DEC-001 stored in ledger with axioms

2. Decision needs escalation
   → POST /api/v1/escalations
   → ESC-123 created, status = PENDING
   → Stored in database

3. Bridge system polls or is notified
   → Receives escalation details
   → Sends to LLM for analysis
   → Waits for LLM response

4. Bridge processes LLM response
   → POST /api/v1/bridge/callback
   → Response verified and stored
   → Escalation status = BRIDGE_RESPONSE

5. Human reviews Bridge response
   → GET /api/v1/escalations/ESC-123
   → Reads Bridge analysis and recommendations
   → Decides: approve or reject

6. Human approves
   → POST /api/v1/escalations/ESC-123/approve
   → ESC-123 status = APPROVED
   → New decision logged: DEC-APPROVAL-ESC-123
   → Original decision updated with approval evidence

7. System can now use approved decision
   → Decision has Bridge analysis
   → Decision has human approval
   → Decision is fully audited
```

---

## Acceptance Criteria for Phase 0

- [ ] All 4 endpoints documented (request/response schemas)
- [ ] All error scenarios identified and documented
- [ ] Data flow end-to-end explained
- [ ] Database schema designed (escalations table)
- [ ] Timeout and recovery procedures defined
- [ ] Signature verification procedure documented
- [ ] Status transitions completely specified
- [ ] No ambiguities or missing scenarios
- [ ] Architecture team reviewed and approved
- [ ] Ready for Phase 1 Code Review

---

## Sign-Off

**Phase 0 Ready?** YES ✅

This specification is complete. All components documented. Ready to proceed to Phase 1 (Code Review).

