# Architecture V2: HomeBase, Bridge, Orbit Cross-System Design

**Author:** Architecture Design Session  
**Date:** 2026-07-27  
**Status:** PROPOSED — not yet reviewed  
**Governing Doctrine:** No handwritten implementation, agent conclusion, generated artifact, test result, or formal proof may silently become authoritative outside the scope declared for it.

---

## Executive Architectural Recommendation

### What we have now

Three systems with overlapping authority:

- **HomeBase** (Go, Dafny core): A 5-state graph machine (PLAN→EXECUTE→RECOVER→ESCALATE→REPEAT) with Dafny-compiled pure reducer, append-only journal, and Ed25519 signing. Intended to be the authoritative decision layer. Currently defines its own retry protocol, attempt lifecycle, and event model.

- **Bridge** (Go, 17 packages): A mature execution coordinator with spawn, verify, create pipelines, context assembly (14 sources), worktree isolation, adapter dispatch, and its own append-only ledger with hash chain. Has its own retry policy, escalation protocol, and state machine (19-command protocol). **Bridge currently acts as its own authority layer.**

- **Orbit** (Go, 93 packages): A stateless LLM inference server with token routing, adversarial review, and grind pipeline. Has no durable state and no authority model. Currently acts as a thin shell over inference, but its `orbit-orchestrator` is being designed to own fix-loop orchestration.

### The core problem

**Bridge and HomeBase both claim to own retry policy, escalation, and attempt lifecycle.** This is duplicated authority. If both systems can independently decide to retry, escalate, or conclude, there is no single source of truth — and the audit trail is split across two ledgers.

### The recommendation

**HomeBase becomes the single authority kernel. Bridge becomes a pure execution coordinator that proposes commands and executes only committed effect intents. Orbit becomes a stateless rendering layer that submits typed commands and projects state.**

Key design principles:

1. **No duplicated authority.** Exactly one system decides each class of policy question.
2. **Bridge proposes, HomeBase decides.** Bridge submits typed commands; HomeBase's Dafny reducer accepts or rejects them.
3. **Orbit renders, never mutates.** Orbit reads projections, submits commands, displays state.
4. **Context is candidate, not authoritative.** Retrieval results carry provenance; they never make policy decisions.
5. **Evidence is typed and scoped.** Each piece of evidence declares what it proves and what it doesn't.
6. **External effects are best-effort with explicit outcome tracking.** No claim of exactly-once without external support.

### What this means for implementation

- **HomeBase's Dafny reducer** is the only code that can authorize a retry, accept a conclusion, or increment a recovery counter.
- **Bridge's current protocol package** (15 files, 19 commands) is restructured: its state machine becomes a HomeBase projection, not an independent authority.
- **Bridge's current ledger** (`.bridge/ledger.jsonl`) is replaced by HomeBase's journal — Bridge writes operational telemetry, not authoritative decisions.
- **Orbit's `orbit-orchestrator`** does not own the fix loop; it proposes fixes to Bridge, which proposes effect intents to HomeBase.

---

## 1. System Responsibility Map

For every major responsibility, exactly one authoritative owner.

### User Intent

| Responsibility | Owner | Notes |
|---|---|---|
| Intent capture | **Orbit** | Chat, CLI, dashboard, steering commands |
| Intent clarification | **Orbit** (with Bridge proposing clarification questions) | Orbit renders; Bridge proposes candidate clarifications |
| Intent → task conversion | **Bridge** | Brain dump → ticket decomposition (M4 create pipeline) |
| Task authorization | **HomeBase** | Validates task against policy before execution begins |

### Planning

| Responsibility | Owner | Notes |
|---|---|---|
| Task decomposition | **Bridge** | Atomization, dependency analysis |
| Execution plan construction | **Bridge** | What steps, in what order |
| Plan validation | **HomeBase** | Validates plan against invariants, policy |
| Plan locking | **HomeBase** | Immutable after authorization |
| Plan revision | **HomeBase** | Only via explicit re-plan command, never mid-execution |

### Execution

| Responsibility | Owner | Notes |
|---|---|---|
| Context assembly | **Bridge** | 14 sources, RRF fusion, provenance tracking |
| Worker selection | **Bridge** | Adapter dispatch, capability matching |
| Worktree/sandbox creation | **Bridge** | Git worktree, macOS sandbox profiles |
| Agent spawning | **Bridge** | mintmux session, PTY, adapter launch |
| Command authorization | **HomeBase** | Every command passes through Dafny reducer |
| State transitions | **HomeBase** | Dafny-compiled pure reducer is the only transition function |
| Durable persistence | **HomeBase** | Append-only journal, fsync, hash chain |
| External effect execution | **Bridge** | Bridge adapters execute effects; HomeBase tracks outcomes |
| Artifact collection | **Bridge** | Worker output, diffs, manifests |
| Release coordination | **Bridge** | PR verification, landed-work proof, worktree return |

### Verification

| Responsibility | Owner | Notes |
|---|---|---|
| Verification gate execution | **Bridge** | Build, test, vet, tldr, ccc, custom gates |
| Gate result interpretation | **Bridge** | Pass/fail/skip determination |
| Evidence registration | **HomeBase** | Evidence is an authoritative record with typed scope |
| Evidence acceptance/rejection | **HomeBase** | Dafny reducer validates evidence against claim requirements |
| Claim verification | **Bridge** | Re-runs verification commands in fresh checkout |
| Semantic review | **Bridge** | Adversarial review, diff analysis, scope checking |

### Human Interaction

| Responsibility | Owner | Notes |
|---|---|---|
| Approval requests | **HomeBase** | Escalation is a state transition, not a notification |
| Approval rendering | **Orbit** | Displays pending approvals, captures human decision |
| Human decision capture | **Orbit** | Submit approval/rejection as typed command |
| Status projection | **Orbit** | Read-only, reconstructed from HomeBase events |
| Timeline/transcript | **Orbit** | Read-only, reconstructed from HomeBase events |
| Evidence inspection | **Orbit** | Read-only, render evidence with provenance |
| Discrepancy display | **Orbit** | Read-only, render discrepancies from HomeBase |

### Recovery and Conclusion

| Responsibility | Owner | Notes |
|---|---|---|
| Failure detection | **Bridge** | Timeout, stale, non-zero exit, manifest malformation |
| Recovery proposal | **Bridge** | "This attempt failed; propose retry with X" |
| Recovery authorization | **HomeBase** | Dafny reducer enforces retry budget, idempotency |
| Retry counter increment | **HomeBase** | Only HomeBase can increment the counter |
| Terminal state decision | **HomeBase** | Dafny reducer decides when attempts are exhausted |
| Escalation | **HomeBase** | State transition to ESCALATED is a reducer decision |
| Conclusion acceptance | **HomeBase** | Final state transition to CONCLUDED |

### Knowledge and Context

| Responsibility | Owner | Notes |
|---|---|---|
| Source retrieval | **Bridge** | rg, tldr, githits, Neo4j, ccc, etc. |
| Knowledge graph queries | **Bridge** | Neo4j axiom corpus, LadybugDB |
| Context fusion | **Bridge** | RRF, scoring, assembly |
| Context provenance | **Bridge** | Each source result tagged with origin and freshness |
| Context as policy input | **HomeBase** | Never. Context informs proposals, never decides policy. |

### Currently duplicated authority

These responsibilities currently have competing owners:

| Responsibility | Current owners | Resolution |
|---|---|---|
| Retry policy | Bridge (RetryLimit, SPRT) + HomeBase (RecoveryDispatches) | **HomeBase** owns retry authorization |
| Attempt lifecycle | Bridge (protocol.go, 19 commands) + HomeBase (graph, 5 states) | **HomeBase** owns the lifecycle |
| Escalation protocol | Bridge (Escalation record) + HomeBase (ESCALATE state) | **HomeBase** owns escalation |
| Ledger/audit trail | Bridge (.bridge/ledger.jsonl) + HomeBase (journal) | **HomeBase** owns the authoritative ledger |
| Effect execution tracking | Bridge (spawn retry loop) + HomeBase (EffectState) | **HomeBase** owns effect state; Bridge executes |
| Verification gate policy | Bridge (verify.go) + HomeBase (not yet) | **Bridge** executes gates; **HomeBase** decides what gates are required |

---

## 2. End-to-End Workflow

The complete lifecycle for a software-engineering task, from user request to conclusion.

### Phase: Intent Capture

```
Initiating actor: User (via Orbit)
Command:          SubmitIntent{text, context}
HomeBase:         Opens Attempt with phase=ACTIVE
Events:           AttemptOpened{attemptID}
Effects:          None
Evidence:         User intent text, timestamp
Failure:          Invalid intent → HomeBase rejects, Orbit displays error
Terminal:         N/A (always succeeds unless malformed)
```

### Phase: Requirement Clarification

```
Initiating actor: Bridge (proposes clarification questions)
Command:          ProposeClarification{attemptID, questions}
HomeBase:         Accepts if attempt is ACTIVE; records clarification request
Events:           ClarificationRequested{attemptID, questions}
Effects:          None
Evidence:         Clarification questions, domain context used
Failure:          Attempt in wrong phase → HomeBase rejects
Terminal:         N/A
```

### Phase: Assurance Case Creation

```
Initiating actor: Bridge (proposes assurance case)
Command:          ProposeAssuranceCase{attemptID, claims, requirements, evidence}
HomeBase:         Validates structure; records case
Events:           AssuranceCaseProposed{attemptID, claimCount}
Effects:          None
Evidence:         Claims, requirements, hazard analysis
Failure:          Invalid case structure → HomeBase rejects
Terminal:         N/A
```

### Phase: Planning

```
Initiating actor: Bridge (proposes execution plan)
Command:          ProposePlan{attemptID, steps, dependencies, verificationGates}
HomeBase:         Validates plan against invariants; locks if valid
Events:           PlanLocked{attemptID, stepCount}
Effects:          None
Evidence:         Execution plan, dependency graph
Failure:          Invalid plan → HomeBase rejects with reason
Terminal:         N/A
```

### Phase: Task Decomposition

```
Initiating actor: Bridge
Command:          ProposeDecomposition{attemptID, subtasks}
HomeBase:         Records decomposition; does not independently verify
Events:           TasksDecomposed{attemptID, subtaskCount}
Effects:          None
Evidence:         Subtask definitions, atomization trace
Failure:          Decomposition failure → Bridge reports; HomeBase records
Terminal:         N/A
```

### Phase: Context Assembly

```
Initiating actor: Bridge (automatic, not a command)
HomeBase:         Not involved (context is candidate, not authoritative)
Events:           None (operational telemetry only)
Effects:          None
Evidence:         ContextPacket with provenance per source
Failure:          Missing source → signal drops, pipeline continues
Terminal:         N/A
```

### Phase: Worker Execution

```
Initiating actor: Bridge (executes committed effect intent)
Command:          (Effect was already authorized by HomeBase in RECOVER state)
HomeBase:         Already emitted EffectIntent; Bridge claims it
Events:           EffectClaimed{effectID, workerID}
Effects:          EffectIntent{attemptID, effectID, ordinal}
Evidence:         Worker manifest, assertions, diff, PR URL
Failure:          See Recovery Phase below
Terminal:         N/A
```

### Phase: Artifact Production

```
Initiating actor: Bridge (collects worker output)
HomeBase:         Not directly involved; artifacts are stored in artifact store
Events:           None (Bridge operational telemetry)
Effects:          None
Evidence:         Artifact hashes, locations, types
Failure:          Missing artifact → Bridge reports; triggers verification failure
Terminal:         N/A
```

### Phase: Verification

```
Initiating actor: Bridge (executes verification gates)
Command:          ProposeVerificationResult{attemptID, gateResults, evidence}
HomeBase:         Records verification result; does not interpret pass/fail
Events:           VerificationCompleted{attemptID, gateResults}
Effects:          None
Evidence:         Gate outputs, exit codes, timing, treeSHA
Failure:          Verification tool crash → Bridge reports as gate failure
Terminal:         N/A
```

### Phase: Discrepancy Handling

```
Initiating actor: Bridge (detects discrepancy)
Command:          ProposeDiscrepancy{attemptID, expected, actual, severity}
HomeBase:         Records discrepancy; may trigger recovery or escalation
Events:           DiscrepancyRecorded{attemptID, discrepancyID}
Effects:          May trigger EffectIntent for recovery
Evidence:         Expected vs actual, diff, scope violation details
Failure:          N/A (discrepancy is itself a failure signal)
Terminal:         N/A (always recorded)
```

### Phase: Retry or Recovery

```
Initiating actor: Bridge (proposes recovery)
Command:          ProposeRecovery{attemptID, idempotencyKey, recoveryType}
HomeBase:         Dafny reducer checks:
                    - Is attempt in RECOVERING phase?
                    - Has recovery budget been exhausted? (RecoveryDispatches < maxRecovery)
                    - Is idempotencyKey unique? (ProcessedCmdKeys)
                  If accepted:
                    - Increments RecoveryDispatches
                    - Emits EffectIntent for the recovery action
                    - Records EventRecoveryDispatched
                  If rejected:
                    - Records EventRecoveryRejected with reason
                    - If budget exhausted: transitions to ESCALATED
Events:           EventRecoveryDispatched OR EventRecoveryRejected
Effects:          EffectIntent for recovery action (if accepted)
Evidence:         Failure history, recovery attempt details
Failure:          Budget exhausted → HomeBase transitions to ESCALATED
Terminal:         ESCALATED if budget exhausted; otherwise returns to EXECUTION
```

### Phase: Acceptance

```
Initiating actor: Bridge (proposes conclusion)
Command:          Conclude{attemptID}
HomeBase:         Dafny reducer checks:
                    - Are all verification gates passing?
                    - Are all discrepancies resolved or accepted?
                    - Are all effects in terminal state?
                  If accepted:
                    - Transitions to CONCLUDED
                    - Records EventConcluded
                  If rejected:
                    - Returns rejection reason
Events:           EventConcluded{attemptID} OR rejection
Effects:          None
Evidence:         All verification results, discrepancy resolutions
Failure:          Unresolved issues → HomeBase rejects conclusion
Terminal:         CONCLUDED (accepted) or remains ACTIVE/RECOVERING (rejected)
```

### Phase: Release

```
Initiating actor: Bridge (executes release)
Command:          ProposeRelease{attemptID, landedWorkProof}
HomeBase:         Records release; validates landed-work proof exists
Events:           ReleaseCompleted{attemptID, reference}
Effects:          None (release is a recording, not an external effect)
Evidence:         LandedWorkProof (PR URL + MERGED status, or file existence)
Failure:          Invalid landed-work proof → HomeBase rejects
Terminal:         CONCLUDED (if previously accepted)
```

### Phase: Conclusion

```
Initiating actor: HomeBase (automatic after Conclude + Release)
HomeBase:         Attempt is CONCLUDED; no further commands accepted
Events:           (Already emitted EventConcluded)
Effects:          None
Evidence:         Complete audit trail
Terminal:         CONCLUDED is terminal
```

---

## 3. Typed Protocol Vocabulary

The minimum shared protocol. Start small; extend only when a concrete workflow requires it.

### Identity Types

```go
// AttemptID uniquely identifies a workflow attempt.
// Created by HomeBase when an attempt is opened.
// Format: opaque string, constructed only via ParseAttemptID.
type AttemptID struct { value string }

// CommandID uniquely identifies a command submitted to HomeBase.
// Generated by the submitter (Bridge or Orbit).
// Used for idempotency: HomeBase rejects duplicate CommandIDs.
type CommandID string

// EffectID uniquely identifies an effect intent emitted by HomeBase.
// Generated by HomeBase when an effect is authorized.
type EffectID string

// WorkerID identifies a worker instance.
// Generated by Bridge when a worker is spawned.
type WorkerID string

// ArtifactID identifies an artifact in the artifact store.
// Generated by Bridge when an artifact is collected.
// Content-addressed (hash of artifact content).
type ArtifactID string

// EvidenceID identifies an evidence record in the evidence store.
// Generated by HomeBase when evidence is registered.
type EvidenceID string

// ClaimID identifies a claim within an assurance case.
type ClaimID string

// DiscrepancyID identifies a recorded discrepancy.
type DiscrepancyID string

// VerificationRunID identifies a verification run.
type VerificationRunID string
```

### Core Commands (submitted to HomeBase)

```go
// Command is the interface for all commands submitted to HomeBase.
// HomeBase's Dafny reducer is the ONLY code that processes these.
type Command interface { isCommand() }

// OpenAttempt opens a new workflow attempt.
// Submitted by: Orbit (when user submits intent)
type OpenAttempt struct {
    CommandID   CommandID   // idempotency key
    Intent      string      // user's original intent text
    Metadata    map[string]string // project, repo, etc.
}

// ProposePlan proposes a locked execution plan.
// Submitted by: Bridge (after decomposition)
type ProposePlan struct {
    CommandID   CommandID
    AttemptID   AttemptID
    Steps       []PlanStep
    MaxRetries  uint8       // bridge's recommendation; HomeBase may reject
}

// ProposeRecovery proposes a recovery action after failure.
// Submitted by: Bridge (after detecting failure)
type ProposeRecovery struct {
    CommandID      CommandID
    AttemptID      AttemptID
    IdempotencyKey string    // unique per recovery proposal
    Version        uint64    // optimistic concurrency
    RecoveryType   string    // "retry", "different_worker", "different_adapter"
}

// Conclude attempts to conclude the attempt.
// Submitted by: Bridge (after all verification passes)
type Conclude struct {
    CommandID CommandID
    AttemptID AttemptID
}

// RegisterEvidence registers a piece of evidence.
// Submitted by: Bridge (after verification, artifact collection)
type RegisterEvidence struct {
    CommandID   CommandID
    AttemptID   AttemptID
    Evidence    EvidenceRecord
}

// RegisterArtifact registers an artifact.
// Submitted by: Bridge (after worker produces output)
type RegisterArtifact struct {
    CommandID   CommandID
    AttemptID   AttemptID
    Artifact    ArtifactRecord
}

// SubmitObservation submits an external observation.
// Submitted by: Bridge (after effect execution, or Orbit for human input)
type SubmitObservation struct {
    CommandID   CommandID
    AttemptID   AttemptID
    EffectID    EffectID   // which effect this observation relates to
    Observation ObservationRecord
}

// AcceptConclusion is the human approval command.
// Submitted by: Orbit (after human approves escalated attempt)
type AcceptConclusion struct {
    CommandID   CommandID
    AttemptID   AttemptID
    Reason      string
}

// RejectConclusion is the human rejection command.
// Submitted by: Orbit (after human rejects escalated attempt)
type RejectConclusion struct {
    CommandID   CommandID
    AttemptID   AttemptID
    Reason      string
}
```

### Core Events (emitted by HomeBase)

```go
// Event is the interface for all events emitted by HomeBase.
// Events are facts — they have already happened.
type Event interface { isEvent() }

type AttemptOpened struct {
    AttemptID   AttemptID
    Intent      string
    OpenedAt    time.Time
}

type PlanLocked struct {
    AttemptID   AttemptID
    Steps       []PlanStep
}

type RecoveryDispatched struct {
    AttemptID      AttemptID
    EffectID       EffectID
    Ordinal        uint8
    IdempotencyKey string
}

type RecoveryRejected struct {
    AttemptID AttemptID
    Reason    string
}

type EffectClaimed struct {
    EffectID   EffectID
    AttemptID  AttemptID
    WorkerID   WorkerID
    ClaimedAt  time.Time
}

type EffectCompleted struct {
    EffectID   EffectID
    AttemptID  AttemptID
    Outcome    EffectOutcome // succeeded, failed_retryable, failed_terminal, unknown
    ObservedAt time.Time
}

type EvidenceRegistered struct {
    EvidenceID EvidenceID
    AttemptID  AttemptID
    Type       EvidenceType
    Hash       string
}

type ArtifactRegistered struct {
    ArtifactID ArtifactID
    AttemptID  AttemptID
    Type       ArtifactType
    Hash       string
    Location   string
}

type DiscrepancyRecorded struct {
    DiscrepancyID DiscrepancyID
    AttemptID     AttemptID
    Severity      DiscrepancySeverity
    Description   string
}

type VerificationCompleted struct {
    AttemptID   AttemptID
    RunID       VerificationRunID
    GateResults []GateResult
}

type Concluded struct {
    AttemptID   AttemptID
    Outcome     ConclusionOutcome // accepted, rejected, escalated
    Reason      string
}
```

### Effect Intents (emitted by HomeBase, executed by Bridge)

```go
// EffectIntent is a durable request for external execution.
// Emitted by HomeBase as part of a Decision.
// Claimed by Bridge before execution.
type EffectIntent struct {
    AttemptID   AttemptID
    EffectID    EffectID
    Ordinal     uint8
    EffectType  EffectType
    Parameters  json.RawMessage
}

type EffectType string

const (
    EffectSpawnWorker    EffectType = "spawn_worker"
    EffectExecuteGate    EffectType = "execute_gate"
    EffectCollectArtifact EffectType = "collect_artifact"
    EffectReleaseWorktree EffectType = "release_worktree"
    EffectNotifyHuman    EffectType = "notify_human"
)
```

### Observation Records (submitted by Bridge to HomeBase)

```go
type ObservationRecord struct {
    EffectID    EffectID
    Outcome     EffectOutcome
    Output      json.RawMessage  // structured output from the effect
    Error       string           // error message if any
    ObservedAt  time.Time
    ObserverID  string           // which Bridge instance observed this
}

type EffectOutcome string

const (
    EffectSucceeded       EffectOutcome = "succeeded"
    EffectFailedRetryable EffectOutcome = "failed_retryable"
    EffectFailedTerminal  EffectOutcome = "failed_terminal"
    EffectOutcomeUnknown  EffectOutcome = "unknown"  // network timeout, etc.
)
```

### Evidence Records

```go
type EvidenceRecord struct {
    Type        EvidenceType
    Scope       EvidenceScope   // what this evidence proves
    Limitations []string        // what it explicitly does NOT prove
    Hash        string          // content hash of the evidence artifact
    Location    string          // where to find the evidence
    ProducedBy  string          // tool, human, or system that produced it
    ProducedAt  time.Time
}

type EvidenceType string

const (
    EvidenceTestPass     EvidenceType = "test_pass"
    EvidenceGatePass     EvidenceType = "gate_pass"
    EvidenceFormalProof  EvidenceType = "formal_proof"
    EvidenceTLA Model    EvidenceType = "tla_model"
    EvidenceHumanReview  EvidenceType = "human_review"
    EvidenceLLMReview    EvidenceType = "llm_review"
    EvidenceOperational  EvidenceType = "operational_observation"
)

type EvidenceScope struct {
    Proves       []string  // what this evidence establishes
    Assumes      []string  // what must be true for the evidence to hold
    DoesNotProve []string  // explicit non-goals
}
```

---

## 4. Authority Matrix

For every system, what it may do and what it is prohibited from doing.

### HomeBase

| Action | May? | Notes |
|---|---|---|
| Decide state transitions | **Yes** | Dafny reducer is the only transition function |
| Increment recovery counter | **Yes** | Only HomeBase can increment RecoveryDispatches |
| Authorize retry | **Yes** | Only HomeBase can emit a RecoveryDispatched event |
| Reject recovery | **Yes** | When budget exhausted or idempotency key duplicate |
| Accept conclusion | **Yes** | When all gates pass and discrepancies resolved |
| Escalate to human | **Yes** | When recovery budget exhausted |
| Register evidence | **Yes** | Evidence is an authoritative record |
| Reject invalid evidence | **Yes** | Evidence with wrong scope or type |
| Maintain durable journal | **Yes** | Append-only, fsync, hash-chained |
| Propose work | **No** | HomeBase never initiates work |
| Execute external effects | **No** | HomeBase emits intents; Bridge executes |
| Render UI | **No** | Orbit renders |
| Retrieve context | **No** | Bridge retrieves context |
| Run verification gates | **No** | Bridge runs gates |
| Spawn workers | **No** | Bridge spawns workers |
| Modify events after emission | **No** | Immutability invariant |
| Reinterpret terminal states | **No** | Once CONCLUDED, no further transitions |

### Bridge

| Action | May? | Notes |
|---|---|---|
| Propose commands | **Yes** | All commands go through HomeBase |
| Execute effect intents | **Yes** | Only after HomeBase emits them |
| Observe effect outcomes | **Yes** | Submit observations to HomeBase |
| Spawn workers | **Yes** | Worktree, sandbox, mintmux session |
| Assemble context | **Yes** | 14 sources, provenance tracking |
| Decompose tasks | **Yes** | Brain dump → tickets |
| Run verification gates | **Yes** | Build, test, vet, tldr, ccc, custom |
| Verify claims | **Yes** | Fresh checkout, re-run commands |
| Collect artifacts | **Yes** | Worker output, diffs, manifests |
| Coordinate release | **Yes** | PR verification, worktree return |
| Propose recovery | **Yes** | "This failed; I recommend retry with X" |
| Detect failures | **Yes** | Timeout, stale, non-zero exit |
| Decide to retry | **No** | Must propose; HomeBase decides |
| Increment retry counter | **No** | Only HomeBase can |
| Silently skip verification | **No** | Plan is locked; cannot skip steps |
| Fabricate success | **No** | Cannot claim pass without evidence |
| Reinterpret terminal states | **No** | Cannot override CONCLUDED |
| Authorize policy changes | **No** | Cannot change retry budget, gate requirements |
| Mutate HomeBase state directly | **No** | Only through commands |
| Accept conclusions | **No** | Only HomeBase accepts conclusions |

### Orbit

| Action | May? | Notes |
|---|---|---|
| Submit typed commands | **Yes** | SubmitIntent, AcceptConclusion, RejectConclusion |
| Render state projections | **Yes** | Read-only, reconstructed from HomeBase events |
| Display timelines | **Yes** | Read-only |
| Display transcripts | **Yes** | Read-only |
| Display evidence | **Yes** | Read-only, with provenance |
| Display discrepancies | **Yes** | Read-only |
| Display approval requests | **Yes** | Read-only, with accept/reject UI |
| Capture human decisions | **Yes** | Submit as AcceptConclusion/RejectConclusion |
| Capture user intent | **Yes** | Submit as OpenAttempt |
| Provide steering interface | **Yes** | Forward to Bridge as SubmitObservation |
| Mutate authoritative state | **No** | Never directly |
| Execute effects | **No** | Bridge executes |
| Decide policy | **No** | HomeBase decides |
| Generate evidence | **No** | Bridge generates evidence |
| Run verification | **No** | Bridge runs verification |
| Spawn workers | **No** | Bridge spawns workers |

### Context Systems

| Action | May? | Notes |
|---|---|---|
| Return candidate context | **Yes** | With provenance per source |
| Score relevance | **Yes** | RRF, signal levels |
| Report source freshness | **Yes** | When was this source last updated? |
| Make policy decisions | **No** | Context informs proposals; never decides |
| Claim authority | **No** | Candidate, not authoritative |
| Substitute for verification | **No** | Retrieval ≠ verification |

### Verification Systems

| Action | May? | Notes |
|---|---|---|
| Produce typed evidence | **Yes** | With explicit scope and limitations |
| Report pass/fail | **Yes** | Gate results |
| Crash | **Yes** | Treated as gate failure |
| Claim proof beyond scope | **No** | Test pass ≠ formal proof |
| Claim semantic correctness | **No** | Gate pass ≠ bug-free |
| Authorize decisions | **No** | Evidence informs; HomeBase decides |

---

## 5. Data and Ledger Model

### Authoritative Event Journal (HomeBase)

**What goes here:** Every event emitted by the Dafny reducer. This is the single source of truth for all state transitions.

```
Format: Append-only JSONL with hash chain
Location: ~/.homebase/journal/
Content:  Events only (not commands, not observations, not artifacts)
          Each line: {event, hash, timestamp, version}
          Hash chain: each line's hash = SHA-256(previous line)
```

**Properties:**
- Immutable (I1): append-only, no UPDATE/DELETE
- Durable (I3): fsync after every write
- Verifiable: hash chain can be recomputed
- Replayable: apply events in order → reconstruct any state

**What does NOT go here:**
- Artifacts (too large, go in artifact store)
- Context (candidate, not authoritative)
- Operational telemetry (goes in telemetry store)
- Worker transcripts (go in artifact store)
- Full evidence files (go in evidence store; journal stores hashes)

### Evidence Store (HomeBase)

**What goes here:** Evidence records with typed scope and limitations.

```
Format: Content-addressed store
Location: ~/.homebase/evidence/
Content:  EvidenceRecord + pointer to evidence artifact
Index:    By AttemptID, EvidenceType, ClaimID
```

**Properties:**
- Content-addressed: SHA-256 of evidence content
- Typed: every record has EvidenceType and EvidenceScope
- Scoped: explicit "proves X, assumes Y, does not prove Z"
- Immutable: once registered, cannot be modified

### Artifact Store (Bridge)

**What goes here:** Worker output, diffs, manifests, transcripts.

```
Format: Content-addressed store
Location: ~/.bridge/artifacts/ (or worktree .bridge/)
Content:  Diffs, manifests, PR URLs, transcripts, build logs
Index:    By AttemptID, ArtifactType
```

**Properties:**
- Content-addressed: SHA-256 of artifact content
- Temporary: may be cleaned up after conclusion
- Not authoritative: artifacts are outputs, not decisions

### Read Projections (Orbit)

**What goes here:** Materialized views reconstructed from HomeBase events.

```
Format: In-memory or local DB (SQLite)
Location: Orbit process memory
Content:  Attempt status, timelines, claim status, discrepancy lists
Built by: Replaying HomeBase events from journal
```

**Properties:**
- Rebuildable: can always reconstruct from journal
- Read-only: Orbit never writes to projections directly
- Eventually consistent: updated when new events are observed

### Context Indexes (Bridge)

**What goes here:** Search indexes, knowledge graphs, TLDR summaries.

```
Format: Various (Neo4j, CocoIndex, .bridge/index/)
Location: Bridge-managed
Content:  Source code indexes, documentation, axiom corpus, research
```

**Properties:**
- Candidate: not authoritative
- Rebuildable: can be regenerated from sources
- Stale-tolerant: missing source = dropped signal, not failure

### Operational Telemetry (Bridge)

**What goes here:** Timing, resource usage, adapter performance, context assembly stats.

```
Format: JSONL or metrics
Location: ~/.bridge/telemetry/
Content:  Latency histograms, quota snapshots, source freshness, error rates
```

**Properties:**
- Non-authoritative: for debugging and optimization
- Lossy: can be dropped without affecting correctness
- Not in the event journal: operational, not decisional

### Temporary Worker State (Bridge)

**What goes here:** Worktree contents, sandbox profiles, mintmux sessions.

```
Format: Filesystem
Location: ~/projects/.worktrees/, ~/.mintmux/
Content:  Git worktrees, PTY sessions, environment
```

**Properties:**
- Ephemeral: cleaned up after attempt or on crash recovery
- Isolated: per-worker, per-attempt
- Not durable: may be lost on machine restart

### Connecting the Stores

```
Event Journal ──(AttemptID)──> Evidence Store ──(hash)──> Artifact Store
       │                            │
       │ (EffectID)                 │ (ClaimID)
       ▼                            ▼
  Effect State                 Assurance Case
       │
       │ (WorkerID, ArtifactID)
       ▼
  Operational Telemetry
```

**Identity hashes needed:**
- `ArtifactID = SHA-256(artifact_content)` — content-addressed
- `EvidenceID = SHA-256(evidence_record)` — content-addressed
- `CommandID = SHA-256(command_type + attemptID + nonce)` — idempotency
- Journal hash chain: `SHA-256(previous_line_json)` — tamper evidence

---

## 6. Failure and Recovery Model

### Worker Crash

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (stale detection via mtime, or hard timeout) |
| Who decides response? | Bridge proposes recovery; HomeBase decides |
| Durable record | EffectCompleted with outcome=failed_terminal; RecoveryDispatched or RecoveryRejected event |
| Recovery action | Bridge proposes new spawn with different worker; HomeBase authorizes if budget remains |
| Terminal behavior | After maxRecovery exhausted, HomeBase transitions to ESCALATED |

### Orchestrator (Bridge) Crash

| Aspect | Detail |
|---|---|
| Who observes it? | HomeBase (effect timeout, no observation submitted) |
| Who decides response? | HomeBase marks effect as OutcomeUnknown; Bridge on restart scans for orphaned effects |
| Durable record | EffectCompleted with outcome=unknown |
| Recovery action | Bridge restart: scan HomeBase for effects in non-terminal state; propose recovery for each |
| Terminal behavior | Orphaned effects with no recovery budget → ESCALATED |

### HomeBase Restart

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (command submission fails, event subscription drops) |
| Who decides response? | HomeBase replays journal on restart; Bridge reconnects and resubscribes |
| Durable record | Journal is fsynced; no events lost on restart |
| Recovery action | HomeBase replays journal → reconstructs all state; Bridge re-subscribes to events |
| Terminal behavior | In-flight commands are idempotent; duplicate CommandIDs are rejected |

### Journal Corruption

| Aspect | Detail |
|---|---|
| Who observes it? | HomeBase (hash chain verification fails on read) |
| Who decides response? | HomeBase: refuse to serve state beyond last valid hash |
| Durable record | Corruption detected event; manual recovery required |
| Recovery action | Restore from backup OR accept data loss (truncate at last valid hash) |
| Terminal behavior | Cannot proceed without human intervention |

### Stale Worker Result

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (mtime-based staleness detection) |
| Who decides response? | Bridge proposes kill + retry; HomeBase decides |
| Durable record | StaleError detected; EffectCompleted with outcome=failed_terminal |
| Recovery action | Bridge kills session, returns worktree, proposes new spawn |
| Terminal behavior | Same as worker crash |

### Duplicated Commands

| Aspect | Detail |
|---|---|
| Who observes it? | HomeBase (ProcessedCmdKeys contains duplicate CommandID) |
| Who decides response? | HomeBase Dafny reducer: DecisionNoOp |
| Durable record | No new event; original event already recorded |
| Recovery action | None needed; idempotency is built into the reducer |
| Terminal behavior | N/A (benign) |

### Repeated Effects

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (sees effect intent for already-completed effect) |
| Who decides response? | Bridge: check if effect already completed; if so, re-submit observation |
| Durable record | No duplicate; DispatchedEffectIDs prevents re-dispatch |
| Recovery action | Bridge re-submits existing observation |
| Terminal behavior | N/A (benign) |

### Network Timeout

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (effect execution timeout) |
| Who decides response? | Bridge proposes recovery; HomeBase decides |
| Durable record | EffectCompleted with outcome=unknown |
| Recovery action | Bridge may propose retry; HomeBase checks budget |
| Terminal behavior | If budget exhausted → ESCALATED |

### Unknown Remote Outcome

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (effect executed but outcome unclear — e.g., PR created but gh crashed) |
| Who decides response? | Bridge proposes recovery with reconciliation check |
| Durable record | EffectCompleted with outcome=unknown + reconciliation attempt |
| Recovery action | Bridge checks observable truth (e.g., `gh pr list`) before proposing |
| Terminal behavior | If reconciliation finds success → submit as EffectSucceeded; if not → propose retry |

### Verification Tool Failure

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (gate exits non-zero or crashes) |
| Who decides response? | Bridge proposes ProposeDiscrepancy; HomeBase records |
| Durable record | VerificationCompleted with failed gate; DiscrepancyRecorded |
| Recovery action | Bridge may propose retry with different verification approach |
| Terminal behavior | Unresolved verification failures block conclusion |

### Conflicting Evidence

| Aspect | Detail |
|---|---|
| Who observes it? | Bridge (two verification runs produce different results) |
| Who decides response? | Bridge proposes discrepancy; HomeBase records |
| Durable record | Both evidence records + DiscrepancyRecorded |
| Recovery action | Human review required (escalation) |
| Terminal behavior | Blocked until human resolves |

### Unsupported Event Version

| Aspect | Detail |
|---|---|
| Who observes it? | Orbit (projection cannot parse event) |
| Who decides response? | Orbit: display as "unknown event type" with raw data |
| Durable record | Event is already in journal; Orbit cannot parse it |
| Recovery action | Update Orbit projection to support new event version |
| Terminal behavior | N/A (forward compatibility issue, not a failure) |

### Human Cancellation

| Aspect | Detail |
|---|---|
| Who observes it? | Orbit (human submits Cancel command) |
| Who decides response? | HomeBase: transition to CONCLUDED with outcome=rejected |
| Durable record | Concluded{outcome: rejected, reason: "human cancelled"} |
| Recovery action | None; attempt is terminal |
| Terminal behavior | CONCLUDED; no further commands accepted |

---

## 7. Assurance Model

### Object Definitions

```
Requirement
  A statement of what must be true for the system to be correct.
  Example: "No worker can modify files outside allowed_paths."
  Owned by: Assurance Case (Bridge proposes, HomeBase records)

Hazard
  A condition that could violate a requirement.
  Example: "Worker has full shell access and can write anywhere."
  Owned by: Assurance Case

Claim
  An assertion that a requirement is satisfied, backed by evidence.
  Example: "Sandbox deny-default profile restricts writes to allowed_paths."
  Owned by: HomeBase (evidence store)
  Lifecycle: proposed → verified → accepted → (may be invalidated later)

Assumption
  A condition that must hold for a claim to be valid.
  Example: "macOS Seatbelt sandbox is correctly configured and enforced by kernel."
  Owned by: Assurance Case
  If invalidated: all dependent claims become unverified.

Evidence
  A typed artifact that supports or refutes a claim.
  Example: "sandbox profile test passes; seatbelt-exec denies write to /tmp/other"
  Owned by: HomeBase (evidence store)
  Has explicit scope: what it proves, what it assumes, what it does NOT prove.

Decision
  The output of HomeBase's Dafny reducer applying a command to state.
  Contains: accepted/rejected status, events, effect intents.
  Owned by: HomeBase (event journal)

Command
  A request to change state, submitted to HomeBase.
  Owned by: HomeBase (recorded in journal as part of decision)

Event
  A fact that has occurred. Emitted by HomeBase.
  Owned by: HomeBase (event journal)

Artifact
  A concrete output: diff, manifest, PR, build log, transcript.
  Owned by: Bridge (artifact store)
  Referenced by evidence via content hash.

Discrepancy
  A recorded gap between expected and actual.
  Example: "Worker modified files outside allowed_paths."
  Owned by: HomeBase (event journal)
  Blocks conclusion until resolved.
```

### Dependency Graph

```
Requirements
    │
    ├── Hazards (threats to requirements)
    │
    ├── Claims (requirements are satisfied)
    │       │
    │       ├── Evidence (supports claim)
    │       │       │
    │       │       └── Artifacts (concrete proof)
    │       │
    │       └── Assumptions (must hold for claim)
    │
    └── Decisions (authorize actions)
            │
            ├── Commands (request decisions)
            │
            └── Events (record decisions)
```

### Invalidation Propagation

```
Changed requirement:
  → All claims citing that requirement become unverified
  → All evidence supporting those claims becomes irrelevant
  → All decisions based on those claims are flagged for review
  → Any CONCLUDED attempt that depended on those claims is flagged

Failed test (evidence invalidated):
  → The specific claim the evidence supported becomes unverified
  → If no alternative evidence exists, the claim is refuted
  → Any decision that depended on that claim is flagged
  → The attempt cannot be concluded until the claim is re-verified

Modified generated artifact:
  → Evidence that hashes the artifact now fails hash verification
  → The claim is flagged as tampered
  → The attempt enters discrepancy state

Refuted external-tool claim:
  → If the tool was the sole evidence for a claim, the claim is refuted
  → Downstream claims that depended on the refuted claim become unverified
  → Cascade stops at independently-verified claims
```

---

## 8. Boundary Contracts (Transport-Independent)

### Contract 1: SubmitCommand

```
Submitter: Bridge or Orbit
Receiver:  HomeBase

Input:  Command (typed, with CommandID for idempotency)
Output: Decision {Status, Events[], Effects[]}

Contract:
  - Idempotent: same CommandID → same Decision (no side effects)
  - Atomic: command is fully accepted or fully rejected
  - Versioned: optimistic concurrency via Version field
  - Durable: Decision is fsynced before response

Error cases:
  - Duplicate CommandID → DecisionNoOp (return original events)
  - Version mismatch → DecisionRejected (caller must re-read state)
  - Invalid phase → DecisionRejected (command not valid in current phase)
  - Malformed command → DecisionRejected (validation error)
```

### Contract 2: SubscribeEvents

```
Subscriber: Bridge or Orbit
Publisher:  HomeBase

Input:  AttemptID (or empty for all), starting version
Output: Stream of Events

Contract:
  - Ordered: events delivered in journal order
  - Durable: subscriber can restart from last seen version
  - At-least-once: may redeliver; subscriber must handle idempotently
  - Filtered: subscriber specifies which event types it wants

Error cases:
  - Unknown AttemptID → error
  - Version too old (journal truncated) → full resync required
```

### Contract 3: ClaimEffect

```
Claimer:  Bridge
Authority: HomeBase

Input:  EffectID, WorkerID
Output: Claim confirmed or rejected

Contract:
  - Only one claimer per effect (enforced by HomeBase)
  - Claim is durable (recorded in journal)
  - Claim is revocable (if worker dies, Bridge can unclaim)
  - Claim timeout: if no observation within T, effect is marked unknown

Error cases:
  - Effect already claimed → rejected
  - Effect in terminal state → rejected
  - Unknown EffectID → rejected
```

### Contract 4: SubmitObservation

```
Submitter: Bridge
Receiver:  HomeBase

Input:  EffectID, ObservationRecord {Outcome, Output, Error}
Output: Observation accepted or rejected

Contract:
  - Immutable: once submitted, observation cannot be changed
  - May be submitted multiple times for the same effect (idempotent)
  - Transitions effect to terminal state if outcome is terminal

Error cases:
  - Unknown EffectID → rejected
  - Effect already in terminal state → accepted but no state change (idempotent)
```

### Contract 5: RegisterEvidence

```
Submitter: Bridge
Receiver:  HomeBase

Input:  EvidenceRecord {Type, Scope, Hash, Location, ProducedBy}
Output: EvidenceID

Contract:
  - Content-addressed: EvidenceID = SHA-256(evidence)
  - Immutable: evidence cannot be modified after registration
  - Scoped: must declare what it proves and what it doesn't
  - Linked: associated with AttemptID and optionally ClaimID

Error cases:
  - Invalid scope (proves nothing) → rejected
  - Missing required fields → rejected
```

### Contract 6: RegisterArtifact

```
Submitter: Bridge
Receiver:  HomeBase

Input:  ArtifactRecord {Type, Hash, Location}
Output: ArtifactID

Contract:
  - Content-addressed: ArtifactID = SHA-256(artifact)
  - HomeBase stores the record (hash + location), not the artifact
  - Artifact itself stays in Bridge's artifact store

Error cases:
  - Hash mismatch → rejected
  - Missing location → rejected
```

### Contract 7: RequestVerification

```
Submitter: Bridge
Receiver:  Bridge (local operation, not a HomeBase command)

This is a Bridge-internal operation, not a HomeBase boundary.
Bridge runs verification gates locally.
Result is submitted to HomeBase as RegisterEvidence.

Contract:
  - Verification is re-runnable: fresh checkout, same commands
  - Gate failure ≠ system failure: gates can crash, be missing, or timeout
  - Missing gate binary → skip (not fail)
  - Gate timeout → fail
```

### Contract 8: AcceptConclusion / RejectConclusion

```
Submitter: Orbit (on behalf of human)
Receiver:  HomeBase

Input:  AttemptID, Reason
Output: Decision {Status, Events}

Contract:
  - Only valid when attempt is in ESCALATED phase
  - Accept → transition to CONCLUDED with outcome=accepted
  - Reject → transition to CONCLUDED with outcome=rejected
  - Human decision is recorded as an event with reason

Error cases:
  - Attempt not in ESCALATED phase → rejected
  - Already CONCLUDED → rejected
```

---

## 9. Minimal Initial Vertical Slice

### The Slice: "One Human Task, One Worker, One Gate"

```
Orbit submits a coding task
  → HomeBase opens an attempt
  → Bridge plans: one spawn effect, one verify gate
  → HomeBase locks plan and emits EffectIntent(spawn_worker)
  → Bridge claims effect, spawns one isolated worker
  → Worker produces one patch artifact
  → Bridge submits observation: worker done, artifact at hash X
  → Bridge runs one deterministic gate (go build)
  → Bridge registers evidence: gate passed
  → Bridge proposes Conclude
  → HomeBase validates: all effects complete, evidence registered
  → HomeBase accepts conclusion
  → Orbit renders the complete replayable timeline
```

### What is intentionally excluded:

- **No retry logic.** One attempt only. Recovery protocol is tested separately.
- **No task decomposition.** Single task, no subtasks.
- **No context assembly.** Pre-provided context, no 14-source fan-out.
- **No adapter dispatch.** Single hardcoded adapter.
- **No sandbox.** Plain worktree, no macOS Seatbelt.
- **No formal verification.** No Dafny proofs of the workflow itself (HomeBase's reducer is Dafny-verified, but the workflow integration is not).
- **No release coordination.** Worker produces diff; no PR, no landed-work proof.
- **No human escalation.** No ESCALATED state exercised.
- **No discrepancy handling.** Happy path only.
- **No Orbit UI.** CLI-only; Orbit is a command-line invocation.
- **No evidence scope validation.** Evidence is registered but not validated against claims.
- **No hash chain verification.** Journal is append-only but not hash-chained.

### What this slice proves:

1. That HomeBase's Dafny reducer can process real commands from Bridge.
2. That Bridge can execute HomeBase's effect intents and submit observations.
3. That the event journal is replayable end-to-end.
4. That the typed protocol vocabulary is sufficient for the simplest workflow.
5. That the authority boundaries hold: Bridge proposes, HomeBase decides.

### What this slice does NOT prove:

- That the system handles failures (tested in slice 2).
- That the system handles concurrency (tested in slice 3).
- That the system scales to real workloads (slice 4+).
- That the contracts are sufficient for complex workflows (slice 4+).

---

## 10. Sequencing Plan

### Design Now (this document)

- [x] Responsibility boundaries
- [x] Protocol vocabulary
- [x] Authority matrix
- [x] End-to-end sequence
- [x] Storage and ledger boundaries
- [x] Failure model
- [x] Assurance graph
- [x] Minimal vertical slice
- [ ] Open questions and unresolved risks
- [ ] Claims requiring primary-source verification

### Prototype Behind Interfaces (now, in parallel)

**HomeBase side:**
- [ ] Implement the 8 boundary contracts as Go interfaces (not HTTP/gRPC yet)
- [ ] Extend Dafny reducer to handle the full command vocabulary (OpenAttempt, ProposePlan, ProposeRecovery, Conclude, RegisterEvidence, RegisterArtifact, SubmitObservation)
- [ ] Add event types for the full vocabulary
- [ ] Implement journal replay for the full event vocabulary
- [ ] Stub the evidence store and artifact registry

**Bridge side:**
- [ ] Define Bridge-side adapter interfaces for HomeBase (SubmitCommand, SubscribeEvents, ClaimEffect, SubmitObservation)
- [ ] Implement a HomeBase client that wraps the interfaces
- [ ] Wire the spawn path to use HomeBase for attempt lifecycle (instead of Bridge's own protocol.go)
- [ ] Add a `--homebase` flag to bridge spawn (default off; when on, uses HomeBase)
- [ ] Keep existing Bridge ledger and protocol for backward compatibility

**Orbit side:**
- [ ] Define Orbit's projection interface (SubscribeEvents, render state)
- [ ] Implement a minimal CLI: `orbit submit "<task>"` → displays attempt timeline
- [ ] Implement event replay → timeline projection

### Blocked Until HomeBase Semantics Stabilize

These require the command vocabulary and reducer semantics to be frozen:

- [ ] Bridge's protocol.go retirement (replaced by HomeBase)
- [ ] Bridge's ledger.jsonl retirement (replaced by HomeBase journal)
- [ ] Bridge's retry loop rewritten to propose Recovery to HomeBase
- [ ] Bridge's escalation rewritten to react to HomeBase ESCALATED state
- [ ] Orbit's full dashboard (depends on stable event schema)

### Formally Verified Later

- [ ] Dafny proof that the reducer maintains all 6 invariants (I1-I6) for the full command vocabulary
- [ ] TLA+ model of the complete attempt lifecycle (including Bridge and Orbit)
- [ ] Proof that the protocol is deadlock-free under concurrent command submission
- [ ] Proof that idempotency holds for all command types

### Operational Hardening Later

- [ ] Journal backup and restore
- [ ] Journal compaction (truncate old versions)
- [ ] Hash chain verification on read
- [ ] Multi-process HomeBase (Raft consensus for HA)
- [ ] gRPC/HTTP transport layer
- [ ] Authentication and authorization between systems
- [ ] Monitoring and alerting on escalation rate
- [ ] Performance benchmarks

---

## 11. Open Questions and Unresolved Risks

### Q1: Does the Dafny reducer need to be extended for the full command vocabulary?

The current Dafny reducer (`internal/dafny_reducer/Reducer.go`, 30KB) handles `CommandProposeRecovery` and `CommandConclude`. The full vocabulary (Section 3) adds 8 more commands. Each requires a Dafny type, a reducer case, and invariants. **Risk:** extending the Dafny model is the bottleneck. **Mitigation:** start with the minimal slice (2 commands: OpenAttempt + Conclude); add commands incrementally.

### Q2: What is the transport layer?

The contracts are transport-independent, but something must carry bytes. **Options:** Unix socket + JSON (simplest), gRPC (if we need streaming), in-process (if HomeBase is a library). **Recommendation:** start in-process (Go interface calls); add Unix socket when Orbit needs to be a separate process. Do not decide prematurely.

### Q3: How does Bridge discover HomeBase?

If HomeBase is a separate process, Bridge needs to find it. **Options:** well-known Unix socket path, environment variable, config file. **Recommendation:** `$HOME/.homebase/socket` as default; `HOMEBASE_SOCKET` env var override.

### Q4: What happens to Bridge's existing ledger data?

Bridge's `.bridge/ledger.jsonl` has 102+ entries of real operational history. This is valuable data. **Options:** migrate to HomeBase journal (complex, lossy), keep as read-only archive, dual-write during transition. **Recommendation:** keep as read-only archive; new attempts use HomeBase journal. No migration.

### Q5: Does Orbit need its own persistence?

Orbit currently has no persistence. With HomeBase, Orbit can reconstruct all state from the event journal. **Recommendation:** Orbit does not need its own persistence. It can cache projections locally for performance, but the cache is rebuildable.

### Q6: What is the concurrency model?

If multiple Bridge instances submit commands concurrently, HomeBase must serialize them. **Options:** mutex (single process), Raft (multi-process), optimistic concurrency (version field). **Current design:** optimistic concurrency via Version field. **Risk:** high contention under load. **Mitigation:** single-process HomeBase is sufficient for the captain's machine; Raft is operational hardening.

### Q7: How does the human interact with escalated attempts?

The design says Orbit submits AcceptConclusion/RejectConclusion. But what if the human is not using Orbit? **Options:** CLI tool, web dashboard, filesystem signal (touch a file). **Recommendation:** CLI tool (`homebase approve <attemptID>`) as fallback; Orbit as primary.

### Q8: What about the existing Bridge features that work?

Bridge has a working spawn pipeline, verify pipeline, create pipeline, context assembly, and dispatch. These are valuable and should not be blocked. **Recommendation:** Bridge continues to work as-is. The HomeBase integration is opt-in via `--homebase` flag. Existing workflows are unaffected during the transition.

### Q9: Is the protocol vocabulary too small?

The vocabulary has 8 commands, 11 events, 5 effect types. This is intentionally minimal. **Risk:** we discover a missing command during implementation. **Mitigation:** the protocol is versioned; new commands can be added without breaking existing ones. The Dafny reducer can be extended incrementally.

### Q10: What about the axiom corpus and Neo4j?

The current HomeBase design requires axiom grounding (I6): every decision must cite an axiom from Neo4j. The architecture design does not change this. Bridge's context assembly queries Neo4j; the axioms are included in the context packet. HomeBase's axiom grounding is a separate concern: it validates that the axioms cited in the assurance case actually exist in the corpus.

---

## 12. Claims Requiring Primary-Source Verification Before Implementation

These claims are made in this document and must be verified against primary sources before any code is written:

| # | Claim | Primary Source | Verification Method |
|---|---|---|---|
| C1 | "Dafny compiler can generate Go code that implements the full command vocabulary" | Dafny documentation, Dafny Go compiler README | Write a small Dafny model with all 8 command types; compile to Go; verify it builds |
| C2 | "HomeBase's current Dafny reducer (Reducer.go, 30KB) was generated by the Dafny compiler and not hand-written" | Dafny source file (if it exists), git history | Locate the `.dfy` source; verify `Reducer.go` has a `// Generated by Dafny` header or equivalent |
| C3 | "HomeBase's journal (internal/journal/binary.go) is append-only with fsync" | Source code | Verify no UPDATE/DELETE paths; verify Sync() call after write |
| C4 | "Bridge's protocol.go state machine has 19 commands" | Source code | Count command types in internal/protocol/ |
| C5 | "Bridge's ledger (.bridge/ledger.jsonl) has a hash chain" | Source code, existing ledger file | Verify SHA-256 chain in ledger.go; verify existing ledger file validates |
| C6 | "Orbit's cmd/sia-v2 does not exist in the tree" | Filesystem | `ls ~/projects/orbit/cmd/sia-v2`; verify CLAUDE.md claim |
| C7 | "orbit-orchestrator is built but not started" | Filesystem, process list | `ls ~/projects/orbit/cmd/orbit-orchestrator/`; `ps aux | grep orbit-orchestrator` |
| C8 | "Neo4j has 2231 axioms" | Neo4j query | `cypher-shell "MATCH (a:Axiom) RETURN count(a)"` |
| C9 | "The Dafny runtime library (DafnyRuntimeGo) supports the types we need (sequences, sets, datatypes)" | DafnyRuntimeGo documentation | Verify the Go runtime supports SeqOfString, SetOf, datatype companions |
| C10 | "Bridge's context assembler has 14 sources" | Source code | Count Source implementations in internal/context/ |

---

## Appendix A: What This Design Does NOT Do

This design intentionally does NOT:

1. **Rewrite HomeBase as a general workflow engine.** HomeBase is an authority and assurance kernel, not Airflow or Temporal.
2. **Put agent reasoning inside the authoritative reducer.** The Dafny reducer is a pure function; it has no LLM calls, no context retrieval, no heuristics.
3. **Let Bridge become a second policy authority.** Bridge proposes; HomeBase decides. This is a hard boundary.
4. **Let Orbit directly mutate state.** Orbit submits commands; HomeBase processes them. Orbit never writes to the journal.
5. **Treat retrieval results as verified facts.** Context carries provenance; it is candidate, not authoritative.
6. **Claim exactly-once external execution.** Effect outcomes can be unknown. The system handles this explicitly.
7. **Create fake generated-code provenance.** Every artifact has a content hash and a producer identity.
8. **Claim mathematical proof from tests.** Evidence types are explicit: test_pass ≠ formal_proof.
9. **Design a giant platform before proving one vertical slice.** The minimal slice is one task, one worker, one gate.
10. **Mandate cloud infrastructure.** Everything runs on the captain's machine. Unix sockets, local files, local processes.
11. **Make any component irreplaceable.** Every system is replaceable behind its protocol boundary.

---

## Appendix B: Current State Assessment

### What is healthy

- **HomeBase's Dafny core is real.** The `Reducer.go` is 30KB of generated code from a Dafny model. This is the right architecture — the pure semantic core is formally specified and compiled, not hand-written.
- **Bridge's spawn/verify pipeline works.** It has produced real PRs across multiple repos. The context assembly, worktree isolation, and adapter dispatch are mature.
- **Orbit's tokenrouter is simple and correct.** Per-second rate limiting, key rotation, cooldown. The adversarial review pipeline produces real findings.

### What needs attention

- **Duplicated authority.** Both Bridge and HomeBase define retry policies, attempt lifecycles, and ledgers. This is the most urgent architectural issue.
- **Bridge's protocol package (15 files, 19 commands).** This is a significant investment that overlaps with HomeBase's state machine. The transition plan must respect this.
- **Orbit has no state.** It's a stateless shell, which is correct for its role, but it has no way to render historical state. The event subscription model fixes this.
- **HomeBase is early.** The Dafny reducer handles only 2 commands. The journal exists but is not yet integrated with the reducer. The evidence store is not implemented.

### What is a real risk

- **The Dafny bottleneck.** Extending the Dafny model for the full command vocabulary requires Dafny expertise. If the captain is the only person who can extend the Dafny model, this is a single-point-of-failure. **Mitigation:** the protocol vocabulary is minimal; Dafny model extensions are incremental; the Go interfaces can be tested with a fake reducer while the Dafny model is extended.
- **Bridge migration complexity.** Bridge has 17 packages and 60+ source files. Rewiring it to use HomeBase is a significant effort. **Mitigation:** opt-in via `--homebase` flag; existing code paths unchanged; migrate incrementally.
- **Protocol ossification.** If the protocol vocabulary is frozen too early, it may not support real workflows. **Mitigation:** the vocabulary is versioned; the minimal slice proves the protocol before freezing.