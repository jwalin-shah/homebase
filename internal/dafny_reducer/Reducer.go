// Package Reducer
// Dafny module Reducer compiled into Go

package Reducer

import (
  os "os"
  _dafny "github.com/dafny-lang/DafnyRuntimeGo/v4/dafny"
  m__System "github.com/dafny-lang/DafnyRuntimeGo/v4/System_"
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
var Companion_Default___ = CompanionStruct_Default___ {
}

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
  return [](*_dafny.TraitID){};
}
var _ _dafny.TraitOffspring = &Default__{}

func (_static *CompanionStruct_Default___) Apply(state _dafny.Int, event Event) _dafny.Int {
  var _source0 Event = event
  _ = _source0
  {
    if (_source0.Is_EventRecoveryDispatched()) {
      return ((state)).Plus(_dafny.One)
    }
  }
  {
    return state
  }
}
// End of class Default__

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
var Companion_Event_ = CompanionStruct_Event_ {
}

type Event_EventRecoveryDispatched struct {
  EffectID _dafny.Sequence
  Ordinal _dafny.Int
}

func (Event_EventRecoveryDispatched) isEvent() {}

func (CompanionStruct_Event_) Create_EventRecoveryDispatched_(EffectID _dafny.Sequence, Ordinal _dafny.Int) Event {
  return Event{Event_EventRecoveryDispatched{EffectID, Ordinal}}
}

func (_this Event) Is_EventRecoveryDispatched() bool {
  _, ok := _this.Get_().(Event_EventRecoveryDispatched)
  return ok
}

type Event_EventRecoveryRejected struct {
  Reason _dafny.Sequence
}

func (Event_EventRecoveryRejected) isEvent() {}

func (CompanionStruct_Event_) Create_EventRecoveryRejected_(Reason _dafny.Sequence) Event {
  return Event{Event_EventRecoveryRejected{Reason}}
}

func (_this Event) Is_EventRecoveryRejected() bool {
  _, ok := _this.Get_().(Event_EventRecoveryRejected)
  return ok
}

func (CompanionStruct_Event_) Default() Event {
  return Companion_Event_.Create_EventRecoveryDispatched_(_dafny.EmptySeq, _dafny.Zero)
}

func (_this Event) Dtor_effectID() _dafny.Sequence {
  return _this.Get_().(Event_EventRecoveryDispatched).EffectID
}

func (_this Event) Dtor_ordinal() _dafny.Int {
  return _this.Get_().(Event_EventRecoveryDispatched).Ordinal
}

func (_this Event) Dtor_reason() _dafny.Sequence {
  return _this.Get_().(Event_EventRecoveryRejected).Reason
}

func (_this Event) String() string {
  switch data := _this.Get_().(type) {
    case nil: return "null"
    case Event_EventRecoveryDispatched: {
      return "Reducer.Event.EventRecoveryDispatched" + "(" + data.EffectID.VerbatimString(true) + ", " + _dafny.String(data.Ordinal) + ")"
    }
    case Event_EventRecoveryRejected: {
      return "Reducer.Event.EventRecoveryRejected" + "(" + data.Reason.VerbatimString(true) + ")"
    }
    default: {
      return "<unexpected>"
    }
  }
}

func (_this Event) Equals(other Event) bool {
  switch data1 := _this.Get_().(type) {
    case Event_EventRecoveryDispatched: {
      data2, ok := other.Get_().(Event_EventRecoveryDispatched)
      return ok && data1.EffectID.Equals(data2.EffectID) && data1.Ordinal.Cmp(data2.Ordinal) == 0
    }
    case Event_EventRecoveryRejected: {
      data2, ok := other.Get_().(Event_EventRecoveryRejected)
      return ok && data1.Reason.Equals(data2.Reason)
    }
    default: {
      return false; // unexpected
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
  return Companion_Event_.Default();
}

func (_this type_Event_) String() string {
  return "Reducer.Event"
}
func (_this Event) ParentTraits_() []*_dafny.TraitID {
  return [](*_dafny.TraitID){};
}
var _ _dafny.TraitOffspring = Event{}

// End of datatype Event

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
var Companion_AttemptState_ = CompanionStruct_AttemptState_ {
}

type AttemptState_AttemptState struct {
  RecoveryDispatches _dafny.Int
}

func (AttemptState_AttemptState) isAttemptState() {}

func (CompanionStruct_AttemptState_) Create_AttemptState_(RecoveryDispatches _dafny.Int) AttemptState {
  return AttemptState{AttemptState_AttemptState{RecoveryDispatches}}
}

func (_this AttemptState) Is_AttemptState() bool {
  _, ok := _this.Get_().(AttemptState_AttemptState)
  return ok
}

func (CompanionStruct_AttemptState_) Default() _dafny.Int {
  return _dafny.Zero
}

func (_this AttemptState) Dtor_recoveryDispatches() _dafny.Int {
  return _this.Get_().(AttemptState_AttemptState).RecoveryDispatches
}

func (_this AttemptState) String() string {
  switch data := _this.Get_().(type) {
    case nil: return "null"
    case AttemptState_AttemptState: {
      return "Reducer.AttemptState.AttemptState" + "(" + _dafny.String(data.RecoveryDispatches) + ")"
    }
    default: {
      return "<unexpected>"
    }
  }
}

func (_this AttemptState) Equals(other AttemptState) bool {
  switch data1 := _this.Get_().(type) {
    case AttemptState_AttemptState: {
      data2, ok := other.Get_().(AttemptState_AttemptState)
      return ok && data1.RecoveryDispatches.Cmp(data2.RecoveryDispatches) == 0
    }
    default: {
      return false; // unexpected
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
  return Companion_AttemptState_.Default();
}

func (_this type_AttemptState_) String() string {
  return "Reducer.AttemptState"
}
func (_this AttemptState) ParentTraits_() []*_dafny.TraitID {
  return [](*_dafny.TraitID){};
}
var _ _dafny.TraitOffspring = AttemptState{}

// End of datatype AttemptState
