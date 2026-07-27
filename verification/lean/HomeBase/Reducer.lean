-- HomeBase Reducer (Repaired)
-- Pure state transitions with full precondition checking

import HomeBase.Domain

namespace HomeBase

-- Valid State Predicate
def ValidState (state : TaskState) : Prop :=
  -- Contract consistency
  (state.contract.isSome → state.status = TaskStatus.Active) ∧

  -- Attempt consistency
  (match state.active_attempt with
   | none => true
   | some aid =>
     lookup aid state.attempts |>.map (fun a => a.status = AttemptStatus.Open) |>.getD false
   ) ∧

  -- No duplicate attempt IDs
  state.attempts.Nodup (fun (a1, _) (a2, _) => a1 = a2) ∧

  -- All effect intents reference existing attempts
  (∀ eid intent, lookup eid state.effect_intents = some intent →
    lookup intent.attempt_id state.attempts |>.isSome) ∧

  -- All observations reference existing intents
  (∀ oid obs, lookup oid state.observations = some obs →
    lookup obs.effect_id state.effect_intents |>.isSome) ∧

  -- All accepted evidence references existing observations
  (∀ evid ev, lookup evid state.accepted_evidence = some ev →
    lookup ev.source_observation_id state.observations |>.isSome) ∧

  -- All satisfied obligations are required by contract
  (∀ obid obs, lookup obid state.satisfied_obligations = some obs →
    (match state.contract with
     | none => false
     | some c => obid ∈ c.required_obligations
     end)) ∧

  -- Attempt count <= max_attempts
  (match state.contract with
   | none => true
   | some c => state.attempts.length ≤ c.max_attempts
   end) ∧

  -- Terminal state consistency
  ((state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated) →
   state.active_attempt = none) ∧

  -- Completed requires all obligations
  (state.status = TaskStatus.Completed →
   match state.contract with
   | none => false
   | some c =>
     ∀ obid, obid ∈ c.required_obligations →
     (lookup obid state.satisfied_obligations).isSome
   end) ∧

  -- Escalated requires escalation data
  (state.status = TaskStatus.Escalated → state.escalation.isSome)

-- Apply a single domain event to state
def applyEvent (state : TaskState) (event : DomainEvent) : Option TaskState :=
  if ¬ValidState state then none else

  match event with

  | DomainEvent.ContractLocked cid cver obls efks max_att digest =>
    if state.status = TaskStatus.Active ∧ state.contract = none ∧ max_att > 0 then
      some {
        state with
        version := state.version + 1
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
          let new_attempt_status :=
            match outcome with
            | ObservationOutcome.Succeeded => AttemptStatus.Succeeded
            | ObservationOutcome.Failed => AttemptStatus.Failed
            | ObservationOutcome.Unknown => AttemptStatus.OutcomeUnknown
            | _ => AttemptStatus.Open
          let updated_attempt : Attempt :=
            if new_attempt_status = AttemptStatus.Open then
              (lookup aid state.attempts |>.getD { attempt_id := aid, ordinal := 0, status := AttemptStatus.Open, effect_ids := ∅ })
            else
              let a := lookup aid state.attempts |>.getD { attempt_id := aid, ordinal := 0, status := AttemptStatus.Open, effect_ids := ∅ }
              { a with status := new_attempt_status }
          let new_active_attempt := if new_attempt_status = AttemptStatus.Open then state.active_attempt else none
          some {
            state with
            version := state.version + 1
            observations := assocUpdate oid new_observation state.observations
            effect_intents := assocUpdate eid updated_intent state.effect_intents
            attempts := assocUpdate aid updated_attempt state.attempts
            active_attempt := new_active_attempt
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

-- Fold events into state
def foldEvents (state : TaskState) (events : List DomainEvent) : Option TaskState :=
  events.foldl (fun accum event =>
    match accum with
    | none => none
    | some s => applyEvent s event
  ) (some state)

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

-- Terminal trapping: terminal states reject mutations
theorem terminal_trapping (state : TaskState) (event : DomainEvent) :
    (state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated) →
    applyEvent state event = none := by
  intro h
  simp [applyEvent]
  split_ifs <|> omega

end HomeBase
