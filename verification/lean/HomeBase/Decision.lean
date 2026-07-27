-- HomeBase Decision Function (Repaired)
-- Evaluates commands with explicit precedence and full precondition checking
-- Fixes: structural authorization, no omnipotent recovery role, clear precedence

import HomeBase.Domain
import HomeBase.Reducer

namespace HomeBase

-- Authorization check using structural matching on command body (not strings)
def isAuthorized (authority : Authority) (body : CommandBody) : Bool :=
  match authority.role, body with
  -- TaskInitiator: only LockContract
  | AuthorityRole.TaskInitiator, CommandBody.LockContract _ _ _ _ _ _ => true
  | AuthorityRole.TaskInitiator, _ => false

  -- Orchestrator: CreateAttempt, ConcludeAttempt, SatisfyObligation, ProposeCompletion, RequestEscalation
  | AuthorityRole.Orchestrator, CommandBody.CreateAttempt _ _ => true
  | AuthorityRole.Orchestrator, CommandBody.ConcludeAttempt _ => true
  | AuthorityRole.Orchestrator, CommandBody.SatisfyObligation _ _ => true
  | AuthorityRole.Orchestrator, CommandBody.ProposeCompletion => true
  | AuthorityRole.Orchestrator, CommandBody.RequestEscalation _ _ _ => true
  | AuthorityRole.Orchestrator, _ => false

  -- BridgeAdapter: only RecordEffectObservation
  | AuthorityRole.BridgeAdapter, CommandBody.RecordEffectObservation _ _ _ _ _ => true
  | AuthorityRole.BridgeAdapter, _ => false

  -- Verifier: AcceptEvidence, SatisfyObligation
  | AuthorityRole.Verifier, CommandBody.AcceptEvidence _ _ _ _ => true
  | AuthorityRole.Verifier, CommandBody.SatisfyObligation _ _ => true
  | AuthorityRole.Verifier, _ => false

  -- RecoveryController: restricted to reconciliation only
  -- Can record observations, create attempts, and commit effect intents
  -- Cannot accept evidence, satisfy obligations, or complete tasks
  | AuthorityRole.RecoveryController, CommandBody.RecordEffectObservation _ _ _ _ _ => true
  | AuthorityRole.RecoveryController, CommandBody.CreateAttempt _ _ => true
  | AuthorityRole.RecoveryController, CommandBody.CommitEffectIntent _ _ _ _ => true
  | AuthorityRole.RecoveryController, _ => false

-- Check if a command is already persisted (exact fingerprint match)
def isCommandApplied (state : TaskState) (cmd_id : CommandID) (fingerprint : Hash) : Bool :=
  match lookup cmd_id state.command_receipts with
  | none => false
  | some receipt => receipt.command_fingerprint = fingerprint

-- Check if command ID exists with different fingerprint (conflict)
def isCommandConflict (state : TaskState) (cmd_id : CommandID) (fingerprint : Hash) : Bool :=
  match lookup cmd_id state.command_receipts with
  | none => false
  | some receipt => receipt.command_fingerprint ≠ fingerprint

-- Main decision function with explicit, fixed precedence
def decide (state : TaskState) (cmd : CommandEnvelope) : Decision :=
  -- 1. Task ID mismatch check (highest priority)
  if cmd.task_id ≠ state.task_id then
    Decision.Rejected RejectionReason.TASK_ID_MISMATCH

  -- 2. Accepted CommandID replay/conflict check
  else if isCommandApplied state cmd.command_id cmd.command_fingerprint then
    Decision.NoOp NoOpReason.COMMAND_ALREADY_APPLIED
  else if isCommandConflict state cmd.command_id cmd.command_fingerprint then
    Decision.Rejected RejectionReason.COMMAND_ID_CONFLICT

  -- 3. Expected version check
  else if cmd.expected_version ≠ state.version then
    Decision.Rejected RejectionReason.STALE_VERSION

  -- 4. Authority check (structural matching)
  else if ¬isAuthorized cmd.authority cmd.body then
    Decision.Rejected RejectionReason.UNAUTHORIZED

  -- 5. Terminal state check
  else if state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated then
    match cmd.body with
    | CommandBody.ProposeCompletion =>
      if state.status = TaskStatus.Completed then
        Decision.NoOp NoOpReason.ALREADY_COMPLETED
      else
        Decision.Rejected RejectionReason.TERMINAL_STATE
    | _ =>
      Decision.Rejected RejectionReason.TERMINAL_STATE

  -- 6-8. Command-specific logic and semantic checking
  else match cmd.body with

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

  | CommandBody.ConcludeAttempt aid =>
    if state.status ≠ TaskStatus.Active then
      Decision.Rejected RejectionReason.INVALID_STATUS
    else
      match lookup aid state.attempts with
      | none =>
        Decision.Rejected RejectionReason.ATTEMPT_NOT_FOUND
      | some attempt =>
        -- All effect intents in this attempt must be Terminal
        let all_intents_terminal := attempt.effect_ids.all fun eid =>
          (lookup eid state.effect_intents).map (fun intent => intent.status = IntentStatus.Terminal) |>.getD false
        if ¬all_intents_terminal then
          Decision.Rejected RejectionReason.INVALID_STATUS
        else
          let event := DomainEvent.AttemptConcluded aid
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

-- Determinism: function application is deterministic
theorem decide_deterministic (state : TaskState) (cmd : CommandEnvelope) :
    decide state cmd = decide state cmd := by
  rfl

end HomeBase
