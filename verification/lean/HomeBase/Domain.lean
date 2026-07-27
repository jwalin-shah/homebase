-- HomeBase Domain Model
-- Abstract domain concepts for decision governance
-- No persistence, cryptography, or external effects

import Data.List
import Data.Finmap

namespace HomeBase

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
  | Active
  | Completed
  | Escalated
  deriving DecidableEq

inductive AttemptStatus where
  | Open
  | Succeeded
  | Failed
  | OutcomeUnknown
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

structure ContractRef where
  contract_id : ContractID
  contract_version : ContractVersion
  contract_digest : String  -- Hash as string for Lean
  deriving DecidableEq

structure Attempt where
  attempt_id : AttemptID
  ordinal : Nat
  status : AttemptStatus
  effect_ids : Finset EffectID
  deriving DecidableEq

structure EffectIntent where
  effect_id : EffectID
  effect_kind : String
  request_digest : String
  status : IntentStatus
  deriving DecidableEq

structure Observation where
  observation_id : ObservationID
  attempt_id : AttemptID
  effect_id : EffectID
  outcome : ObservationOutcome
  result_digest : Option String
  deriving DecidableEq

structure Evidence where
  evidence_id : EvidenceID
  attempt_id : AttemptID
  source_observation_id : ObservationID
  evidence_digest : String
  deriving DecidableEq

structure ObligationSatisfaction where
  obligation_id : ObligationID
  evidence_ids : Finset EvidenceID
  deriving DecidableEq

structure CommandReceipt where
  command_fingerprint : String
  resulting_event_types : List String
  deriving DecidableEq

structure Escalation where
  failure_class : String
  reason : String
  related_effect_id : Option EffectID
  requested_at : Nat  -- version when escalation occurred
  deriving DecidableEq

-- Current State Projection
structure TaskState where
  task_id : TaskID
  version : Nat
  status : TaskStatus

  -- Contract reference
  contract : Option ContractRef

  -- Attempts and effects (simplified: map via list of key-value pairs)
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
      allowed_effect_kinds : Finset String →
      max_attempts : Nat →
      contract_digest : String →
      DomainEvent

  | AttemptCreated :
      attempt_id : AttemptID →
      ordinal : Nat →
      DomainEvent

  | EffectIntentCommitted :
      attempt_id : AttemptID →
      effect_id : EffectID →
      effect_kind : String →
      request_digest : String →
      DomainEvent

  | EffectObserved :
      observation_id : ObservationID →
      attempt_id : AttemptID →
      effect_id : EffectID →
      outcome : ObservationOutcome →
      result_digest : Option String →
      DomainEvent

  | EvidenceAccepted :
      evidence_id : EvidenceID →
      attempt_id : AttemptID →
      source_observation_id : ObservationID →
      evidence_digest : String →
      DomainEvent

  | ObligationSatisfied :
      obligation_id : ObligationID →
      evidence_ids : Finset EvidenceID →
      DomainEvent

  | TaskCompleted :
      DomainEvent

  | EscalationRequested :
      failure_class : String →
      reason : String →
      related_effect_id : Option EffectID →
      DomainEvent

  deriving DecidableEq

-- Command Origin Metadata
structure CommandOrigin where
  command_id : CommandID
  command_fingerprint : String
  authority : Authority
  correlation_id : CorrelationID
  causation_id : Option (CommandID ⊕ String)  -- CommandID or EventID (as string)
  deriving DecidableEq

-- Recorded Domain Event (for fixtures)
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
      allowed_effect_kinds : Finset String →
      max_attempts : Nat →
      contract_digest : String →
      CommandBody

  | CreateAttempt :
      attempt_id : AttemptID →
      ordinal : Nat →
      CommandBody

  | CommitEffectIntent :
      attempt_id : AttemptID →
      effect_id : EffectID →
      effect_kind : String →
      request_digest : String →
      CommandBody

  | RecordEffectObservation :
      observation_id : ObservationID →
      attempt_id : AttemptID →
      effect_id : EffectID →
      outcome : ObservationOutcome →
      result_digest : Option String →
      CommandBody

  | AcceptEvidence :
      evidence_id : EvidenceID →
      attempt_id : AttemptID →
      source_observation_id : ObservationID →
      evidence_digest : String →
      CommandBody

  | SatisfyObligation :
      obligation_id : ObligationID →
      evidence_ids : Finset EvidenceID →
      CommandBody

  | ProposeCompletion :
      CommandBody

  | RequestEscalation :
      failure_class : String →
      reason : String →
      CommandBody

  deriving DecidableEq

structure CommandEnvelope where
  command_id : CommandID
  task_id : TaskID
  expected_version : Nat
  authority : Authority
  correlation_id : CorrelationID
  causation_id : Option (CommandID ⊕ String)
  body : CommandBody
  deriving DecidableEq

-- Decisions
inductive Decision where
  | Rejected :
      reason_code : RejectionReason →
      message : String →
      Decision

  | NoOp :
      reason_code : NoOpReason →
      message : String →
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

end HomeBase
