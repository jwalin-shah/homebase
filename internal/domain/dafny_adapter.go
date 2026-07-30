package domain

import (
	"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
	dafny_reducer "homebase/internal/dafny_reducer"
)

// toDafnyState converts Go AttemptState to Dafny AttemptState
func toDafnyState(state AttemptState) dafny_reducer.AttemptState {
	// Convert Phase
	var phase dafny_reducer.Phase
	switch state.Phase {
	case AttemptActive:
		phase = dafny_reducer.Companion_Phase_.Create_Active_()
	case AttemptRecovering:
		phase = dafny_reducer.Companion_Phase_.Create_Recovering_()
	case AttemptConcluded:
		phase = dafny_reducer.Companion_Phase_.Create_Concluded_()
	}

	// Convert DispatchedEffects
	var effectsSet []interface{}
	for k := range state.DispatchedEffectIDs {
		effectsSet = append(effectsSet, dafny.SeqOfString(string(k)))
	}

	// Convert ProcessedCmdKeys
	var keysSet []interface{}
	for k := range state.ProcessedCmdKeys {
		keysSet = append(keysSet, dafny.SeqOfString(k))
	}

	return dafny_reducer.Companion_AttemptState_.Create_AttemptState_(
		phase,
		dafny.IntOfInt64(int64(state.RecoveryDispatches)),
		dafny.SetOf(effectsSet...),
		dafny.SetOf(keysSet...),
		dafny.IntOfInt64(int64(state.Version)),
	)
}

// fromDafnyState converts Dafny AttemptState back to Go AttemptState
func fromDafnyState(dafnyState dafny_reducer.AttemptState, originalID AttemptID, originalVersion uint64) AttemptState {
	var phase AttemptPhase
	if dafnyState.Dtor_phase().Is_Active() {
		phase = AttemptActive
	} else if dafnyState.Dtor_phase().Is_Recovering() {
		phase = AttemptRecovering
	} else if dafnyState.Dtor_phase().Is_Concluded() {
		phase = AttemptConcluded
	}

	dispatchedEffects := make(map[EffectID]struct{})
	iterE := dafnyState.Dtor_dispatchedEffects().Iterator()
	for {
		e, ok := iterE()
		if !ok {
			break
		}
		seq := e.(dafny.Sequence)
		dispatchedEffects[EffectID(dafny.String(seq))] = struct{}{}
	}

	processedKeys := make(map[string]struct{})
	iterK := dafnyState.Dtor_processedCmdKeys().Iterator()
	for {
		k, ok := iterK()
		if !ok {
			break
		}
		seq := k.(dafny.Sequence)
		processedKeys[dafny.String(seq)] = struct{}{}
	}

	return AttemptState{
		ID:                  originalID,
		Version:             originalVersion,
		Phase:               phase,
		RecoveryDispatches:  uint8(dafnyState.Dtor_recoveryDispatches().Int64()),
		DispatchedEffectIDs: dispatchedEffects,
		ProcessedCmdKeys:    processedKeys,
	}
}

// toDafnyCommand converts Go Command to Dafny Command
func toDafnyCommand(cmd Command) dafny_reducer.Command {
	switch c := cmd.(type) {
	case CommandProposeRecovery:
		return dafny_reducer.Companion_Command_.Create_CommandProposeRecovery_(
			dafny.SeqOfString(c.AttemptID.String()),
			dafny.SeqOfString(c.IdempotencyKey),
			dafny.IntOfInt64(int64(c.Version)),
		)
	case CommandConclude:
		return dafny_reducer.Companion_Command_.Create_CommandConclude_(
			dafny.SeqOfString(c.AttemptID.String()),
		)
	default:
		// Convert any unknown command to Dafny CommandUnknown
		return dafny_reducer.Companion_Command_.Create_CommandUnknown_(
			dafny.SeqOfString("unknown"),
		)
	}
}

// fromDafnyEvent converts Dafny Event to Go Event
func fromDafnyEvent(dafnyEvent dafny_reducer.Event) Event {
	if dafnyEvent.Is_EventRecoveryDispatched() {
		aid, _ := ParseAttemptID(dafny.String(dafnyEvent.Dtor_attemptID()))
		return EventRecoveryDispatched{
			AttemptID:      aid,
			EffectID:       EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
			Ordinal:        uint8(dafnyEvent.Dtor_ordinal().Int64()),
			IdempotencyKey: dafny.String(dafnyEvent.Dtor_idempotencyKey()),
		}
	} else if dafnyEvent.Is_EventRecoveryRejected() {
		aid, _ := ParseAttemptID(dafny.String(dafnyEvent.Dtor_attemptID()))
		return EventRecoveryRejected{
			AttemptID: aid,
			Reason:    dafny.String(dafnyEvent.Dtor_reason()),
		}
	} else if dafnyEvent.Is_EventConcluded() {
		aid, _ := ParseAttemptID(dafny.String(dafnyEvent.Dtor_attemptID()))
		return EventConcluded{
			AttemptID: aid,
		}
	}
	panic("unknown dafny event")
}

// fromDafnyDecision converts Dafny Decision to Go Decision
func fromDafnyDecision(dafnyDecision dafny_reducer.Decision) Decision {
	var status DecisionStatus
	if dafnyDecision.Dtor_status().Is_Accepted() {
		status = DecisionAccepted
	} else if dafnyDecision.Dtor_status().Is_Rejected() {
		status = DecisionRejected
	} else if dafnyDecision.Dtor_status().Is_NoOp() {
		status = DecisionNoOp
	}

	var events []Event
	seqEvents := dafnyDecision.Dtor_events()
	for i := uint32(0); i < seqEvents.Cardinality(); i++ {
		e := seqEvents.Select(i)
		events = append(events, fromDafnyEvent(e.(dafny_reducer.Event)))
	}

	var effects []EffectIntent
	seqEffects := dafnyDecision.Dtor_effects()
	for i := uint32(0); i < seqEffects.Cardinality(); i++ {
		ef := seqEffects.Select(i).(dafny_reducer.EffectIntent)
		aid, _ := ParseAttemptID(dafny.String(ef.Dtor_attemptID()))
		effects = append(effects, EffectIntent{
			AttemptID: aid,
			EffectID:  EffectID(dafny.String(ef.Dtor_effectID())),
			Ordinal:   uint8(ef.Dtor_ordinal().Int64()),
		})
	}

	return Decision{
		Status:  status,
		Events:  events,
		Effects: effects,
	}
}

// toDafnyEvent converts Go Event to Dafny Event
func toDafnyEvent(e Event) dafny_reducer.Event {
	switch ev := e.(type) {
	case EventRecoveryDispatched:
		return dafny_reducer.Companion_Event_.Create_EventRecoveryDispatched_(
			dafny.SeqOfString(ev.AttemptID.String()),
			dafny.SeqOfString(string(ev.EffectID)),
			dafny.IntOfInt64(int64(ev.Ordinal)),
			dafny.SeqOfString(ev.IdempotencyKey),
		)
	case EventRecoveryRejected:
		return dafny_reducer.Companion_Event_.Create_EventRecoveryRejected_(
			dafny.SeqOfString(ev.AttemptID.String()),
			dafny.SeqOfString(ev.Reason),
		)
	case EventConcluded:
		return dafny_reducer.Companion_Event_.Create_EventConcluded_(
			dafny.SeqOfString(ev.AttemptID.String()),
		)
	default:
		panic("unknown event")
	}
}
