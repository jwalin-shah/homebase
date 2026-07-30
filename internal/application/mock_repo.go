package application

import (
	"context"
	"errors"
	"homebase/internal/domain"
	"sync"
)

// MockAttemptRepository is an explicitly non-durable in-memory repository for Milestone 1.
type MockAttemptRepository struct {
	mu      sync.Mutex
	states  map[domain.AttemptID]domain.AttemptState
	version map[domain.AttemptID]uint64
}

func NewMockAttemptRepository() *MockAttemptRepository {
	return &MockAttemptRepository{
		states:  make(map[domain.AttemptID]domain.AttemptState),
		version: make(map[domain.AttemptID]uint64),
	}
}

var ErrVersionConflict = errors.New("version conflict")

func (r *MockAttemptRepository) Load(ctx context.Context, id domain.AttemptID) (domain.AttemptState, uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, ok := r.states[id]
	if !ok {
		return domain.AttemptState{ID: id}, 0, nil
	}
	ver := r.version[id]
	return state, ver, nil
}

func (r *MockAttemptRepository) Append(ctx context.Context, id domain.AttemptID, expectedVersion uint64, events []domain.Event) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentVer := r.version[id]
	if currentVer != expectedVersion {
		return currentVer, ErrVersionConflict
	}

	state, ok := r.states[id]
	if !ok {
		state = domain.AttemptState{ID: id}
	}

	for _, e := range events {
		state = domain.Apply(state, e)
	}

	newVer := currentVer + 1
	r.states[id] = state
	r.version[id] = newVer

	return newVer, nil
}
