package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"homebase/internal/domain"
	"homebase/internal/journal"
)

type TypedEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type EventBatch struct {
	AttemptID string       `json:"attempt_id"`
	Version   uint64       `json:"version"`
	Events    []TypedEvent `json:"events"`
}

// Ensure interface implementation
var _ AttemptRepository = (*JournalAttemptRepository)(nil)

type JournalAttemptRepository struct {
	mu      sync.Mutex
	journal *journal.BinaryJournal
	// In-memory cache of states built from journal replay
	states  map[domain.AttemptID]domain.AttemptState
	version map[domain.AttemptID]uint64
}

func NewJournalAttemptRepository(j *journal.BinaryJournal) (*JournalAttemptRepository, error) {
	repo := &JournalAttemptRepository{
		journal: j,
		states:  make(map[domain.AttemptID]domain.AttemptState),
		version: make(map[domain.AttemptID]uint64),
	}
	
	if err := repo.replay(); err != nil {
		return nil, err
	}
	
	return repo, nil
}

func (r *JournalAttemptRepository) replay() error {
	return r.journal.Replay(func(seq uint64, payload []byte) error {
		var batch EventBatch
		if err := json.Unmarshal(payload, &batch); err != nil {
			return err
		}
		
		id, err := domain.ParseAttemptID(batch.AttemptID)
		if err != nil {
			return err
		}
		
		state, ok := r.states[id]
		if !ok {
			state = domain.AttemptState{ID: id}
		}
		
		// Expected version strictly enforced during replay
		expected := r.version[id]
		if batch.Version != expected {
			return fmt.Errorf("journal corruption: attempt %s expected version %d, got batch version %d", id, expected, batch.Version)
		}
		
		for _, te := range batch.Events {
			var event domain.Event
			switch te.Type {
			case "EventRecoveryDispatched":
				var e domain.EventRecoveryDispatched
				if err := json.Unmarshal(te.Data, &e); err != nil {
					return err
				}
				event = e
			case "EventRecoveryRejected":
				var e domain.EventRecoveryRejected
				if err := json.Unmarshal(te.Data, &e); err != nil {
					return err
				}
				event = e
			default:
				return fmt.Errorf("unknown event type: %s", te.Type)
			}
			state = domain.Apply(state, event)
		}
		
		r.states[id] = state
		r.version[id] = batch.Version + 1
		
		return nil
	})
}

func (r *JournalAttemptRepository) Load(ctx context.Context, id domain.AttemptID) (domain.AttemptState, uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	state, ok := r.states[id]
	if !ok {
		return domain.AttemptState{ID: id}, 0, nil
	}
	ver := r.version[id]
	return state, ver, nil
}

func (r *JournalAttemptRepository) Append(ctx context.Context, id domain.AttemptID, expectedVersion uint64, events []domain.Event) (uint64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	currentVer := r.version[id]
	if currentVer != expectedVersion {
		return currentVer, ErrVersionConflict
	}

	var typedEvents []TypedEvent
	for _, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return currentVer, err
		}
		
		var t string
		switch e.(type) {
		case domain.EventRecoveryDispatched:
			t = "EventRecoveryDispatched"
		case domain.EventRecoveryRejected:
			t = "EventRecoveryRejected"
		default:
			return currentVer, fmt.Errorf("unsupported event type")
		}
		
		typedEvents = append(typedEvents, TypedEvent{
			Type: t,
			Data: data,
		})
	}

	batch := EventBatch{
		AttemptID: id.String(),
		Version:   currentVer,
		Events:    typedEvents,
	}
	
	payload, err := json.Marshal(batch)
	if err != nil {
		return currentVer, fmt.Errorf("failed to encode event batch: %w", err)
	}

	_, err = r.journal.Append(payload)
	if err != nil {
		return currentVer, fmt.Errorf("journal append failed: %w", err)
	}

	// Update memory state only after successful durable write
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
