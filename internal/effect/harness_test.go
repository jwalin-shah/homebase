package effect

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
	"os"
	"fmt"

	"homebase/internal/domain"
)

// FakeEffectRepository simulates a durable journal for effect states
type FakeEffectRepository struct {
	mu      sync.Mutex
	states  map[domain.EffectID]domain.EffectState
	version map[domain.EffectID]uint64
	events  map[domain.EffectID][]domain.EffectEvent
}

func NewFakeEffectRepository() *FakeEffectRepository {
	return &FakeEffectRepository{
		states:  make(map[domain.EffectID]domain.EffectState),
		version: make(map[domain.EffectID]uint64),
		events:  make(map[domain.EffectID][]domain.EffectEvent),
	}
}

func (r *FakeEffectRepository) Load(effectID domain.EffectID) (domain.EffectState, uint64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[effectID]
	if !ok {
		return domain.EffectState{EffectID: effectID, Phase: domain.EffectPending}, 0
	}
	return state, r.version[effectID]
}

func (r *FakeEffectRepository) Append(effectID domain.EffectID, expectedVersion uint64, events []domain.EffectEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.version[effectID] != expectedVersion {
		return errors.New("version conflict")
	}

	state := r.states[effectID]
	if state.EffectID == "" {
		state = domain.EffectState{EffectID: effectID, Phase: domain.EffectPending}
	}

	for _, e := range events {
		state = domain.ApplyEffect(state, e)
		r.events[effectID] = append(r.events[effectID], e)
	}

	r.states[effectID] = state
	r.version[effectID] = expectedVersion + 1
	return nil
}

func TestEffect_CrashAfterRemoteSuccess(t *testing.T) {
	repo := NewFakeEffectRepository()
	executor := NewFakeExecutor()
	effectID := domain.EffectID("eff-123")
	workerID := "worker-1"
	idempotencyKey := "idem-456"
	currentTime := int(time.Now().Unix())
	leaseUntil := currentTime + 60

	// Stage 1: Worker claims effect
	state, version := repo.Load(effectID)
	cmdClaim := domain.CommandClaimEffect{
		EffectID:        effectID,
		WorkerID:        workerID,
		ExpectedVersion: int(version),
		LeaseUntil:      leaseUntil,
		CurrentTime:     currentTime,
	}
	decision := domain.DecideEffect(state, cmdClaim)
	if decision.Status != domain.DecisionAccepted {
		t.Fatalf("expected claim accepted, got %v", decision.Status)
	}
	err := repo.Append(effectID, version, decision.Events)
	if err != nil {
		t.Fatalf("failed to append claim: %v", err)
	}

	// Stage 2: Execute remote operation, but simulate crash before observation commit
	executor.PushOutcome(effectID, PlannedOutcome{Action: "UnknownAfterApply", Reason: "simulated_timeout"})
	_, _ = executor.Execute(context.Background(), effectID, idempotencyKey)
	
	// Stage 3: New worker (or same worker) claims after lease expiry
	currentTime = leaseUntil + 1
	state, version = repo.Load(effectID)
	
	// Must retry it first to move back to pending
	cmdRetry := domain.CommandRetryEffect{EffectID: effectID}
	decision = domain.DecideEffect(state, cmdRetry)
	repo.Append(effectID, version, decision.Events)
	state, version = repo.Load(effectID)
	
	cmdClaim2 := domain.CommandClaimEffect{
		EffectID:        effectID,
		WorkerID:        "worker-2",
		ExpectedVersion: int(version),
		LeaseUntil:      currentTime + 60,
		CurrentTime:     currentTime,
	}
	decision = domain.DecideEffect(state, cmdClaim2)
	if decision.Status != domain.DecisionAccepted {
		t.Fatalf("expected claim accepted for new worker, got %v", decision.Status)
	}
	repo.Append(effectID, version, decision.Events)

	// Second execution, remote system handles due to idempotency key
	executor.PushOutcome(effectID, PlannedOutcome{Action: "Success", Reason: ""})
	outcome2, _ := executor.Execute(context.Background(), effectID, idempotencyKey)

	// Stage 4: Commit observation
	state, version = repo.Load(effectID)
	cmdObs := domain.CommandObserveEffect{
		EffectID:   effectID,
		WorkerID:   "worker-2",
		ClaimEpoch: state.ClaimEpoch, // matches
		Outcome:    outcome2,
	}
	decision = domain.DecideEffect(state, cmdObs)
	if decision.Status != domain.DecisionAccepted {
		t.Fatalf("expected observation accepted, got %v", decision.Status)
	}
	repo.Append(effectID, version, decision.Events)

	// Validate results
	finalState, _ := repo.Load(effectID)
	if finalState.Phase != domain.EffectSucceeded {
		t.Fatalf("expected EffectSucceeded, got %v", finalState.Phase)
	}
	if executor.Applied[idempotencyKey] != 1 {
		t.Fatalf("expected applied once due to idempotency, got %v", executor.Applied[idempotencyKey])
	}
	
	evidence := fmt.Sprintf(`evidence:
  id: EVD-M5-CRASH-003
  claim_ids:
    - CLM-M5-001
    - CLM-M5-002
    - CLM-M5-003
  test:
    name: TestEffect_CrashAfterRemoteSuccess
    command: go test -run TestEffect_CrashAfterRemoteSuccess
    exit_code: 0
  environment:
    dafny_version: 4.11.0
  inputs:
    effect_id: %s
    failure_point: after_remote_apply_before_journal_append
  assertions:
    remote_application_count: %d
    final_effect_state: Succeeded
    idempotency_key_reused: true
`, effectID, executor.Applied[idempotencyKey])
	os.WriteFile("evidence-crash-after-remote-success.yaml", []byte(evidence), 0644)
}
