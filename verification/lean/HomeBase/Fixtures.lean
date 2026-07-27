-- HomeBase Conformance Fixtures (Exact Propositions)
-- 6 fixtures with exact executable assertions

import HomeBase.Domain
import HomeBase.Reducer
import HomeBase.Decision

namespace HomeBase.Fixtures

-- Empty state for testing
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

-- Test data
def taskId1 := TaskID.mk "task-001"
def taskId3 := TaskID.mk "task-003"
def taskId4 := TaskID.mk "task-004"
def taskId5 := TaskID.mk "task-005"
def taskId6 := TaskID.mk "task-006"

def contractId := ContractID.mk "contract-xyz"
def contractVer := ContractVersion.mk 1
def contractDigest := Hash.mk "sha256-contract-xyz"

def attemptId1 := AttemptID.mk "attempt-1"
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
def correlationId := CorrelationID.mk "corr-001"

def effectKind := EffectKind.mk "spawn_worker"

-- Fixture 1: happy_path
-- Full workflow from LockContract to TaskCompleted
theorem fixture_happy_path :
    let init := emptyState taskId1
    let cmdLock : CommandEnvelope := {
      command_id := CommandID.mk "cmd-001"
      task_id := taskId1
      expected_version := 0
      command_fingerprint := Hash.mk "fp-001"
      authority := ⟨principalId, AuthorityRole.TaskInitiator⟩
      correlation_id := correlationId
      causation_id := none
      body := CommandBody.LockContract contractId contractVer {obligationId1} {effectKind} 3 contractDigest
    }
    decide init cmdLock = Decision.Accepted [
      DomainEvent.ContractLocked contractId contractVer {obligationId1} {effectKind} 3 contractDigest
    ] := by
  decide

-- Fixture 2: stale_command
-- Optimistic concurrency rejection
theorem fixture_stale_command :
    let state : TaskState := {
      (emptyState taskId3) with
      version := 2
      contract := some {
        contract_id := ContractID.mk "contract-def"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-def"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-stale"
      task_id := taskId3
      expected_version := 1  -- Stale!
      command_fingerprint := Hash.mk "fp-stale"
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-002"
      causation_id := none
      body := CommandBody.CreateAttempt attemptId3 1
    }
    decide state cmd = Decision.Rejected RejectionReason.STALE_VERSION := by
  decide

-- Fixture 3: duplicate_command_id
-- Exact fingerprint match returns NoOp
theorem fixture_duplicate_command_id :
    let state : TaskState := {
      (emptyState taskId3) with
      version := 2
      contract := some {
        contract_id := ContractID.mk "contract-def"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-def"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      command_receipts := [(
        CommandID.mk "cmd-dup",
        { command_fingerprint := Hash.mk "fp-dup", resulting_event_types := ["AttemptCreated"] }
      )]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-dup"
      task_id := taskId3
      expected_version := 2
      command_fingerprint := Hash.mk "fp-dup"  -- Identical fingerprint
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-003"
      causation_id := none
      body := CommandBody.CreateAttempt attemptId3 1
    }
    decide state cmd = Decision.NoOp NoOpReason.COMMAND_ALREADY_APPLIED := by
  decide

-- Fixture 4: duplicate_effect_intent_conflicting
-- Same effect_id with different request_digest rejected
theorem fixture_duplicate_effect_intent_conflicting :
    let state : TaskState := {
      (emptyState taskId4) with
      version := 4
      active_attempt := some attemptId4
      attempts := [(attemptId4, {
        attempt_id := attemptId4
        ordinal := 1
        status := AttemptStatus.Open
        effect_ids := {effectId4}
      })]
      contract := some {
        contract_id := ContractID.mk "contract-ghi"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-ghi"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      effect_intents := [(effectId4, {
        effect_id := effectId4
        attempt_id := attemptId4
        effect_kind := effectKind
        request_digest := Hash.mk "sha256-v1"
        status := IntentStatus.Committed
      })]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-conflict"
      task_id := taskId4
      expected_version := 4
      command_fingerprint := Hash.mk "fp-conflict"
      authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
      correlation_id := CorrelationID.mk "corr-004"
      causation_id := none
      body := CommandBody.CommitEffectIntent attemptId4 effectId4 effectKind (Hash.mk "sha256-v2")
    }
    decide state cmd = Decision.Rejected RejectionReason.CONFLICTING_EFFECT_ID := by
  decide

-- Fixture 5: missing_obligation
-- ProposeCompletion fails with unmet obligations
theorem fixture_missing_obligation :
    let state : TaskState := {
      (emptyState taskId5) with
      version := 6
      contract := some {
        contract_id := ContractID.mk "contract-jkl"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-jkl"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-missing"
      task_id := taskId5
      expected_version := 6
      command_fingerprint := Hash.mk "fp-missing"
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-005"
      causation_id := none
      body := CommandBody.ProposeCompletion
    }
    decide state cmd = Decision.Rejected RejectionReason.UNMET_OBLIGATIONS := by
  decide

-- Fixture 6: recovery_unknown
-- Unknown outcome triggers escalation; terminal state traps commands
theorem fixture_recovery_unknown :
    let state : TaskState := {
      (emptyState taskId6) with
      version := 6
      status := TaskStatus.Escalated
      contract := some {
        contract_id := ContractID.mk "contract-mno"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-mno"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      escalation := some {
        failure_class := FailureClass.mk "INDETERMINATE_OUTCOME"
        reason := "Unknown outcome"
        related_effect_id := some effectId6
        requested_at := 5
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-after-escalation"
      task_id := taskId6
      expected_version := 6
      command_fingerprint := Hash.mk "fp-after-escalation"
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-006"
      causation_id := none
      body := CommandBody.CreateAttempt attemptId7 2
    }
    -- Terminal state should reject operational commands
    decide state cmd = Decision.Rejected RejectionReason.TERMINAL_STATE := by
  decide

-- Fixture 7: unauthorized_recovery_complete (Authorization: RecoveryController cannot complete)
-- Adversarial: RecoveryController tries to call ProposeCompletion (restricted role)
theorem fixture_unauthorized_recovery_complete :
    let state : TaskState := {
      (emptyState taskId1) with
      version := 5
      status := TaskStatus.Active
      contract := some {
        contract_id := ContractID.mk "contract-adv7"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv7"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      satisfied_obligations := [(obligationId1, { obligation_id := obligationId1, evidence_ids := ∅ })]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv7"
      task_id := taskId1
      expected_version := 5
      command_fingerprint := Hash.mk "fp-adv7"
      authority := ⟨AuthorityID.mk "recovery", AuthorityRole.RecoveryController⟩
      correlation_id := CorrelationID.mk "corr-adv7"
      causation_id := none
      body := CommandBody.ProposeCompletion
    }
    -- RecoveryController cannot complete tasks
    decide state cmd = Decision.Rejected RejectionReason.UNAUTHORIZED := by
  decide

-- Fixture 8: effect_kind_not_allowed (Semantic: Effect kind violates contract)
-- Adversarial: Attempt to commit effect intent with kind not in allowed_effect_kinds
theorem fixture_effect_kind_not_allowed :
    let badKind := EffectKind.mk "forbidden_effect"
    let state : TaskState := {
      (emptyState taskId3) with
      version := 2
      status := TaskStatus.Active
      active_attempt := some attemptId3
      attempts := [(attemptId3, {
        attempt_id := attemptId3
        ordinal := 1
        status := AttemptStatus.Open
        effect_ids := ∅
      })]
      contract := some {
        contract_id := ContractID.mk "contract-adv8"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv8"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}  -- Only effectKind allowed
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv8"
      task_id := taskId3
      expected_version := 2
      command_fingerprint := Hash.mk "fp-adv8"
      authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
      correlation_id := CorrelationID.mk "corr-adv8"
      causation_id := none
      body := CommandBody.CommitEffectIntent attemptId3 effectId1 badKind (Hash.mk "sha256-bad-kind")
    }
    decide state cmd = Decision.Rejected RejectionReason.EFFECT_KIND_NOT_ALLOWED := by
  decide

-- Fixture 9: observation_without_intent (Integrity: Orphaned observation)
-- Adversarial: Record observation for nonexistent effect intent
theorem fixture_observation_without_intent :
    let state : TaskState := {
      (emptyState taskId4) with
      version := 1
      status := TaskStatus.Active
      contract := some {
        contract_id := ContractID.mk "contract-adv9"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv9"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv9"
      task_id := taskId4
      expected_version := 1
      command_fingerprint := Hash.mk "fp-adv9"
      authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
      correlation_id := CorrelationID.mk "corr-adv9"
      causation_id := none
      body := CommandBody.RecordEffectObservation observationId1 attemptId4 effectId4
                 ObservationOutcome.Succeeded (some (Hash.mk "sha256-result"))
    }
    -- Cannot record observation for nonexistent effect intent
    decide state cmd = Decision.Rejected RejectionReason.EFFECT_NOT_FOUND := by
  decide

-- Fixture 10: evidence_without_successful_observation (Integrity: Evidence without proof)
-- Adversarial: Accept evidence from observation with non-Succeeded outcome
theorem fixture_evidence_without_successful_observation :
    let state : TaskState := {
      (emptyState taskId5) with
      version := 3
      status := TaskStatus.Active
      observations := [(observationId1, {
        observation_id := observationId1
        attempt_id := attemptId1
        effect_id := effectId1
        outcome := ObservationOutcome.Failed  -- Not Succeeded!
        result_digest := some (Hash.mk "sha256-failed")
      })]
      effect_intents := [(effectId1, {
        effect_id := effectId1
        attempt_id := attemptId1
        effect_kind := effectKind
        request_digest := Hash.mk "sha256-req"
        status := IntentStatus.Terminal
      })]
      contract := some {
        contract_id := ContractID.mk "contract-adv10"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv10"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv10"
      task_id := taskId5
      expected_version := 3
      command_fingerprint := Hash.mk "fp-adv10"
      authority := ⟨AuthorityID.mk "verifier", AuthorityRole.Verifier⟩
      correlation_id := CorrelationID.mk "corr-adv10"
      causation_id := none
      body := CommandBody.AcceptEvidence evidenceId1 attemptId1 observationId1 (Hash.mk "sha256-evidence")
    }
    -- Cannot accept evidence from failed observation
    decide state cmd = Decision.Rejected RejectionReason.OBSERVATION_NOT_SUCCESSFUL := by
  decide

-- Fixture 11: attempt_limit_exceeded (Constraint: Max attempts boundary)
-- Adversarial: Create attempt beyond max_attempts limit
theorem fixture_attempt_limit_exceeded :
    let state : TaskState := {
      (emptyState taskId6) with
      version := 5
      status := TaskStatus.Active
      attempts := [
        (attemptId3, {attempt_id := attemptId3, ordinal := 1, status := AttemptStatus.Open, effect_ids := ∅}),
        (attemptId4, {attempt_id := attemptId4, ordinal := 2, status := AttemptStatus.Open, effect_ids := ∅}),
        (attemptId6, {attempt_id := attemptId6, ordinal := 3, status := AttemptStatus.Open, effect_ids := ∅})
      ]
      contract := some {
        contract_id := ContractID.mk "contract-adv11"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv11"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3  -- Already at limit
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv11"
      task_id := taskId6
      expected_version := 5
      command_fingerprint := Hash.mk "fp-adv11"
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-adv11"
      causation_id := none
      body := CommandBody.CreateAttempt attemptId7 4
    }
    decide state cmd = Decision.Rejected RejectionReason.ATTEMPT_LIMIT_REACHED := by
  decide

-- Fixture 12: obligation_not_required (Semantic: Obligation not in contract)
-- Adversarial: Try to satisfy obligation not required by contract
theorem fixture_obligation_not_required :
    let badObId := ObligationID.mk "obs-not-required"
    let state : TaskState := {
      (emptyState taskId1) with
      version := 4
      status := TaskStatus.Active
      contract := some {
        contract_id := ContractID.mk "contract-adv12"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv12"
        required_obligations := {obligationId1}  -- Only obligationId1 required
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      accepted_evidence := [(evidenceId1, {
        evidence_id := evidenceId1
        attempt_id := attemptId1
        source_observation_id := observationId1
        evidence_digest := Hash.mk "sha256-ev"
      })]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv12"
      task_id := taskId1
      expected_version := 4
      command_fingerprint := Hash.mk "fp-adv12"
      authority := ⟨AuthorityID.mk "verifier", AuthorityRole.Verifier⟩
      correlation_id := CorrelationID.mk "corr-adv12"
      causation_id := none
      body := CommandBody.SatisfyObligation badObId {evidenceId1}
    }
    decide state cmd = Decision.Rejected RejectionReason.OBLIGATION_NOT_REQUIRED := by
  decide

-- Fixture 13: conflicting_command_id (Idempotency: Fingerprint mismatch)
-- Adversarial: Replay command with same ID but different fingerprint
theorem fixture_conflicting_command_id :
    let state : TaskState := {
      (emptyState taskId3) with
      version := 1
      contract := some {
        contract_id := ContractID.mk "contract-adv13"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv13"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      command_receipts := [(
        CommandID.mk "cmd-conflict-v1",
        { command_fingerprint := Hash.mk "fp-original", resulting_event_types := ["LockContract"] }
      )]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-conflict-v1"  -- Same ID
      task_id := taskId3
      expected_version := 1
      command_fingerprint := Hash.mk "fp-modified"  -- Different fingerprint
      authority := ⟨AuthorityID.mk "init", AuthorityRole.TaskInitiator⟩
      correlation_id := CorrelationID.mk "corr-adv13"
      causation_id := none
      body := CommandBody.LockContract (ContractID.mk "different") (ContractVersion.mk 2)
                 {obligationId1} {effectKind} 3 (Hash.mk "sha256-different")
    }
    decide state cmd = Decision.Rejected RejectionReason.COMMAND_ID_CONFLICT := by
  decide

-- Fixture 14: escalation_invalid_effect_id (Integrity: Dangling escalation reference)
-- Adversarial: Request escalation with invalid related_effect_id
theorem fixture_escalation_invalid_effect_id :
    let state : TaskState := {
      (emptyState taskId4) with
      version := 2
      status := TaskStatus.Active
      contract := some {
        contract_id := ContractID.mk "contract-adv14"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-adv14"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-adv14"
      task_id := taskId4
      expected_version := 2
      command_fingerprint := Hash.mk "fp-adv14"
      authority := ⟨AuthorityID.mk "adapter", AuthorityRole.BridgeAdapter⟩
      correlation_id := CorrelationID.mk "corr-adv14"
      causation_id := none
      body := CommandBody.RequestEscalation (FailureClass.mk "ORPHANED_EFFECT")
                 "Effect intent not found" (some (EffectID.mk "nonexistent"))
    }
    decide state cmd = Decision.Rejected RejectionReason.EFFECT_NOT_FOUND := by
  decide

-- Fixture 15: conclude_attempt (New command: Explicit attempt conclusion)
-- Tests that ConcludeAttempt requires all intents to be Terminal
theorem fixture_conclude_attempt :
    let state : TaskState := {
      (emptyState taskId1) with
      version := 4
      status := TaskStatus.Active
      active_attempt := some attemptId1
      attempts := [(attemptId1, {
        attempt_id := attemptId1
        ordinal := 1
        status := AttemptStatus.Open
        effect_ids := {effectId1}
      })]
      contract := some {
        contract_id := ContractID.mk "contract-conclude"
        contract_version := ContractVersion.mk 1
        contract_digest := Hash.mk "sha256-conclude"
        required_obligations := {obligationId1}
        allowed_effect_kinds := {effectKind}
        max_attempts := 3
      }
      effect_intents := [(effectId1, {
        effect_id := effectId1
        attempt_id := attemptId1
        effect_kind := effectKind
        request_digest := Hash.mk "sha256-req"
        status := IntentStatus.Terminal
      })]
    }
    let cmd : CommandEnvelope := {
      command_id := CommandID.mk "cmd-conclude"
      task_id := taskId1
      expected_version := 4
      command_fingerprint := Hash.mk "fp-conclude"
      authority := ⟨AuthorityID.mk "orch", AuthorityRole.Orchestrator⟩
      correlation_id := CorrelationID.mk "corr-conclude"
      causation_id := none
      body := CommandBody.ConcludeAttempt attemptId1
    }
    -- Concluding attempt clears active_attempt
    decide state cmd = Decision.Accepted [DomainEvent.AttemptConcluded attemptId1] := by
  decide

end HomeBase.Fixtures
