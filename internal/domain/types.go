package domain

import "errors"

// AttemptID uniquely identifies a workflow attempt. Constructed only via ParseAttemptID.
type AttemptID struct {
	value string
}

func ParseAttemptID(raw string) (AttemptID, error) {
	if raw == "" {
		return AttemptID{}, errors.New("AttemptID cannot be empty")
	}
	return AttemptID{value: raw}, nil
}

func (a AttemptID) String() string {
	return a.value
}

// EffectID uniquely identifies a deterministic effect.
type EffectID string

// AttemptPhase defines the lifecycle states of an Attempt.
type AttemptPhase uint8

const (
	AttemptActive AttemptPhase = iota
	AttemptRecovering
	AttemptConcluded
)

// AttemptState is the persistent, materialized state of an attempt.
type AttemptState struct {
	ID                  AttemptID
	Version             uint64
	Phase               AttemptPhase
	RecoveryDispatches  uint8
	DispatchedEffectIDs map[EffectID]struct{}
	ProcessedCmdKeys    map[string]struct{}
}

// EffectIntent is a durable request for execution.

type EffectIntent struct {
	AttemptID AttemptID
	EffectID  EffectID
	Ordinal   uint8
}

// DecisionStatus indicates the semantic result of a command.
type DecisionStatus uint8

const (
	DecisionUnknown DecisionStatus = iota
	DecisionAccepted
	DecisionRejected
	DecisionNoOp
)

// Decision represents the pure output of a reducer applying a command to state.
type Decision struct {
	Status  DecisionStatus
	Events  []Event
	Effects []EffectIntent
}
