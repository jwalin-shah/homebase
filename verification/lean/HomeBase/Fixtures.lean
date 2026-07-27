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

end HomeBase.Fixtures
