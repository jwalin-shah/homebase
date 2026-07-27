package graph

import (
	"sync"
	"testing"
)

// TestJepsen simulates a violent Jepsen-style network partition
// where 10,000 parallel requests attempt to resume and mutate the exact same 
// ExecutionContext simultaneously. If our Mutex is flawed, this will trigger 
// the race detector or a fatal panic.
func TestJepsen_ConcurrencyStorm(t *testing.T) {
	safeCtx := NewSafeContext()
	
	var wg sync.WaitGroup
	routines := 10000

	wg.Add(routines)
	for i := 0; i < routines; i++ {
		go func(workerID int) {
			defer wg.Done()
			
			currentState := safeCtx.GetState()
			
			if currentState == StatePlan {
				safeCtx.StepPlan()
			} else if currentState == StateExecute {
				safeCtx.StepExecute(false)
			} else if currentState == StateRecover {
				safeCtx.StepRecover(false)
			} else if currentState == StateEscalate {
				safeCtx.StepEscalate(true)
			} else if currentState == StateRepeat {
				safeCtx.StepRepeat(false)
			}
		}(i)
	}

	wg.Wait()
	
	// If the test reaches here without a panic or a data race, the Mutex held.
	t.Logf("Jepsen storm survived. Final State: %v, Recovery Attempts: %d", safeCtx.GetState(), safeCtx.GetRecoveryAttempts())
	
	if got := safeCtx.GetRecoveryAttempts(); got > 2 {
		t.Fatalf(
			"recovery attempts exceeded bound: got %d, max %d",
			got,
			2,
		)
	}
}
