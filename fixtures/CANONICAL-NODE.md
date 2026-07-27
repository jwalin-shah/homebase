# Canonical Fixture: Graph Node Execution

**Purpose:** This is the gold-standard example of how an AI agent must write a Graph Node in the HomeBase ecosystem. It demonstrates the integration of Gobra Separation Logic, TLA+ state boundaries, and strict Mutex handling.

## The Rule of Examples
Agents learn best from examples. When asked to create a new state transition or a new Node in the Graph Engine, you MUST pattern-match this exact structure. 

### 1. The Gobra Annotations (Physics)
Before the function executes, you must mathematically prove you have access to the lock, and mathematically guarantee the state boundary.

```go
package graph

import (
	"context"
	"fmt"
)

// CanonicalNode represents a perfectly bounded graph transition.
type CanonicalNode struct{}

// Execute performs the node's logic under strict mathematical bounds.
// 
// Gobra Separation Logic Annotations:
// requires acc(&execCtx.Mutex)          // PROOF: We hold the memory lock
// ensures execCtx.RecoveryAttempts <= 2 // PROOF: TLA+ Bounded Iteration
// ensures err == nil ==> result == StateRepeat || result == StateExecute
func (n *CanonicalNode) Execute(ctx context.Context, execCtx *ExecutionContext) (StateType, error) {
	
	// 1. The state is guaranteed to be locked by the Runner before this is called.
	// We do NOT call execCtx.Lock() here. We inherited it.

	// 2. Perform the operation
	err := performSafeOperation()

	// 3. Handle Failure strictly according to TLA+ bounds
	if err != nil {
		execCtx.RecoveryAttempts++
		
		if execCtx.RecoveryAttempts == 1 {
			execCtx.FailureHistory = append(execCtx.FailureHistory, fmt.Errorf("canonical attempt 1 failed: %v", err))
			// Valid transition: Loop back to RECOVER
			return StateRecover, nil 
		} else if execCtx.RecoveryAttempts == 2 {
			execCtx.FailureHistory = append(execCtx.FailureHistory, fmt.Errorf("canonical attempt 2 exhausted"))
			// Valid transition: Halt and Escalate
			return StateEscalate, nil
		}
		
		// This line is mathematically unreachable due to the Gobra post-condition, 
		// but Go requires it for compilation.
		return StateEscalate, fmt.Errorf("fatal bounds violation")
	}

	// 4. Handle Success
	// We do NOT reset RecoveryAttempts to 0 here (prevents livelock).
	return StateExecute, nil
}

func performSafeOperation() error {
	return nil // Simulated success
}
```

### Checklist for Agents
Before submitting this code to HomeBase, the agent MUST verify:
- [x] Does the `requires` annotation match the memory lock?
- [x] Does the `ensures` annotation match the `homebase.tla` invariant?
- [x] Is the Fallthrough bug prevented with an `else if`?
- [x] Does success return a mathematically valid `StateType`?
