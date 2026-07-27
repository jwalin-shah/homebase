// Package EffectReducer
// Dafny module EffectReducer compiled into Go

package dafny_effect_reducer

import (
	m__System "github.com/dafny-lang/DafnyRuntimeGo/v4/System_"
	_dafny "github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
	os "os"
)

var _ = os.Args
var _ _dafny.Dummy__
var _ m__System.Dummy__

type Dummy__ struct{}

// Definition of class Default__
type Default__ struct {
	dummy byte
}

func New_Default___() *Default__ {
	_this := Default__{}

	return &_this
}

type CompanionStruct_Default___ struct {
}

var Companion_Default___ = CompanionStruct_Default___{}

func (_this *Default__) Equals(other *Default__) bool {
	return _this == other
}

func (_this *Default__) EqualsGeneric(x interface{}) bool {
	other, ok := x.(*Default__)
	return ok && _this.Equals(other)
}

func (*Default__) String() string {
	return "_module.Default__"
}
func (_this *Default__) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = &Default__{}

func (_static *CompanionStruct_Default___) Decide(state EffectState, cmd Command) Decision {
	var _source0 Command = cmd
	_ = _source0
	{
		if _source0.Is_CommandClaimEffect() {
			var _0_effectID _dafny.Sequence = _source0.Get_().(Command_CommandClaimEffect).EffectID
			_ = _0_effectID
			var _1_workerID _dafny.Sequence = _source0.Get_().(Command_CommandClaimEffect).WorkerID
			_ = _1_workerID
			var _2_expectedVersion _dafny.Int = _source0.Get_().(Command_CommandClaimEffect).ExpectedVersion
			_ = _2_expectedVersion
			var _3_leaseUntil _dafny.Int = _source0.Get_().(Command_CommandClaimEffect).LeaseUntil
			_ = _3_leaseUntil
			var _4_currentTime _dafny.Int = _source0.Get_().(Command_CommandClaimEffect).CurrentTime
			_ = _4_currentTime
			if (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_SucceededPhase_())) || (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_FailedTerminalPhase_())) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_0_effectID, _dafny.UnicodeSeqOfUtf8Bytes("effect already terminal"))))
			} else if (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_Claimed_())) && ((_4_currentTime).Cmp((state).Dtor_leaseUntil()) < 0) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_0_effectID, _dafny.UnicodeSeqOfUtf8Bytes("effect already claimed and unexpired"))))
			} else if ((state).Dtor_retries()).Cmp(_dafny.IntOfInt64(3)) >= 0 {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectTerminalized_(_0_effectID, _dafny.UnicodeSeqOfUtf8Bytes("retry limit reached"))))
			} else {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectClaimed_(_0_effectID, _1_workerID, ((state).Dtor_claimEpoch()).Plus(_dafny.One), _3_leaseUntil)))
			}
		}
	}
	{
		if _source0.Is_CommandObserveEffect() {
			var _5_effectID _dafny.Sequence = _source0.Get_().(Command_CommandObserveEffect).EffectID
			_ = _5_effectID
			var _6_workerID _dafny.Sequence = _source0.Get_().(Command_CommandObserveEffect).WorkerID
			_ = _6_workerID
			var _7_claimEpoch _dafny.Int = _source0.Get_().(Command_CommandObserveEffect).ClaimEpoch
			_ = _7_claimEpoch
			var _8_outcome Outcome = _source0.Get_().(Command_CommandObserveEffect).Outcome
			_ = _8_outcome
			if (_7_claimEpoch).Cmp((state).Dtor_claimEpoch()) < 0 {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_5_effectID, _dafny.UnicodeSeqOfUtf8Bytes("stale claim epoch"))))
			} else if !((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_Claimed_()) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_5_effectID, _dafny.UnicodeSeqOfUtf8Bytes("effect not claimed"))))
			} else {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectObserved_(_5_effectID, _7_claimEpoch, _8_outcome)))
			}
		}
	}
	{
		if _source0.Is_CommandRetryEffect() {
			var _9_effectID _dafny.Sequence = _source0.Get_().(Command_CommandRetryEffect).EffectID
			_ = _9_effectID
			if (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_SucceededPhase_())) || (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_FailedTerminalPhase_())) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_9_effectID, _dafny.UnicodeSeqOfUtf8Bytes("effect already terminal"))))
			} else if (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_FailedRetryablePhase_())) || (((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_OutcomeUnknownPhase_())) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRetried_(_9_effectID)))
			} else {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_9_effectID, _dafny.UnicodeSeqOfUtf8Bytes("not in retryable state"))))
			}
		}
	}
	{
		var _10_effectID _dafny.Sequence = _source0.Get_().(Command_CommandResolveUnknown).EffectID
		_ = _10_effectID
		var _11_caps CapabilityDescriptor = _source0.Get_().(Command_CommandResolveUnknown).Caps
		_ = _11_caps
		if !((state).Dtor_phase()).Equals(Companion_EffectPhase_.Create_OutcomeUnknownPhase_()) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventEffectRejected_(_10_effectID, _dafny.UnicodeSeqOfUtf8Bytes("effect is not in unknown state"))))
		} else if !((_11_caps).Dtor_status()).Equals(Companion_VerificationStatus_.Create_Verified_()) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventManualResolutionRequired_(_10_effectID)))
		} else if !((_11_caps).Dtor_resultLookup()).Equals(Companion_ResultLookupCapability_.Create_ResultLookupNone_()) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventReconciliationRequired_(_10_effectID)))
		} else if (!((_11_caps).Dtor_idempotency()).Equals(Companion_IdempotencyCapability_.Create_IdempotencyNone_())) && (((_11_caps).Dtor_unknownOutcome()).Equals(Companion_UnknownOutcomeCapability_.Create_CanSafelyRetry_())) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventRetryAuthorized_(_10_effectID)))
		} else {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventManualResolutionRequired_(_10_effectID)))
		}
	}
}
func (_static *CompanionStruct_Default___) Apply(state EffectState, event Event) EffectState {
	var _source0 Event = event
	_ = _source0
	{
		if _source0.Is_EventEffectClaimed() {
			var _0_workerID _dafny.Sequence = _source0.Get_().(Event_EventEffectClaimed).WorkerID
			_ = _0_workerID
			var _1_claimEpoch _dafny.Int = _source0.Get_().(Event_EventEffectClaimed).ClaimEpoch
			_ = _1_claimEpoch
			var _2_leaseUntil _dafny.Int = _source0.Get_().(Event_EventEffectClaimed).LeaseUntil
			_ = _2_leaseUntil
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_Claimed_(), _0_workerID, _1_claimEpoch, _2_leaseUntil, ((state).Dtor_retries()).Plus(_dafny.One))
		}
	}
	{
		if _source0.Is_EventEffectObserved() {
			var _3_outcome Outcome = _source0.Get_().(Event_EventEffectObserved).Outcome
			_ = _3_outcome
			var _4_newPhase EffectPhase = func() EffectPhase {
				var _source1 Outcome = _3_outcome
				_ = _source1
				{
					if _source1.Is_Succeeded() {
						return Companion_EffectPhase_.Create_SucceededPhase_()
					}
				}
				{
					if _source1.Is_FailedRetryable() {
						return Companion_EffectPhase_.Create_FailedRetryablePhase_()
					}
				}
				{
					if _source1.Is_FailedTerminal() {
						return Companion_EffectPhase_.Create_FailedTerminalPhase_()
					}
				}
				{
					return Companion_EffectPhase_.Create_OutcomeUnknownPhase_()
				}
			}()
			_ = _4_newPhase
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), _4_newPhase, (state).Dtor_workerID(), (state).Dtor_claimEpoch(), (state).Dtor_leaseUntil(), (state).Dtor_retries())
		}
	}
	{
		if _source0.Is_EventEffectRetried() {
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_Pending_(), _dafny.UnicodeSeqOfUtf8Bytes(""), (state).Dtor_claimEpoch(), _dafny.Zero, (state).Dtor_retries())
		}
	}
	{
		if _source0.Is_EventEffectTerminalized() {
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_FailedTerminalPhase_(), (state).Dtor_workerID(), (state).Dtor_claimEpoch(), (state).Dtor_leaseUntil(), (state).Dtor_retries())
		}
	}
	{
		if _source0.Is_EventEffectRejected() {
			return state
		}
	}
	{
		if _source0.Is_EventManualResolutionRequired() {
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_ManualResolutionRequiredPhase_(), (state).Dtor_workerID(), (state).Dtor_claimEpoch(), (state).Dtor_leaseUntil(), (state).Dtor_retries())
		}
	}
	{
		if _source0.Is_EventReconciliationRequired() {
			return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_ReconciliationRequiredPhase_(), (state).Dtor_workerID(), (state).Dtor_claimEpoch(), (state).Dtor_leaseUntil(), (state).Dtor_retries())
		}
	}
	{
		return Companion_EffectState_.Create_EffectState_((state).Dtor_effectID(), (state).Dtor_attemptID(), Companion_EffectPhase_.Create_Pending_(), _dafny.UnicodeSeqOfUtf8Bytes(""), (state).Dtor_claimEpoch(), _dafny.Zero, (state).Dtor_retries())
	}
}
func (_static *CompanionStruct_Default___) ApplyBatch(state EffectState, events _dafny.Sequence) EffectState {
	goto TAIL_CALL_START
TAIL_CALL_START:
	if (_dafny.IntOfUint32((events).Cardinality())).Sign() == 0 {
		return state
	} else {
		var _in0 EffectState = Companion_Default___.Apply(state, (events).Select(0).(Event))
		_ = _in0
		var _in1 _dafny.Sequence = (events).Drop(1)
		_ = _in1
		state = _in0
		events = _in1
		goto TAIL_CALL_START
	}
}

// End of class Default__

// Definition of datatype Outcome
type Outcome struct {
	Data_Outcome_
}

func (_this Outcome) Get_() Data_Outcome_ {
	return _this.Data_Outcome_
}

type Data_Outcome_ interface {
	isOutcome()
}

type CompanionStruct_Outcome_ struct {
}

var Companion_Outcome_ = CompanionStruct_Outcome_{}

type Outcome_Succeeded struct {
}

func (Outcome_Succeeded) isOutcome() {}

func (CompanionStruct_Outcome_) Create_Succeeded_() Outcome {
	return Outcome{Outcome_Succeeded{}}
}

func (_this Outcome) Is_Succeeded() bool {
	_, ok := _this.Get_().(Outcome_Succeeded)
	return ok
}

type Outcome_FailedRetryable struct {
}

func (Outcome_FailedRetryable) isOutcome() {}

func (CompanionStruct_Outcome_) Create_FailedRetryable_() Outcome {
	return Outcome{Outcome_FailedRetryable{}}
}

func (_this Outcome) Is_FailedRetryable() bool {
	_, ok := _this.Get_().(Outcome_FailedRetryable)
	return ok
}

type Outcome_FailedTerminal struct {
}

func (Outcome_FailedTerminal) isOutcome() {}

func (CompanionStruct_Outcome_) Create_FailedTerminal_() Outcome {
	return Outcome{Outcome_FailedTerminal{}}
}

func (_this Outcome) Is_FailedTerminal() bool {
	_, ok := _this.Get_().(Outcome_FailedTerminal)
	return ok
}

type Outcome_OutcomeUnknown struct {
}

func (Outcome_OutcomeUnknown) isOutcome() {}

func (CompanionStruct_Outcome_) Create_OutcomeUnknown_() Outcome {
	return Outcome{Outcome_OutcomeUnknown{}}
}

func (_this Outcome) Is_OutcomeUnknown() bool {
	_, ok := _this.Get_().(Outcome_OutcomeUnknown)
	return ok
}

func (CompanionStruct_Outcome_) Default() Outcome {
	return Companion_Outcome_.Create_Succeeded_()
}

func (_ CompanionStruct_Outcome_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_Outcome_.Create_Succeeded_(), true
		case 1:
			return Companion_Outcome_.Create_FailedRetryable_(), true
		case 2:
			return Companion_Outcome_.Create_FailedTerminal_(), true
		case 3:
			return Companion_Outcome_.Create_OutcomeUnknown_(), true
		default:
			return Outcome{}, false
		}
	}
}

func (_this Outcome) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case Outcome_Succeeded:
		{
			return "EffectReducer.Outcome.Succeeded"
		}
	case Outcome_FailedRetryable:
		{
			return "EffectReducer.Outcome.FailedRetryable"
		}
	case Outcome_FailedTerminal:
		{
			return "EffectReducer.Outcome.FailedTerminal"
		}
	case Outcome_OutcomeUnknown:
		{
			return "EffectReducer.Outcome.OutcomeUnknown"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Outcome) Equals(other Outcome) bool {
	switch _this.Get_().(type) {
	case Outcome_Succeeded:
		{
			_, ok := other.Get_().(Outcome_Succeeded)
			return ok
		}
	case Outcome_FailedRetryable:
		{
			_, ok := other.Get_().(Outcome_FailedRetryable)
			return ok
		}
	case Outcome_FailedTerminal:
		{
			_, ok := other.Get_().(Outcome_FailedTerminal)
			return ok
		}
	case Outcome_OutcomeUnknown:
		{
			_, ok := other.Get_().(Outcome_OutcomeUnknown)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this Outcome) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(Outcome)
	return ok && _this.Equals(typed)
}

func Type_Outcome_() _dafny.TypeDescriptor {
	return type_Outcome_{}
}

type type_Outcome_ struct {
}

func (_this type_Outcome_) Default() interface{} {
	return Companion_Outcome_.Default()
}

func (_this type_Outcome_) String() string {
	return "EffectReducer.Outcome"
}
func (_this Outcome) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Outcome{}

// End of datatype Outcome

// Definition of datatype EffectPhase
type EffectPhase struct {
	Data_EffectPhase_
}

func (_this EffectPhase) Get_() Data_EffectPhase_ {
	return _this.Data_EffectPhase_
}

type Data_EffectPhase_ interface {
	isEffectPhase()
}

type CompanionStruct_EffectPhase_ struct {
}

var Companion_EffectPhase_ = CompanionStruct_EffectPhase_{}

type EffectPhase_Pending struct {
}

func (EffectPhase_Pending) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_Pending_() EffectPhase {
	return EffectPhase{EffectPhase_Pending{}}
}

func (_this EffectPhase) Is_Pending() bool {
	_, ok := _this.Get_().(EffectPhase_Pending)
	return ok
}

type EffectPhase_Claimed struct {
}

func (EffectPhase_Claimed) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_Claimed_() EffectPhase {
	return EffectPhase{EffectPhase_Claimed{}}
}

func (_this EffectPhase) Is_Claimed() bool {
	_, ok := _this.Get_().(EffectPhase_Claimed)
	return ok
}

type EffectPhase_SucceededPhase struct {
}

func (EffectPhase_SucceededPhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_SucceededPhase_() EffectPhase {
	return EffectPhase{EffectPhase_SucceededPhase{}}
}

func (_this EffectPhase) Is_SucceededPhase() bool {
	_, ok := _this.Get_().(EffectPhase_SucceededPhase)
	return ok
}

type EffectPhase_FailedRetryablePhase struct {
}

func (EffectPhase_FailedRetryablePhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_FailedRetryablePhase_() EffectPhase {
	return EffectPhase{EffectPhase_FailedRetryablePhase{}}
}

func (_this EffectPhase) Is_FailedRetryablePhase() bool {
	_, ok := _this.Get_().(EffectPhase_FailedRetryablePhase)
	return ok
}

type EffectPhase_FailedTerminalPhase struct {
}

func (EffectPhase_FailedTerminalPhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_FailedTerminalPhase_() EffectPhase {
	return EffectPhase{EffectPhase_FailedTerminalPhase{}}
}

func (_this EffectPhase) Is_FailedTerminalPhase() bool {
	_, ok := _this.Get_().(EffectPhase_FailedTerminalPhase)
	return ok
}

type EffectPhase_OutcomeUnknownPhase struct {
}

func (EffectPhase_OutcomeUnknownPhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_OutcomeUnknownPhase_() EffectPhase {
	return EffectPhase{EffectPhase_OutcomeUnknownPhase{}}
}

func (_this EffectPhase) Is_OutcomeUnknownPhase() bool {
	_, ok := _this.Get_().(EffectPhase_OutcomeUnknownPhase)
	return ok
}

type EffectPhase_ReconciliationRequiredPhase struct {
}

func (EffectPhase_ReconciliationRequiredPhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_ReconciliationRequiredPhase_() EffectPhase {
	return EffectPhase{EffectPhase_ReconciliationRequiredPhase{}}
}

func (_this EffectPhase) Is_ReconciliationRequiredPhase() bool {
	_, ok := _this.Get_().(EffectPhase_ReconciliationRequiredPhase)
	return ok
}

type EffectPhase_ManualResolutionRequiredPhase struct {
}

func (EffectPhase_ManualResolutionRequiredPhase) isEffectPhase() {}

func (CompanionStruct_EffectPhase_) Create_ManualResolutionRequiredPhase_() EffectPhase {
	return EffectPhase{EffectPhase_ManualResolutionRequiredPhase{}}
}

func (_this EffectPhase) Is_ManualResolutionRequiredPhase() bool {
	_, ok := _this.Get_().(EffectPhase_ManualResolutionRequiredPhase)
	return ok
}

func (CompanionStruct_EffectPhase_) Default() EffectPhase {
	return Companion_EffectPhase_.Create_Pending_()
}

func (_ CompanionStruct_EffectPhase_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_EffectPhase_.Create_Pending_(), true
		case 1:
			return Companion_EffectPhase_.Create_Claimed_(), true
		case 2:
			return Companion_EffectPhase_.Create_SucceededPhase_(), true
		case 3:
			return Companion_EffectPhase_.Create_FailedRetryablePhase_(), true
		case 4:
			return Companion_EffectPhase_.Create_FailedTerminalPhase_(), true
		case 5:
			return Companion_EffectPhase_.Create_OutcomeUnknownPhase_(), true
		case 6:
			return Companion_EffectPhase_.Create_ReconciliationRequiredPhase_(), true
		case 7:
			return Companion_EffectPhase_.Create_ManualResolutionRequiredPhase_(), true
		default:
			return EffectPhase{}, false
		}
	}
}

func (_this EffectPhase) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case EffectPhase_Pending:
		{
			return "EffectReducer.EffectPhase.Pending"
		}
	case EffectPhase_Claimed:
		{
			return "EffectReducer.EffectPhase.Claimed"
		}
	case EffectPhase_SucceededPhase:
		{
			return "EffectReducer.EffectPhase.SucceededPhase"
		}
	case EffectPhase_FailedRetryablePhase:
		{
			return "EffectReducer.EffectPhase.FailedRetryablePhase"
		}
	case EffectPhase_FailedTerminalPhase:
		{
			return "EffectReducer.EffectPhase.FailedTerminalPhase"
		}
	case EffectPhase_OutcomeUnknownPhase:
		{
			return "EffectReducer.EffectPhase.OutcomeUnknownPhase"
		}
	case EffectPhase_ReconciliationRequiredPhase:
		{
			return "EffectReducer.EffectPhase.ReconciliationRequiredPhase"
		}
	case EffectPhase_ManualResolutionRequiredPhase:
		{
			return "EffectReducer.EffectPhase.ManualResolutionRequiredPhase"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this EffectPhase) Equals(other EffectPhase) bool {
	switch _this.Get_().(type) {
	case EffectPhase_Pending:
		{
			_, ok := other.Get_().(EffectPhase_Pending)
			return ok
		}
	case EffectPhase_Claimed:
		{
			_, ok := other.Get_().(EffectPhase_Claimed)
			return ok
		}
	case EffectPhase_SucceededPhase:
		{
			_, ok := other.Get_().(EffectPhase_SucceededPhase)
			return ok
		}
	case EffectPhase_FailedRetryablePhase:
		{
			_, ok := other.Get_().(EffectPhase_FailedRetryablePhase)
			return ok
		}
	case EffectPhase_FailedTerminalPhase:
		{
			_, ok := other.Get_().(EffectPhase_FailedTerminalPhase)
			return ok
		}
	case EffectPhase_OutcomeUnknownPhase:
		{
			_, ok := other.Get_().(EffectPhase_OutcomeUnknownPhase)
			return ok
		}
	case EffectPhase_ReconciliationRequiredPhase:
		{
			_, ok := other.Get_().(EffectPhase_ReconciliationRequiredPhase)
			return ok
		}
	case EffectPhase_ManualResolutionRequiredPhase:
		{
			_, ok := other.Get_().(EffectPhase_ManualResolutionRequiredPhase)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this EffectPhase) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(EffectPhase)
	return ok && _this.Equals(typed)
}

func Type_EffectPhase_() _dafny.TypeDescriptor {
	return type_EffectPhase_{}
}

type type_EffectPhase_ struct {
}

func (_this type_EffectPhase_) Default() interface{} {
	return Companion_EffectPhase_.Default()
}

func (_this type_EffectPhase_) String() string {
	return "EffectReducer.EffectPhase"
}
func (_this EffectPhase) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = EffectPhase{}

// End of datatype EffectPhase

// Definition of datatype IdempotencyCapability
type IdempotencyCapability struct {
	Data_IdempotencyCapability_
}

func (_this IdempotencyCapability) Get_() Data_IdempotencyCapability_ {
	return _this.Data_IdempotencyCapability_
}

type Data_IdempotencyCapability_ interface {
	isIdempotencyCapability()
}

type CompanionStruct_IdempotencyCapability_ struct {
}

var Companion_IdempotencyCapability_ = CompanionStruct_IdempotencyCapability_{}

type IdempotencyCapability_IdempotencyNone struct {
}

func (IdempotencyCapability_IdempotencyNone) isIdempotencyCapability() {}

func (CompanionStruct_IdempotencyCapability_) Create_IdempotencyNone_() IdempotencyCapability {
	return IdempotencyCapability{IdempotencyCapability_IdempotencyNone{}}
}

func (_this IdempotencyCapability) Is_IdempotencyNone() bool {
	_, ok := _this.Get_().(IdempotencyCapability_IdempotencyNone)
	return ok
}

type IdempotencyCapability_StableKey struct {
}

func (IdempotencyCapability_StableKey) isIdempotencyCapability() {}

func (CompanionStruct_IdempotencyCapability_) Create_StableKey_() IdempotencyCapability {
	return IdempotencyCapability{IdempotencyCapability_StableKey{}}
}

func (_this IdempotencyCapability) Is_StableKey() bool {
	_, ok := _this.Get_().(IdempotencyCapability_StableKey)
	return ok
}

type IdempotencyCapability_StableKeyWithConflictDetection struct {
}

func (IdempotencyCapability_StableKeyWithConflictDetection) isIdempotencyCapability() {}

func (CompanionStruct_IdempotencyCapability_) Create_StableKeyWithConflictDetection_() IdempotencyCapability {
	return IdempotencyCapability{IdempotencyCapability_StableKeyWithConflictDetection{}}
}

func (_this IdempotencyCapability) Is_StableKeyWithConflictDetection() bool {
	_, ok := _this.Get_().(IdempotencyCapability_StableKeyWithConflictDetection)
	return ok
}

func (CompanionStruct_IdempotencyCapability_) Default() IdempotencyCapability {
	return Companion_IdempotencyCapability_.Create_IdempotencyNone_()
}

func (_ CompanionStruct_IdempotencyCapability_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_IdempotencyCapability_.Create_IdempotencyNone_(), true
		case 1:
			return Companion_IdempotencyCapability_.Create_StableKey_(), true
		case 2:
			return Companion_IdempotencyCapability_.Create_StableKeyWithConflictDetection_(), true
		default:
			return IdempotencyCapability{}, false
		}
	}
}

func (_this IdempotencyCapability) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case IdempotencyCapability_IdempotencyNone:
		{
			return "EffectReducer.IdempotencyCapability.IdempotencyNone"
		}
	case IdempotencyCapability_StableKey:
		{
			return "EffectReducer.IdempotencyCapability.StableKey"
		}
	case IdempotencyCapability_StableKeyWithConflictDetection:
		{
			return "EffectReducer.IdempotencyCapability.StableKeyWithConflictDetection"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this IdempotencyCapability) Equals(other IdempotencyCapability) bool {
	switch _this.Get_().(type) {
	case IdempotencyCapability_IdempotencyNone:
		{
			_, ok := other.Get_().(IdempotencyCapability_IdempotencyNone)
			return ok
		}
	case IdempotencyCapability_StableKey:
		{
			_, ok := other.Get_().(IdempotencyCapability_StableKey)
			return ok
		}
	case IdempotencyCapability_StableKeyWithConflictDetection:
		{
			_, ok := other.Get_().(IdempotencyCapability_StableKeyWithConflictDetection)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this IdempotencyCapability) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(IdempotencyCapability)
	return ok && _this.Equals(typed)
}

func Type_IdempotencyCapability_() _dafny.TypeDescriptor {
	return type_IdempotencyCapability_{}
}

type type_IdempotencyCapability_ struct {
}

func (_this type_IdempotencyCapability_) Default() interface{} {
	return Companion_IdempotencyCapability_.Default()
}

func (_this type_IdempotencyCapability_) String() string {
	return "EffectReducer.IdempotencyCapability"
}
func (_this IdempotencyCapability) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = IdempotencyCapability{}

// End of datatype IdempotencyCapability

// Definition of datatype ResultLookupCapability
type ResultLookupCapability struct {
	Data_ResultLookupCapability_
}

func (_this ResultLookupCapability) Get_() Data_ResultLookupCapability_ {
	return _this.Data_ResultLookupCapability_
}

type Data_ResultLookupCapability_ interface {
	isResultLookupCapability()
}

type CompanionStruct_ResultLookupCapability_ struct {
}

var Companion_ResultLookupCapability_ = CompanionStruct_ResultLookupCapability_{}

type ResultLookupCapability_ResultLookupNone struct {
}

func (ResultLookupCapability_ResultLookupNone) isResultLookupCapability() {}

func (CompanionStruct_ResultLookupCapability_) Create_ResultLookupNone_() ResultLookupCapability {
	return ResultLookupCapability{ResultLookupCapability_ResultLookupNone{}}
}

func (_this ResultLookupCapability) Is_ResultLookupNone() bool {
	_, ok := _this.Get_().(ResultLookupCapability_ResultLookupNone)
	return ok
}

type ResultLookupCapability_ByExternalReference struct {
}

func (ResultLookupCapability_ByExternalReference) isResultLookupCapability() {}

func (CompanionStruct_ResultLookupCapability_) Create_ByExternalReference_() ResultLookupCapability {
	return ResultLookupCapability{ResultLookupCapability_ByExternalReference{}}
}

func (_this ResultLookupCapability) Is_ByExternalReference() bool {
	_, ok := _this.Get_().(ResultLookupCapability_ByExternalReference)
	return ok
}

type ResultLookupCapability_ByIdempotencyKey struct {
}

func (ResultLookupCapability_ByIdempotencyKey) isResultLookupCapability() {}

func (CompanionStruct_ResultLookupCapability_) Create_ByIdempotencyKey_() ResultLookupCapability {
	return ResultLookupCapability{ResultLookupCapability_ByIdempotencyKey{}}
}

func (_this ResultLookupCapability) Is_ByIdempotencyKey() bool {
	_, ok := _this.Get_().(ResultLookupCapability_ByIdempotencyKey)
	return ok
}

func (CompanionStruct_ResultLookupCapability_) Default() ResultLookupCapability {
	return Companion_ResultLookupCapability_.Create_ResultLookupNone_()
}

func (_ CompanionStruct_ResultLookupCapability_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_ResultLookupCapability_.Create_ResultLookupNone_(), true
		case 1:
			return Companion_ResultLookupCapability_.Create_ByExternalReference_(), true
		case 2:
			return Companion_ResultLookupCapability_.Create_ByIdempotencyKey_(), true
		default:
			return ResultLookupCapability{}, false
		}
	}
}

func (_this ResultLookupCapability) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case ResultLookupCapability_ResultLookupNone:
		{
			return "EffectReducer.ResultLookupCapability.ResultLookupNone"
		}
	case ResultLookupCapability_ByExternalReference:
		{
			return "EffectReducer.ResultLookupCapability.ByExternalReference"
		}
	case ResultLookupCapability_ByIdempotencyKey:
		{
			return "EffectReducer.ResultLookupCapability.ByIdempotencyKey"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this ResultLookupCapability) Equals(other ResultLookupCapability) bool {
	switch _this.Get_().(type) {
	case ResultLookupCapability_ResultLookupNone:
		{
			_, ok := other.Get_().(ResultLookupCapability_ResultLookupNone)
			return ok
		}
	case ResultLookupCapability_ByExternalReference:
		{
			_, ok := other.Get_().(ResultLookupCapability_ByExternalReference)
			return ok
		}
	case ResultLookupCapability_ByIdempotencyKey:
		{
			_, ok := other.Get_().(ResultLookupCapability_ByIdempotencyKey)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this ResultLookupCapability) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(ResultLookupCapability)
	return ok && _this.Equals(typed)
}

func Type_ResultLookupCapability_() _dafny.TypeDescriptor {
	return type_ResultLookupCapability_{}
}

type type_ResultLookupCapability_ struct {
}

func (_this type_ResultLookupCapability_) Default() interface{} {
	return Companion_ResultLookupCapability_.Default()
}

func (_this type_ResultLookupCapability_) String() string {
	return "EffectReducer.ResultLookupCapability"
}
func (_this ResultLookupCapability) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = ResultLookupCapability{}

// End of datatype ResultLookupCapability

// Definition of datatype TransactionCapability
type TransactionCapability struct {
	Data_TransactionCapability_
}

func (_this TransactionCapability) Get_() Data_TransactionCapability_ {
	return _this.Data_TransactionCapability_
}

type Data_TransactionCapability_ interface {
	isTransactionCapability()
}

type CompanionStruct_TransactionCapability_ struct {
}

var Companion_TransactionCapability_ = CompanionStruct_TransactionCapability_{}

type TransactionCapability_TransactionNone struct {
}

func (TransactionCapability_TransactionNone) isTransactionCapability() {}

func (CompanionStruct_TransactionCapability_) Create_TransactionNone_() TransactionCapability {
	return TransactionCapability{TransactionCapability_TransactionNone{}}
}

func (_this TransactionCapability) Is_TransactionNone() bool {
	_, ok := _this.Get_().(TransactionCapability_TransactionNone)
	return ok
}

type TransactionCapability_SingleOperation struct {
}

func (TransactionCapability_SingleOperation) isTransactionCapability() {}

func (CompanionStruct_TransactionCapability_) Create_SingleOperation_() TransactionCapability {
	return TransactionCapability{TransactionCapability_SingleOperation{}}
}

func (_this TransactionCapability) Is_SingleOperation() bool {
	_, ok := _this.Get_().(TransactionCapability_SingleOperation)
	return ok
}

type TransactionCapability_AtomicBatch struct {
}

func (TransactionCapability_AtomicBatch) isTransactionCapability() {}

func (CompanionStruct_TransactionCapability_) Create_AtomicBatch_() TransactionCapability {
	return TransactionCapability{TransactionCapability_AtomicBatch{}}
}

func (_this TransactionCapability) Is_AtomicBatch() bool {
	_, ok := _this.Get_().(TransactionCapability_AtomicBatch)
	return ok
}

func (CompanionStruct_TransactionCapability_) Default() TransactionCapability {
	return Companion_TransactionCapability_.Create_TransactionNone_()
}

func (_ CompanionStruct_TransactionCapability_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_TransactionCapability_.Create_TransactionNone_(), true
		case 1:
			return Companion_TransactionCapability_.Create_SingleOperation_(), true
		case 2:
			return Companion_TransactionCapability_.Create_AtomicBatch_(), true
		default:
			return TransactionCapability{}, false
		}
	}
}

func (_this TransactionCapability) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case TransactionCapability_TransactionNone:
		{
			return "EffectReducer.TransactionCapability.TransactionNone"
		}
	case TransactionCapability_SingleOperation:
		{
			return "EffectReducer.TransactionCapability.SingleOperation"
		}
	case TransactionCapability_AtomicBatch:
		{
			return "EffectReducer.TransactionCapability.AtomicBatch"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this TransactionCapability) Equals(other TransactionCapability) bool {
	switch _this.Get_().(type) {
	case TransactionCapability_TransactionNone:
		{
			_, ok := other.Get_().(TransactionCapability_TransactionNone)
			return ok
		}
	case TransactionCapability_SingleOperation:
		{
			_, ok := other.Get_().(TransactionCapability_SingleOperation)
			return ok
		}
	case TransactionCapability_AtomicBatch:
		{
			_, ok := other.Get_().(TransactionCapability_AtomicBatch)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this TransactionCapability) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(TransactionCapability)
	return ok && _this.Equals(typed)
}

func Type_TransactionCapability_() _dafny.TypeDescriptor {
	return type_TransactionCapability_{}
}

type type_TransactionCapability_ struct {
}

func (_this type_TransactionCapability_) Default() interface{} {
	return Companion_TransactionCapability_.Default()
}

func (_this type_TransactionCapability_) String() string {
	return "EffectReducer.TransactionCapability"
}
func (_this TransactionCapability) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = TransactionCapability{}

// End of datatype TransactionCapability

// Definition of datatype UnknownOutcomeCapability
type UnknownOutcomeCapability struct {
	Data_UnknownOutcomeCapability_
}

func (_this UnknownOutcomeCapability) Get_() Data_UnknownOutcomeCapability_ {
	return _this.Data_UnknownOutcomeCapability_
}

type Data_UnknownOutcomeCapability_ interface {
	isUnknownOutcomeCapability()
}

type CompanionStruct_UnknownOutcomeCapability_ struct {
}

var Companion_UnknownOutcomeCapability_ = CompanionStruct_UnknownOutcomeCapability_{}

type UnknownOutcomeCapability_RequiresManualResolution struct {
}

func (UnknownOutcomeCapability_RequiresManualResolution) isUnknownOutcomeCapability() {}

func (CompanionStruct_UnknownOutcomeCapability_) Create_RequiresManualResolution_() UnknownOutcomeCapability {
	return UnknownOutcomeCapability{UnknownOutcomeCapability_RequiresManualResolution{}}
}

func (_this UnknownOutcomeCapability) Is_RequiresManualResolution() bool {
	_, ok := _this.Get_().(UnknownOutcomeCapability_RequiresManualResolution)
	return ok
}

type UnknownOutcomeCapability_CanReconcile struct {
}

func (UnknownOutcomeCapability_CanReconcile) isUnknownOutcomeCapability() {}

func (CompanionStruct_UnknownOutcomeCapability_) Create_CanReconcile_() UnknownOutcomeCapability {
	return UnknownOutcomeCapability{UnknownOutcomeCapability_CanReconcile{}}
}

func (_this UnknownOutcomeCapability) Is_CanReconcile() bool {
	_, ok := _this.Get_().(UnknownOutcomeCapability_CanReconcile)
	return ok
}

type UnknownOutcomeCapability_CanSafelyRetry struct {
}

func (UnknownOutcomeCapability_CanSafelyRetry) isUnknownOutcomeCapability() {}

func (CompanionStruct_UnknownOutcomeCapability_) Create_CanSafelyRetry_() UnknownOutcomeCapability {
	return UnknownOutcomeCapability{UnknownOutcomeCapability_CanSafelyRetry{}}
}

func (_this UnknownOutcomeCapability) Is_CanSafelyRetry() bool {
	_, ok := _this.Get_().(UnknownOutcomeCapability_CanSafelyRetry)
	return ok
}

func (CompanionStruct_UnknownOutcomeCapability_) Default() UnknownOutcomeCapability {
	return Companion_UnknownOutcomeCapability_.Create_RequiresManualResolution_()
}

func (_ CompanionStruct_UnknownOutcomeCapability_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_UnknownOutcomeCapability_.Create_RequiresManualResolution_(), true
		case 1:
			return Companion_UnknownOutcomeCapability_.Create_CanReconcile_(), true
		case 2:
			return Companion_UnknownOutcomeCapability_.Create_CanSafelyRetry_(), true
		default:
			return UnknownOutcomeCapability{}, false
		}
	}
}

func (_this UnknownOutcomeCapability) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case UnknownOutcomeCapability_RequiresManualResolution:
		{
			return "EffectReducer.UnknownOutcomeCapability.RequiresManualResolution"
		}
	case UnknownOutcomeCapability_CanReconcile:
		{
			return "EffectReducer.UnknownOutcomeCapability.CanReconcile"
		}
	case UnknownOutcomeCapability_CanSafelyRetry:
		{
			return "EffectReducer.UnknownOutcomeCapability.CanSafelyRetry"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this UnknownOutcomeCapability) Equals(other UnknownOutcomeCapability) bool {
	switch _this.Get_().(type) {
	case UnknownOutcomeCapability_RequiresManualResolution:
		{
			_, ok := other.Get_().(UnknownOutcomeCapability_RequiresManualResolution)
			return ok
		}
	case UnknownOutcomeCapability_CanReconcile:
		{
			_, ok := other.Get_().(UnknownOutcomeCapability_CanReconcile)
			return ok
		}
	case UnknownOutcomeCapability_CanSafelyRetry:
		{
			_, ok := other.Get_().(UnknownOutcomeCapability_CanSafelyRetry)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this UnknownOutcomeCapability) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(UnknownOutcomeCapability)
	return ok && _this.Equals(typed)
}

func Type_UnknownOutcomeCapability_() _dafny.TypeDescriptor {
	return type_UnknownOutcomeCapability_{}
}

type type_UnknownOutcomeCapability_ struct {
}

func (_this type_UnknownOutcomeCapability_) Default() interface{} {
	return Companion_UnknownOutcomeCapability_.Default()
}

func (_this type_UnknownOutcomeCapability_) String() string {
	return "EffectReducer.UnknownOutcomeCapability"
}
func (_this UnknownOutcomeCapability) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = UnknownOutcomeCapability{}

// End of datatype UnknownOutcomeCapability

// Definition of datatype VerificationStatus
type VerificationStatus struct {
	Data_VerificationStatus_
}

func (_this VerificationStatus) Get_() Data_VerificationStatus_ {
	return _this.Data_VerificationStatus_
}

type Data_VerificationStatus_ interface {
	isVerificationStatus()
}

type CompanionStruct_VerificationStatus_ struct {
}

var Companion_VerificationStatus_ = CompanionStruct_VerificationStatus_{}

type VerificationStatus_Proposed struct {
}

func (VerificationStatus_Proposed) isVerificationStatus() {}

func (CompanionStruct_VerificationStatus_) Create_Proposed_() VerificationStatus {
	return VerificationStatus{VerificationStatus_Proposed{}}
}

func (_this VerificationStatus) Is_Proposed() bool {
	_, ok := _this.Get_().(VerificationStatus_Proposed)
	return ok
}

type VerificationStatus_Verified struct {
}

func (VerificationStatus_Verified) isVerificationStatus() {}

func (CompanionStruct_VerificationStatus_) Create_Verified_() VerificationStatus {
	return VerificationStatus{VerificationStatus_Verified{}}
}

func (_this VerificationStatus) Is_Verified() bool {
	_, ok := _this.Get_().(VerificationStatus_Verified)
	return ok
}

type VerificationStatus_Invalidated struct {
}

func (VerificationStatus_Invalidated) isVerificationStatus() {}

func (CompanionStruct_VerificationStatus_) Create_Invalidated_() VerificationStatus {
	return VerificationStatus{VerificationStatus_Invalidated{}}
}

func (_this VerificationStatus) Is_Invalidated() bool {
	_, ok := _this.Get_().(VerificationStatus_Invalidated)
	return ok
}

func (CompanionStruct_VerificationStatus_) Default() VerificationStatus {
	return Companion_VerificationStatus_.Create_Proposed_()
}

func (_ CompanionStruct_VerificationStatus_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_VerificationStatus_.Create_Proposed_(), true
		case 1:
			return Companion_VerificationStatus_.Create_Verified_(), true
		case 2:
			return Companion_VerificationStatus_.Create_Invalidated_(), true
		default:
			return VerificationStatus{}, false
		}
	}
}

func (_this VerificationStatus) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case VerificationStatus_Proposed:
		{
			return "EffectReducer.VerificationStatus.Proposed"
		}
	case VerificationStatus_Verified:
		{
			return "EffectReducer.VerificationStatus.Verified"
		}
	case VerificationStatus_Invalidated:
		{
			return "EffectReducer.VerificationStatus.Invalidated"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this VerificationStatus) Equals(other VerificationStatus) bool {
	switch _this.Get_().(type) {
	case VerificationStatus_Proposed:
		{
			_, ok := other.Get_().(VerificationStatus_Proposed)
			return ok
		}
	case VerificationStatus_Verified:
		{
			_, ok := other.Get_().(VerificationStatus_Verified)
			return ok
		}
	case VerificationStatus_Invalidated:
		{
			_, ok := other.Get_().(VerificationStatus_Invalidated)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this VerificationStatus) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(VerificationStatus)
	return ok && _this.Equals(typed)
}

func Type_VerificationStatus_() _dafny.TypeDescriptor {
	return type_VerificationStatus_{}
}

type type_VerificationStatus_ struct {
}

func (_this type_VerificationStatus_) Default() interface{} {
	return Companion_VerificationStatus_.Default()
}

func (_this type_VerificationStatus_) String() string {
	return "EffectReducer.VerificationStatus"
}
func (_this VerificationStatus) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = VerificationStatus{}

// End of datatype VerificationStatus

// Definition of datatype CapabilityDescriptor
type CapabilityDescriptor struct {
	Data_CapabilityDescriptor_
}

func (_this CapabilityDescriptor) Get_() Data_CapabilityDescriptor_ {
	return _this.Data_CapabilityDescriptor_
}

type Data_CapabilityDescriptor_ interface {
	isCapabilityDescriptor()
}

type CompanionStruct_CapabilityDescriptor_ struct {
}

var Companion_CapabilityDescriptor_ = CompanionStruct_CapabilityDescriptor_{}

type CapabilityDescriptor_CapabilityDescriptor struct {
	Idempotency      IdempotencyCapability
	ResultLookup     ResultLookupCapability
	Transactionality TransactionCapability
	UnknownOutcome   UnknownOutcomeCapability
	Status           VerificationStatus
}

func (CapabilityDescriptor_CapabilityDescriptor) isCapabilityDescriptor() {}

func (CompanionStruct_CapabilityDescriptor_) Create_CapabilityDescriptor_(Idempotency IdempotencyCapability, ResultLookup ResultLookupCapability, Transactionality TransactionCapability, UnknownOutcome UnknownOutcomeCapability, Status VerificationStatus) CapabilityDescriptor {
	return CapabilityDescriptor{CapabilityDescriptor_CapabilityDescriptor{Idempotency, ResultLookup, Transactionality, UnknownOutcome, Status}}
}

func (_this CapabilityDescriptor) Is_CapabilityDescriptor() bool {
	_, ok := _this.Get_().(CapabilityDescriptor_CapabilityDescriptor)
	return ok
}

func (CompanionStruct_CapabilityDescriptor_) Default() CapabilityDescriptor {
	return Companion_CapabilityDescriptor_.Create_CapabilityDescriptor_(Companion_IdempotencyCapability_.Default(), Companion_ResultLookupCapability_.Default(), Companion_TransactionCapability_.Default(), Companion_UnknownOutcomeCapability_.Default(), Companion_VerificationStatus_.Default())
}

func (_this CapabilityDescriptor) Dtor_idempotency() IdempotencyCapability {
	return _this.Get_().(CapabilityDescriptor_CapabilityDescriptor).Idempotency
}

func (_this CapabilityDescriptor) Dtor_resultLookup() ResultLookupCapability {
	return _this.Get_().(CapabilityDescriptor_CapabilityDescriptor).ResultLookup
}

func (_this CapabilityDescriptor) Dtor_transactionality() TransactionCapability {
	return _this.Get_().(CapabilityDescriptor_CapabilityDescriptor).Transactionality
}

func (_this CapabilityDescriptor) Dtor_unknownOutcome() UnknownOutcomeCapability {
	return _this.Get_().(CapabilityDescriptor_CapabilityDescriptor).UnknownOutcome
}

func (_this CapabilityDescriptor) Dtor_status() VerificationStatus {
	return _this.Get_().(CapabilityDescriptor_CapabilityDescriptor).Status
}

func (_this CapabilityDescriptor) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case CapabilityDescriptor_CapabilityDescriptor:
		{
			return "EffectReducer.CapabilityDescriptor.CapabilityDescriptor" + "(" + _dafny.String(data.Idempotency) + ", " + _dafny.String(data.ResultLookup) + ", " + _dafny.String(data.Transactionality) + ", " + _dafny.String(data.UnknownOutcome) + ", " + _dafny.String(data.Status) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this CapabilityDescriptor) Equals(other CapabilityDescriptor) bool {
	switch data1 := _this.Get_().(type) {
	case CapabilityDescriptor_CapabilityDescriptor:
		{
			data2, ok := other.Get_().(CapabilityDescriptor_CapabilityDescriptor)
			return ok && data1.Idempotency.Equals(data2.Idempotency) && data1.ResultLookup.Equals(data2.ResultLookup) && data1.Transactionality.Equals(data2.Transactionality) && data1.UnknownOutcome.Equals(data2.UnknownOutcome) && data1.Status.Equals(data2.Status)
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this CapabilityDescriptor) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(CapabilityDescriptor)
	return ok && _this.Equals(typed)
}

func Type_CapabilityDescriptor_() _dafny.TypeDescriptor {
	return type_CapabilityDescriptor_{}
}

type type_CapabilityDescriptor_ struct {
}

func (_this type_CapabilityDescriptor_) Default() interface{} {
	return Companion_CapabilityDescriptor_.Default()
}

func (_this type_CapabilityDescriptor_) String() string {
	return "EffectReducer.CapabilityDescriptor"
}
func (_this CapabilityDescriptor) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = CapabilityDescriptor{}

// End of datatype CapabilityDescriptor

// Definition of datatype EffectState
type EffectState struct {
	Data_EffectState_
}

func (_this EffectState) Get_() Data_EffectState_ {
	return _this.Data_EffectState_
}

type Data_EffectState_ interface {
	isEffectState()
}

type CompanionStruct_EffectState_ struct {
}

var Companion_EffectState_ = CompanionStruct_EffectState_{}

type EffectState_EffectState struct {
	EffectID   _dafny.Sequence
	AttemptID  _dafny.Sequence
	Phase      EffectPhase
	WorkerID   _dafny.Sequence
	ClaimEpoch _dafny.Int
	LeaseUntil _dafny.Int
	Retries    _dafny.Int
}

func (EffectState_EffectState) isEffectState() {}

func (CompanionStruct_EffectState_) Create_EffectState_(EffectID _dafny.Sequence, AttemptID _dafny.Sequence, Phase EffectPhase, WorkerID _dafny.Sequence, ClaimEpoch _dafny.Int, LeaseUntil _dafny.Int, Retries _dafny.Int) EffectState {
	return EffectState{EffectState_EffectState{EffectID, AttemptID, Phase, WorkerID, ClaimEpoch, LeaseUntil, Retries}}
}

func (_this EffectState) Is_EffectState() bool {
	_, ok := _this.Get_().(EffectState_EffectState)
	return ok
}

func (CompanionStruct_EffectState_) Default() EffectState {
	return Companion_EffectState_.Create_EffectState_(_dafny.EmptySeq, _dafny.EmptySeq, Companion_EffectPhase_.Default(), _dafny.EmptySeq, _dafny.Zero, _dafny.Zero, _dafny.Zero)
}

func (_this EffectState) Dtor_effectID() _dafny.Sequence {
	return _this.Get_().(EffectState_EffectState).EffectID
}

func (_this EffectState) Dtor_attemptID() _dafny.Sequence {
	return _this.Get_().(EffectState_EffectState).AttemptID
}

func (_this EffectState) Dtor_phase() EffectPhase {
	return _this.Get_().(EffectState_EffectState).Phase
}

func (_this EffectState) Dtor_workerID() _dafny.Sequence {
	return _this.Get_().(EffectState_EffectState).WorkerID
}

func (_this EffectState) Dtor_claimEpoch() _dafny.Int {
	return _this.Get_().(EffectState_EffectState).ClaimEpoch
}

func (_this EffectState) Dtor_leaseUntil() _dafny.Int {
	return _this.Get_().(EffectState_EffectState).LeaseUntil
}

func (_this EffectState) Dtor_retries() _dafny.Int {
	return _this.Get_().(EffectState_EffectState).Retries
}

func (_this EffectState) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case EffectState_EffectState:
		{
			return "EffectReducer.EffectState.EffectState" + "(" + data.EffectID.VerbatimString(true) + ", " + data.AttemptID.VerbatimString(true) + ", " + _dafny.String(data.Phase) + ", " + data.WorkerID.VerbatimString(true) + ", " + _dafny.String(data.ClaimEpoch) + ", " + _dafny.String(data.LeaseUntil) + ", " + _dafny.String(data.Retries) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this EffectState) Equals(other EffectState) bool {
	switch data1 := _this.Get_().(type) {
	case EffectState_EffectState:
		{
			data2, ok := other.Get_().(EffectState_EffectState)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.AttemptID.Equals(data2.AttemptID) && data1.Phase.Equals(data2.Phase) && data1.WorkerID.Equals(data2.WorkerID) && data1.ClaimEpoch.Cmp(data2.ClaimEpoch) == 0 && data1.LeaseUntil.Cmp(data2.LeaseUntil) == 0 && data1.Retries.Cmp(data2.Retries) == 0
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this EffectState) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(EffectState)
	return ok && _this.Equals(typed)
}

func Type_EffectState_() _dafny.TypeDescriptor {
	return type_EffectState_{}
}

type type_EffectState_ struct {
}

func (_this type_EffectState_) Default() interface{} {
	return Companion_EffectState_.Default()
}

func (_this type_EffectState_) String() string {
	return "EffectReducer.EffectState"
}
func (_this EffectState) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = EffectState{}

// End of datatype EffectState

// Definition of datatype Command
type Command struct {
	Data_Command_
}

func (_this Command) Get_() Data_Command_ {
	return _this.Data_Command_
}

type Data_Command_ interface {
	isCommand()
}

type CompanionStruct_Command_ struct {
}

var Companion_Command_ = CompanionStruct_Command_{}

type Command_CommandClaimEffect struct {
	EffectID        _dafny.Sequence
	WorkerID        _dafny.Sequence
	ExpectedVersion _dafny.Int
	LeaseUntil      _dafny.Int
	CurrentTime     _dafny.Int
}

func (Command_CommandClaimEffect) isCommand() {}

func (CompanionStruct_Command_) Create_CommandClaimEffect_(EffectID _dafny.Sequence, WorkerID _dafny.Sequence, ExpectedVersion _dafny.Int, LeaseUntil _dafny.Int, CurrentTime _dafny.Int) Command {
	return Command{Command_CommandClaimEffect{EffectID, WorkerID, ExpectedVersion, LeaseUntil, CurrentTime}}
}

func (_this Command) Is_CommandClaimEffect() bool {
	_, ok := _this.Get_().(Command_CommandClaimEffect)
	return ok
}

type Command_CommandObserveEffect struct {
	EffectID   _dafny.Sequence
	WorkerID   _dafny.Sequence
	ClaimEpoch _dafny.Int
	Outcome    Outcome
}

func (Command_CommandObserveEffect) isCommand() {}

func (CompanionStruct_Command_) Create_CommandObserveEffect_(EffectID _dafny.Sequence, WorkerID _dafny.Sequence, ClaimEpoch _dafny.Int, Outcome Outcome) Command {
	return Command{Command_CommandObserveEffect{EffectID, WorkerID, ClaimEpoch, Outcome}}
}

func (_this Command) Is_CommandObserveEffect() bool {
	_, ok := _this.Get_().(Command_CommandObserveEffect)
	return ok
}

type Command_CommandRetryEffect struct {
	EffectID _dafny.Sequence
}

func (Command_CommandRetryEffect) isCommand() {}

func (CompanionStruct_Command_) Create_CommandRetryEffect_(EffectID _dafny.Sequence) Command {
	return Command{Command_CommandRetryEffect{EffectID}}
}

func (_this Command) Is_CommandRetryEffect() bool {
	_, ok := _this.Get_().(Command_CommandRetryEffect)
	return ok
}

type Command_CommandResolveUnknown struct {
	EffectID _dafny.Sequence
	Caps     CapabilityDescriptor
}

func (Command_CommandResolveUnknown) isCommand() {}

func (CompanionStruct_Command_) Create_CommandResolveUnknown_(EffectID _dafny.Sequence, Caps CapabilityDescriptor) Command {
	return Command{Command_CommandResolveUnknown{EffectID, Caps}}
}

func (_this Command) Is_CommandResolveUnknown() bool {
	_, ok := _this.Get_().(Command_CommandResolveUnknown)
	return ok
}

func (CompanionStruct_Command_) Default() Command {
	return Companion_Command_.Create_CommandClaimEffect_(_dafny.EmptySeq, _dafny.EmptySeq, _dafny.Zero, _dafny.Zero, _dafny.Zero)
}

func (_this Command) Dtor_effectID() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Command_CommandClaimEffect:
		return data.EffectID
	case Command_CommandObserveEffect:
		return data.EffectID
	case Command_CommandRetryEffect:
		return data.EffectID
	default:
		return data.(Command_CommandResolveUnknown).EffectID
	}
}

func (_this Command) Dtor_workerID() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Command_CommandClaimEffect:
		return data.WorkerID
	default:
		return data.(Command_CommandObserveEffect).WorkerID
	}
}

func (_this Command) Dtor_expectedVersion() _dafny.Int {
	return _this.Get_().(Command_CommandClaimEffect).ExpectedVersion
}

func (_this Command) Dtor_leaseUntil() _dafny.Int {
	return _this.Get_().(Command_CommandClaimEffect).LeaseUntil
}

func (_this Command) Dtor_currentTime() _dafny.Int {
	return _this.Get_().(Command_CommandClaimEffect).CurrentTime
}

func (_this Command) Dtor_claimEpoch() _dafny.Int {
	return _this.Get_().(Command_CommandObserveEffect).ClaimEpoch
}

func (_this Command) Dtor_outcome() Outcome {
	return _this.Get_().(Command_CommandObserveEffect).Outcome
}

func (_this Command) Dtor_caps() CapabilityDescriptor {
	return _this.Get_().(Command_CommandResolveUnknown).Caps
}

func (_this Command) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Command_CommandClaimEffect:
		{
			return "EffectReducer.Command.CommandClaimEffect" + "(" + data.EffectID.VerbatimString(true) + ", " + data.WorkerID.VerbatimString(true) + ", " + _dafny.String(data.ExpectedVersion) + ", " + _dafny.String(data.LeaseUntil) + ", " + _dafny.String(data.CurrentTime) + ")"
		}
	case Command_CommandObserveEffect:
		{
			return "EffectReducer.Command.CommandObserveEffect" + "(" + data.EffectID.VerbatimString(true) + ", " + data.WorkerID.VerbatimString(true) + ", " + _dafny.String(data.ClaimEpoch) + ", " + _dafny.String(data.Outcome) + ")"
		}
	case Command_CommandRetryEffect:
		{
			return "EffectReducer.Command.CommandRetryEffect" + "(" + data.EffectID.VerbatimString(true) + ")"
		}
	case Command_CommandResolveUnknown:
		{
			return "EffectReducer.Command.CommandResolveUnknown" + "(" + data.EffectID.VerbatimString(true) + ", " + _dafny.String(data.Caps) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Command) Equals(other Command) bool {
	switch data1 := _this.Get_().(type) {
	case Command_CommandClaimEffect:
		{
			data2, ok := other.Get_().(Command_CommandClaimEffect)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.WorkerID.Equals(data2.WorkerID) && data1.ExpectedVersion.Cmp(data2.ExpectedVersion) == 0 && data1.LeaseUntil.Cmp(data2.LeaseUntil) == 0 && data1.CurrentTime.Cmp(data2.CurrentTime) == 0
		}
	case Command_CommandObserveEffect:
		{
			data2, ok := other.Get_().(Command_CommandObserveEffect)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.WorkerID.Equals(data2.WorkerID) && data1.ClaimEpoch.Cmp(data2.ClaimEpoch) == 0 && data1.Outcome.Equals(data2.Outcome)
		}
	case Command_CommandRetryEffect:
		{
			data2, ok := other.Get_().(Command_CommandRetryEffect)
			return ok && data1.EffectID.Equals(data2.EffectID)
		}
	case Command_CommandResolveUnknown:
		{
			data2, ok := other.Get_().(Command_CommandResolveUnknown)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.Caps.Equals(data2.Caps)
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this Command) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(Command)
	return ok && _this.Equals(typed)
}

func Type_Command_() _dafny.TypeDescriptor {
	return type_Command_{}
}

type type_Command_ struct {
}

func (_this type_Command_) Default() interface{} {
	return Companion_Command_.Default()
}

func (_this type_Command_) String() string {
	return "EffectReducer.Command"
}
func (_this Command) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Command{}

// End of datatype Command

// Definition of datatype Event
type Event struct {
	Data_Event_
}

func (_this Event) Get_() Data_Event_ {
	return _this.Data_Event_
}

type Data_Event_ interface {
	isEvent()
}

type CompanionStruct_Event_ struct {
}

var Companion_Event_ = CompanionStruct_Event_{}

type Event_EventEffectClaimed struct {
	EffectID   _dafny.Sequence
	WorkerID   _dafny.Sequence
	ClaimEpoch _dafny.Int
	LeaseUntil _dafny.Int
}

func (Event_EventEffectClaimed) isEvent() {}

func (CompanionStruct_Event_) Create_EventEffectClaimed_(EffectID _dafny.Sequence, WorkerID _dafny.Sequence, ClaimEpoch _dafny.Int, LeaseUntil _dafny.Int) Event {
	return Event{Event_EventEffectClaimed{EffectID, WorkerID, ClaimEpoch, LeaseUntil}}
}

func (_this Event) Is_EventEffectClaimed() bool {
	_, ok := _this.Get_().(Event_EventEffectClaimed)
	return ok
}

type Event_EventEffectObserved struct {
	EffectID   _dafny.Sequence
	ClaimEpoch _dafny.Int
	Outcome    Outcome
}

func (Event_EventEffectObserved) isEvent() {}

func (CompanionStruct_Event_) Create_EventEffectObserved_(EffectID _dafny.Sequence, ClaimEpoch _dafny.Int, Outcome Outcome) Event {
	return Event{Event_EventEffectObserved{EffectID, ClaimEpoch, Outcome}}
}

func (_this Event) Is_EventEffectObserved() bool {
	_, ok := _this.Get_().(Event_EventEffectObserved)
	return ok
}

type Event_EventEffectRetried struct {
	EffectID _dafny.Sequence
}

func (Event_EventEffectRetried) isEvent() {}

func (CompanionStruct_Event_) Create_EventEffectRetried_(EffectID _dafny.Sequence) Event {
	return Event{Event_EventEffectRetried{EffectID}}
}

func (_this Event) Is_EventEffectRetried() bool {
	_, ok := _this.Get_().(Event_EventEffectRetried)
	return ok
}

type Event_EventEffectTerminalized struct {
	EffectID _dafny.Sequence
	Reason   _dafny.Sequence
}

func (Event_EventEffectTerminalized) isEvent() {}

func (CompanionStruct_Event_) Create_EventEffectTerminalized_(EffectID _dafny.Sequence, Reason _dafny.Sequence) Event {
	return Event{Event_EventEffectTerminalized{EffectID, Reason}}
}

func (_this Event) Is_EventEffectTerminalized() bool {
	_, ok := _this.Get_().(Event_EventEffectTerminalized)
	return ok
}

type Event_EventEffectRejected struct {
	EffectID _dafny.Sequence
	Reason   _dafny.Sequence
}

func (Event_EventEffectRejected) isEvent() {}

func (CompanionStruct_Event_) Create_EventEffectRejected_(EffectID _dafny.Sequence, Reason _dafny.Sequence) Event {
	return Event{Event_EventEffectRejected{EffectID, Reason}}
}

func (_this Event) Is_EventEffectRejected() bool {
	_, ok := _this.Get_().(Event_EventEffectRejected)
	return ok
}

type Event_EventManualResolutionRequired struct {
	EffectID _dafny.Sequence
}

func (Event_EventManualResolutionRequired) isEvent() {}

func (CompanionStruct_Event_) Create_EventManualResolutionRequired_(EffectID _dafny.Sequence) Event {
	return Event{Event_EventManualResolutionRequired{EffectID}}
}

func (_this Event) Is_EventManualResolutionRequired() bool {
	_, ok := _this.Get_().(Event_EventManualResolutionRequired)
	return ok
}

type Event_EventReconciliationRequired struct {
	EffectID _dafny.Sequence
}

func (Event_EventReconciliationRequired) isEvent() {}

func (CompanionStruct_Event_) Create_EventReconciliationRequired_(EffectID _dafny.Sequence) Event {
	return Event{Event_EventReconciliationRequired{EffectID}}
}

func (_this Event) Is_EventReconciliationRequired() bool {
	_, ok := _this.Get_().(Event_EventReconciliationRequired)
	return ok
}

type Event_EventRetryAuthorized struct {
	EffectID _dafny.Sequence
}

func (Event_EventRetryAuthorized) isEvent() {}

func (CompanionStruct_Event_) Create_EventRetryAuthorized_(EffectID _dafny.Sequence) Event {
	return Event{Event_EventRetryAuthorized{EffectID}}
}

func (_this Event) Is_EventRetryAuthorized() bool {
	_, ok := _this.Get_().(Event_EventRetryAuthorized)
	return ok
}

func (CompanionStruct_Event_) Default() Event {
	return Companion_Event_.Create_EventEffectClaimed_(_dafny.EmptySeq, _dafny.EmptySeq, _dafny.Zero, _dafny.Zero)
}

func (_this Event) Dtor_effectID() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Event_EventEffectClaimed:
		return data.EffectID
	case Event_EventEffectObserved:
		return data.EffectID
	case Event_EventEffectRetried:
		return data.EffectID
	case Event_EventEffectTerminalized:
		return data.EffectID
	case Event_EventEffectRejected:
		return data.EffectID
	case Event_EventManualResolutionRequired:
		return data.EffectID
	case Event_EventReconciliationRequired:
		return data.EffectID
	default:
		return data.(Event_EventRetryAuthorized).EffectID
	}
}

func (_this Event) Dtor_workerID() _dafny.Sequence {
	return _this.Get_().(Event_EventEffectClaimed).WorkerID
}

func (_this Event) Dtor_claimEpoch() _dafny.Int {
	switch data := _this.Get_().(type) {
	case Event_EventEffectClaimed:
		return data.ClaimEpoch
	default:
		return data.(Event_EventEffectObserved).ClaimEpoch
	}
}

func (_this Event) Dtor_leaseUntil() _dafny.Int {
	return _this.Get_().(Event_EventEffectClaimed).LeaseUntil
}

func (_this Event) Dtor_outcome() Outcome {
	return _this.Get_().(Event_EventEffectObserved).Outcome
}

func (_this Event) Dtor_reason() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Event_EventEffectTerminalized:
		return data.Reason
	default:
		return data.(Event_EventEffectRejected).Reason
	}
}

func (_this Event) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Event_EventEffectClaimed:
		{
			return "EffectReducer.Event.EventEffectClaimed" + "(" + data.EffectID.VerbatimString(true) + ", " + data.WorkerID.VerbatimString(true) + ", " + _dafny.String(data.ClaimEpoch) + ", " + _dafny.String(data.LeaseUntil) + ")"
		}
	case Event_EventEffectObserved:
		{
			return "EffectReducer.Event.EventEffectObserved" + "(" + data.EffectID.VerbatimString(true) + ", " + _dafny.String(data.ClaimEpoch) + ", " + _dafny.String(data.Outcome) + ")"
		}
	case Event_EventEffectRetried:
		{
			return "EffectReducer.Event.EventEffectRetried" + "(" + data.EffectID.VerbatimString(true) + ")"
		}
	case Event_EventEffectTerminalized:
		{
			return "EffectReducer.Event.EventEffectTerminalized" + "(" + data.EffectID.VerbatimString(true) + ", " + data.Reason.VerbatimString(true) + ")"
		}
	case Event_EventEffectRejected:
		{
			return "EffectReducer.Event.EventEffectRejected" + "(" + data.EffectID.VerbatimString(true) + ", " + data.Reason.VerbatimString(true) + ")"
		}
	case Event_EventManualResolutionRequired:
		{
			return "EffectReducer.Event.EventManualResolutionRequired" + "(" + data.EffectID.VerbatimString(true) + ")"
		}
	case Event_EventReconciliationRequired:
		{
			return "EffectReducer.Event.EventReconciliationRequired" + "(" + data.EffectID.VerbatimString(true) + ")"
		}
	case Event_EventRetryAuthorized:
		{
			return "EffectReducer.Event.EventRetryAuthorized" + "(" + data.EffectID.VerbatimString(true) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Event) Equals(other Event) bool {
	switch data1 := _this.Get_().(type) {
	case Event_EventEffectClaimed:
		{
			data2, ok := other.Get_().(Event_EventEffectClaimed)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.WorkerID.Equals(data2.WorkerID) && data1.ClaimEpoch.Cmp(data2.ClaimEpoch) == 0 && data1.LeaseUntil.Cmp(data2.LeaseUntil) == 0
		}
	case Event_EventEffectObserved:
		{
			data2, ok := other.Get_().(Event_EventEffectObserved)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.ClaimEpoch.Cmp(data2.ClaimEpoch) == 0 && data1.Outcome.Equals(data2.Outcome)
		}
	case Event_EventEffectRetried:
		{
			data2, ok := other.Get_().(Event_EventEffectRetried)
			return ok && data1.EffectID.Equals(data2.EffectID)
		}
	case Event_EventEffectTerminalized:
		{
			data2, ok := other.Get_().(Event_EventEffectTerminalized)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.Reason.Equals(data2.Reason)
		}
	case Event_EventEffectRejected:
		{
			data2, ok := other.Get_().(Event_EventEffectRejected)
			return ok && data1.EffectID.Equals(data2.EffectID) && data1.Reason.Equals(data2.Reason)
		}
	case Event_EventManualResolutionRequired:
		{
			data2, ok := other.Get_().(Event_EventManualResolutionRequired)
			return ok && data1.EffectID.Equals(data2.EffectID)
		}
	case Event_EventReconciliationRequired:
		{
			data2, ok := other.Get_().(Event_EventReconciliationRequired)
			return ok && data1.EffectID.Equals(data2.EffectID)
		}
	case Event_EventRetryAuthorized:
		{
			data2, ok := other.Get_().(Event_EventRetryAuthorized)
			return ok && data1.EffectID.Equals(data2.EffectID)
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this Event) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(Event)
	return ok && _this.Equals(typed)
}

func Type_Event_() _dafny.TypeDescriptor {
	return type_Event_{}
}

type type_Event_ struct {
}

func (_this type_Event_) Default() interface{} {
	return Companion_Event_.Default()
}

func (_this type_Event_) String() string {
	return "EffectReducer.Event"
}
func (_this Event) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Event{}

// End of datatype Event

// Definition of datatype DecisionStatus
type DecisionStatus struct {
	Data_DecisionStatus_
}

func (_this DecisionStatus) Get_() Data_DecisionStatus_ {
	return _this.Data_DecisionStatus_
}

type Data_DecisionStatus_ interface {
	isDecisionStatus()
}

type CompanionStruct_DecisionStatus_ struct {
}

var Companion_DecisionStatus_ = CompanionStruct_DecisionStatus_{}

type DecisionStatus_Accepted struct {
}

func (DecisionStatus_Accepted) isDecisionStatus() {}

func (CompanionStruct_DecisionStatus_) Create_Accepted_() DecisionStatus {
	return DecisionStatus{DecisionStatus_Accepted{}}
}

func (_this DecisionStatus) Is_Accepted() bool {
	_, ok := _this.Get_().(DecisionStatus_Accepted)
	return ok
}

type DecisionStatus_Rejected struct {
}

func (DecisionStatus_Rejected) isDecisionStatus() {}

func (CompanionStruct_DecisionStatus_) Create_Rejected_() DecisionStatus {
	return DecisionStatus{DecisionStatus_Rejected{}}
}

func (_this DecisionStatus) Is_Rejected() bool {
	_, ok := _this.Get_().(DecisionStatus_Rejected)
	return ok
}

type DecisionStatus_NoOp struct {
}

func (DecisionStatus_NoOp) isDecisionStatus() {}

func (CompanionStruct_DecisionStatus_) Create_NoOp_() DecisionStatus {
	return DecisionStatus{DecisionStatus_NoOp{}}
}

func (_this DecisionStatus) Is_NoOp() bool {
	_, ok := _this.Get_().(DecisionStatus_NoOp)
	return ok
}

func (CompanionStruct_DecisionStatus_) Default() DecisionStatus {
	return Companion_DecisionStatus_.Create_Accepted_()
}

func (_ CompanionStruct_DecisionStatus_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_DecisionStatus_.Create_Accepted_(), true
		case 1:
			return Companion_DecisionStatus_.Create_Rejected_(), true
		case 2:
			return Companion_DecisionStatus_.Create_NoOp_(), true
		default:
			return DecisionStatus{}, false
		}
	}
}

func (_this DecisionStatus) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case DecisionStatus_Accepted:
		{
			return "EffectReducer.DecisionStatus.Accepted"
		}
	case DecisionStatus_Rejected:
		{
			return "EffectReducer.DecisionStatus.Rejected"
		}
	case DecisionStatus_NoOp:
		{
			return "EffectReducer.DecisionStatus.NoOp"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this DecisionStatus) Equals(other DecisionStatus) bool {
	switch _this.Get_().(type) {
	case DecisionStatus_Accepted:
		{
			_, ok := other.Get_().(DecisionStatus_Accepted)
			return ok
		}
	case DecisionStatus_Rejected:
		{
			_, ok := other.Get_().(DecisionStatus_Rejected)
			return ok
		}
	case DecisionStatus_NoOp:
		{
			_, ok := other.Get_().(DecisionStatus_NoOp)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this DecisionStatus) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(DecisionStatus)
	return ok && _this.Equals(typed)
}

func Type_DecisionStatus_() _dafny.TypeDescriptor {
	return type_DecisionStatus_{}
}

type type_DecisionStatus_ struct {
}

func (_this type_DecisionStatus_) Default() interface{} {
	return Companion_DecisionStatus_.Default()
}

func (_this type_DecisionStatus_) String() string {
	return "EffectReducer.DecisionStatus"
}
func (_this DecisionStatus) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = DecisionStatus{}

// End of datatype DecisionStatus

// Definition of datatype Decision
type Decision struct {
	Data_Decision_
}

func (_this Decision) Get_() Data_Decision_ {
	return _this.Data_Decision_
}

type Data_Decision_ interface {
	isDecision()
}

type CompanionStruct_Decision_ struct {
}

var Companion_Decision_ = CompanionStruct_Decision_{}

type Decision_Decision struct {
	Status DecisionStatus
	Events _dafny.Sequence
}

func (Decision_Decision) isDecision() {}

func (CompanionStruct_Decision_) Create_Decision_(Status DecisionStatus, Events _dafny.Sequence) Decision {
	return Decision{Decision_Decision{Status, Events}}
}

func (_this Decision) Is_Decision() bool {
	_, ok := _this.Get_().(Decision_Decision)
	return ok
}

func (CompanionStruct_Decision_) Default() Decision {
	return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Default(), _dafny.EmptySeq)
}

func (_this Decision) Dtor_status() DecisionStatus {
	return _this.Get_().(Decision_Decision).Status
}

func (_this Decision) Dtor_events() _dafny.Sequence {
	return _this.Get_().(Decision_Decision).Events
}

func (_this Decision) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Decision_Decision:
		{
			return "EffectReducer.Decision.Decision" + "(" + _dafny.String(data.Status) + ", " + _dafny.String(data.Events) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Decision) Equals(other Decision) bool {
	switch data1 := _this.Get_().(type) {
	case Decision_Decision:
		{
			data2, ok := other.Get_().(Decision_Decision)
			return ok && data1.Status.Equals(data2.Status) && data1.Events.Equals(data2.Events)
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this Decision) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(Decision)
	return ok && _this.Equals(typed)
}

func Type_Decision_() _dafny.TypeDescriptor {
	return type_Decision_{}
}

type type_Decision_ struct {
}

func (_this type_Decision_) Default() interface{} {
	return Companion_Decision_.Default()
}

func (_this type_Decision_) String() string {
	return "EffectReducer.Decision"
}
func (_this Decision) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Decision{}

// End of datatype Decision
