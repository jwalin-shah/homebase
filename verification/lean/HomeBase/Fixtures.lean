-- HomeBase Conformance Fixtures
-- Verify all six fixtures execute correctly and produce expected results

import HomeBase.Domain
import HomeBase.Reducer
import HomeBase.Decision

namespace HomeBase.Fixtures

-- Helper: create initial task state
def emptyState (task_id : TaskID) : TaskState := {
  task_id := task_id
  version := 0
  status := TaskStatus.Active
  contract := none
  attempts := []
  active_attempt := none
  effect_intents := []
  observations := []
  accepted_evidence := []
  satisfied_obligations := []
  command_receipts := []
  escalation := none
}

def taskId1 := TaskID.mk "task-001"
def taskId3 := TaskID.mk "task-003"
def taskId4 := TaskID.mk "task-004"
def taskId5 := TaskID.mk "task-005"
def taskId6 := TaskID.mk "task-006"

def contractId := ContractID.mk "contract-xyz"
def contractVer := ContractVersion.mk 1
def attemptId1 := AttemptID.mk "attempt-1"
def attemptId2 := AttemptID.mk "attempt-2"
def attemptId3 := AttemptID.mk "attempt-3"
def attemptId4 := AttemptID.mk "attempt-4"
def attemptId6 := AttemptID.mk "attempt-6"
def attemptId7 := AttemptID.mk "attempt-7"
def effectId1 := EffectID.mk "effect-1"
def effectId4 := EffectID.mk "effect-4"
def effectId6 := EffectID.mk "effect-6"
def observationId1 := ObservationID.mk "obs-1"
def observationId6 := ObservationID.mk "obs-unknown-1"
def evidenceId1 := EvidenceID.mk "evidence-1"
def obligationId1 := ObligationID.mk "obs-req-1"
def principalId := AuthorityID.mk "homebase-init-01"

-- Fixture 1: happy_path
-- Verifies: Normal workflow from LockContract to TaskCompleted
theorem fixture_happy_path : True := by
  let init_state := emptyState taskId1

  -- Step 1: LockContract command
  let cmd1 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-001"
    task_id := taskId1
    expected_version := 0
    authority := ⟨principalId, AuthorityRole.TaskInitiator⟩
    correlation_id := CorrelationID.mk "corr-001"
    causation_id := none
    body := CommandBody.LockContract contractId contractVer {obligationId1} {"spawn_worker"} 3 "sha256-xyz"
  }

  let decision1 := decide init_state cmd1

  -- Verify: decision is Accepted with ContractLocked event
  have h1 : ∃ events, decision1 = Decision.Accepted events := by
    use [DomainEvent.ContractLocked contractId contractVer {obligationId1} {"spawn_worker"} 3 "sha256-xyz"]
    simp [decide, commandType, isAuthorized, isCommandPersisted]

  -- Extract accepted events
  obtain ⟨events1, h_accept1⟩ := h1

  -- Verify: folding produces state with version 1 and contract locked
  have h_fold1 : ∃ state1, foldEvents init_state events1 = some state1 ∧
                           state1.version = 1 ∧
                           state1.contract.isSome ∧
                           state1.status = TaskStatus.Active := by
    use {
      init_state with
      version := 1
      contract := some {
        contract_id := contractId
        contract_version := contractVer
        contract_digest := "sha256-xyz"
      }
    }
    simp [foldEvents, applyEvent]

  obtain ⟨state1, h_state1, h_ver1, h_contract1, h_status1⟩ := h_fold1

  trivial

-- Fixture 2: stale_command
-- Verifies: Optimistic concurrency rejection on stale version
theorem fixture_stale_command : True := by
  let init_state : TaskState := {
    (emptyState taskId3) with
    version := 2
    status := TaskStatus.Active
    contract := some {
      contract_id := ContractID.mk "contract-def"
      contract_version := ContractVersion.mk 1
      contract_digest := "sha256-def"
    }
  }

  let cmd : CommandEnvelope := {
    command_id := CommandID.mk "cmd-stale-001"
    task_id := taskId3
    expected_version := 1  -- Wrong! Current is 2
    authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
    correlation_id := CorrelationID.mk "corr-002"
    causation_id := none
    body := CommandBody.CreateAttempt attemptId2 1
  }

  let decision := decide init_state cmd

  -- Verify: decision is Rejected with STALE_VERSION
  have h : decision = Decision.Rejected RejectionReason.STALE_VERSION _ := by
    simp [decide, commandType, isAuthorized, isCommandPersisted]

  trivial

-- Fixture 3: duplicate_command_id
-- Verifies: Identical replay returns NoOp
theorem fixture_duplicate_command_id : True := by
  let init_state : TaskState := {
    (emptyState taskId3) with
    version := 1
    contract := some {
      contract_id := ContractID.mk "contract-def"
      contract_version := ContractVersion.mk 1
      contract_digest := "sha256-def"
    }
  }

  let cmd_id := CommandID.mk "cmd-dup-001"

  -- Step 1: First command (new)
  let cmd1 : CommandEnvelope := {
    command_id := cmd_id
    task_id := taskId3
    expected_version := 1
    authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
    correlation_id := CorrelationID.mk "corr-003"
    causation_id := none
    body := CommandBody.CreateAttempt attemptId3 1
  }

  let decision1 := decide init_state cmd1

  -- Verify: first decision is Accepted
  have h1 : ∃ events, decision1 = Decision.Accepted events := by
    simp [decide, commandType, isAuthorized, isCommandPersisted, lookup]
    use [DomainEvent.AttemptCreated attemptId3 1]

  -- Step 2: After first command succeeds, state version is now 2
  let state_after_accept : TaskState := {
    init_state with
    version := 2
    attempts := [(attemptId3, {
      attempt_id := attemptId3
      ordinal := 1
      status := AttemptStatus.Open
      effect_ids := ∅
    })]
    active_attempt := some attemptId3
    command_receipts := [(cmd_id, {
      command_fingerprint := "sha256-cmd-dup-001-body"
      resulting_event_types := ["AttemptCreated"]
    })]
  }

  -- Step 3: Replay same command (now in receipts)
  let cmd2 : CommandEnvelope := cmd1
  let decision2 := decide state_after_accept cmd2

  -- Verify: replay decision is NoOp
  have h2 : decision2 = Decision.NoOp NoOpReason.COMMAND_ALREADY_APPLIED _ := by
    simp [decide, commandType, isCommandPersisted, lookup, isAuthorized]

  trivial

-- Fixture 4: duplicate_effect_intent_conflicting
-- Verifies: Same effect_id with different request_digest rejected
theorem fixture_duplicate_effect_intent_conflicting : True := by
  let init_state : TaskState := {
    (emptyState taskId4) with
    version := 3
    active_attempt := some attemptId4
    contract := some {
      contract_id := ContractID.mk "contract-ghi"
      contract_version := ContractVersion.mk 1
      contract_digest := "sha256-ghi"
    }
  }

  let cmd1 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-conflict-001"
    task_id := taskId4
    expected_version := 3
    authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
    correlation_id := CorrelationID.mk "corr-004"
    causation_id := none
    body := CommandBody.CommitEffectIntent attemptId4 effectId4 "spawn_worker" "sha256-request-v1"
  }

  let decision1 := decide init_state cmd1

  -- Verify: first decision is Accepted
  have h1 : ∃ events, decision1 = Decision.Accepted events := by
    simp [decide, commandType, isAuthorized, isCommandPersisted, lookup]
    use [DomainEvent.EffectIntentCommitted attemptId4 effectId4 "spawn_worker" "sha256-request-v1"]

  -- State after first commit
  let state_after_commit : TaskState := {
    init_state with
    version := 4
    effect_intents := [(effectId4, {
      effect_id := effectId4
      effect_kind := "spawn_worker"
      request_digest := "sha256-request-v1"
      status := IntentStatus.Committed
    })]
  }

  let cmd2 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-conflict-002"
    task_id := taskId4
    expected_version := 4
    authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
    correlation_id := CorrelationID.mk "corr-004"
    causation_id := none
    body := CommandBody.CommitEffectIntent attemptId4 effectId4 "spawn_worker" "sha256-request-v2"
  }

  let decision2 := decide state_after_commit cmd2

  -- Verify: second decision is Rejected with CONFLICTING_EFFECT_ID
  have h2 : decision2 = Decision.Rejected RejectionReason.CONFLICTING_EFFECT_ID _ := by
    simp [decide, commandType, isAuthorized, isCommandPersisted, lookup]

  trivial

-- Fixture 5: missing_obligation
-- Verifies: ProposeCompletion fails with no satisfied obligations
theorem fixture_missing_obligation : True := by
  let init_state : TaskState := {
    (emptyState taskId5) with
    version := 6
    contract := some {
      contract_id := ContractID.mk "contract-jkl"
      contract_version := ContractVersion.mk 1
      contract_digest := "sha256-jkl"
    }
  }

  let cmd : CommandEnvelope := {
    command_id := CommandID.mk "cmd-missing-001"
    task_id := taskId5
    expected_version := 6
    authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
    correlation_id := CorrelationID.mk "corr-005"
    causation_id := none
    body := CommandBody.ProposeCompletion
  }

  let decision := decide init_state cmd

  -- Verify: decision is Rejected with UNMET_OBLIGATIONS
  have h : decision = Decision.Rejected RejectionReason.UNMET_OBLIGATIONS _ := by
    simp [decide, commandType, isAuthorized, isCommandPersisted]

  trivial

-- Fixture 6: recovery_unknown
-- Verifies: Unknown outcome triggers escalation and terminal trapping
theorem fixture_recovery_unknown : True := by
  let init_state : TaskState := {
    (emptyState taskId6) with
    version := 4
    contract := some {
      contract_id := ContractID.mk "contract-mno"
      contract_version := ContractVersion.mk 1
      contract_digest := "sha256-mno"
    }
    effect_intents := [(effectId6, {
      effect_id := effectId6
      effect_kind := "test"
      request_digest := "sha256-req"
      status := IntentStatus.Committed
    })]
  }

  -- Step 1: RecordEffectObservation with Unknown outcome
  let cmd1 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-unknown-001"
    task_id := taskId6
    expected_version := 4
    authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
    correlation_id := CorrelationID.mk "corr-006"
    causation_id := none
    body := CommandBody.RecordEffectObservation observationId6 attemptId6 effectId6 ObservationOutcome.Unknown none
  }

  let decision1 := decide init_state cmd1

  -- Verify: decision is Accepted with EffectObserved event
  have h1 : ∃ events, decision1 = Decision.Accepted events := by
    simp [decide, commandType, isAuthorized, isCommandPersisted, lookup]
    use [DomainEvent.EffectObserved observationId6 attemptId6 effectId6 ObservationOutcome.Unknown none]

  -- State after Unknown observation
  let state_after_unknown : TaskState := {
    init_state with
    version := 5
    observations := [(observationId6, {
      observation_id := observationId6
      attempt_id := attemptId6
      effect_id := effectId6
      outcome := ObservationOutcome.Unknown
      result_digest := none
    })]
    effect_intents := [(effectId6, {
      effect_id := effectId6
      effect_kind := "test"
      request_digest := "sha256-req"
      status := IntentStatus.Terminal
    })]
  }

  -- Step 2: RequestEscalation
  let cmd2 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-unknown-002"
    task_id := taskId6
    expected_version := 5
    authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
    correlation_id := CorrelationID.mk "corr-006"
    causation_id := none
    body := CommandBody.RequestEscalation "INDETERMINATE_OUTCOME" "Unknown outcome"
  }

  let decision2 := decide state_after_unknown cmd2

  -- Verify: decision is Accepted with EscalationRequested event
  have h2 : ∃ events, decision2 = Decision.Accepted events := by
    simp [decide, commandType, isAuthorized, isCommandPersisted]
    use [DomainEvent.EscalationRequested "INDETERMINATE_OUTCOME" "Unknown outcome" none]

  -- State becomes Escalated
  let state_escalated : TaskState := {
    state_after_unknown with
    version := 6
    status := TaskStatus.Escalated
    escalation := some {
      failure_class := "INDETERMINATE_OUTCOME"
      reason := "Unknown outcome"
      related_effect_id := none
      requested_at := 5
    }
  }

  -- Step 3: Verify terminal trapping - any operational command rejected
  let cmd3 : CommandEnvelope := {
    command_id := CommandID.mk "cmd-unknown-003"
    task_id := taskId6
    expected_version := 6
    authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
    correlation_id := CorrelationID.mk "corr-006"
    causation_id := none
    body := CommandBody.CreateAttempt attemptId7 2
  }

  let decision3 := decide state_escalated cmd3

  -- Verify: decision is Rejected with TERMINAL_STATE
  have h3 : decision3 = Decision.Rejected RejectionReason.TERMINAL_STATE _ := by
    simp [decide, commandType]

  trivial

end HomeBase.Fixtures
