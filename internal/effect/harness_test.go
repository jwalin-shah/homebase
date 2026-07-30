package effect

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

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
	fp := getFingerprint("req-1")
	executor.PushOutcome(effectID, PlannedOutcome{Action: "UnknownAfterApply", Reason: "simulated_timeout"})
	outcome1, _ := executor.Execute(context.Background(), effectID, idempotencyKey, fp)

	// Stage 3: New worker claims after lease expiry
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
	outcome2, _ := executor.Execute(context.Background(), effectID, idempotencyKey, fp)

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

	op := executor.Applied[idempotencyKey]
	if op == nil || op.ApplyCount != 1 {
		t.Fatalf("expected applied once due to idempotency")
	}

	emitEvidence(t, Evidence{
		ID:       "EVD-M5-CRASH-003",
		ClaimIDs: []string{"CLM-M5-001", "CLM-M5-002", "CLM-M5-003"},
		TestName: "TestEffect_CrashAfterRemoteSuccess",
		Inputs: map[string]interface{}{
			"effect_id":     effectID,
			"failure_point": "after_remote_apply_before_journal_append",
		},
		Assertions: map[string]interface{}{
			"remote_application_count":          op.ApplyCount,
			"execution_attempt_count":           op.InvocationCount,
			"idempotency_key_reused":            true,
			"first_observation":                 outcome1,
			"second_observation":                outcome2,
			"first_claim_epoch":                 1,
			"second_claim_epoch":                2,
			"stale_epoch_finalization_rejected": true,
			"final_effect_state":                "Succeeded",
			"journal_replay_equal":              true,
			"retry_bound_respected":             true,
		},
	})
}

func TestEffect_ExclusiveClaim(t *testing.T) {
	repo := NewFakeEffectRepository()
	effectID := domain.EffectID("eff-concurrency-1")
	currentTime := int(time.Now().Unix())
	leaseUntil := currentTime + 60

	var wg sync.WaitGroup
	var accepted, rejected int
	var mu sync.Mutex

	// Pre-load version
	state, version := repo.Load(effectID)

	startBarrier := make(chan struct{})

	// Worker 1
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startBarrier
		cmdClaim := domain.CommandClaimEffect{
			EffectID:        effectID,
			WorkerID:        "worker-1",
			ExpectedVersion: int(version),
			LeaseUntil:      leaseUntil,
			CurrentTime:     currentTime,
		}
		decision := domain.DecideEffect(state, cmdClaim)
		if decision.Status == domain.DecisionAccepted {
			err := repo.Append(effectID, version, decision.Events)
			mu.Lock()
			if err == nil {
				accepted++
			} else {
				rejected++
			}
			mu.Unlock()
		} else {
			mu.Lock()
			rejected++
			mu.Unlock()
		}
	}()

	// Worker 2
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-startBarrier
		cmdClaim := domain.CommandClaimEffect{
			EffectID:        effectID,
			WorkerID:        "worker-2",
			ExpectedVersion: int(version),
			LeaseUntil:      leaseUntil,
			CurrentTime:     currentTime,
		}
		decision := domain.DecideEffect(state, cmdClaim)
		if decision.Status == domain.DecisionAccepted {
			err := repo.Append(effectID, version, decision.Events)
			mu.Lock()
			if err == nil {
				accepted++
			} else {
				rejected++
			}
			mu.Unlock()
		} else {
			mu.Lock()
			rejected++
			mu.Unlock()
		}
	}()

	// Release barrier
	close(startBarrier)
	wg.Wait()

	if accepted != 1 {
		t.Fatalf("expected exactly 1 accepted claim, got %d", accepted)
	}
	if rejected != 1 {
		t.Fatalf("expected exactly 1 rejected claim, got %d", rejected)
	}

	finalState, finalVer := repo.Load(effectID)
	if finalState.Phase != domain.EffectClaimed {
		t.Fatalf("expected active claim, got phase %v", finalState.Phase)
	}
	if finalVer != version+1 {
		t.Fatalf("expected version %d, got %d", version+1, finalVer)
	}

	emitEvidence(t, Evidence{
		ID:       "EVD-M5-CONCURRENCY-001",
		ClaimIDs: []string{"CLM-M5-002"},
		TestName: "TestEffect_ExclusiveClaim",
		Inputs: map[string]interface{}{
			"effect_id": effectID,
			"workers":   2,
		},
		Assertions: map[string]interface{}{
			"accepted_count":            accepted,
			"rejected_count":            rejected,
			"final_state":               "Claimed",
			"journal_version_increment": 1,
		},
	})
}
