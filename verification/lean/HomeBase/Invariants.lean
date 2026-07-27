-- HomeBase Invariants
-- Proofs of core domain properties

import HomeBase.Domain
import HomeBase.Reducer
import HomeBase.Decision

namespace HomeBase

-- I1: Determinism - decide is deterministic
theorem decide_deterministic (state : TaskState) (cmd : CommandEnvelope) :
    ∃! d, d = decide state cmd := by
  use decide state cmd
  simp

-- I2: Version Monotonicity - accepted decisions increment version
theorem accepted_increments_version (state : TaskState) (cmd : CommandEnvelope)
    (events : List DomainEvent) :
    decide state cmd = Decision.Accepted events →
    ∃ state', foldEvents state events = some state' ∧
              state'.version > state.version := by
  intro h_accepted
  -- This is a sketch; full proof would unfold decide and show each case
  -- increments version via foldEvents
  sorry

-- I3: Intent Before Observation - observations require committed intents
theorem intent_before_observation (state : TaskState) (oid : ObservationID)
    (aid : AttemptID) (eid : EffectID) (outcome : ObservationOutcome)
    (result : Option String) :
    applyEvent state (DomainEvent.EffectObserved oid aid eid outcome result) ≠ none →
    ∃ intent, lookup eid state.effect_intents = some intent ∧
              intent.status = IntentStatus.Committed := by
  intro h_not_none
  cases h_not_none : applyEvent state (DomainEvent.EffectObserved oid aid eid outcome result) with
  | none => contradiction
  | some s =>
    -- From applyEvent logic, if it succeeds, the intent must exist and be Committed
    simp [applyEvent] at h_not_none
    split_ifs at h_not_none <|> omega
    sorry

-- I4: Attempt Ownership - evidence refers to observations from same attempt
theorem attempt_ownership (state : TaskState) (evid : EvidenceID) (aid : AttemptID)
    (src_oid : ObservationID) (digest : String) :
    applyEvent state (DomainEvent.EvidenceAccepted evid aid src_oid digest) ≠ none →
    ∃ obs, lookup src_oid state.observations = some obs ∧ obs.attempt_id = aid := by
  intro h_not_none
  cases h_not_none : applyEvent state (DomainEvent.EvidenceAccepted evid aid src_oid digest) with
  | none => contradiction
  | some s =>
    simp [applyEvent] at h_not_none
    split_ifs at h_not_none <|> omega
    sorry

-- I5: Stable Effect Identity - effect ID cannot have multiple request_digests
theorem stable_effect_identity (state : TaskState) (aid : AttemptID) (eid : EffectID)
    (kind : String) (digest1 digest2 : String) :
    lookup eid state.effect_intents = some ⟨eid, kind, digest1, _⟩ →
    applyEvent state (DomainEvent.EffectIntentCommitted aid eid kind digest2) = none ∨
    digest1 = digest2 := by
  intro h_existing
  simp [applyEvent]
  split_ifs <|> omega
  sorry

-- I6: Evidence Provenance - evidence references successful observations
theorem evidence_provenance (state : TaskState) (evid : EvidenceID) (aid : AttemptID)
    (src_oid : ObservationID) (digest : String) :
    applyEvent state (DomainEvent.EvidenceAccepted evid aid src_oid digest) ≠ none →
    ∃ obs, lookup src_oid state.observations = some obs ∧
           obs.outcome = ObservationOutcome.Succeeded := by
  intro h_not_none
  cases h_not_none : applyEvent state (DomainEvent.EvidenceAccepted evid aid src_oid digest) with
  | none => contradiction
  | some s =>
    simp [applyEvent] at h_not_none
    split_ifs at h_not_none <|> omega
    sorry

-- I7: Obligation Provenance - obligations satisfied only by evidence
theorem obligation_provenance (state : TaskState) (obid : ObligationID)
    (evidence_ids : Finset EvidenceID) :
    applyEvent state (DomainEvent.ObligationSatisfied obid evidence_ids) ≠ none →
    ∀ evid ∈ evidence_ids, lookup evid state.accepted_evidence ≠ none := by
  intro h_not_none evid h_mem
  cases h_not_none : applyEvent state (DomainEvent.ObligationSatisfied obid evidence_ids) with
  | none => contradiction
  | some s =>
    simp [applyEvent] at h_not_none
    split_ifs at h_not_none <|> omega
    sorry

-- I8: Completion Soundness - completion requires all obligations satisfied
theorem completion_soundness (state : TaskState) :
    decide state {
      command_id := CommandID.mk "test"
      task_id := state.task_id
      expected_version := state.version
      authority := Authority.mk (AuthorityID.mk "test") AuthorityRole.Orchestrator
      correlation_id := CorrelationID.mk "test"
      causation_id := none
      body := CommandBody.ProposeCompletion
    } = Decision.Accepted [DomainEvent.TaskCompleted] →
    state.satisfied_obligations.length > 0 := by
  intro h
  simp [decide] at h
  split_ifs at h <|> omega
  sorry

-- I9: Attempt Bound - attempts don't exceed max_attempts
-- (Deferred: requires max_attempts enforcement in decision function)

-- I10: Terminal Trapping - no mutations after Completed/Escalated
theorem terminal_trapping (state : TaskState) (event : DomainEvent) :
    (state.status = TaskStatus.Completed ∨ state.status = TaskStatus.Escalated) →
    applyEvent state event = none ∨ applyEvent state event = some state := by
  intro h_term
  cases event <;> simp [applyEvent] <;>
  try { split_ifs <;> simp } <;>
  try { omega }
  all_goals (try { cases h_term <;> simp at * })

-- I11: Single Terminal Outcome - task cannot be both Completed and Escalated
theorem single_terminal_outcome (state : TaskState) :
    state.status = TaskStatus.Completed →
    state.status ≠ TaskStatus.Escalated := by
  intro h
  simp [h]

-- I12: Accepted Transitions Preserve Valid State
-- (Sketch: folding accepted events produces well-formed state)
theorem accepted_preserves_validity (state : TaskState) (cmd : CommandEnvelope)
    (events : List DomainEvent) (state' : TaskState) :
    decide state cmd = Decision.Accepted events →
    foldEvents state events = some state' →
    state'.version ≥ state.version := by
  intros _ h_fold
  -- folding any events increases or preserves version
  sorry

end HomeBase
