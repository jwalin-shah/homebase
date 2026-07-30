package effect

import (
	"context"
	"homebase/internal/domain"
	"sync"
)

type PlannedOutcome struct {
	Action string // "Success", "RetryableFailure", "TerminalFailure", "UnknownAfterApply", "CrashBeforeApply"
	Reason string
}

type AppliedOperation struct {
	EffectID           domain.EffectID
	IdempotencyKey     string
	RequestFingerprint [32]byte
	ApplyCount         uint64
	InvocationCount    uint64
}

type FakeExecutor struct {
	mu       sync.Mutex
	Outcomes map[domain.EffectID][]PlannedOutcome
	Applied  map[string]*AppliedOperation
}

func NewFakeExecutor() *FakeExecutor {
	return &FakeExecutor{
		Outcomes: make(map[domain.EffectID][]PlannedOutcome),
		Applied:  make(map[string]*AppliedOperation),
	}
}

// PushOutcome queues an outcome for an effect.
func (e *FakeExecutor) PushOutcome(effectID domain.EffectID, outcome PlannedOutcome) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Outcomes[effectID] = append(e.Outcomes[effectID], outcome)
}

// Execute simulates an external operation based on queued outcomes.
func (e *FakeExecutor) Execute(ctx context.Context, effectID domain.EffectID, idempotencyKey string, fingerprint [32]byte) (domain.EffectOutcome, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check idempotency conflict
	if existing, ok := e.Applied[idempotencyKey]; ok {
		if existing.RequestFingerprint != fingerprint {
			return domain.OutcomeFailedTerminal, "IdempotencyConflict"
		}
		existing.InvocationCount++
	} else {
		e.Applied[idempotencyKey] = &AppliedOperation{
			EffectID:           effectID,
			IdempotencyKey:     idempotencyKey,
			RequestFingerprint: fingerprint,
			InvocationCount:    1,
		}
	}

	op := e.Applied[idempotencyKey]

	outcomes := e.Outcomes[effectID]
	if len(outcomes) == 0 {
		if op.ApplyCount == 0 {
			op.ApplyCount++
		}
		return domain.OutcomeSucceeded, ""
	}

	next := outcomes[0]
	e.Outcomes[effectID] = outcomes[1:]

	switch next.Action {
	case "Success":
		if op.ApplyCount == 0 {
			op.ApplyCount++
		}
		return domain.OutcomeSucceeded, ""
	case "RetryableFailure":
		return domain.OutcomeFailedRetryable, next.Reason
	case "TerminalFailure":
		return domain.OutcomeFailedTerminal, next.Reason
	case "UnknownAfterApply":
		if op.ApplyCount == 0 {
			op.ApplyCount++
		}
		return domain.OutcomeUnknown, next.Reason
	case "CrashBeforeApply":
		// Simulates crash before actually doing the work
		panic("CrashBeforeApply simulated panic")
	default:
		return domain.OutcomeFailedTerminal, "invalid_planned_outcome"
	}
}
