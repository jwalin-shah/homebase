# HomeBase Domain Semantics

**Status:** Under audit (not locked)  
**Version:** 1.0  
**Authority:** Captain  

---

## 1. Core Identity Types

All identities are opaque, typed, and globally unique within their domain.

```
TaskID              // Stable task identity across all attempts
ContractID          // Identifies a contract specification
ContractVersion     // Monotonic revision (1, 2, 3, ...)
AttemptID           // Identifies one execution attempt of a task
EffectID            // Identifies an external effect (e.g., spawn worker)
ObservationID       // Identifies a single observation from Bridge
EvidenceID          // Identifies verification evidence from a verifier
ObligationID        // Identifies a contract requirement
CommandID           // Identifies an issued command (for idempotency)
EventID             // Assigned by ledger (not used in domain decisions)
AuthorityID         // Principal issuing a command (e.g., "homebase-runtime-01")
```

Do not use plain strings for these. Each type is an opaque struct:

```
type TaskID struct { value string }
type AttemptID struct { value string }
// etc.
```

---

## 2. Domain Events (Pure Business Logic)

Events represent state changes. They are immutable once committed.

```
DomainEvent =
    ContractLocked {
        contract_id: ContractID
        contract_version: ContractVersion
        required_obligations: Set[ObligationID]
        allowed_effect_kinds: Set[EffectKind]
        max_attempts: uint32
        contract_digest: Hash
    }
  | AttemptCreated {
        attempt_id: AttemptID
        ordinal: uint32
    }
  | EffectIntentCommitted {
        attempt_id: AttemptID
        effect_id: EffectID
        effect_kind: EffectKind
        request_digest: Hash
    }
  | EffectObserved {
        observation_id: ObservationID
        attempt_id: AttemptID
        effect_id: EffectID
        outcome: ObservationOutcome
        result_digest: Option[Hash]
    }
  | EvidenceAccepted {
        evidence_id: EvidenceID
        attempt_id: AttemptID
        source_observation_id: ObservationID
        evidence_digest: Hash
    }
  | ObligationSatisfied {
        obligation_id: ObligationID
        evidence_ids: Set[EvidenceID]
    }
  | TaskCompleted {}
  | EscalationRequested {
        failure_class: FailureClass
        reason: String
        related_effect_id: Option[EffectID]
    }
```

---

## 3. Recorded Domain Event (Replay Metadata)

When events are persisted, they carry origin metadata needed for reconstruction:

```
RecordedDomainEvent {
    aggregate_version: uint64        // TaskState.version before applying event
    domain_event: DomainEvent        // The state change
    origin: CommandOrigin
}

CommandOrigin {
    command_id: CommandID            // Issued command (for idempotency)
    command_fingerprint: Hash        // Hash of command body
    authority: Authority             // Who issued it
    correlation_id: CorrelationID    // Workflow trace
    causation_id: Option[...]        // Prior command or event that caused this
}
```

Recorded domain events are used in Lean, Dafny, and Go conformance fixtures.

Persisted event envelopes (with SQLite-specific fields) are defined in LEDGER.md.

---

## 4. Current-State Projection (TaskState)

TaskState is derived from the ordered sequence of events via Fold:

```
fold(initialState, events: Seq[Event]) → TaskState
```

It contains only:

* current aggregate version
* current status
* active references (contract, active attempt)
* current collections (attempts, effect intents, observations, evidence, obligations)
* command receipts (for idempotency)
* escalation info (if applicable)

**It does not store:**

* full event history (the ledger stores that)
* derived statistics (these are computed on-demand)
* time-based fields (only for audit metadata)
* NoOp or Rejected decision results (recomputed deterministically on replay)

```
TaskState {
    task_id: TaskID
    version: uint64                              // Incremented by each accepted event
    status: TaskStatus                           // Active | Completed | Escalated
    
    // Contract
    contract: Option[ContractRef]
    
    // Attempts and effects
    attempts: Map[AttemptID, Attempt]
    active_attempt: Option[AttemptID]
    effect_intents: Map[EffectID, EffectIntent]
    observations: Map[ObservationID, Observation]
    
    // Evidence and obligations
    accepted_evidence: Map[EvidenceID, Evidence]
    satisfied_obligations: Map[ObligationID, ObligationSatisfaction]
    
    // Idempotency
    command_receipts: Map[CommandID, CommandReceipt]
    
    // Escalation
    escalation: Option[Escalation]
}

ContractRef {
    contract_id: ContractID
    contract_version: ContractVersion
    contract_digest: Hash
}

Attempt {
    attempt_id: AttemptID
    ordinal: uint32
    status: AttemptStatus            // Open | Succeeded | Failed | OutcomeUnknown
    effect_ids: Set[EffectID]
}

EffectIntent {
    effect_id: EffectID
    effect_kind: EffectKind
    request_digest: Hash
    status: IntentStatus             // Committed | OutcomeNeeded | Terminal
}

Observation {
    observation_id: ObservationID
    attempt_id: AttemptID
    effect_id: EffectID
    outcome: ObservationOutcome      // NotStarted | Running | Succeeded | Failed | Unknown
    result_digest: Option[Hash]
}

Evidence {
    evidence_id: EvidenceID
    attempt_id: AttemptID
    source_observation_id: ObservationID
    evidence_digest: Hash
}

ObligationSatisfaction {
    obligation_id: ObligationID
    evidence_ids: Set[EvidenceID]
}

CommandReceipt {
    command_fingerprint: Hash        // Fingerprint of command body
    resulting_event_types: Seq[EventType]  // Types of events emitted (only for Accepted)
}
// CommandReceipt is reconstructed from event origins (Accepted commands only).
// NoOp and Rejected decisions are recomputed deterministically and not persisted.

Escalation {
    failure_class: FailureClass
    reason: String
    related_effect_id: Option[EffectID]
    requested_at: TaskVersion        // version when escalation occurred
}

TaskStatus = Active | Completed | Escalated
AttemptStatus = Open | Succeeded | Failed | OutcomeUnknown
IntentStatus = Committed | OutcomeNeeded | Terminal
ObservationOutcome = NotStarted | Running | Succeeded | Failed | Unknown
DecisionKind = Accepted | NoOp | Rejected
```

---

## 5. Commands (Requests to Change State)

A command requests a state transition. It includes enough context for idempotency and auditing.

```
CommandEnvelope {
    command_id: CommandID            // Globally unique for this invocation
    task_id: TaskID                  // Which task to modify
    expected_version: uint64         // Optimistic concurrency: only apply if state.version == expected
    authority: Authority             // Who is making this request
    correlation_id: CorrelationID    // Trace ID for this workflow
    causation_id: Option[CommandID | EventID]  // What caused this command
    issued_at: Timestamp             // When it was issued
    body: CommandBody
}

Authority {
    principal_id: AuthorityID        // WHO (e.g., "homebase-runtime-01")
    role: AuthorityRole              // WHAT they can do (see AUTHORITY.md)
}

CommandBody = 
    LockContract {...}
  | CreateAttempt {...}
  | CommitEffectIntent {...}
  | RecordEffectObservation {...}
  | AcceptEvidence {...}
  | SatisfyObligation {...}
  | ProposeCompletion
  | RequestEscalation {...}
```

---

## 6. Decision Results

The kernel evaluates a command against the current state and returns a Decision.

```
Decision =
    Rejected {
        reason_code: RejectionReason
        message: String
    }
  | NoOp {
        reason_code: NoOpReason
        message: String
    }
  | Accepted {
        events: Seq[DomainEvent]
    }
```

The result determines the next action:

* **Rejected**: Command is invalid; state does not change. Retry is unsafe.
* **NoOp**: Command is idempotent (already applied or harmless duplicate); state does not change. Retry is safe.
* **Accepted**: One or more events are emitted. Each event increments the aggregate version.

```
RejectionReason =
    STALE_VERSION
  | UNAUTHORIZED
  | INVALID_STATUS
  | ATTEMPT_LIMIT_REACHED
  | ATTEMPT_NOT_FOUND
  | ATTEMPT_NOT_ACTIVE
  | EFFECT_NOT_FOUND
  | EFFECT_KIND_NOT_ALLOWED
  | CONFLICTING_EFFECT_ID
  | OBSERVATION_NOT_FOUND
  | CONFLICTING_OBSERVATION_ID
  | OBSERVATION_NOT_SUCCESSFUL
  | EVIDENCE_NOT_FOUND
  | CONFLICTING_EVIDENCE_ID
  | OBLIGATION_NOT_REQUIRED
  | UNMET_OBLIGATIONS
  | TERMINAL_STATE
  | OUTCOME_UNKNOWN
  | COMMAND_ID_CONFLICT

NoOpReason =
    COMMAND_ALREADY_APPLIED
  | IDENTICAL_CONTRACT
  | IDENTICAL_ATTEMPT
  | IDENTICAL_EFFECT_INTENT
  | IDENTICAL_OBSERVATION
  | IDENTICAL_EVIDENCE
  | OBLIGATION_ALREADY_SATISFIED
  | ALREADY_COMPLETED
  | ALREADY_ESCALATED
```

---

## 7. Command Idempotency Rules

Idempotency is deterministic and operates at two levels:

1. **Command-level (replay detection)**: Same CommandID + fingerprint = NoOp(COMMAND_ALREADY_APPLIED).
2. **Semantic-level (state evaluation)**: Re-evaluation against current state determines NoOp vs Rejected vs Accepted.

**Replay Detection (First Check):**

```
if CommandID already persisted in ledger:
    if command_fingerprint matches: return NoOp(COMMAND_ALREADY_APPLIED)
    if command_fingerprint differs: return Rejected(COMMAND_ID_CONFLICT)

if CommandID not in ledger:
    proceed to semantic evaluation
```

The replay check happens before any state evaluation. If a command was previously accepted and persisted (found in event origins), replaying it with identical fingerprint always returns NoOp, NOT the original Accepted decision.

**Semantic Idempotency (After Replay Check):**

For each command type, re-evaluation against current state is deterministic:

| Command | Identical Duplicate (after first acceptance) | Conflicting State |
|---------|---|---|
| LockContract | NoOp(IDENTICAL_CONTRACT) | Rejected(COMMAND_ID_CONFLICT) |
| CreateAttempt | NoOp(IDENTICAL_ATTEMPT) | Rejected(COMMAND_ID_CONFLICT) |
| CommitEffectIntent | NoOp(IDENTICAL_EFFECT_INTENT) | Rejected(CONFLICTING_EFFECT_ID) |
| RecordEffectObservation | NoOp(IDENTICAL_OBSERVATION) | Rejected(CONFLICTING_OBSERVATION_ID) |
| AcceptEvidence | NoOp(IDENTICAL_EVIDENCE) | Rejected(CONFLICTING_EVIDENCE_ID) |
| SatisfyObligation | NoOp (if evidence set identical) | Rejected(CONFLICTING_EVIDENCE_ID) |
| ProposeCompletion | NoOp(ALREADY_COMPLETED) | Rejected(TERMINAL_STATE) |
| RequestEscalation | NoOp(ALREADY_ESCALATED) | Rejected(TERMINAL_STATE) |

**Critical Distinction:**

* **Identical Replay**: Previously accepted CommandID + identical fingerprint → **NoOp(COMMAND_ALREADY_APPLIED)**. Events are not re-appended. Caller can inspect the ledger to discover the original events.
* **Conflicting Replay**: Previously accepted CommandID + different fingerprint → **Rejected(COMMAND_ID_CONFLICT)**. This is a protocol violation.
* **First Submission**: CommandID not in ledger + valid command → **Accepted([events])**. Events are persisted with origin metadata (command_id + fingerprint).

---

## 8. Decision Evaluation Precedence

The Decide function must evaluate commands in a specific order. When multiple preconditions fail, the first (in this order) determines the reason code returned.

```
Decide(state: TaskState, command: CommandEnvelope) → Decision {
  
  // 1. Replay/Conflict Check
  if CommandID in ledger:
    if fingerprint matches: return NoOp(COMMAND_ALREADY_APPLIED)
    else: return Rejected(COMMAND_ID_CONFLICT)
  
  // 2. Expected Version Check
  if command.expected_version != state.version:
    return Rejected(STALE_VERSION)
  
  // 3. Authority Check
  if not isAuthorized(command.authority, command.type):
    return Rejected(UNAUTHORIZED)
  
  // 4. Terminal State Check
  if state.status in [Completed, Escalated]:
    if command.type == ProposeCompletion and state.status == Completed:
      return NoOp(ALREADY_COMPLETED)
    else:
      return Rejected(TERMINAL_STATE)
  
  // 5-7. Command-Specific Logic
  // (contract checks, attempt checks, effect checks, observation checks, etc.)
  // (semantic duplicate and conflict detection)
  // Then emit ordered domain events or appropriate NoOp/Rejected
  
  // If all preconditions pass and events are valid:
  return Accepted(events: Seq[DomainEvent])
}
```

**Why this order matters:**

* **Replay check first**: Prevents duplicate command processing before any state mutation logic.
* **Version check early**: Detects concurrent writes (optimistic concurrency) before wasting computation.
* **Authority check**: Security gate.
* **Terminal state check**: Prevents any mutation after Completed or Escalated.
* **Command-specific checks**: Domain logic that may have multiple failure modes; order within this section is documented per command.

---

## 9. Observation Outcomes

Bridge reports observations with one of five outcomes:

```
ObservationOutcome =
    NotStarted          // Effect has not yet executed
  | Running             // Effect is in progress
  | Succeeded           // Effect completed successfully
  | Failed              // Effect completed with failure
  | Unknown             // Outcome is indeterminate (e.g., crash-before-observation)
```

Transitions:

* **NotStarted** → (same effect_id) dispatch the effect again (idempotent)
* **Running** → wait or query later
* **Succeeded** → record successful observation, evidence can be accepted
* **Failed** → record failed observation, attempt may fail or retry
* **Unknown** → mark attempt OutcomeUnknown, escalate

---

## 10. Recovery Transitions

When an effect intent exists without a terminal observation, the system must recover using Bridge's Lookup contract.

**Recovery Protocol:**

```
Committed intent without terminal observation
    ↓
Lookup(effect_id) via Bridge
    ├─ NotStarted → dispatch same effect_id (idempotent by effect_id)
    ├─ Running    → wait or query later
    ├─ Succeeded  → RecordEffectObservation(outcome=Succeeded)
    ├─ Failed     → RecordEffectObservation(outcome=Failed)
    └─ Unknown    → mark attempt OutcomeUnknown, request escalation
```

Do NOT create a new attempt while the previous attempt's outcome is unresolved.

---

## 11. Completion and Escalation

**Completion** requires all required obligations to be satisfied:

```
ProposeCompletion only succeeds if:
    state.status == Active
    AND satisfied_obligations == contract.required_obligations
    AND no attempt is OutcomeUnknown
```

**Escalation** terminates the task and prevents further commands:

```
RequestEscalation only succeeds if:
    state.status == Active
    AND outcome == Unknown (or policy choice to abandon)
    
Result: status → Escalated, no further operational commands accepted
```

Terminal states:

* **Completed**: All obligations satisfied, no further changes.
* **Escalated**: Awaiting external decision, no further changes.

---

## 12. Version Progression

**Exact versioning rule:**

Each DomainEvent receives exactly one version number, assigned sequentially:

```
state.version = current_version = V
Decision → events: [event_1, event_2, event_3]

After append:
event_1.aggregate_version = V + 1
event_2.aggregate_version = V + 2
event_3.aggregate_version = V + 3
state.version = V + 3
```

**Kernel vs. Ledger:**

The domain kernel (Decide function) produces pure DomainEvents without version numbers.
The ledger (compare-and-append) assigns aggregate_version as part of PersistedEventEnvelope.
The kernel verifies expected_version matches current state.version once before producing events.
If match succeeds, all events are accepted atomically; if mismatch, entire batch is rejected.

---

## 13. Reducer Rules (Fold Function)

The reducer is the pure function that transitions from one state to the next:

```
Reduce(state: TaskState, event: DomainEvent) → TaskState'
```

The event already carries its assigned aggregate_version (or state.version is incremented by 1 per event in pure domain fold).

For each event type:

**ContractLocked**
```
requires: state.status == Active, state.contract == None
produces: state.contract = Some(ContractRef {...})
          state.version += 1
```

**AttemptCreated**
```
requires: state.status == Active
          state.attempts.size() < contract.max_attempts
produces: attempts[event.attempt_id] = Attempt(status=Open)
          active_attempt = Some(event.attempt_id)
          state.version += 1
```

**EffectIntentCommitted**
```
requires: state.status == Active
          active_attempt == Some(event.attempt_id)
          event.effect_kind in contract.allowed_effect_kinds
produces: effect_intents[event.effect_id] = EffectIntent(status=Committed)
          attempts[event.attempt_id].effect_ids += event.effect_id
          state.version += 1
```

**EffectObserved**
```
requires: state.status == Active
          effect_intents[event.effect_id].status == Committed
          attempts[event.attempt_id].effect_ids contains event.effect_id
produces: observations[event.observation_id] = Observation(...)
          if outcome == Succeeded: effect_intents[event.effect_id].status = Terminal
          if outcome == Failed: effect_intents[event.effect_id].status = Terminal
          state.version += 1
```

**EvidenceAccepted**
```
requires: state.status == Active
          observations[event.source_observation_id].outcome == Succeeded
          attempts[event.attempt_id] contains source observation
produces: accepted_evidence[event.evidence_id] = Evidence(...)
          state.version += 1
```

**ObligationSatisfied**
```
requires: state.status == Active
          event.obligation_id in contract.required_obligations
          all evidence_ids are accepted
produces: satisfied_obligations[event.obligation_id] = ObligationSatisfaction(...)
          state.version += 1
```

**TaskCompleted**
```
requires: state.status == Active
          satisfied_obligations == contract.required_obligations
produces: status = Completed
          state.version += 1
```

**EscalationRequested**
```
requires: state.status == Active
produces: status = Escalated
          escalation = Some(Escalation {...})
          state.version += 1
```

---

## 14. Invariants (Global Properties)

These properties must hold after every accepted event:

1. **Version Monotonicity**  
   Every accepted event increments aggregate version exactly once.

2. **Expected-Version Safety**  
   A command accepted against state.version V produces events that result in state.version = V + 1.
   A command against stale expected_version is rejected before any event is produced.

3. **Intent-Before-Observation**  
   An observation (ObservationID) requires a matching committed effect intent.
   No observation for an effect_id that has no committed intent.

4. **Attempt Ownership**  
   Intent, observation, and evidence must belong to the same task and attempt.
   No evidence can refer to an observation from a different attempt.

5. **Stable Effect Identity**  
   An EffectID cannot refer to two different request_digests.
   An identical effect_id with different request_digest is rejected.

6. **Evidence Provenance**  
   Accepted evidence must reference a successful (Succeeded) observation.
   Evidence from Failed observations is rejected.

7. **Obligation Provenance**  
   An obligation can only be satisfied by accepted evidence.
   No obligation satisfied without evidence.

8. **Completion Soundness**  
   Completion requires every required obligation to be satisfied.
   No partial completion.

9. **Attempt-Bound Safety**  
   Attempt count never exceeds contract.max_attempts.
   Creating a new attempt when attempts.size() == max_attempts is rejected.

10. **Terminal Trapping**  
    No operational command (other than read queries) is accepted after Completed or Escalated.
    ProposeCompletion against Completed is NoOp or Rejected based on idempotency.
    Any command against Escalated is rejected.

11. **Single Terminal Outcome**  
    A task cannot be both Completed and Escalated.
    Status transitions are: Active → {Completed | Escalated}, no reversals.

12. **Deterministic Decision**  
    Reduce is a pure function: equal state and event inputs produce equal outputs.
    No time-based decisions, no random choices, no side effects.

---

## 15. Fixture Semantics

Domain conformance fixtures verify that:

1. The reducer correctly transforms state given events.
2. Commands produce expected decisions and resulting events.
3. Idempotency rules are applied correctly.
4. Invariants are never violated.
5. Ordering is preserved (Seq, not Set).

Fixtures use RecordedDomainEvent for the replay input (which includes origin metadata but not ledger-specific fields).

Ledger conformance fixtures (in LEDGER.md) verify:

1. Hash chain integrity.
2. Canonical serialization.
3. SQLite transaction atomicity.
4. Crash recovery.

See testdata/domain/schema.json for the fixture structure.

---

## 16. Deferred (Not in First Slice)

* Contract amendment (new contract versions after lock)
* Multi-step recovery strategies beyond Lookup
* Policy-based completion (only exact-set-match for now)
* Timeout-based escalation
* Priority-based effect ordering
* Distributed coordination
