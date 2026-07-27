# HomeBase Authority Model

**Status:** Under audit (not locked)  
**Version:** 1.0  

---

## 1. AuthorityID

An opaque, globally unique identifier for a principal (e.g., a runtime service, verifier, Bridge adapter).

```
AuthorityID = string  // Opaque; examples: "homebase-runtime-01", "verifier-attestation-svc"
```

Do not infer permissions from an AuthorityID. Always include a role.

---

## 2. AuthorityRole

Defines what kind of decisions a principal can make.

```
AuthorityRole =
    TaskInitiator        // Issues LockContract commands
  | Orchestrator         // Issues CreateAttempt, ProposeCompletion, RequestEscalation
  | BridgeAdapter        // Issues CommitEffectIntent, RecordEffectObservation
  | Verifier             // Issues AcceptEvidence
  | RecoveryController   // Issues commands in RECOVER state (special privileges)
```

An Authority is the pair **(principal_id, role)**.

```
Authority {
    principal_id: AuthorityID
    role: AuthorityRole
}
```

Same principal_id may hold multiple roles (in different commands), but each command carries exactly one Authority.

---

## 3. Role Permissions

| Role | LockContract | CreateAttempt | CommitEffectIntent | RecordEffectObservation | AcceptEvidence | SatisfyObligation | ProposeCompletion | RequestEscalation |
|------|---|---|---|---|---|---|---|---|
| TaskInitiator | ✓ | | | | | | | |
| Orchestrator | | ✓ | | | | | ✓ | ✓ |
| BridgeAdapter | | | ✓ | ✓ | | | | |
| Verifier | | | | | ✓ | ✓ | | |
| RecoveryController | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |

**Notes:**

* **RecoveryController** has all permissions (for recovery after crash or unrecoverable failure).
* Other roles have narrow, specific permissions.
* A principal with TaskInitiator role cannot create attempts; that requires Orchestrator.
* SatisfyObligation is split: Verifier issues it (after accepting evidence), Orchestrator proposes completion.

---

## 4. Authorization Rules

Before executing any command, verify:

1. **Authority is present**: Command must include Authority (principal_id + role).
2. **Role permits command**: Check the permission table above.
3. **Principal is allowed**: Check against a trusted principal registry (deferred to first integration).

```
IsAuthorized(command, authority) → bool {
    if authority == None: return False
    if authority.role not in permitted_roles_for(command.type): return False
    if not is_trusted_principal(authority.principal_id): return False
    return True
}

// If any check fails:
Decision → Rejected(UNAUTHORIZED)
```

---

## 5. Audit Metadata

Every command carries authority information for audit:

* **principal_id**: WHO made the decision (identifies the principal).
* **role**: WHAT they are authorized to do (identifies the capability).
* **correlation_id**: Trace ID linking this command to others in the workflow.
* **causation_id**: Links this command to its prior cause (prior command or event).

Together, these fields create an immutable audit trail.

---

## 6. Bridge Adapter Assumptions

The Bridge Adapter (BridgeAdapter role) has specific responsibilities and assumptions:

1. **Effect Commitment** (CommitEffectIntent): The adapter commits that an effect will be dispatched to Bridge.
2. **Effect Observation** (RecordEffectObservation): The adapter reports the outcome from Bridge.
3. **No Interpretation**: The adapter does not interpret outcomes. It reports them verbatim (NotStarted, Running, Succeeded, Failed, Unknown).
4. **Idempotent Dispatch**: The adapter ensures effects are dispatched idempotently by effect_id.

**Adapter Interface (pseudo-code):**

```
// Commit an effect to Bridge
CommitEffectIntent(effect_id, effect_kind, request_digest)
    → sends to Bridge
    → records intent in HomeBase

// Query Bridge for effect status
Lookup(effect_id)
    → queries Bridge
    → returns ObservationOutcome (NotStarted | Running | Succeeded | Failed | Unknown)

// Record result
RecordEffectObservation(observation_id, attempt_id, effect_id, outcome, result_digest)
    → reports outcome back to HomeBase
```

The adapter does not make decisions about retries, recovery, or escalation. Those decisions happen in RECOVER, ESCALATE, and REPEAT states (not in the adapter).

---

## 7. Trusted Principal Registry (Deferred)

For the first vertical slice, principal trust is assumed (not validated).

In production, a trusted registry (e.g., service mesh identity, signed certificates) will verify:

* Principal identity (e.g., mTLS certificate, service account)
* Role binding (e.g., from a policy engine or RBAC system)

Integration point: See ticket 300+ (Phase 2+).

---

## 8. Correlation and Causation

Commands link via **correlation_id** and **causation_id** to create a directed graph of causality:

* **correlation_id**: Groups all commands in one logical workflow. All commands in the same task share the same correlation_id.
* **causation_id**: Points to the prior command or event that caused this command. Creates a directed edge: prior → current.

Example:

```
Command 1 (LockContract):
  correlation_id = "workflow-abc123"
  causation_id = None

Command 2 (CreateAttempt):
  correlation_id = "workflow-abc123"
  causation_id = Command 1's event ID

Command 3 (CommitEffectIntent):
  correlation_id = "workflow-abc123"
  causation_id = Command 2's event ID
```

This creates a causal chain:
```
LockContract
  ↓ (causation_id)
CreateAttempt
  ↓ (causation_id)
CommitEffectIntent
```

All share the same correlation_id.

---

## 9. Idempotency by Authority

Command idempotency is keyed by **(command_id, command_fingerprint)**, independent of authority.

However, the Verifier role and Orchestrator role can produce different outcomes for the same logical action:

* **Verifier** issues AcceptEvidence (evidence comes from external verifiers).
* **Orchestrator** issues SatisfyObligation (orchestrator decides when obligations are met).

Both commands are needed in sequence. A Verifier cannot skip the Orchestrator, and vice versa.

---

## 10. Deferred (Not in First Slice)

* Dynamic role assignment (roles are static for now).
* Fine-grained per-effect authorization (all effects are equal for now).
* Cost tracking by principal.
* Audit log export and cryptographic verification.
* Multi-tenancy (single-tenant for now).
