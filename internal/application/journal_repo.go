package application

import (
	"context"
	"encoding/json"
	"fmt"
	"homebase/internal/domain"
	"homebase/internal/journal"
	"sync"
)

type TypedEvent struct {
	Type          string          `json:"type"`
	SchemaVersion uint32          `json:"schema_version"`
	Data          json.RawMessage `json:"data"`
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
		kind, body, err := decodeJournalRecord(payload)
		if err != nil {
			return fmt.Errorf("journal record %d: %w", seq, err)
		}
		if kind == journal.RecordKindDecisionRecord {
			// Decision records share the journal but are not part of the
			// attempt reducer. Their owner validates them separately.
			return nil
		}
		if kind == journal.RecordKindSharedRecord {
			// Shared records have their own reducer and are intentionally not
			// interpreted as attempt events.
			return nil
		}
		if kind != journal.RecordKindEventBatch {
			return fmt.Errorf("unsupported journal record kind: %s", kind)
		}

		var batch EventBatch
		if err := json.Unmarshal(body, &batch); err != nil {
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
			if te.SchemaVersion != 1 {
				return fmt.Errorf("unsupported event schema version %d for %s", te.SchemaVersion, te.Type)
			}
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
			case "EventConcluded":
				var e domain.EventConcluded
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
		case domain.EventConcluded:
			t = "EventConcluded"
		default:
			return currentVer, fmt.Errorf("unsupported event type")
		}

		typedEvents = append(typedEvents, TypedEvent{
			Type:          t,
			SchemaVersion: 1,
			Data:          data,
		})
	}

	batch := EventBatch{
		AttemptID: id.String(),
		Version:   currentVer,
		Events:    typedEvents,
	}

	batchPayload, err := json.Marshal(batch)
	if err != nil {
		return currentVer, fmt.Errorf("failed to encode event batch: %w", err)
	}
	payload, err := journal.EncodeRecord(journal.RecordKindEventBatch, batchPayload)
	if err != nil {
		return currentVer, fmt.Errorf("failed to encode event batch envelope: %w", err)
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

// decodeJournalRecord accepts the explicit envelope and one legacy shape only.
// A legacy payload is accepted only when it has the complete EventBatch field
// set and a valid attempt id; arbitrary JSON is never guessed as a batch.
func decodeJournalRecord(payload []byte) (string, []byte, error) {
	envelope, err := journal.DecodeRecord(payload)
	if err == nil {
		return envelope.Kind, envelope.Payload, nil
	}
	if err != journal.ErrNotEnvelope {
		return "", nil, err
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(payload, &fields); err != nil {
		return "", nil, err
	}
	for _, field := range []string{"attempt_id", "version", "events"} {
		if _, ok := fields[field]; !ok {
			return "", nil, fmt.Errorf("legacy journal record missing %q and has no explicit envelope", field)
		}
	}
	var batch EventBatch
	if err := json.Unmarshal(payload, &batch); err != nil {
		return "", nil, fmt.Errorf("decode legacy EventBatch: %w", err)
	}
	if _, err := domain.ParseAttemptID(batch.AttemptID); err != nil {
		return "", nil, fmt.Errorf("legacy journal record is not an EventBatch: %w", err)
	}
	return journal.RecordKindEventBatch, payload, nil
}
