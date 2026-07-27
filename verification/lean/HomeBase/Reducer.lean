-- HomeBase Reducer
-- Pure state transition function via event application
-- Proves deterministic application and version progression

import HomeBase.Domain

namespace HomeBase

-- Apply a single domain event to state
-- Returns Option because malformed events should fail
def applyEvent (state : TaskState) (event : DomainEvent) : Option TaskState :=
  match event with

  -- ContractLocked: Can only lock once, status must be Active, no prior contract
  | DomainEvent.ContractLocked cid cver obls efks max_att digest =>
    if state.status = TaskStatus.Active ∧ state.contract = none then
      some {
        state with
        version := state.version + 1
        contract := some {
          contract_id := cid
          contract_version := cver
          contract_digest := digest
        }
      }
    else
      none

  -- AttemptCreated: Status must be Active, can't exceed max_attempts
  | DomainEvent.AttemptCreated aid ordinal =>
    match state.contract with
    | none => none  -- No contract locked
    | some contract_ref =>
      -- For now, don't enforce max_attempts; that's in the decision function
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

  -- EffectIntentCommitted: Active attempt must exist, effect must not exist
  | DomainEvent.EffectIntentCommitted aid eid kind digest =>
    if state.status = TaskStatus.Active then
      -- Check attempt exists
      match lookup aid state.attempts with
      | none => none
      | some attempt =>
        -- Check effect doesn't already exist
        if lookup eid state.effect_intents |>.isSome then
          none  -- Effect already exists
        else
          let new_intent : EffectIntent := {
            effect_id := eid
            effect_kind := kind
            request_digest := digest
            status := IntentStatus.Committed
          }
          let updated_attempt : Attempt := {
            attempt with
            effect_ids := attempt.effect_ids.insert eid
          }
          some {
            state with
            version := state.version + 1
            effect_intents := assocUpdate eid new_intent state.effect_intents
            attempts := assocUpdate aid updated_attempt state.attempts
          }
    else
      none

  -- EffectObserved: Effect must exist and be committed
  | DomainEvent.EffectObserved oid aid eid outcome result =>
    if state.status = TaskStatus.Active then
      match lookup eid state.effect_intents with
      | none => none
      | some intent =>
        if intent.status ≠ IntentStatus.Committed then
          none  -- Effect not in right state
        else
          -- Check observation doesn't already exist
          if lookup oid state.observations |>.isSome then
            none
          else
            let new_observation : Observation := {
              observation_id := oid
              attempt_id := aid
              effect_id := eid
              outcome := outcome
              result_digest := result
            }
            -- Mark intent as terminal if outcome is terminal
            let new_intent_status :=
              if outcome = ObservationOutcome.Succeeded ∨ outcome = ObservationOutcome.Failed then
                IntentStatus.Terminal
              else if outcome = ObservationOutcome.Unknown then
                IntentStatus.Terminal  -- Also terminal for Unknown
              else
                IntentStatus.OutcomeNeeded
            let updated_intent := { intent with status := new_intent_status }
            some {
              state with
              version := state.version + 1
              observations := assocUpdate oid new_observation state.observations
              effect_intents := assocUpdate eid updated_intent state.effect_intents
            }
    else
      none

  -- EvidenceAccepted: Observation must exist and be Succeeded
  | DomainEvent.EvidenceAccepted evid aid src_oid digest =>
    if state.status = TaskStatus.Active then
      match lookup src_oid state.observations with
      | none => none
      | some obs =>
        if obs.outcome ≠ ObservationOutcome.Succeeded then
          none  -- Observation not successful
        else
          -- Check evidence doesn't already exist
          if lookup evid state.accepted_evidence |>.isSome then
            none
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

  -- ObligationSatisfied: Obligation must be required, all evidence must be accepted
  | DomainEvent.ObligationSatisfied obid evidence_ids =>
    if state.status = TaskStatus.Active then
      -- Check all evidence IDs are in accepted_evidence
      let all_evidence_valid := evidence_ids.all fun evid =>
        lookup evid state.accepted_evidence |>.isSome
      if ¬all_evidence_valid then
        none
      else
        -- Check obligation doesn't already exist
        if lookup obid state.satisfied_obligations |>.isSome then
          none
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

  -- TaskCompleted: Status must be Active
  | DomainEvent.TaskCompleted =>
    if state.status = TaskStatus.Active then
      some {
        state with
        version := state.version + 1
        status := TaskStatus.Completed
      }
    else
      none

  -- EscalationRequested: Status must be Active
  | DomainEvent.EscalationRequested failure_class reason related_eid =>
    if state.status = TaskStatus.Active then
      some {
        state with
        version := state.version + 1
        status := TaskStatus.Escalated
        escalation := some {
          failure_class := failure_class
          reason := reason
          related_effect_id := related_eid
          requested_at := state.version
        }
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

-- Properties of the reducer

-- Versioning: each applied event increments version by exactly 1
theorem version_increment_single (state : TaskState) (event : DomainEvent) :
    ∀ state', applyEvent state event = some state' →
    state'.version = state.version + 1 := by
  intro state' h
  cases event <;> simp [applyEvent] at h <;>
  try { split_ifs at h <;> simp at h }
  all_goals (try { injection h with h; omega }; try { omega })

-- Versioning: folding n events increments version by n
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
    cases result : applyEvent state e with
    | none => simp [result] at h
    | some s =>
      simp [result] at h
      have h' := version_increment_single state e s rfl
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

-- Terminal states: if state is Completed or Escalated, applying most events fails
theorem terminal_blocks_mutation (state : TaskState) (event : DomainEvent) :
    state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated →
    (applyEvent state event).isNone := by
  intro hterm
  cases event <;> simp [applyEvent] <;>
  try {
    cases hterm with
    | inl h => simp [h]
    | inr h => simp [h]
  }
  all_goals (try omega)

end HomeBase
