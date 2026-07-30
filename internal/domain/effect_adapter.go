package domain

import (
	"github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
	"homebase/internal/dafny_effect_reducer"
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
	EffectReconciliationRequired
	EffectManualResolutionRequired
)

type CapabilityDescriptor struct {
	Idempotency      IdempotencyCapability
	ResultLookup     ResultLookupCapability
	Transactionality TransactionCapability
	UnknownOutcome   UnknownOutcomeCapability
	Status           VerificationStatus
}

type IdempotencyCapability uint8

const (
	IdempotencyNone IdempotencyCapability = iota
	IdempotencyStableKey
	IdempotencyStableKeyWithConflictDetection
)

type ResultLookupCapability uint8

const (
	ResultLookupNone ResultLookupCapability = iota
	ResultLookupByExternalReference
	ResultLookupByIdempotencyKey
)

type TransactionCapability uint8

const (
	TransactionNone TransactionCapability = iota
	TransactionSingleOperation
	TransactionAtomicBatch
)

type UnknownOutcomeCapability uint8

const (
	UnknownRequiresManualResolution UnknownOutcomeCapability = iota
	UnknownCanReconcile
	UnknownCanSafelyRetry
)

type VerificationStatus uint8

const (
	StatusProposed VerificationStatus = iota
	StatusVerified
	StatusInvalidated
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

func toDafnyEffectState(state EffectState) dafny_effect_reducer.EffectState {
	var phase dafny_effect_reducer.EffectPhase
	switch state.Phase {
	case EffectPending:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_Pending_()
	case EffectClaimed:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_Claimed_()
	case EffectSucceeded:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_SucceededPhase_()
	case EffectFailedRetryable:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_FailedRetryablePhase_()
	case EffectFailedTerminal:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_FailedTerminalPhase_()
	case EffectOutcomeUnknown:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_OutcomeUnknownPhase_()
	case EffectReconciliationRequired:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_ReconciliationRequiredPhase_()
	case EffectManualResolutionRequired:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_ManualResolutionRequiredPhase_()
	default:
		phase = dafny_effect_reducer.Companion_EffectPhase_.Create_Pending_()
	}

	return dafny_effect_reducer.Companion_EffectState_.Create_EffectState_(
		dafny.SeqOfChars([]dafny.Char(state.EffectID)...),
		dafny.SeqOfChars([]dafny.Char(state.AttemptID.String())...),
		phase,
		dafny.SeqOfChars([]dafny.Char(state.WorkerID)...),
		dafny.IntOfInt64(int64(state.ClaimEpoch)),
		dafny.IntOfInt64(int64(state.LeaseUntil)),
		dafny.IntOfInt64(int64(state.Retries)),
	)
}

func fromDafnyEffectPhase(phase dafny_effect_reducer.EffectPhase) EffectPhase {
	if phase.Is_Pending() {
		return EffectPending
	} else if phase.Is_Claimed() {
		return EffectClaimed
	} else if phase.Is_SucceededPhase() {
		return EffectSucceeded
	} else if phase.Is_FailedRetryablePhase() {
		return EffectFailedRetryable
	} else if phase.Is_FailedTerminalPhase() {
		return EffectFailedTerminal
	} else if phase.Is_OutcomeUnknownPhase() {
		return EffectOutcomeUnknown
	} else if phase.Is_ReconciliationRequiredPhase() {
		return EffectReconciliationRequired
	} else if phase.Is_ManualResolutionRequiredPhase() {
		return EffectManualResolutionRequired
	}
	return EffectPending
}

func fromDafnyEffectState(dafnyState dafny_effect_reducer.EffectState) EffectState {
	phase := fromDafnyEffectPhase(dafnyState.Dtor_phase())
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

// EffectCommand defines the commands for the Effect lifecycle.
type EffectCommand interface {
	isEffectCommand()
}

type CommandClaimEffect struct {
	EffectID        EffectID
	WorkerID        string
	ExpectedVersion int
	LeaseUntil      int
	CurrentTime     int
}

func (CommandClaimEffect) isEffectCommand() {}

type CommandObserveEffect struct {
	EffectID   EffectID
	WorkerID   string
	ClaimEpoch int
	Outcome    EffectOutcome
}

func (CommandObserveEffect) isEffectCommand() {}

type CommandRetryEffect struct {
	EffectID EffectID
}

func (CommandRetryEffect) isEffectCommand() {}

type CommandResolveUnknown struct {
	EffectID     EffectID
	Capabilities CapabilityDescriptor
}

func (CommandResolveUnknown) isEffectCommand() {}

func toDafnyEffectCommand(cmd EffectCommand) dafny_effect_reducer.Command {
	switch c := cmd.(type) {
	case CommandClaimEffect:
		return dafny_effect_reducer.Companion_Command_.Create_CommandClaimEffect_(
			dafny.SeqOfString(string(c.EffectID)),
			dafny.SeqOfString(c.WorkerID),
			dafny.IntOfInt64(int64(c.ExpectedVersion)),
			dafny.IntOfInt64(int64(c.LeaseUntil)),
			dafny.IntOfInt64(int64(c.CurrentTime)),
		)
	case CommandObserveEffect:
		var outcome dafny_effect_reducer.Outcome
		switch c.Outcome {
		case OutcomeSucceeded:
			outcome = dafny_effect_reducer.Companion_Outcome_.Create_Succeeded_()
		case OutcomeFailedRetryable:
			outcome = dafny_effect_reducer.Companion_Outcome_.Create_FailedRetryable_()
		case OutcomeFailedTerminal:
			outcome = dafny_effect_reducer.Companion_Outcome_.Create_FailedTerminal_()
		case OutcomeUnknown:
			outcome = dafny_effect_reducer.Companion_Outcome_.Create_OutcomeUnknown_()
		}
		return dafny_effect_reducer.Companion_Command_.Create_CommandObserveEffect_(
			dafny.SeqOfString(string(c.EffectID)),
			dafny.SeqOfString(c.WorkerID),
			dafny.IntOfInt64(int64(c.ClaimEpoch)),
			outcome,
		)
	case CommandRetryEffect:
		return dafny_effect_reducer.Companion_Command_.Create_CommandRetryEffect_(
			dafny.SeqOfString(string(c.EffectID)),
		)
	case CommandResolveUnknown:
		return dafny_effect_reducer.Companion_Command_.Create_CommandResolveUnknown_(
			dafny.SeqOfString(string(c.EffectID)),
			toDafnyCapabilityDescriptor(c.Capabilities),
		)
	default:
		panic("unknown effect command")
	}
}

func toDafnyCapabilityDescriptor(caps CapabilityDescriptor) dafny_effect_reducer.CapabilityDescriptor {
	var idempotency dafny_effect_reducer.IdempotencyCapability
	switch caps.Idempotency {
	case IdempotencyNone:
		idempotency = dafny_effect_reducer.Companion_IdempotencyCapability_.Create_IdempotencyNone_()
	case IdempotencyStableKey:
		idempotency = dafny_effect_reducer.Companion_IdempotencyCapability_.Create_StableKey_()
	case IdempotencyStableKeyWithConflictDetection:
		idempotency = dafny_effect_reducer.Companion_IdempotencyCapability_.Create_StableKeyWithConflictDetection_()
	}

	var resultLookup dafny_effect_reducer.ResultLookupCapability
	switch caps.ResultLookup {
	case ResultLookupNone:
		resultLookup = dafny_effect_reducer.Companion_ResultLookupCapability_.Create_ResultLookupNone_()
	case ResultLookupByExternalReference:
		resultLookup = dafny_effect_reducer.Companion_ResultLookupCapability_.Create_ByExternalReference_()
	case ResultLookupByIdempotencyKey:
		resultLookup = dafny_effect_reducer.Companion_ResultLookupCapability_.Create_ByIdempotencyKey_()
	}

	var transactionality dafny_effect_reducer.TransactionCapability
	switch caps.Transactionality {
	case TransactionNone:
		transactionality = dafny_effect_reducer.Companion_TransactionCapability_.Create_TransactionNone_()
	case TransactionSingleOperation:
		transactionality = dafny_effect_reducer.Companion_TransactionCapability_.Create_SingleOperation_()
	case TransactionAtomicBatch:
		transactionality = dafny_effect_reducer.Companion_TransactionCapability_.Create_AtomicBatch_()
	}

	var unknownOutcome dafny_effect_reducer.UnknownOutcomeCapability
	switch caps.UnknownOutcome {
	case UnknownRequiresManualResolution:
		unknownOutcome = dafny_effect_reducer.Companion_UnknownOutcomeCapability_.Create_RequiresManualResolution_()
	case UnknownCanReconcile:
		unknownOutcome = dafny_effect_reducer.Companion_UnknownOutcomeCapability_.Create_CanReconcile_()
	case UnknownCanSafelyRetry:
		unknownOutcome = dafny_effect_reducer.Companion_UnknownOutcomeCapability_.Create_CanSafelyRetry_()
	}

	var status dafny_effect_reducer.VerificationStatus
	switch caps.Status {
	case StatusProposed:
		status = dafny_effect_reducer.Companion_VerificationStatus_.Create_Proposed_()
	case StatusVerified:
		status = dafny_effect_reducer.Companion_VerificationStatus_.Create_Verified_()
	case StatusInvalidated:
		status = dafny_effect_reducer.Companion_VerificationStatus_.Create_Invalidated_()
	}

	return dafny_effect_reducer.Companion_CapabilityDescriptor_.Create_CapabilityDescriptor_(
		idempotency,
		resultLookup,
		transactionality,
		unknownOutcome,
		status,
	)
}

// EffectEvent defines the events for the Effect lifecycle.
type EffectEvent interface {
	isEffectEvent()
}

type EventEffectClaimed struct {
	EffectID   EffectID
	WorkerID   string
	ClaimEpoch int
	LeaseUntil int
}

func (EventEffectClaimed) isEffectEvent() {}

type EventEffectObserved struct {
	EffectID   EffectID
	ClaimEpoch int
	Outcome    EffectOutcome
}

func (EventEffectObserved) isEffectEvent() {}

type EventEffectRetried struct {
	EffectID EffectID
}

func (EventEffectRetried) isEffectEvent() {}

type EventEffectTerminalized struct {
	EffectID EffectID
	Reason   string
}

func (EventEffectTerminalized) isEffectEvent() {}

type EventManualResolutionRequired struct {
	EffectID EffectID
}

func (EventManualResolutionRequired) isEffectEvent() {}

type EventReconciliationRequired struct {
	EffectID EffectID
}

func (EventReconciliationRequired) isEffectEvent() {}

type EventRetryAuthorized struct {
	EffectID EffectID
}

func (EventRetryAuthorized) isEffectEvent() {}

type EventEffectRejected struct {
	EffectID EffectID
	Reason   string
}

func (EventEffectRejected) isEffectEvent() {}

func fromDafnyEffectEvent(dafnyEvent dafny_effect_reducer.Event) EffectEvent {
	if dafnyEvent.Is_EventEffectClaimed() {
		return EventEffectClaimed{
			EffectID:   EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
			WorkerID:   dafny.String(dafnyEvent.Dtor_workerID()),
			ClaimEpoch: int(dafnyEvent.Dtor_claimEpoch().Int64()),
			LeaseUntil: int(dafnyEvent.Dtor_leaseUntil().Int64()),
		}
	} else if dafnyEvent.Is_EventEffectObserved() {
		var outcome EffectOutcome
		if dafnyEvent.Dtor_outcome().Is_Succeeded() {
			outcome = OutcomeSucceeded
		} else if dafnyEvent.Dtor_outcome().Is_FailedRetryable() {
			outcome = OutcomeFailedRetryable
		} else if dafnyEvent.Dtor_outcome().Is_FailedTerminal() {
			outcome = OutcomeFailedTerminal
		} else {
			outcome = OutcomeUnknown
		}
		return EventEffectObserved{
			EffectID:   EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
			ClaimEpoch: int(dafnyEvent.Dtor_claimEpoch().Int64()),
			Outcome:    outcome,
		}
	} else if dafnyEvent.Is_EventEffectRetried() {
		return EventEffectRetried{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
		}
	} else if dafnyEvent.Is_EventEffectTerminalized() {
		return EventEffectTerminalized{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
			Reason:   string(dafny.String(dafnyEvent.Dtor_reason())),
		}
	} else if dafnyEvent.Is_EventEffectRejected() {
		return EventEffectRejected{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
			Reason:   string(dafny.String(dafnyEvent.Dtor_reason())),
		}
	} else if dafnyEvent.Is_EventManualResolutionRequired() {
		return EventManualResolutionRequired{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
		}
	} else if dafnyEvent.Is_EventReconciliationRequired() {
		return EventReconciliationRequired{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
		}
	} else if dafnyEvent.Is_EventRetryAuthorized() {
		return EventRetryAuthorized{
			EffectID: EffectID(dafny.String(dafnyEvent.Dtor_effectID())),
		}
	}
	panic("unknown effect event")
}

// EffectDecision is the result of DecideEffect.
type EffectDecision struct {
	Status DecisionStatus
	Events []EffectEvent
}

func DecideEffect(state EffectState, cmd EffectCommand) EffectDecision {
	dafnyState := toDafnyEffectState(state)
	dafnyCmd := toDafnyEffectCommand(cmd)

	dafnyDecision := dafny_effect_reducer.Companion_Default___.Decide(dafnyState, dafnyCmd)

	var status DecisionStatus
	if dafnyDecision.Dtor_status().Is_Accepted() {
		status = DecisionAccepted
	} else if dafnyDecision.Dtor_status().Is_Rejected() {
		status = DecisionRejected
	} else {
		status = DecisionNoOp
	}

	var events []EffectEvent
	seqEvents := dafnyDecision.Dtor_events()
	for i := uint32(0); i < seqEvents.Cardinality(); i++ {
		e := seqEvents.Select(i).(dafny_effect_reducer.Event)
		events = append(events, fromDafnyEffectEvent(e))
	}

	return EffectDecision{
		Status: status,
		Events: events,
	}
}

func ApplyEffect(state EffectState, event EffectEvent) EffectState {
	// Purity: apply locally to avoid translating back and forth
	return applyEffectGo(state, event)
}

func applyEffectGo(state EffectState, event EffectEvent) EffectState {
	switch e := event.(type) {
	case EventEffectClaimed:
		state.Phase = EffectClaimed
		state.WorkerID = e.WorkerID
		state.ClaimEpoch = e.ClaimEpoch
		state.LeaseUntil = e.LeaseUntil
		state.Retries++
	case EventEffectObserved:
		switch e.Outcome {
		case OutcomeSucceeded:
			state.Phase = EffectSucceeded
		case OutcomeFailedRetryable:
			state.Phase = EffectFailedRetryable
		case OutcomeFailedTerminal:
			state.Phase = EffectFailedTerminal
		case OutcomeUnknown:
			state.Phase = EffectOutcomeUnknown
		}
	case EventEffectRetried:
		state.Phase = EffectPending
		state.WorkerID = ""
		state.LeaseUntil = 0
	case EventEffectTerminalized:
		state.Phase = EffectFailedTerminal
	case EventEffectRejected:
		// no-op
	case EventManualResolutionRequired:
		state.Phase = EffectManualResolutionRequired
	case EventReconciliationRequired:
		state.Phase = EffectReconciliationRequired
	case EventRetryAuthorized:
		state.Phase = EffectPending
		state.WorkerID = ""
		state.LeaseUntil = 0
	}
	return state
}
