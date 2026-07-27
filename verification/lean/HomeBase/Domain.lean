-- HomeBase Domain Model (Repaired)
-- Complete abstract domain for decision governance
-- No persistence, cryptography, or external effects

namespace HomeBase

-- Opaque typed value wrappers
structure Hash where
  value : String
  deriving DecidableEq, Hashable

structure EffectKind where
  value : String
  deriving DecidableEq, Hashable

structure FailureClass where
  value : String
  deriving DecidableEq, Hashable

structure EventID where
  value : String
  deriving DecidableEq, Hashable

-- Identity Types (opaque)
structure TaskID where
  value : String
  deriving DecidableEq, Hashable

structure ContractID where
  value : String
  deriving DecidableEq, Hashable

structure ContractVersion where
  value : Nat
  deriving DecidableEq, Hashable

structure AttemptID where
  value : String
  deriving DecidableEq, Hashable

structure EffectID where
  value : String
  deriving DecidableEq, Hashable

structure ObservationID where
  value : String
  deriving DecidableEq, Hashable

structure EvidenceID where
  value : String
  deriving DecidableEq, Hashable

structure ObligationID where
  value : String
  deriving DecidableEq, Hashable

structure CommandID where
  value : String
  deriving DecidableEq, Hashable

structure AuthorityID where
  value : String
  deriving DecidableEq, Hashable

structure CorrelationID where
  value : String
  deriving DecidableEq, Hashable

-- Enumerations
inductive TaskStatus where
  | Draft        -- Initial state: no contract yet
  | Active       -- Contract locked, executing attempts
  | Completed    -- All obligations satisfied
  | Escalated    -- Escalated to human decision
  deriving DecidableEq

inductive AttemptStatus where
  | Open
  | Succeeded
  | Failed
  | OutcomeUnknown
  deriving DecidableEq

-- Attempt conclusion outcome (explicit reason for conclusion)
inductive AttemptOutcome where
  | Succeeded       -- All effects succeeded
  | Failed          -- One or more effects failed
  | OutcomeUnknown  -- Outcome indeterminate
  | Cancelled       -- Attempt cancelled/abandoned
  deriving DecidableEq

inductive IntentStatus where
  | Committed
  | OutcomeNeeded
  | Terminal
  deriving DecidableEq

inductive ObservationOutcome where
  | NotStarted
  | Running
  | Succeeded
  | Failed
  | Unknown
  deriving DecidableEq

inductive AuthorityRole where
  | TaskInitiator
  | Orchestrator
  | BridgeAdapter
  | Verifier
  | RecoveryController
  deriving DecidableEq

inductive RejectionReason where
  | STALE_VERSION
  | UNAUTHORIZED
  | INVALID_STATUS
  | TASK_ID_MISMATCH
  | CONTRACT_ALREADY_LOCKED
  | CONFLICTING_CONTRACT
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
  deriving DecidableEq

inductive NoOpReason where
  | COMMAND_ALREADY_APPLIED
  | IDENTICAL_CONTRACT
  | IDENTICAL_ATTEMPT
  | IDENTICAL_EFFECT_INTENT
  | IDENTICAL_OBSERVATION
  | IDENTICAL_EVIDENCE
  | OBLIGATION_ALREADY_SATISFIED
  | ALREADY_COMPLETED
  | ALREADY_ESCALATED
  deriving DecidableEq

-- Core Domain Types
structure Authority where
  principal_id : AuthorityID
  role : AuthorityRole
  deriving DecidableEq

-- Complete Contract (stored in TaskState, not just ContractRef)
structure Contract where
  contract_id : ContractID
  contract_version : ContractVersion
  contract_digest : Hash
  required_obligations : Finset ObligationID
  allowed_effect_kinds : Finset EffectKind
  max_attempts : Nat
  deriving DecidableEq

structure Attempt where
  attempt_id : AttemptID
  ordinal : Nat
  status : AttemptStatus
  effect_ids : Finset EffectID
  deriving DecidableEq

-- EffectIntent now includes attempt_id for ownership tracking
structure EffectIntent where
  effect_id : EffectID
  attempt_id : AttemptID
  effect_kind : EffectKind
  request_digest : Hash
  status : IntentStatus
  deriving DecidableEq

structure Observation where
  observation_id : ObservationID
  attempt_id : AttemptID
  effect_id : EffectID
  outcome : ObservationOutcome
  result_digest : Option Hash
  deriving DecidableEq

structure Evidence where
  evidence_id : EvidenceID
  attempt_id : AttemptID
  source_observation_id : ObservationID
  evidence_digest : Hash
  deriving DecidableEq

structure ObligationSatisfaction where
  obligation_id : ObligationID
  evidence_ids : Finset EvidenceID
  deriving DecidableEq

structure CommandReceipt where
  command_fingerprint : Hash
  resulting_event_types : List String
  deriving DecidableEq

structure Escalation where
  failure_class : FailureClass
  reason : String
  related_effect_id : Option EffectID
  requested_at : Nat
  deriving DecidableEq

-- Current State Projection
structure TaskState where
  task_id : TaskID
  version : Nat
  status : TaskStatus

  -- Complete locked contract (stores all fields)
  contract : Option Contract

  -- Attempts and effects
  attempts : List (AttemptID × Attempt)
  active_attempt : Option AttemptID
  effect_intents : List (EffectID × EffectIntent)
  observations : List (ObservationID × Observation)

  -- Evidence and obligations
  accepted_evidence : List (EvidenceID × Evidence)
  satisfied_obligations : List (ObligationID × ObligationSatisfaction)

  -- Idempotency (only for Accepted commands)
  command_receipts : List (CommandID × CommandReceipt)

  -- Escalation
  escalation : Option Escalation
  deriving DecidableEq

-- Domain Events
inductive DomainEvent where
  | ContractLocked :
      contract_id : ContractID →
      contract_version : ContractVersion →
      required_obligations : Finset ObligationID →
      allowed_effect_kinds : Finset EffectKind →
      max_attempts : Nat →
      contract_digest : Hash →
      DomainEvent

  | AttemptCreated :
      attempt_id : AttemptID →
      ordinal : Nat →
      DomainEvent

  | EffectIntentCommitted :
      attempt_id : AttemptID →
      effect_id : EffectID →
      effect_kind : EffectKind →
      request_digest : Hash →
      DomainEvent

  | EffectObserved :
      observation_id : ObservationID →
      attempt_id : AttemptID →
      effect_id : EffectID →
      outcome : ObservationOutcome →
      result_digest : Option Hash →
      DomainEvent

  | EvidenceAccepted :
      evidence_id : EvidenceID →
      attempt_id : AttemptID →
      source_observation_id : ObservationID →
      evidence_digest : Hash →
      DomainEvent

  | ObligationSatisfied :
      obligation_id : ObligationID →
      evidence_ids : Finset EvidenceID →
      DomainEvent

  | AttemptConcluded :
      attempt_id : AttemptID →
      outcome : AttemptOutcome →
      DomainEvent

  | TaskCompleted :
      DomainEvent

  | EscalationRequested :
      failure_class : FailureClass →
      reason : String →
      related_effect_id : Option EffectID →
      DomainEvent

  | EscalationApproved :
      DomainEvent

  | EscalationRejected :
      DomainEvent

  deriving DecidableEq

-- Command Origin Metadata
structure CommandOrigin where
  command_id : CommandID
  command_fingerprint : Hash
  authority : Authority
  correlation_id : CorrelationID
  causation_id : Option (CommandID ⊕ EventID)
  deriving DecidableEq

-- Recorded Domain Event (for fixtures and replay)
structure RecordedDomainEvent where
  aggregate_version : Nat
  domain_event : DomainEvent
  origin : CommandOrigin
  deriving DecidableEq

-- Commands
inductive CommandBody where
  | LockContract :
      contract_id : ContractID →
      contract_version : ContractVersion →
      required_obligations : Finset ObligationID →
      allowed_effect_kinds : Finset EffectKind →
      max_attempts : Nat →
      contract_digest : Hash →
      CommandBody

  | CreateAttempt :
      attempt_id : AttemptID →
      ordinal : Nat →
      CommandBody

  | CommitEffectIntent :
      attempt_id : AttemptID →
      effect_id : EffectID →
      effect_kind : EffectKind →
      request_digest : Hash →
      CommandBody

  | RecordEffectObservation :
      observation_id : ObservationID →
      attempt_id : AttemptID →
      effect_id : EffectID →
      outcome : ObservationOutcome →
      result_digest : Option Hash →
      CommandBody

  | AcceptEvidence :
      evidence_id : EvidenceID →
      attempt_id : AttemptID →
      source_observation_id : ObservationID →
      evidence_digest : Hash →
      CommandBody

  | SatisfyObligation :
      obligation_id : ObligationID →
      evidence_ids : Finset EvidenceID →
      CommandBody

  | ConcludeAttempt :
      attempt_id : AttemptID →
      outcome : AttemptOutcome →
      CommandBody

  | ProposeCompletion :
      CommandBody

  | RequestEscalation :
      failure_class : FailureClass →
      reason : String →
      related_effect_id : Option EffectID →
      CommandBody

  | ApproveEscalation :
      CommandBody

  | RejectEscalation :
      CommandBody

  deriving DecidableEq

-- CommandEnvelope now includes commandFingerprint
structure CommandEnvelope where
  command_id : CommandID
  task_id : TaskID
  expected_version : Nat
  command_fingerprint : Hash
  authority : Authority
  correlation_id : CorrelationID
  causation_id : Option (CommandID ⊕ EventID)
  body : CommandBody
  deriving DecidableEq

-- Decisions
inductive Decision where
  | Rejected :
      reason_code : RejectionReason →
      Decision

  | NoOp :
      reason_code : NoOpReason →
      Decision

  | Accepted :
      events : List DomainEvent →
      Decision

  deriving DecidableEq

-- Utility: lookup in association list
def lookup {α β : Type} [DecidableEq α] (key : α) (lst : List (α × β)) : Option β :=
  lst.findMap fun (k, v) => if k = key then some v else none

-- Utility: update association list
def assocUpdate {α β : Type} [DecidableEq α] (key : α) (value : β) (lst : List (α × β)) : List (α × β) :=
  (key, value) :: (lst.filter fun (k, _) => k ≠ key)

-- Utility: remove from association list
def assocRemove {α β : Type} [DecidableEq α] (key : α) (lst : List (α × β)) : List (α × β) :=
  lst.filter fun (k, _) => k ≠ key

end HomeBase
