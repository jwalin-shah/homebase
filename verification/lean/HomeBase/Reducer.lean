-- HomeBase Reducer (Repaired)
-- Pure state transitions with full precondition checking

import HomeBase.Domain

namespace HomeBase

-- Modular Invariants (Verdi-style Architecture)

-- 1. Contract Consistency
def ValidContractState (state : TaskState) : Prop :=
  (state.status = TaskStatus.Draft → state.contract.isNone) ∧
  (state.contract.isSome →
    (state.status = TaskStatus.Active ∨ state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated)) ∧
  ((state.attempts.length > 0 → state.contract.isSome) ∧
   (match state.contract with
    | none => true
    | some c => state.attempts.length ≤ c.max_attempts
    end))

-- 2. Attempt Integrity
def ValidAttemptState (state : TaskState) : Prop :=
  (state.status = TaskStatus.Draft →
   state.attempts.isEmpty ∧ state.active_attempt.isNone) ∧
  (match state.active_attempt with
   | none => true
   | some aid =>
     lookup aid state.attempts |>.map (fun a => a.status = AttemptStatus.Open) |>.getD false
   ) ∧
  state.attempts.Nodup (fun (a1, _) (a2, _) => a1 = a2) ∧
  state.command_receipts.Nodup (fun (c1, _) (c2, _) => c1 = c2)

-- 3. Provenance Chain
def ValidProvenanceState (state : TaskState) : Prop :=
  (∀ eid intent, lookup eid state.effect_intents = some intent →
    (lookup intent.attempt_id state.attempts |>.isSome ∧
     state.contract.isSome)) ∧
  (∀ attempt, lookup attempt.attempt_id state.attempts = some attempt →
    ∀ eid, eid ∈ attempt.effect_ids →
    ∃ intent, lookup eid state.effect_intents = some intent ∧
              intent.attempt_id = attempt.attempt_id) ∧
  (∀ eid intent, lookup eid state.effect_intents = some intent →
    ∃ attempt, lookup intent.attempt_id state.attempts = some attempt ∧
               eid ∈ attempt.effect_ids) ∧
  (∀ oid obs, lookup oid state.observations = some obs →
    ∃ intent, lookup obs.effect_id state.effect_intents = some intent ∧
              intent.attempt_id = obs.attempt_id ∧
              lookup intent.attempt_id state.attempts |>.isSome)

-- 4. Evidence Soundness
def ValidEvidenceState (state : TaskState) : Prop :=
  (∀ evid ev, lookup evid state.accepted_evidence = some ev →
    ∃ obs, lookup ev.source_observation_id state.observations = some obs ∧
           obs.outcome = ObservationOutcome.Succeeded ∧
           ev.attempt_id = obs.attempt_id ∧
           (lookup obs.effect_id state.effect_intents).isSome) ∧
  (∀ obid obs, lookup obid state.satisfied_obligations = some obs →
    (match state.contract with
     | none => false
     | some c => obid ∈ c.required_obligations ∧
                 obs.evidence_ids.all fun evid =>
                   (lookup evid state.accepted_evidence).isSome
     end))

-- 5. Terminal Trapping
def ValidTerminalState (state : TaskState) : Prop :=
  ((state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated) →
   state.active_attempt = none) ∧
  (state.status = TaskStatus.Completed →
   match state.contract with
   | none => false
   | some c =>
     ∀ obid, obid ∈ c.required_obligations →
     (lookup obid state.satisfied_obligations).isSome
   end) ∧
  (state.status = TaskStatus.Escalated →
   state.escalation.isSome ∧
   (match state.escalation with
    | none => false
    | some esc =>
      match esc.related_effect_id with
      | none => true
      | some eid => (lookup eid state.effect_intents).isSome
      end
    end))

-- Valid State Predicate (Composed)
def ValidState (state : TaskState) : Prop :=
  ValidContractState state ∧
  ValidAttemptState state ∧
  ValidProvenanceState state ∧
  ValidEvidenceState state ∧
  ValidTerminalState state

-- Apply a single domain event to state
def applyEvent (state : TaskState) (event : DomainEvent) : Option TaskState :=
  if ¬ValidState state then none else

  match event with

  | DomainEvent.ContractLocked cid cver obls efks max_att digest =>
    if state.status = TaskStatus.Draft ∧ state.contract = none ∧ max_att > 0 then
      some {
        state with
        version := state.version + 1
        status := TaskStatus.Active
        contract := some {
          contract_id := cid
          contract_version := cver
          contract_digest := Hash.mk digest.value
          required_obligations := obls
          allowed_effect_kinds := efks
          max_attempts := max_att
        }
      }
    else
      none

  | DomainEvent.AttemptCreated aid ordinal =>
    if state.status = TaskStatus.Active ∧ state.contract.isSome then
      match state.contract with
      | none => none
      | some contract =>
        if lookup aid state.attempts |>.isSome then none
        else if state.attempts.length ≥ contract.max_attempts then none
        else if state.active_attempt |>.isSome then none
        else
          let new_attempt : Attempt := {
            attempt_id := aid
            ordinal := ordinal
            status := AttemptStatus.Open
            effect_ids := ∅
          }
          some {
            state with
            version := state.version + 1
            attempts := assocUpdate aid new_attempt state.attempts
            active_attempt := some aid
          }
    else
      none

  | DomainEvent.EffectIntentCommitted aid eid kind digest =>
    if state.status = TaskStatus.Active then
      match state.active_attempt with
      | none => none
      | some active =>
        if active ≠ aid then none
        else
          match lookup aid state.attempts with
          | none => none
          | some attempt =>
            if attempt.status ≠ AttemptStatus.Open then none
            else if lookup eid state.effect_intents |>.isSome then none
            else
              match state.contract with
              | none => none
              | some contract =>
                let kind_obj := EffectKind.mk kind.value
                if kind_obj ∉ contract.allowed_effect_kinds then none
                else
                  let new_intent : EffectIntent := {
                    effect_id := eid
                    attempt_id := aid
                    effect_kind := kind
                    request_digest := digest
                    status := IntentStatus.Committed
                  }
                  let updated_attempt := { attempt with effect_ids := attempt.effect_ids.insert eid }
                  some {
                    state with
                    version := state.version + 1
                    effect_intents := assocUpdate eid new_intent state.effect_intents
                    attempts := assocUpdate aid updated_attempt state.attempts
                  }
    else
      none

  | DomainEvent.EffectObserved oid aid eid outcome result =>
    if state.status = TaskStatus.Active then
      match lookup eid state.effect_intents with
      | none => none
      | some intent =>
        if intent.attempt_id ≠ aid then none
        else if lookup oid state.observations |>.isSome then none
        else
          let new_observation : Observation := {
            observation_id := oid
            attempt_id := aid
            effect_id := eid
            outcome := outcome
            result_digest := result
          }
          -- Update effect intent status based on observation outcome
          let new_intent_status :=
            match intent.status, outcome with
            | IntentStatus.Committed, ObservationOutcome.NotStarted => IntentStatus.OutcomeNeeded
            | IntentStatus.Committed, ObservationOutcome.Running => IntentStatus.OutcomeNeeded
            | IntentStatus.Committed, ObservationOutcome.Succeeded => IntentStatus.Terminal
            | IntentStatus.Committed, ObservationOutcome.Failed => IntentStatus.Terminal
            | IntentStatus.Committed, ObservationOutcome.Unknown => IntentStatus.Terminal
            | IntentStatus.OutcomeNeeded, ObservationOutcome.NotStarted => IntentStatus.OutcomeNeeded
            | IntentStatus.OutcomeNeeded, ObservationOutcome.Running => IntentStatus.OutcomeNeeded
            | IntentStatus.OutcomeNeeded, ObservationOutcome.Succeeded => IntentStatus.Terminal
            | IntentStatus.OutcomeNeeded, ObservationOutcome.Failed => IntentStatus.Terminal
            | IntentStatus.OutcomeNeeded, ObservationOutcome.Unknown => IntentStatus.Terminal
            | _, _ => intent.status
          let updated_intent := { intent with status := new_intent_status }
          -- Note: Attempt status is NOT changed by effect observation
          -- Attempt conclusion must be explicit via a separate command/event
          some {
            state with
            version := state.version + 1
            observations := assocUpdate oid new_observation state.observations
            effect_intents := assocUpdate eid updated_intent state.effect_intents
          }
    else
      none

  | DomainEvent.EvidenceAccepted evid aid src_oid digest =>
    if state.status = TaskStatus.Active then
      match lookup src_oid state.observations with
      | none => none
      | some obs =>
        if obs.attempt_id ≠ aid then none
        else if obs.outcome ≠ ObservationOutcome.Succeeded then none
        else if lookup evid state.accepted_evidence |>.isSome then none
        else
          let new_evidence : Evidence := {
            evidence_id := evid
            attempt_id := aid
            source_observation_id := src_oid
            evidence_digest := digest
          }
          some {
            state with
            version := state.version + 1
            accepted_evidence := assocUpdate evid new_evidence state.accepted_evidence
          }
    else
      none

  | DomainEvent.ObligationSatisfied obid evidence_ids =>
    if state.status = TaskStatus.Active then
      match state.contract with
      | none => none
      | some contract =>
        if obid ∉ contract.required_obligations then none
        else if lookup obid state.satisfied_obligations |>.isSome then none
        else
          let all_evidence_valid := evidence_ids.all fun evid =>
            (lookup evid state.accepted_evidence).isSome
          if ¬all_evidence_valid then none
          else
            let new_satisfaction : ObligationSatisfaction := {
              obligation_id := obid
              evidence_ids := evidence_ids
            }
            some {
              state with
              version := state.version + 1
              satisfied_obligations := assocUpdate obid new_satisfaction state.satisfied_obligations
            }
    else
      none

  | DomainEvent.AttemptConcluded aid outcome =>
    if state.status = TaskStatus.Active then
      -- Must be concluding the active attempt (blocker 4)
      if state.active_attempt ≠ some aid then none
      else
        match lookup aid state.attempts with
        | none => none
        | some attempt =>
          -- All effect intents must be Terminal before conclusion
          let all_intents_terminal := attempt.effect_ids.all fun eid =>
            (lookup eid state.effect_intents).map (fun intent => intent.status = IntentStatus.Terminal) |>.getD false
          if ¬all_intents_terminal then none
          else
            -- Map AttemptOutcome to AttemptStatus (blocker 2)
            let new_status := match outcome with
              | AttemptOutcome.Succeeded => AttemptStatus.Succeeded
              | AttemptOutcome.Failed => AttemptStatus.Failed
              | AttemptOutcome.OutcomeUnknown => AttemptStatus.OutcomeUnknown
              | AttemptOutcome.Cancelled => AttemptStatus.Failed
            let updated_attempt := { attempt with status := new_status }
            some {
              state with
              version := state.version + 1
              active_attempt := none
              attempts := assocUpdate aid updated_attempt state.attempts
            }
    else
      none

  | DomainEvent.TaskCompleted =>
    if state.status = TaskStatus.Active then
      match state.contract with
      | none => none
      | some contract =>
        let all_required_satisfied := contract.required_obligations.all fun obid =>
          (lookup obid state.satisfied_obligations).isSome
        if ¬all_required_satisfied then none
        else if state.active_attempt |>.isSome then none
        else
          some {
            state with
            version := state.version + 1
            status := TaskStatus.Completed
          }
    else
      none

  | DomainEvent.EscalationRequested failure_class reason related_eid =>
    if state.status = TaskStatus.Active then
      match related_eid with
      | none =>
        some {
          state with
          version := state.version + 1
          status := TaskStatus.Escalated
          escalation := some {
            failure_class := failure_class
            reason := reason
            related_effect_id := none
            requested_at := state.version
          }
          active_attempt := none
        }
      | some eid =>
        if lookup eid state.effect_intents |>.isNone then none
        else
          some {
            state with
            version := state.version + 1
            status := TaskStatus.Escalated
            escalation := some {
              failure_class := failure_class
              reason := reason
              related_effect_id := some eid
              requested_at := state.version
            }
            active_attempt := none
          }
    else
      none

  | DomainEvent.EscalationApproved =>
    if state.status = TaskStatus.Escalated then
      some {
        state with
        version := state.version + 1
        status := TaskStatus.Active
        escalation := none
      }
    else
      none

  | DomainEvent.EscalationRejected =>
    if state.status = TaskStatus.Escalated then
      some {
        state with
        version := state.version + 1
        status := TaskStatus.Completed
      }
    else
      none

-- Fold events into state
def foldEvents (state : TaskState) (events : List DomainEvent) : Option TaskState :=
  events.foldl (fun accum event =>
    match accum with
    | none => none
    | some s => applyEvent s event
  ) (some state)

-- Record command atomically with its events (idempotency receipt)
-- This ensures command receipt persistence is atomic with event folding
def recordCommand (state : TaskState) (cmd : CommandEnvelope)
    (events : List DomainEvent) : Option TaskState :=
  match foldEvents state events with
  | none => none
  | some state' =>
    let event_type_strs := events.map fun e =>
      match e with
      | DomainEvent.ContractLocked _ _ _ _ _ _ => "ContractLocked"
      | DomainEvent.AttemptCreated _ _ => "AttemptCreated"
      | DomainEvent.EffectIntentCommitted _ _ _ _ => "EffectIntentCommitted"
      | DomainEvent.EffectObserved _ _ _ _ _ => "EffectObserved"
      | DomainEvent.EvidenceAccepted _ _ _ _ => "EvidenceAccepted"
      | DomainEvent.ObligationSatisfied _ _ => "ObligationSatisfied"
      | DomainEvent.AttemptConcluded _ _ => "AttemptConcluded"
      | DomainEvent.TaskCompleted => "TaskCompleted"
      | DomainEvent.EscalationRequested _ _ _ => "EscalationRequested"
      | DomainEvent.EscalationApproved => "EscalationApproved"
      | DomainEvent.EscalationRejected => "EscalationRejected"
    let receipt : CommandReceipt := {
      command_fingerprint := cmd.command_fingerprint
      resulting_event_types := event_type_strs
    }
    some {
      state' with
      command_receipts := assocUpdate cmd.command_id receipt state'.command_receipts
    }

-- Version Progression: each applied event increments version by exactly 1
theorem version_increment_single (state : TaskState) (event : DomainEvent) :
    ∀ state', applyEvent state event = some state' →
    state'.version = state.version + 1 := by
  intro state' h
  simp [applyEvent] at h
  split_ifs at h <|> (injection h with h; omega)

-- Version Progression: folding n events increments version by n
theorem version_increment_fold (state : TaskState) (events : List DomainEvent) :
    ∀ state', foldEvents state events = some state' →
    state'.version = state.version + events.length := by
  induction events with
  | nil =>
    intro state' h
    simp [foldEvents] at h
    injection h with h
    simp
  | cons e es ih =>
    intro state' h
    simp [foldEvents] at h
    split_ifs at h
    have h' := version_increment_single state e _ rfl
    have := ih h
    omega

-- Determinism: applying the same event to the same state produces the same result
theorem applyEvent_det (state : TaskState) (event : DomainEvent) (s1 s2 : TaskState) :
    applyEvent state event = some s1 →
    applyEvent state event = some s2 →
    s1 = s2 := by
  intro h1 h2
  rw [h1] at h2
  injection h2

-- Completed trapping: completed states reject mutations
theorem completed_trapping (state : TaskState) (event : DomainEvent) :
    state.status = TaskStatus.Completed →
    applyEvent state event = none := by
  intro h
  simp [applyEvent]
  split_ifs <|> omega

end HomeBase
