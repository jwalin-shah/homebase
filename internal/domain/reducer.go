package domain

import (
	"crypto/sha256"
	"fmt"
)

// Decide is the pure transition function for an attempt.
func Decide(state AttemptState, command Command) Decision {
	switch c := command.(type) {
	case CommandProposeRecovery:
		return handleProposeRecovery(state, c)
	case CommandConclude:
		return handleConclude(state, c)
	default:
		return Decision{}
	}
}

func handleProposeRecovery(state AttemptState, c CommandProposeRecovery) Decision {
	if state.Phase == AttemptConcluded {
		return Decision{
			Events: []Event{
				EventRecoveryRejected{
					AttemptID: c.AttemptID,
					Reason:    "attempt already concluded",
				},
			},
		}
	}

	if state.RecoveryDispatches >= 2 {
		return Decision{
			Events: []Event{
				EventRecoveryRejected{
					AttemptID: c.AttemptID,
					Reason:    "recovery budget exhausted",
				},
			},
		}
	}

	ordinal := state.RecoveryDispatches + 1
	effectID := generateEffectID(c.AttemptID, ordinal)

	// Idempotency: if effect already dispatched, no-op
	if state.DispatchedEffectIDs != nil {
		if _, exists := state.DispatchedEffectIDs[effectID]; exists {
			return Decision{}
		}
	}

	effect := EffectIntent{
		AttemptID: c.AttemptID,
		EffectID:  effectID,
		Ordinal:   ordinal,
	}

	event := EventRecoveryDispatched{
		AttemptID: c.AttemptID,
		EffectID:  effectID,
		Ordinal:   ordinal,
	}

	return Decision{
		Events:  []Event{event},
		Effects: []EffectIntent{effect},
	}
}

func handleConclude(state AttemptState, c CommandConclude) Decision {
	if state.Phase == AttemptConcluded {
		return Decision{}
	}
	return Decision{
		Events: []Event{
			EventConcluded{AttemptID: c.AttemptID},
		},
	}
}

func generateEffectID(attemptID AttemptID, ordinal uint8) EffectID {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-recovery-%d", attemptID, ordinal)))
	return EffectID(fmt.Sprintf("%x", hash)[:16])
}

// Apply evolves the state by consuming an event.
func Apply(state AttemptState, event Event) AttemptState {
	switch e := event.(type) {
	case EventRecoveryDispatched:
		state.Phase = AttemptRecovering
		state.RecoveryDispatches++
		if state.DispatchedEffectIDs == nil {
			state.DispatchedEffectIDs = make(map[EffectID]struct{})
		}
		state.DispatchedEffectIDs[e.EffectID] = struct{}{}
	case EventRecoveryRejected:
		// State does not change
	case EventConcluded:
		state.Phase = AttemptConcluded
	}
	return state
}
