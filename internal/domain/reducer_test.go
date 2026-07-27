package domain

import (
	"testing"
)

func TestReducer_BoundedRecovery(t *testing.T) {
	aid, _ := ParseAttemptID("attempt-123")
	state := AttemptState{ID: aid}
	
	// 1. First recovery accepted
	cmd1 := CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req-1"}
	decision1 := Decide(state, cmd1)
	
	if decision1.Status != DecisionAccepted {
		t.Fatalf("expected DecisionAccepted, got %d", decision1.Status)
	}
	if len(decision1.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(decision1.Effects))
	}
	if decision1.Effects[0].Ordinal != 1 {
		t.Errorf("expected ordinal 1, got %d", decision1.Effects[0].Ordinal)
	}
	
	// Apply event to state (verify pure immutability of input)
	newState := Apply(state, decision1.Events[0])
	if state.Phase != AttemptActive {
		t.Fatalf("expected input state to remain AttemptActive, got %v", state.Phase)
	}
	if newState.Phase != AttemptRecovering {
		t.Fatalf("expected new state to be AttemptRecovering")
	}
	state = newState
	
	// Duplicate command returns NoOp
	cmdDup := CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req-1"}
	// Test duplicate idempotency
	decisionDup := Decide(state, cmdDup)
	if decisionDup.Status != DecisionNoOp {
		t.Fatalf("expected DecisionNoOp on duplicate, got %d", decisionDup.Status)
	}
	// We will just proceed to the second recovery.
	
	// 2. Second recovery accepted (distinct command)
	cmd2 := CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req-2"}
	decision2 := Decide(state, cmd2)
	
	if decision2.Status != DecisionAccepted {
		t.Fatalf("expected DecisionAccepted, got %d", decision2.Status)
	}
	if len(decision2.Effects) != 1 {
		t.Fatalf("expected 1 effect on second recovery, got %d", len(decision2.Effects))
	}
	if decision2.Effects[0].Ordinal != 2 {
		t.Errorf("expected ordinal 2, got %d", decision2.Effects[0].Ordinal)
	}
	
	state = Apply(state, decision2.Events[0])
	
	// 3. Third rejected
	cmd3 := CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req-3"}
	decision3 := Decide(state, cmd3)
	
	if decision3.Status != DecisionRejected {
		t.Fatalf("expected DecisionRejected, got %d", decision3.Status)
	}
	if len(decision3.Effects) != 0 {
		t.Fatalf("expected 0 effects on third recovery, got %d", len(decision3.Effects))
	}
	if len(decision3.Events) != 1 {
		t.Fatalf("expected 1 rejection event, got %d", len(decision3.Events))
	}
	if _, ok := decision3.Events[0].(EventRecoveryRejected); !ok {
		t.Errorf("expected EventRecoveryRejected, got %T", decision3.Events[0])
	}
	
	// 4. Recovery after conclusion rejected
	aid2, _ := ParseAttemptID("attempt-456")
	stateConcluded := AttemptState{ID: aid2}
	stateConcluded = Apply(stateConcluded, EventConcluded{AttemptID: aid2})
	
	cmd4 := CommandProposeRecovery{AttemptID: aid2, IdempotencyKey: "req-4"}
	decision4 := Decide(stateConcluded, cmd4)
	
	if decision4.Status != DecisionRejected {
		t.Fatalf("expected DecisionRejected, got %d", decision4.Status)
	}
	if len(decision4.Effects) != 0 {
		t.Fatalf("expected 0 effects on concluded attempt, got %d", len(decision4.Effects))
	}
	
	// 5. Unknown command rejected
	type UnknownCommand struct{ AttemptID AttemptID }
	// We create a dummy implementation just for the test
	var _ Command = (*unknownCmd)(nil)
	decision5 := Decide(state, &unknownCmd{AttemptID: aid})
	if decision5.Status != DecisionRejected {
		t.Fatalf("expected DecisionRejected for unknown command, got %d", decision5.Status)
	}

	// 6. Stale version rejected
	stateVersioned := AttemptState{ID: aid, Version: 5}
	cmdStale := CommandProposeRecovery{AttemptID: aid, Version: 2}
	decision6 := Decide(stateVersioned, cmdStale)
	if decision6.Status != DecisionRejected {
		t.Fatalf("expected DecisionRejected for stale command, got %d", decision6.Status)
	}
}

type unknownCmd struct {
	AttemptID AttemptID
}
func (c *unknownCmd) isCommand() {}
