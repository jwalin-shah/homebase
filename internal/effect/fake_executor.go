package effect

import (
	"context"
	"sync"
	"homebase/internal/domain"
)

type PlannedOutcome struct {
	Action string // "Success", "RetryableFailure", "TerminalFailure", "UnknownAfterApply", "CrashBeforeApply"
	Reason string
}

type FakeExecutor struct {
	mu       sync.Mutex
	Outcomes map[domain.EffectID][]PlannedOutcome
	Applied  map[string]int
}

func NewFakeExecutor() *FakeExecutor {
	return &FakeExecutor{
		Outcomes: make(map[domain.EffectID][]PlannedOutcome),
		Applied:  make(map[string]int),
	}
}

// PushOutcome queues an outcome for an effect.
func (e *FakeExecutor) PushOutcome(effectID domain.EffectID, outcome PlannedOutcome) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Outcomes[effectID] = append(e.Outcomes[effectID], outcome)
}

// Execute simulates an external operation based on queued outcomes.
func (e *FakeExecutor) Execute(ctx context.Context, effectID domain.EffectID, idempotencyKey string) (domain.EffectOutcome, string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	outcomes := e.Outcomes[effectID]
	if len(outcomes) == 0 {
		e.Applied[idempotencyKey]++
		return domain.OutcomeSucceeded, ""
	}

	next := outcomes[0]
	e.Outcomes[effectID] = outcomes[1:]

	switch next.Action {
	case "Success":
		if e.Applied[idempotencyKey] == 0 {
			e.Applied[idempotencyKey]++
		}
		return domain.OutcomeSucceeded, ""
	case "RetryableFailure":
		return domain.OutcomeFailedRetryable, next.Reason
	case "TerminalFailure":
		return domain.OutcomeFailedTerminal, next.Reason
	case "UnknownAfterApply":
		if e.Applied[idempotencyKey] == 0 {
			e.Applied[idempotencyKey]++
		}
		return domain.OutcomeUnknown, next.Reason
	case "CrashBeforeApply":
		// Simulates crash before actually doing the work
		panic("CrashBeforeApply simulated panic")
	default:
		return domain.OutcomeFailedTerminal, "invalid_planned_outcome"
	}
}
