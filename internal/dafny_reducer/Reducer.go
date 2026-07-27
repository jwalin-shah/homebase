// Package Reducer
// Dafny module Reducer compiled into Go

package dafny_reducer

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

func (_static *CompanionStruct_Default___) Decide(state AttemptState, cmd Command) Decision {
	var _source0 Command = cmd
	_ = _source0
	{
		if _source0.Is_CommandConclude() {
			var _0_attemptID _dafny.Sequence = _source0.Get_().(Command_CommandConclude).AttemptID
			_ = _0_attemptID
			if ((state).Dtor_phase()).Equals(Companion_Phase_.Create_Concluded_()) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_NoOp_(), _dafny.SeqOf(), _dafny.SeqOf())
			} else {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventConcluded_(_0_attemptID)), _dafny.SeqOf())
			}
		}
	}
	{
		if _source0.Is_CommandUnknown() {
			var _1_attemptID _dafny.Sequence = _source0.Get_().(Command_CommandUnknown).AttemptID
			_ = _1_attemptID
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(), _dafny.SeqOf())
		}
	}
	{
		var _2_attemptID _dafny.Sequence = _source0.Get_().(Command_CommandProposeRecovery).AttemptID
		_ = _2_attemptID
		var _3_idempotencyKey _dafny.Sequence = _source0.Get_().(Command_CommandProposeRecovery).IdempotencyKey
		_ = _3_idempotencyKey
		var _4_version _dafny.Int = _source0.Get_().(Command_CommandProposeRecovery).Version
		_ = _4_version
		if ((_4_version).Sign() == 1) && ((_4_version).Cmp((state).Dtor_version()) < 0) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventRecoveryRejected_(_2_attemptID, _dafny.UnicodeSeqOfUtf8Bytes("stale command"))), _dafny.SeqOf())
		} else if ((state).Dtor_phase()).Equals(Companion_Phase_.Create_Concluded_()) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventRecoveryRejected_(_2_attemptID, _dafny.UnicodeSeqOfUtf8Bytes("attempt already concluded"))), _dafny.SeqOf())
		} else if ((state).Dtor_processedCmdKeys()).Contains(_3_idempotencyKey) {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_NoOp_(), _dafny.SeqOf(), _dafny.SeqOf())
		} else if ((state).Dtor_recoveryDispatches()).Cmp(_dafny.IntOfInt64(2)) >= 0 {
			return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventRecoveryRejected_(_2_attemptID, _dafny.UnicodeSeqOfUtf8Bytes("recovery limit reached"))), _dafny.SeqOf())
		} else {
			var _5_effectID _dafny.Sequence = _dafny.Companion_Sequence_.Concatenate(_dafny.Companion_Sequence_.Concatenate(_2_attemptID, _dafny.UnicodeSeqOfUtf8Bytes("-effect-")), _3_idempotencyKey)
			_ = _5_effectID
			if ((state).Dtor_dispatchedEffects()).Contains(_5_effectID) {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Rejected_(), _dafny.SeqOf(Companion_Event_.Create_EventRecoveryRejected_(_2_attemptID, _dafny.UnicodeSeqOfUtf8Bytes("effect collision"))), _dafny.SeqOf())
			} else {
				return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Create_Accepted_(), _dafny.SeqOf(Companion_Event_.Create_EventRecoveryDispatched_(_2_attemptID, _5_effectID, ((state).Dtor_recoveryDispatches()).Plus(_dafny.One), _3_idempotencyKey)), _dafny.SeqOf(Companion_EffectIntent_.Create_EffectIntent_(_2_attemptID, _5_effectID, ((state).Dtor_recoveryDispatches()).Plus(_dafny.One))))
			}
		}
	}
}
func (_static *CompanionStruct_Default___) Apply(state AttemptState, event Event) AttemptState {
	var _source0 Event = event
	_ = _source0
	{
		if _source0.Is_EventConcluded() {
			return Companion_AttemptState_.Create_AttemptState_(Companion_Phase_.Create_Concluded_(), (state).Dtor_recoveryDispatches(), (state).Dtor_dispatchedEffects(), (state).Dtor_processedCmdKeys(), (state).Dtor_version())
		}
	}
	{
		if _source0.Is_EventRecoveryDispatched() {
			var _0_effectID _dafny.Sequence = _source0.Get_().(Event_EventRecoveryDispatched).EffectID
			_ = _0_effectID
			var _1_idempotencyKey _dafny.Sequence = _source0.Get_().(Event_EventRecoveryDispatched).IdempotencyKey
			_ = _1_idempotencyKey
			return Companion_AttemptState_.Create_AttemptState_(Companion_Phase_.Create_Recovering_(), ((state).Dtor_recoveryDispatches()).Plus(_dafny.One), ((state).Dtor_dispatchedEffects()).Union(_dafny.SetOf(_0_effectID)), ((state).Dtor_processedCmdKeys()).Union(_dafny.SetOf(_1_idempotencyKey)), (state).Dtor_version())
		}
	}
	{
		return state
	}
}
func (_static *CompanionStruct_Default___) ApplyBatch(state AttemptState, events _dafny.Sequence) AttemptState {
	goto TAIL_CALL_START
TAIL_CALL_START:
	if (_dafny.IntOfUint32((events).Cardinality())).Sign() == 0 {
		return state
	} else {
		var _in0 AttemptState = Companion_Default___.Apply(state, (events).Select(0).(Event))
		_ = _in0
		var _in1 _dafny.Sequence = (events).Drop(1)
		_ = _in1
		state = _in0
		events = _in1
		goto TAIL_CALL_START
	}
}

// End of class Default__

// Definition of datatype Phase
type Phase struct {
	Data_Phase_
}

func (_this Phase) Get_() Data_Phase_ {
	return _this.Data_Phase_
}

type Data_Phase_ interface {
	isPhase()
}

type CompanionStruct_Phase_ struct {
}

var Companion_Phase_ = CompanionStruct_Phase_{}

type Phase_Active struct {
}

func (Phase_Active) isPhase() {}

func (CompanionStruct_Phase_) Create_Active_() Phase {
	return Phase{Phase_Active{}}
}

func (_this Phase) Is_Active() bool {
	_, ok := _this.Get_().(Phase_Active)
	return ok
}

type Phase_Recovering struct {
}

func (Phase_Recovering) isPhase() {}

func (CompanionStruct_Phase_) Create_Recovering_() Phase {
	return Phase{Phase_Recovering{}}
}

func (_this Phase) Is_Recovering() bool {
	_, ok := _this.Get_().(Phase_Recovering)
	return ok
}

type Phase_Concluded struct {
}

func (Phase_Concluded) isPhase() {}

func (CompanionStruct_Phase_) Create_Concluded_() Phase {
	return Phase{Phase_Concluded{}}
}

func (_this Phase) Is_Concluded() bool {
	_, ok := _this.Get_().(Phase_Concluded)
	return ok
}

func (CompanionStruct_Phase_) Default() Phase {
	return Companion_Phase_.Create_Active_()
}

func (_ CompanionStruct_Phase_) AllSingletonConstructors() _dafny.Iterator {
	i := -1
	return func() (interface{}, bool) {
		i++
		switch i {
		case 0:
			return Companion_Phase_.Create_Active_(), true
		case 1:
			return Companion_Phase_.Create_Recovering_(), true
		case 2:
			return Companion_Phase_.Create_Concluded_(), true
		default:
			return Phase{}, false
		}
	}
}

func (_this Phase) String() string {
	switch _this.Get_().(type) {
	case nil:
		return "null"
	case Phase_Active:
		{
			return "Reducer.Phase.Active"
		}
	case Phase_Recovering:
		{
			return "Reducer.Phase.Recovering"
		}
	case Phase_Concluded:
		{
			return "Reducer.Phase.Concluded"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Phase) Equals(other Phase) bool {
	switch _this.Get_().(type) {
	case Phase_Active:
		{
			_, ok := other.Get_().(Phase_Active)
			return ok
		}
	case Phase_Recovering:
		{
			_, ok := other.Get_().(Phase_Recovering)
			return ok
		}
	case Phase_Concluded:
		{
			_, ok := other.Get_().(Phase_Concluded)
			return ok
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this Phase) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(Phase)
	return ok && _this.Equals(typed)
}

func Type_Phase_() _dafny.TypeDescriptor {
	return type_Phase_{}
}

type type_Phase_ struct {
}

func (_this type_Phase_) Default() interface{} {
	return Companion_Phase_.Default()
}

func (_this type_Phase_) String() string {
	return "Reducer.Phase"
}
func (_this Phase) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Phase{}

// End of datatype Phase

// Definition of datatype AttemptState
type AttemptState struct {
	Data_AttemptState_
}

func (_this AttemptState) Get_() Data_AttemptState_ {
	return _this.Data_AttemptState_
}

type Data_AttemptState_ interface {
	isAttemptState()
}

type CompanionStruct_AttemptState_ struct {
}

var Companion_AttemptState_ = CompanionStruct_AttemptState_{}

type AttemptState_AttemptState struct {
	Phase              Phase
	RecoveryDispatches _dafny.Int
	DispatchedEffects  _dafny.Set
	ProcessedCmdKeys   _dafny.Set
	Version            _dafny.Int
}

func (AttemptState_AttemptState) isAttemptState() {}

func (CompanionStruct_AttemptState_) Create_AttemptState_(Phase Phase, RecoveryDispatches _dafny.Int, DispatchedEffects _dafny.Set, ProcessedCmdKeys _dafny.Set, Version _dafny.Int) AttemptState {
	return AttemptState{AttemptState_AttemptState{Phase, RecoveryDispatches, DispatchedEffects, ProcessedCmdKeys, Version}}
}

func (_this AttemptState) Is_AttemptState() bool {
	_, ok := _this.Get_().(AttemptState_AttemptState)
	return ok
}

func (CompanionStruct_AttemptState_) Default() AttemptState {
	return Companion_AttemptState_.Create_AttemptState_(Companion_Phase_.Default(), _dafny.Zero, _dafny.EmptySet, _dafny.EmptySet, _dafny.Zero)
}

func (_this AttemptState) Dtor_phase() Phase {
	return _this.Get_().(AttemptState_AttemptState).Phase
}

func (_this AttemptState) Dtor_recoveryDispatches() _dafny.Int {
	return _this.Get_().(AttemptState_AttemptState).RecoveryDispatches
}

func (_this AttemptState) Dtor_dispatchedEffects() _dafny.Set {
	return _this.Get_().(AttemptState_AttemptState).DispatchedEffects
}

func (_this AttemptState) Dtor_processedCmdKeys() _dafny.Set {
	return _this.Get_().(AttemptState_AttemptState).ProcessedCmdKeys
}

func (_this AttemptState) Dtor_version() _dafny.Int {
	return _this.Get_().(AttemptState_AttemptState).Version
}

func (_this AttemptState) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case AttemptState_AttemptState:
		{
			return "Reducer.AttemptState.AttemptState" + "(" + _dafny.String(data.Phase) + ", " + _dafny.String(data.RecoveryDispatches) + ", " + _dafny.String(data.DispatchedEffects) + ", " + _dafny.String(data.ProcessedCmdKeys) + ", " + _dafny.String(data.Version) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this AttemptState) Equals(other AttemptState) bool {
	switch data1 := _this.Get_().(type) {
	case AttemptState_AttemptState:
		{
			data2, ok := other.Get_().(AttemptState_AttemptState)
			return ok && data1.Phase.Equals(data2.Phase) && data1.RecoveryDispatches.Cmp(data2.RecoveryDispatches) == 0 && data1.DispatchedEffects.Equals(data2.DispatchedEffects) && data1.ProcessedCmdKeys.Equals(data2.ProcessedCmdKeys) && data1.Version.Cmp(data2.Version) == 0
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this AttemptState) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(AttemptState)
	return ok && _this.Equals(typed)
}

func Type_AttemptState_() _dafny.TypeDescriptor {
	return type_AttemptState_{}
}

type type_AttemptState_ struct {
}

func (_this type_AttemptState_) Default() interface{} {
	return Companion_AttemptState_.Default()
}

func (_this type_AttemptState_) String() string {
	return "Reducer.AttemptState"
}
func (_this AttemptState) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = AttemptState{}

// End of datatype AttemptState

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

type Command_CommandProposeRecovery struct {
	AttemptID      _dafny.Sequence
	IdempotencyKey _dafny.Sequence
	Version        _dafny.Int
}

func (Command_CommandProposeRecovery) isCommand() {}

func (CompanionStruct_Command_) Create_CommandProposeRecovery_(AttemptID _dafny.Sequence, IdempotencyKey _dafny.Sequence, Version _dafny.Int) Command {
	return Command{Command_CommandProposeRecovery{AttemptID, IdempotencyKey, Version}}
}

func (_this Command) Is_CommandProposeRecovery() bool {
	_, ok := _this.Get_().(Command_CommandProposeRecovery)
	return ok
}

type Command_CommandConclude struct {
	AttemptID _dafny.Sequence
}

func (Command_CommandConclude) isCommand() {}

func (CompanionStruct_Command_) Create_CommandConclude_(AttemptID _dafny.Sequence) Command {
	return Command{Command_CommandConclude{AttemptID}}
}

func (_this Command) Is_CommandConclude() bool {
	_, ok := _this.Get_().(Command_CommandConclude)
	return ok
}

type Command_CommandUnknown struct {
	AttemptID _dafny.Sequence
}

func (Command_CommandUnknown) isCommand() {}

func (CompanionStruct_Command_) Create_CommandUnknown_(AttemptID _dafny.Sequence) Command {
	return Command{Command_CommandUnknown{AttemptID}}
}

func (_this Command) Is_CommandUnknown() bool {
	_, ok := _this.Get_().(Command_CommandUnknown)
	return ok
}

func (CompanionStruct_Command_) Default() Command {
	return Companion_Command_.Create_CommandProposeRecovery_(_dafny.EmptySeq, _dafny.EmptySeq, _dafny.Zero)
}

func (_this Command) Dtor_attemptID() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Command_CommandProposeRecovery:
		return data.AttemptID
	case Command_CommandConclude:
		return data.AttemptID
	default:
		return data.(Command_CommandUnknown).AttemptID
	}
}

func (_this Command) Dtor_idempotencyKey() _dafny.Sequence {
	return _this.Get_().(Command_CommandProposeRecovery).IdempotencyKey
}

func (_this Command) Dtor_version() _dafny.Int {
	return _this.Get_().(Command_CommandProposeRecovery).Version
}

func (_this Command) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Command_CommandProposeRecovery:
		{
			return "Reducer.Command.CommandProposeRecovery" + "(" + data.AttemptID.VerbatimString(true) + ", " + data.IdempotencyKey.VerbatimString(true) + ", " + _dafny.String(data.Version) + ")"
		}
	case Command_CommandConclude:
		{
			return "Reducer.Command.CommandConclude" + "(" + data.AttemptID.VerbatimString(true) + ")"
		}
	case Command_CommandUnknown:
		{
			return "Reducer.Command.CommandUnknown" + "(" + data.AttemptID.VerbatimString(true) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Command) Equals(other Command) bool {
	switch data1 := _this.Get_().(type) {
	case Command_CommandProposeRecovery:
		{
			data2, ok := other.Get_().(Command_CommandProposeRecovery)
			return ok && data1.AttemptID.Equals(data2.AttemptID) && data1.IdempotencyKey.Equals(data2.IdempotencyKey) && data1.Version.Cmp(data2.Version) == 0
		}
	case Command_CommandConclude:
		{
			data2, ok := other.Get_().(Command_CommandConclude)
			return ok && data1.AttemptID.Equals(data2.AttemptID)
		}
	case Command_CommandUnknown:
		{
			data2, ok := other.Get_().(Command_CommandUnknown)
			return ok && data1.AttemptID.Equals(data2.AttemptID)
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
	return "Reducer.Command"
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

type Event_EventRecoveryDispatched struct {
	AttemptID      _dafny.Sequence
	EffectID       _dafny.Sequence
	Ordinal        _dafny.Int
	IdempotencyKey _dafny.Sequence
}

func (Event_EventRecoveryDispatched) isEvent() {}

func (CompanionStruct_Event_) Create_EventRecoveryDispatched_(AttemptID _dafny.Sequence, EffectID _dafny.Sequence, Ordinal _dafny.Int, IdempotencyKey _dafny.Sequence) Event {
	return Event{Event_EventRecoveryDispatched{AttemptID, EffectID, Ordinal, IdempotencyKey}}
}

func (_this Event) Is_EventRecoveryDispatched() bool {
	_, ok := _this.Get_().(Event_EventRecoveryDispatched)
	return ok
}

type Event_EventRecoveryRejected struct {
	AttemptID _dafny.Sequence
	Reason    _dafny.Sequence
}

func (Event_EventRecoveryRejected) isEvent() {}

func (CompanionStruct_Event_) Create_EventRecoveryRejected_(AttemptID _dafny.Sequence, Reason _dafny.Sequence) Event {
	return Event{Event_EventRecoveryRejected{AttemptID, Reason}}
}

func (_this Event) Is_EventRecoveryRejected() bool {
	_, ok := _this.Get_().(Event_EventRecoveryRejected)
	return ok
}

type Event_EventConcluded struct {
	AttemptID _dafny.Sequence
}

func (Event_EventConcluded) isEvent() {}

func (CompanionStruct_Event_) Create_EventConcluded_(AttemptID _dafny.Sequence) Event {
	return Event{Event_EventConcluded{AttemptID}}
}

func (_this Event) Is_EventConcluded() bool {
	_, ok := _this.Get_().(Event_EventConcluded)
	return ok
}

func (CompanionStruct_Event_) Default() Event {
	return Companion_Event_.Create_EventRecoveryDispatched_(_dafny.EmptySeq, _dafny.EmptySeq, _dafny.Zero, _dafny.EmptySeq)
}

func (_this Event) Dtor_attemptID() _dafny.Sequence {
	switch data := _this.Get_().(type) {
	case Event_EventRecoveryDispatched:
		return data.AttemptID
	case Event_EventRecoveryRejected:
		return data.AttemptID
	default:
		return data.(Event_EventConcluded).AttemptID
	}
}

func (_this Event) Dtor_effectID() _dafny.Sequence {
	return _this.Get_().(Event_EventRecoveryDispatched).EffectID
}

func (_this Event) Dtor_ordinal() _dafny.Int {
	return _this.Get_().(Event_EventRecoveryDispatched).Ordinal
}

func (_this Event) Dtor_idempotencyKey() _dafny.Sequence {
	return _this.Get_().(Event_EventRecoveryDispatched).IdempotencyKey
}

func (_this Event) Dtor_reason() _dafny.Sequence {
	return _this.Get_().(Event_EventRecoveryRejected).Reason
}

func (_this Event) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Event_EventRecoveryDispatched:
		{
			return "Reducer.Event.EventRecoveryDispatched" + "(" + data.AttemptID.VerbatimString(true) + ", " + data.EffectID.VerbatimString(true) + ", " + _dafny.String(data.Ordinal) + ", " + data.IdempotencyKey.VerbatimString(true) + ")"
		}
	case Event_EventRecoveryRejected:
		{
			return "Reducer.Event.EventRecoveryRejected" + "(" + data.AttemptID.VerbatimString(true) + ", " + data.Reason.VerbatimString(true) + ")"
		}
	case Event_EventConcluded:
		{
			return "Reducer.Event.EventConcluded" + "(" + data.AttemptID.VerbatimString(true) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this Event) Equals(other Event) bool {
	switch data1 := _this.Get_().(type) {
	case Event_EventRecoveryDispatched:
		{
			data2, ok := other.Get_().(Event_EventRecoveryDispatched)
			return ok && data1.AttemptID.Equals(data2.AttemptID) && data1.EffectID.Equals(data2.EffectID) && data1.Ordinal.Cmp(data2.Ordinal) == 0 && data1.IdempotencyKey.Equals(data2.IdempotencyKey)
		}
	case Event_EventRecoveryRejected:
		{
			data2, ok := other.Get_().(Event_EventRecoveryRejected)
			return ok && data1.AttemptID.Equals(data2.AttemptID) && data1.Reason.Equals(data2.Reason)
		}
	case Event_EventConcluded:
		{
			data2, ok := other.Get_().(Event_EventConcluded)
			return ok && data1.AttemptID.Equals(data2.AttemptID)
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
	return "Reducer.Event"
}
func (_this Event) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Event{}

// End of datatype Event

// Definition of datatype EffectIntent
type EffectIntent struct {
	Data_EffectIntent_
}

func (_this EffectIntent) Get_() Data_EffectIntent_ {
	return _this.Data_EffectIntent_
}

type Data_EffectIntent_ interface {
	isEffectIntent()
}

type CompanionStruct_EffectIntent_ struct {
}

var Companion_EffectIntent_ = CompanionStruct_EffectIntent_{}

type EffectIntent_EffectIntent struct {
	AttemptID _dafny.Sequence
	EffectID  _dafny.Sequence
	Ordinal   _dafny.Int
}

func (EffectIntent_EffectIntent) isEffectIntent() {}

func (CompanionStruct_EffectIntent_) Create_EffectIntent_(AttemptID _dafny.Sequence, EffectID _dafny.Sequence, Ordinal _dafny.Int) EffectIntent {
	return EffectIntent{EffectIntent_EffectIntent{AttemptID, EffectID, Ordinal}}
}

func (_this EffectIntent) Is_EffectIntent() bool {
	_, ok := _this.Get_().(EffectIntent_EffectIntent)
	return ok
}

func (CompanionStruct_EffectIntent_) Default() EffectIntent {
	return Companion_EffectIntent_.Create_EffectIntent_(_dafny.EmptySeq, _dafny.EmptySeq, _dafny.Zero)
}

func (_this EffectIntent) Dtor_attemptID() _dafny.Sequence {
	return _this.Get_().(EffectIntent_EffectIntent).AttemptID
}

func (_this EffectIntent) Dtor_effectID() _dafny.Sequence {
	return _this.Get_().(EffectIntent_EffectIntent).EffectID
}

func (_this EffectIntent) Dtor_ordinal() _dafny.Int {
	return _this.Get_().(EffectIntent_EffectIntent).Ordinal
}

func (_this EffectIntent) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case EffectIntent_EffectIntent:
		{
			return "Reducer.EffectIntent.EffectIntent" + "(" + data.AttemptID.VerbatimString(true) + ", " + data.EffectID.VerbatimString(true) + ", " + _dafny.String(data.Ordinal) + ")"
		}
	default:
		{
			return "<unexpected>"
		}
	}
}

func (_this EffectIntent) Equals(other EffectIntent) bool {
	switch data1 := _this.Get_().(type) {
	case EffectIntent_EffectIntent:
		{
			data2, ok := other.Get_().(EffectIntent_EffectIntent)
			return ok && data1.AttemptID.Equals(data2.AttemptID) && data1.EffectID.Equals(data2.EffectID) && data1.Ordinal.Cmp(data2.Ordinal) == 0
		}
	default:
		{
			return false // unexpected
		}
	}
}

func (_this EffectIntent) EqualsGeneric(other interface{}) bool {
	typed, ok := other.(EffectIntent)
	return ok && _this.Equals(typed)
}

func Type_EffectIntent_() _dafny.TypeDescriptor {
	return type_EffectIntent_{}
}

type type_EffectIntent_ struct {
}

func (_this type_EffectIntent_) Default() interface{} {
	return Companion_EffectIntent_.Default()
}

func (_this type_EffectIntent_) String() string {
	return "Reducer.EffectIntent"
}
func (_this EffectIntent) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = EffectIntent{}

// End of datatype EffectIntent

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
			return "Reducer.DecisionStatus.Accepted"
		}
	case DecisionStatus_Rejected:
		{
			return "Reducer.DecisionStatus.Rejected"
		}
	case DecisionStatus_NoOp:
		{
			return "Reducer.DecisionStatus.NoOp"
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
	return "Reducer.DecisionStatus"
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
	Status  DecisionStatus
	Events  _dafny.Sequence
	Effects _dafny.Sequence
}

func (Decision_Decision) isDecision() {}

func (CompanionStruct_Decision_) Create_Decision_(Status DecisionStatus, Events _dafny.Sequence, Effects _dafny.Sequence) Decision {
	return Decision{Decision_Decision{Status, Events, Effects}}
}

func (_this Decision) Is_Decision() bool {
	_, ok := _this.Get_().(Decision_Decision)
	return ok
}

func (CompanionStruct_Decision_) Default() Decision {
	return Companion_Decision_.Create_Decision_(Companion_DecisionStatus_.Default(), _dafny.EmptySeq, _dafny.EmptySeq)
}

func (_this Decision) Dtor_status() DecisionStatus {
	return _this.Get_().(Decision_Decision).Status
}

func (_this Decision) Dtor_events() _dafny.Sequence {
	return _this.Get_().(Decision_Decision).Events
}

func (_this Decision) Dtor_effects() _dafny.Sequence {
	return _this.Get_().(Decision_Decision).Effects
}

func (_this Decision) String() string {
	switch data := _this.Get_().(type) {
	case nil:
		return "null"
	case Decision_Decision:
		{
			return "Reducer.Decision.Decision" + "(" + _dafny.String(data.Status) + ", " + _dafny.String(data.Events) + ", " + _dafny.String(data.Effects) + ")"
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
			return ok && data1.Status.Equals(data2.Status) && data1.Events.Equals(data2.Events) && data1.Effects.Equals(data2.Effects)
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
	return "Reducer.Decision"
}
func (_this Decision) ParentTraits_() []*_dafny.TraitID {
	return [](*_dafny.TraitID){}
}

var _ _dafny.TraitOffspring = Decision{}

// End of datatype Decision
