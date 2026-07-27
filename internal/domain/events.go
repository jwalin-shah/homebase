package domain

// Event represents a fact that has occurred.
type Event interface {
	isEvent()
}

type EventRecoveryDispatched struct {
	AttemptID AttemptID
	EffectID  EffectID
	Ordinal   uint8
}

func (e EventRecoveryDispatched) isEvent() {}

type EventRecoveryRejected struct {
	AttemptID AttemptID
	Reason    string
}

func (e EventRecoveryRejected) isEvent() {}

type EventConcluded struct {
	AttemptID AttemptID
}

func (e EventConcluded) isEvent() {}
