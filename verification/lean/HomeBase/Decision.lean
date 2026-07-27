-- HomeBase Decision Function
-- Evaluates commands against state in specified precedence order
-- Returns Decision (Accepted, NoOp, or Rejected)

import HomeBase.Domain
import HomeBase.Reducer

namespace HomeBase

-- Check if a command is already persisted in the ledger via its origin
-- For Lean model, we track this via command_receipts
def isCommandPersisted (state : TaskState) (cmd_id : CommandID) : Bool :=
  (lookup cmd_id state.command_receipts).isSome

-- Check if fingerprints match (simplified: exact string comparison)
def fingerprintMatches (fp1 fp2 : String) : Bool :=
  fp1 = fp2

-- Authorization check
def isAuthorized (authority : Authority) (command_type : String) : Bool :=
  match authority.role with
  | AuthorityRole.TaskInitiator => command_type = "LockContract"
  | AuthorityRole.Orchestrator =>
    command_type = "CreateAttempt" ∨
    command_type = "ProposeCompletion" ∨
    command_type = "RequestEscalation"
  | AuthorityRole.BridgeAdapter =>
    command_type = "CommitEffectIntent" ∨
    command_type = "RecordEffectObservation"
  | AuthorityRole.Verifier =>
    command_type = "AcceptEvidence" ∨
    command_type = "SatisfyObligation"
  | AuthorityRole.RecoveryController => true  -- Has all permissions

-- Get the command type string
def commandType (cmd : CommandBody) : String :=
  match cmd with
  | CommandBody.LockContract _ _ _ _ _ _ => "LockContract"
  | CommandBody.CreateAttempt _ _ => "CreateAttempt"
  | CommandBody.CommitEffectIntent _ _ _ _ => "CommitEffectIntent"
  | CommandBody.RecordEffectObservation _ _ _ _ _ => "RecordEffectObservation"
  | CommandBody.AcceptEvidence _ _ _ _ => "AcceptEvidence"
  | CommandBody.SatisfyObligation _ _ => "SatisfyObligation"
  | CommandBody.ProposeCompletion => "ProposeCompletion"
  | CommandBody.RequestEscalation _ _ => "RequestEscalation"

-- Decide: Main decision function
-- Evaluation order matches SEMANTICS.md Section 8
def decide (state : TaskState) (cmd : CommandEnvelope) : Decision :=
  let cmd_type := commandType cmd.body

  -- 1. Replay/Conflict Check
  -- (Simplified: in actual ledger lookup, check command_receipts)
  if isCommandPersisted state cmd.command_id then
    -- In real system, check fingerprint against persisted command
    -- For now, return COMMAND_ALREADY_APPLIED (simplified)
    Decision.NoOp NoOpReason.COMMAND_ALREADY_APPLIED
      "Identical command (same command_id) was already processed."
  else
  -- 2. Expected Version Check
  if cmd.expected_version ≠ state.version then
    Decision.Rejected RejectionReason.STALE_VERSION
      s!"Expected version {cmd.expected_version} but current version is {state.version}"
  else
  -- 3. Authority Check
  if ¬isAuthorized cmd.authority cmd_type then
    Decision.Rejected RejectionReason.UNAUTHORIZED
      s!"Authority {cmd.authority.principal_id.value} with role {cmd.authority.role} not permitted for {cmd_type}"
  else
  -- 4. Terminal State Check
  if state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated then
    match cmd.body with
    | CommandBody.ProposeCompletion =>
      if state.status = TaskStatus.Completed then
        Decision.NoOp NoOpReason.ALREADY_COMPLETED "Task is already completed"
      else
        Decision.Rejected RejectionReason.TERMINAL_STATE
          "Cannot execute ProposeCompletion: task is in terminal state"
    | _ =>
      Decision.Rejected RejectionReason.TERMINAL_STATE
        "Cannot execute operational command: task is in terminal state"
  else

  -- 5-7. Command-Specific Logic
  match cmd.body with

  | CommandBody.LockContract cid cv obls efks max_att digest =>
    -- Can only lock once
    if state.contract.isSome then
      -- Check if it's identical (same contract)
      match state.contract with
      | some existing =>
        if existing.contract_id = cid ∧ existing.contract_version = cv ∧
           existing.contract_digest = digest then
          Decision.NoOp NoOpReason.IDENTICAL_CONTRACT "Contract already locked identically"
        else
          Decision.Rejected RejectionReason.COMMAND_ID_CONFLICT
            "Contract already locked with different parameters"
      | none => by contradiction
    else
      -- Emit ContractLocked event
      let event := DomainEvent.ContractLocked cid cv obls efks max_att digest
      Decision.Accepted [event]

  | CommandBody.CreateAttempt aid ordinal =>
    -- Check contract is locked
    if state.contract.isNone then
      Decision.Rejected RejectionReason.INVALID_STATUS "No contract locked"
    else
      -- Check attempt doesn't already exist
      match lookup aid state.attempts with
      | some existing =>
        if existing.ordinal = ordinal then
          Decision.NoOp NoOpReason.IDENTICAL_ATTEMPT "Attempt already exists with same ordinal"
        else
          Decision.Rejected RejectionReason.COMMAND_ID_CONFLICT
            "Attempt already exists with different ordinal"
      | none =>
        -- Check attempt limit
        match state.contract with
        | none => by contradiction
        | some contract_ref =>
          -- Simplified: don't enforce max_attempts here; would need to extract from initial state
          let event := DomainEvent.AttemptCreated aid ordinal
          Decision.Accepted [event]

  | CommandBody.CommitEffectIntent aid eid kind digest =>
    -- Check active attempt exists
    match state.active_attempt with
    | none =>
      Decision.Rejected RejectionReason.ATTEMPT_NOT_ACTIVE "No active attempt"
    | some active =>
      if active ≠ aid then
        Decision.Rejected RejectionReason.ATTEMPT_NOT_FOUND "Specified attempt is not active"
      else
        -- Check effect doesn't already exist
        match lookup eid state.effect_intents with
        | some existing =>
          if existing.request_digest = digest then
            Decision.NoOp NoOpReason.IDENTICAL_EFFECT_INTENT
              "Effect already committed with same request_digest"
          else
            Decision.Rejected RejectionReason.CONFLICTING_EFFECT_ID
              s!"Effect {eid.value} already exists with different request_digest"
        | none =>
          let event := DomainEvent.EffectIntentCommitted aid eid kind digest
          Decision.Accepted [event]

  | CommandBody.RecordEffectObservation oid aid eid outcome result =>
    -- Check effect intent exists and is committed
    match lookup eid state.effect_intents with
    | none =>
      Decision.Rejected RejectionReason.EFFECT_NOT_FOUND "Effect not found"
    | some intent =>
      if intent.status ≠ IntentStatus.Committed then
        Decision.Rejected RejectionReason.INVALID_STATUS "Effect intent not in Committed state"
      else
        -- Check observation doesn't already exist
        match lookup oid state.observations with
        | some existing =>
          if existing.outcome = outcome then
            Decision.NoOp NoOpReason.IDENTICAL_OBSERVATION
              "Observation already recorded with same outcome"
          else
            Decision.Rejected RejectionReason.CONFLICTING_OBSERVATION_ID
              s!"Observation {oid.value} already exists with different outcome"
        | none =>
          let event := DomainEvent.EffectObserved oid aid eid outcome result
          Decision.Accepted [event]

  | CommandBody.AcceptEvidence evid aid src_oid digest =>
    -- Check observation exists and is Succeeded
    match lookup src_oid state.observations with
    | none =>
      Decision.Rejected RejectionReason.OBSERVATION_NOT_FOUND "Observation not found"
    | some obs =>
      if obs.outcome ≠ ObservationOutcome.Succeeded then
        Decision.Rejected RejectionReason.OBSERVATION_NOT_SUCCESSFUL
          "Observation outcome is not Succeeded"
      else
        -- Check evidence doesn't already exist
        match lookup evid state.accepted_evidence with
        | some existing =>
          if existing.source_observation_id = src_oid then
            Decision.NoOp NoOpReason.IDENTICAL_EVIDENCE
              "Evidence already accepted with same source"
          else
            Decision.Rejected RejectionReason.CONFLICTING_EVIDENCE_ID
              s!"Evidence {evid.value} already exists with different source"
        | none =>
          let event := DomainEvent.EvidenceAccepted evid aid src_oid digest
          Decision.Accepted [event]

  | CommandBody.SatisfyObligation obid evidence_ids =>
    -- Check all evidence IDs exist
    let all_valid := evidence_ids.all fun evid =>
      (lookup evid state.accepted_evidence).isSome
    if ¬all_valid then
      Decision.Rejected RejectionReason.EVIDENCE_NOT_FOUND
        "One or more evidence IDs not found or not accepted"
    else
      -- Check obligation doesn't already exist
      match lookup obid state.satisfied_obligations with
      | some existing =>
        if existing.evidence_ids = evidence_ids then
          Decision.NoOp NoOpReason.OBLIGATION_ALREADY_SATISFIED
            "Obligation already satisfied with same evidence set"
        else
          Decision.Rejected RejectionReason.CONFLICTING_EVIDENCE_ID
            s!"Obligation {obid.value} already satisfied with different evidence"
      | none =>
        let event := DomainEvent.ObligationSatisfied obid evidence_ids
        Decision.Accepted [event]

  | CommandBody.ProposeCompletion =>
    -- Status must be Active (already checked)
    -- All required obligations must be satisfied
    -- Simplified: check satisfied_obligations is non-empty
    -- (Full implementation would check against contract.required_obligations)
    if state.satisfied_obligations.isEmpty then
      Decision.Rejected RejectionReason.UNMET_OBLIGATIONS
        "Not all required obligations are satisfied"
    else
      let event := DomainEvent.TaskCompleted
      Decision.Accepted [event]

  | CommandBody.RequestEscalation failure_class reason =>
    -- Status must be Active (already checked)
    let event := DomainEvent.EscalationRequested failure_class reason none
    Decision.Accepted [event]

end HomeBase
