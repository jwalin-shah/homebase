package application

import (
	"context"
	"errors"
	"fmt"

	"homebase/internal/domain"
)

// AttemptRepository defines the interface for durable attempt state.
type AttemptRepository interface {
	Load(ctx context.Context, id domain.AttemptID) (domain.AttemptState, error)
	Save(ctx context.Context, state domain.AttemptState, events []domain.Event, effects []domain.EffectIntent) error
}

// AttemptService orchestrates the execution of domain commands.
type AttemptService struct {
	repo AttemptRepository
}

func NewAttemptService(repo AttemptRepository) *AttemptService {
	return &AttemptService{repo: repo}
}

// ExecuteCommand loads state, delegates to the pure reducer, and persists the result.
func (s *AttemptService) ExecuteCommand(ctx context.Context, cmd domain.Command) error {
	var attemptID domain.AttemptID
	switch c := cmd.(type) {
	case domain.CommandProposeRecovery:
		attemptID = c.AttemptID
	case domain.CommandConclude:
		attemptID = c.AttemptID
	default:
		return errors.New("unknown command")
	}

	state, err := s.repo.Load(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	decision := domain.Decide(state, cmd)

	if decision.Status == domain.DecisionRejected {
		return errors.New("command rejected by domain reducer")
	}

	if decision.Status == domain.DecisionNoOp {
		return nil
	}

	for _, e := range decision.Events {
		state = domain.Apply(state, e)
	}

	if err := s.repo.Save(ctx, state, decision.Events, decision.Effects); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	return nil
}
