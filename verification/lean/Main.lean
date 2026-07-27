-- HomeBase Lean Implementation
-- Main entry point

import HomeBase.Domain
import HomeBase.Reducer
import HomeBase.Decision
import HomeBase.Invariants
import HomeBase.Fixtures

#check HomeBase.Domain.TaskState
#check HomeBase.Reducer.applyEvent
#check HomeBase.Decision.decide
#check HomeBase.Invariants.version_monotonicity
