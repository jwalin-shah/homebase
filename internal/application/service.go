package application

import (
	"context"
	"errors"
	"fmt"

	"homebase/internal/domain"
)

// AttemptRepository defines the interface for durable attempt state.
type AttemptRepository interface {
	Load(ctx context.Context, id domain.AttemptID) (domain.AttemptState, uint64, error)
	Append(ctx context.Context, id domain.AttemptID, expectedVersion uint64, events []domain.Event) (uint64, error)
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

	state, version, err := s.repo.Load(ctx, attemptID)
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

	_, err = s.repo.Append(ctx, attemptID, version, decision.Events)
	if err != nil {
		// If errors.Is(err, ErrVersionConflict), we could retry, but for now we just return it
		return fmt.Errorf("persistence failure: %w", err)
	}

	return nil
}
