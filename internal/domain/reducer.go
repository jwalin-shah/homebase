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
		return Decision{Status: DecisionRejected}
	}
}

func handleProposeRecovery(state AttemptState, c CommandProposeRecovery) Decision {
	// Stale version check
	if c.Version > 0 && c.Version < state.Version {
		return Decision{
			Status: DecisionRejected,
			Events: []Event{
				EventRecoveryRejected{
					AttemptID: c.AttemptID,
					Reason:    "stale command version",
				},
			},
		}
	}

	// Check if this specific command was already processed
	if c.IdempotencyKey != "" && state.ProcessedCmdKeys != nil {
		if _, exists := state.ProcessedCmdKeys[c.IdempotencyKey]; exists {
			return Decision{Status: DecisionNoOp}
		}
	}

	if state.Phase == AttemptConcluded {
		return Decision{
			Status: DecisionRejected,
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
			Status: DecisionRejected,
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

	// Idempotency: if effect already dispatched, no-op (sanity check)
	if state.DispatchedEffectIDs != nil {
		if _, exists := state.DispatchedEffectIDs[effectID]; exists {
			return Decision{Status: DecisionNoOp}
		}
	}

	effect := EffectIntent{
		AttemptID: c.AttemptID,
		EffectID:  effectID,
		Ordinal:   ordinal,
	}

	event := EventRecoveryDispatched{
		AttemptID:      c.AttemptID,
		EffectID:       effectID,
		Ordinal:        ordinal,
		IdempotencyKey: c.IdempotencyKey,
	}

	return Decision{
		Status:  DecisionAccepted,
		Events:  []Event{event},
		Effects: []EffectIntent{effect},
	}
}

func handleConclude(state AttemptState, c CommandConclude) Decision {
	if state.Phase == AttemptConcluded {
		return Decision{Status: DecisionNoOp}
	}
	return Decision{
		Status: DecisionAccepted,
		Events: []Event{
			EventConcluded{AttemptID: c.AttemptID},
		},
	}
}

func generateEffectID(attemptID AttemptID, ordinal uint8) EffectID {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-recovery-%d", attemptID.String(), ordinal)))
	return EffectID(fmt.Sprintf("%x", hash)[:16])
}

// Apply evolves the state by consuming an event.
// It returns a strictly new copy of the state.
func Apply(state AttemptState, event Event) AttemptState {
	newState := AttemptState{
		ID:                 state.ID,
		Version:            state.Version + 1,
		Phase:              state.Phase,
		RecoveryDispatches: state.RecoveryDispatches,
	}
	
	if state.DispatchedEffectIDs != nil {
		newState.DispatchedEffectIDs = make(map[EffectID]struct{}, len(state.DispatchedEffectIDs))
		for k, v := range state.DispatchedEffectIDs {
			newState.DispatchedEffectIDs[k] = v
		}
	}

	if state.ProcessedCmdKeys != nil {
		newState.ProcessedCmdKeys = make(map[string]struct{}, len(state.ProcessedCmdKeys))
		for k, v := range state.ProcessedCmdKeys {
			newState.ProcessedCmdKeys[k] = v
		}
	}

	switch e := event.(type) {
	case EventRecoveryDispatched:
		newState.Phase = AttemptRecovering
		newState.RecoveryDispatches++
		if newState.DispatchedEffectIDs == nil {
			newState.DispatchedEffectIDs = make(map[EffectID]struct{})
		}
		newState.DispatchedEffectIDs[e.EffectID] = struct{}{}
		
		if e.IdempotencyKey != "" {
			if newState.ProcessedCmdKeys == nil {
				newState.ProcessedCmdKeys = make(map[string]struct{})
			}
			newState.ProcessedCmdKeys[e.IdempotencyKey] = struct{}{}
		}
	case EventRecoveryRejected:
		// State does not change
	case EventConcluded:
		newState.Phase = AttemptConcluded
	}
	return newState
}
