package domain

import (
	"testing"
)

func TestReducer_BoundedRecovery(t *testing.T) {
	state := AttemptState{ID: "attempt-123"}
	
	// 1. First recovery accepted
	cmd1 := CommandProposeRecovery{AttemptID: "attempt-123"}
	decision1 := Decide(state, cmd1)
	
	if len(decision1.Effects) != 1 {
		t.Fatalf("expected 1 effect, got %d", len(decision1.Effects))
	}
	if decision1.Effects[0].Ordinal != 1 {
		t.Errorf("expected ordinal 1, got %d", decision1.Effects[0].Ordinal)
	}
	
	// Apply event to state
	state = Apply(state, decision1.Events[0])
	
	// Duplicate command returns no-op (idempotency check via exact state match)
	// Actually, if we pass exactly the same state, it should generate ordinal 2.
	// But wait, our ID generation uses ordinal. A truly duplicate *dispatch* of the SAME command 
	// is typically handled at the effect executor or by checking a specific Deduplication ID.
	// In this simple pure reducer, consecutive ProposeRecovery commands just increment the budget.
	// Let's proceed to the second recovery.
	
	// 2. Second recovery accepted
	cmd2 := CommandProposeRecovery{AttemptID: "attempt-123"}
	decision2 := Decide(state, cmd2)
	
	if len(decision2.Effects) != 1 {
		t.Fatalf("expected 1 effect on second recovery, got %d", len(decision2.Effects))
	}
	if decision2.Effects[0].Ordinal != 2 {
		t.Errorf("expected ordinal 2, got %d", decision2.Effects[0].Ordinal)
	}
	
	state = Apply(state, decision2.Events[0])
	
	// 3. Third rejected
	cmd3 := CommandProposeRecovery{AttemptID: "attempt-123"}
	decision3 := Decide(state, cmd3)
	
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
	stateConcluded := AttemptState{ID: "attempt-456"}
	stateConcluded = Apply(stateConcluded, EventConcluded{AttemptID: "attempt-456"})
	
	cmd4 := CommandProposeRecovery{AttemptID: "attempt-456"}
	decision4 := Decide(stateConcluded, cmd4)
	
	if len(decision4.Effects) != 0 {
		t.Fatalf("expected 0 effects on concluded attempt, got %d", len(decision4.Effects))
	}
	if _, ok := decision4.Events[0].(EventRecoveryRejected); !ok {
		t.Errorf("expected EventRecoveryRejected, got %T", decision4.Events[0])
	}
}
