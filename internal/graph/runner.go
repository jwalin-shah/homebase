package graph

import (
	"sync"
)

// SafeContext wraps the auto-generated ExecutionContext with a Mutex
// to ensure thread-safe state transitions.
type SafeContext struct {
	mu  sync.Mutex
	ctx *ExecutionContext
}

func NewSafeContext() *SafeContext {
	return &SafeContext{
		ctx: NewExecutionContext(),
	}
}

func (s *SafeContext) GetState() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctx.State
}

func (s *SafeContext) GetRecoveryAttempts() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ctx.RecoveryAttempts
}

func (s *SafeContext) StepPlan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.StepPlan()
}

func (s *SafeContext) StepExecute(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.StepExecute(success)
}

func (s *SafeContext) StepRecover(success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.StepRecover(success)
}

func (s *SafeContext) StepEscalate(approved bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.StepEscalate(approved)
}

func (s *SafeContext) StepRepeat(moreWork bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx.StepRepeat(moreWork)
}
