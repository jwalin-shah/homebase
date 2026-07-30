package domain

import (
	dafny_reducer "homebase/internal/dafny_reducer"
)

// Decide is the pure transition function for an attempt.
func Decide(state AttemptState, command Command) Decision {
	// Delegate entire semantic slice to Dafny
	dafnyState := toDafnyState(state)
	dafnyCmd := toDafnyCommand(command)

	dafnyDecision := dafny_reducer.Companion_Default___.Decide(dafnyState, dafnyCmd)
	return fromDafnyDecision(dafnyDecision)
}

// Apply produces a new state based on an event.
func Apply(state AttemptState, event Event) AttemptState {
	// Delegate entire semantic slice to Dafny
	dafnyState := toDafnyState(state)
	dafnyEvent := toDafnyEvent(event)

	dafnyNewState := dafny_reducer.Companion_Default___.Apply(dafnyState, dafnyEvent)
	return fromDafnyState(dafnyNewState, state.ID, state.Version)
}
