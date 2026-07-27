package domain

// Command represents an intent to change state.
type Command interface {
	isCommand()
}

type CommandProposeRecovery struct {
	AttemptID AttemptID
}

func (c CommandProposeRecovery) isCommand() {}

type CommandConclude struct {
	AttemptID AttemptID
}

func (c CommandConclude) isCommand() {}
