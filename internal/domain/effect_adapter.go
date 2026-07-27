package domain

import (
	"homebase/internal/dafny_reducer"
	"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
)

// EffectPhase defines the lifecycle states of an Effect.
type EffectPhase uint8

const (
	EffectPending EffectPhase = iota
	EffectClaimed
	EffectSucceeded
	EffectFailedRetryable
	EffectFailedTerminal
	EffectOutcomeUnknown
)

// EffectState is the persistent state of an effect intent.
type EffectState struct {
	EffectID   EffectID
	AttemptID  AttemptID
	Phase      EffectPhase
	WorkerID   string
	ClaimEpoch int
	LeaseUntil int
	Retries    int
}

// EffectOutcome is the result of an external observation.
type EffectOutcome uint8

const (
	OutcomeSucceeded EffectOutcome = iota
	OutcomeFailedRetryable
	OutcomeFailedTerminal
	OutcomeUnknown
)

func toDafnyEffectState(state EffectState) dafny_reducer.EffectState {
	var phase dafny_reducer.EffectPhase
	switch state.Phase {
	case EffectPending:
		phase = dafny_reducer.Companion_EffectPhase_.Create_Pending_()
	case EffectClaimed:
		phase = dafny_reducer.Companion_EffectPhase_.Create_Claimed_()
	case EffectSucceeded:
		phase = dafny_reducer.Companion_EffectPhase_.Create_SucceededPhase_()
	case EffectFailedRetryable:
		phase = dafny_reducer.Companion_EffectPhase_.Create_FailedRetryablePhase_()
	case EffectFailedTerminal:
		phase = dafny_reducer.Companion_EffectPhase_.Create_FailedTerminalPhase_()
	case EffectOutcomeUnknown:
		phase = dafny_reducer.Companion_EffectPhase_.Create_OutcomeUnknownPhase_()
	}

	return dafny_reducer.Companion_EffectState_.Create_EffectState_(
		dafny.SeqOfString(string(state.EffectID)),
		dafny.SeqOfString(state.AttemptID.String()),
		phase,
		dafny.SeqOfString(state.WorkerID),
		dafny.IntOfInt64(int64(state.ClaimEpoch)),
		dafny.IntOfInt64(int64(state.LeaseUntil)),
		dafny.IntOfInt64(int64(state.Retries)),
	)
}

func fromDafnyEffectState(dafnyState dafny_reducer.EffectState) EffectState {
	var phase EffectPhase
	if dafnyState.Dtor_phase().Is_Pending() {
		phase = EffectPending
	} else if dafnyState.Dtor_phase().Is_Claimed() {
		phase = EffectClaimed
	} else if dafnyState.Dtor_phase().Is_SucceededPhase() {
		phase = EffectSucceeded
	} else if dafnyState.Dtor_phase().Is_FailedRetryablePhase() {
		phase = EffectFailedRetryable
	} else if dafnyState.Dtor_phase().Is_FailedTerminalPhase() {
		phase = EffectFailedTerminal
	} else if dafnyState.Dtor_phase().Is_OutcomeUnknownPhase() {
		phase = EffectOutcomeUnknown
	}

	aid, _ := ParseAttemptID(dafny.String(dafnyState.Dtor_attemptID()))

	return EffectState{
		EffectID:   EffectID(dafny.String(dafnyState.Dtor_effectID())),
		AttemptID:  aid,
		Phase:      phase,
		WorkerID:   dafny.String(dafnyState.Dtor_workerID()),
		ClaimEpoch: int(dafnyState.Dtor_claimEpoch().Int64()),
		LeaseUntil: int(dafnyState.Dtor_leaseUntil().Int64()),
		Retries:    int(dafnyState.Dtor_retries().Int64()),
	}
}
