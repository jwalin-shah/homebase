package domain

// AttemptID uniquely identifies a workflow attempt.
type AttemptID string

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
	Phase               AttemptPhase
	RecoveryDispatches  uint8
	DispatchedEffectIDs map[EffectID]struct{}
}

// EffectIntent is a durable request for execution.
type EffectIntent struct {
	AttemptID AttemptID
	EffectID  EffectID
	Ordinal   uint8
}

// Decision represents the pure output of a reducer applying a command to state.
type Decision struct {
	Events  []Event
	Effects []EffectIntent
}
