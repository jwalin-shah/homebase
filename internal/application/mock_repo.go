package application

import (
	"context"
	"sync"
	"homebase/internal/domain"
)

// MockAttemptRepository is an explicitly non-durable in-memory repository for Milestone 1.
type MockAttemptRepository struct {
	mu    sync.Mutex
	store map[string]domain.AttemptState
}

func NewMockAttemptRepository() *MockAttemptRepository {
	return &MockAttemptRepository{
		store: make(map[string]domain.AttemptState),
	}
}

func (r *MockAttemptRepository) Load(ctx context.Context, id domain.AttemptID) (domain.AttemptState, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, exists := r.store[id.String()]
	if !exists {
		return domain.AttemptState{ID: id}, nil
	}
	return state, nil
}

func (r *MockAttemptRepository) Save(ctx context.Context, state domain.AttemptState, events []domain.Event, effects []domain.EffectIntent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store[state.ID.String()] = state
	return nil
}
