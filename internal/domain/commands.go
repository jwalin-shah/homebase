package domain

// Command represents an intent to change state.
type Command interface {
	isCommand()
}

type CommandProposeRecovery struct {
	AttemptID      AttemptID
	IdempotencyKey string
	Version        uint64
}

func (c CommandProposeRecovery) isCommand() {}

type CommandConclude struct {
	AttemptID AttemptID
}

func (c CommandConclude) isCommand() {}
