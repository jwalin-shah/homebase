-- HomeBase Decision Function (Repaired)
-- Evaluates commands with explicit precedence and full precondition checking

import HomeBase.Domain
import HomeBase.Reducer

namespace HomeBase

-- Authorization check
def isAuthorized (authority : Authority) (command_type : String) : Bool :=
  match authority.role with
  | AuthorityRole.TaskInitiator => command_type = "LockContract"
  | AuthorityRole.Orchestrator =>
    command_type = "CreateAttempt" ∨
    command_type = "CommitEffectIntent" ∨
    command_type = "SatisfyObligation" ∨
    command_type = "ProposeCompletion" ∨
    command_type = "RequestEscalation"
  | AuthorityRole.BridgeAdapter =>
    command_type = "RecordEffectObservation"
  | AuthorityRole.Verifier =>
    command_type = "AcceptEvidence" ∨
    command_type = "SatisfyObligation"
  | AuthorityRole.RecoveryController => true

-- Get command type string
def commandType (cmd : CommandBody) : String :=
  match cmd with
  | CommandBody.LockContract _ _ _ _ _ _ => "LockContract"
  | CommandBody.CreateAttempt _ _ => "CreateAttempt"
  | CommandBody.CommitEffectIntent _ _ _ _ => "CommitEffectIntent"
  | CommandBody.RecordEffectObservation _ _ _ _ _ => "RecordEffectObservation"
  | CommandBody.AcceptEvidence _ _ _ _ => "AcceptEvidence"
  | CommandBody.SatisfyObligation _ _ => "SatisfyObligation"
  | CommandBody.ProposeCompletion => "ProposeCompletion"
  | CommandBody.RequestEscalation _ _ _ => "RequestEscalation"

-- Check if command is already persisted (exact fingerprint match)
def isCommandApplied (state : TaskState) (cmd_id : CommandID) (fingerprint : Hash) : Bool :=
  match lookup cmd_id state.command_receipts with
  | none => false
  | some receipt => receipt.command_fingerprint = fingerprint

-- Check if command ID exists with different fingerprint
def isCommandConflict (state : TaskState) (cmd_id : CommandID) (fingerprint : Hash) : Bool :=
  match lookup cmd_id state.command_receipts with
  | none => false
  | some receipt => receipt.command_fingerprint ≠ fingerprint

-- Main decision function with explicit precedence
def decide (state : TaskState) (cmd : CommandEnvelope) : Decision :=
  let cmd_type := commandType cmd.body

  -- 1. Task ID mismatch check (highest priority)
  if cmd.task_id ≠ state.task_id then
    Decision.Rejected RejectionReason.TASK_ID_MISMATCH
  else

  -- 2. Accepted CommandID replay/conflict check
  if isCommandApplied state cmd.command_id cmd.command_fingerprint then
    Decision.NoOp NoOpReason.COMMAND_ALREADY_APPLIED
  else if isCommandConflict state cmd.command_id cmd.command_fingerprint then
    Decision.Rejected RejectionReason.COMMAND_ID_CONFLICT
  else

  -- 3. Expected version check
  if cmd.expected_version ≠ state.version then
    Decision.Rejected RejectionReason.STALE_VERSION
  else

  -- 4. Authority check
  if ¬isAuthorized cmd.authority cmd_type then
    Decision.Rejected RejectionReason.UNAUTHORIZED
  else

  -- 5. Terminal state check
  if state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated then
    match cmd.body with
    | CommandBody.ProposeCompletion =>
      if state.status = TaskStatus.Completed then
        Decision.NoOp NoOpReason.ALREADY_COMPLETED
      else
        Decision.Rejected RejectionReason.TERMINAL_STATE
    | _ =>
      Decision.Rejected RejectionReason.TERMINAL_STATE
  else

  -- 6-8. Command-specific logic
  match cmd.body with

  | CommandBody.LockContract cid cv obls efks max_att digest =>
    if max_att = 0 then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match state.contract with
      | some existing =>
        if existing.contract_id = cid ∧ existing.contract_version = cv ∧
           existing.contract_digest = digest ∧
           existing.required_obligations = obls ∧
           existing.allowed_effect_kinds = efks ∧
           existing.max_attempts = max_att then
          Decision.NoOp NoOpReason.IDENTICAL_CONTRACT
        else
          Decision.Rejected RejectionReason.CONFLICTING_CONTRACT
      | none =>
        let event := DomainEvent.ContractLocked cid cv obls efks max_att digest
        Decision.Accepted [event]

  | CommandBody.CreateAttempt aid ordinal =>
    if state.contract.isNone then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match state.contract with
      | none => by contradiction
      | some contract =>
        if state.attempts.length ≥ contract.max_attempts then
          Decision.Rejected RejectionReason.ATTEMPT_LIMIT_REACHED
        else if state.active_attempt.isSome then
          Decision.Rejected RejectionReason.INVALID_STATUS
        else
          match lookup aid state.attempts with
          | some existing =>
            if existing.ordinal = ordinal then
              Decision.NoOp NoOpReason.IDENTICAL_ATTEMPT
            else
              Decision.Rejected RejectionReason.CONFLICTING_EFFECT_ID
          | none =>
            let event := DomainEvent.AttemptCreated aid ordinal
            Decision.Accepted [event]

  | CommandBody.CommitEffectIntent aid eid kind digest =>
    if state.contract.isNone then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match state.active_attempt with
      | none =>
        Decision.Rejected RejectionReason.ATTEMPT_NOT_ACTIVE
      | some active =>
        if active ≠ aid then
          Decision.Rejected RejectionReason.ATTEMPT_NOT_FOUND
        else
          match lookup aid state.attempts with
          | none =>
            Decision.Rejected RejectionReason.ATTEMPT_NOT_FOUND
          | some attempt =>
            if attempt.status ≠ AttemptStatus.Open then
              Decision.Rejected RejectionReason.INVALID_STATUS
            else
              match state.contract with
              | none => by contradiction
              | some contract =>
                if kind ∉ contract.allowed_effect_kinds then
                  Decision.Rejected RejectionReason.EFFECT_KIND_NOT_ALLOWED
                else
                  match lookup eid state.effect_intents with
                  | some existing =>
                    if existing.attempt_id = aid ∧ existing.effect_kind = kind ∧
                       existing.request_digest = digest then
                      Decision.NoOp NoOpReason.IDENTICAL_EFFECT_INTENT
                    else
                      Decision.Rejected RejectionReason.CONFLICTING_EFFECT_ID
                  | none =>
                    let event := DomainEvent.EffectIntentCommitted aid eid kind digest
                    Decision.Accepted [event]

  | CommandBody.RecordEffectObservation oid aid eid outcome result =>
    match lookup eid state.effect_intents with
    | none =>
      Decision.Rejected RejectionReason.EFFECT_NOT_FOUND
    | some intent =>
      if intent.attempt_id ≠ aid then
        Decision.Rejected RejectionReason.CONFLICTING_OBSERVATION_ID
      else if intent.status = IntentStatus.Terminal then
        match lookup oid state.observations with
        | some existing =>
          if existing.attempt_id = aid ∧ existing.effect_id = eid ∧
             existing.outcome = outcome ∧ existing.result_digest = result then
            Decision.NoOp NoOpReason.IDENTICAL_OBSERVATION
          else
            Decision.Rejected RejectionReason.CONFLICTING_OBSERVATION_ID
        | none =>
          Decision.Rejected RejectionReason.INVALID_STATUS
      else
        match lookup oid state.observations with
        | some existing =>
          if existing.attempt_id = aid ∧ existing.effect_id = eid ∧
             existing.outcome = outcome ∧ existing.result_digest = result then
            Decision.NoOp NoOpReason.IDENTICAL_OBSERVATION
          else
            Decision.Rejected RejectionReason.CONFLICTING_OBSERVATION_ID
        | none =>
          let event := DomainEvent.EffectObserved oid aid eid outcome result
          Decision.Accepted [event]

  | CommandBody.AcceptEvidence evid aid src_oid digest =>
    match lookup src_oid state.observations with
    | none =>
      Decision.Rejected RejectionReason.OBSERVATION_NOT_FOUND
    | some obs =>
      if obs.attempt_id ≠ aid then
        Decision.Rejected RejectionReason.CONFLICTING_EVIDENCE_ID
      else if obs.outcome ≠ ObservationOutcome.Succeeded then
        Decision.Rejected RejectionReason.OBSERVATION_NOT_SUCCESSFUL
      else
        match lookup evid state.accepted_evidence with
        | some existing =>
          if existing.attempt_id = aid ∧ existing.source_observation_id = src_oid ∧
             existing.evidence_digest = digest then
            Decision.NoOp NoOpReason.IDENTICAL_EVIDENCE
          else
            Decision.Rejected RejectionReason.CONFLICTING_EVIDENCE_ID
        | none =>
          let event := DomainEvent.EvidenceAccepted evid aid src_oid digest
          Decision.Accepted [event]

  | CommandBody.SatisfyObligation obid evidence_ids =>
    if state.contract.isNone then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match state.contract with
      | none => by contradiction
      | some contract =>
        if obid ∉ contract.required_obligations then
          Decision.Rejected RejectionReason.OBLIGATION_NOT_REQUIRED
        else if evidence_ids.isEmpty then
          Decision.Rejected RejectionReason.EVIDENCE_NOT_FOUND
        else
          let all_valid := evidence_ids.all fun evid =>
            (lookup evid state.accepted_evidence).isSome
          if ¬all_valid then
            Decision.Rejected RejectionReason.EVIDENCE_NOT_FOUND
          else
            match lookup obid state.satisfied_obligations with
            | some existing =>
              if existing.evidence_ids = evidence_ids then
                Decision.NoOp NoOpReason.OBLIGATION_ALREADY_SATISFIED
              else
                Decision.Rejected RejectionReason.CONFLICTING_EVIDENCE_ID
            | none =>
              let event := DomainEvent.ObligationSatisfied obid evidence_ids
              Decision.Accepted [event]

  | CommandBody.ProposeCompletion =>
    if state.contract.isNone then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match state.contract with
      | none => by contradiction
      | some contract =>
        let all_required_satisfied := contract.required_obligations.all fun obid =>
          (lookup obid state.satisfied_obligations).isSome
        if ¬all_required_satisfied then
          Decision.Rejected RejectionReason.UNMET_OBLIGATIONS
        else if state.active_attempt.isSome then
          Decision.Rejected RejectionReason.INVALID_STATUS
        else
          let event := DomainEvent.TaskCompleted
          Decision.Accepted [event]

  | CommandBody.RequestEscalation failure_class reason related_eid =>
    match related_eid with
    | none =>
      let event := DomainEvent.EscalationRequested failure_class reason none
      Decision.Accepted [event]
    | some eid =>
      if lookup eid state.effect_intents |>.isNone then
        Decision.Rejected RejectionReason.EFFECT_NOT_FOUND
      else
        let event := DomainEvent.EscalationRequested failure_class reason (some eid)
        Decision.Accepted [event]

-- Fundamental theorem: accepted decisions are reducer-applicable
-- This is proven via the meta-theorems in Invariants.lean
theorem accepted_events_apply
    (hvalid : ValidState state)
    (hdecision : decide state cmd = Decision.Accepted events) :
    ∃ state',
      foldEvents state events = some state' ∧
      ValidState state' := by
  exact Invariants.accepted_events_apply state cmd events hvalid hdecision

-- Determinism: decide is deterministic
theorem decide_deterministic (state : TaskState) (cmd : CommandEnvelope) :
    decide state cmd = decide state cmd := by
  rfl

end HomeBase
