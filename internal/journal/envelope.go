package journal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

const (
	RecordKindEventBatch             = "EventBatch"
	RecordKindDecisionRecord         = "DecisionRecord"
	RecordKindSharedRecord           = "SharedRecord"
	RecordKindPromotionCommit        = "PromotionCommit"
	RecordEnvelopeVersion     uint16 = 1
)

var ErrNotEnvelope = errors.New("journal payload is not an explicit record envelope")

// RecordEnvelope is the only on-disk JSON record discriminator. Payloads are
// opaque to the journal so each owner can validate its own record schema.
type RecordEnvelope struct {
	Kind    string          `json:"kind"`
	Version uint16          `json:"version"`
	Payload json.RawMessage `json:"payload"`
}

func EncodeRecord(kind string, payload []byte) ([]byte, error) {
	envelope := RecordEnvelope{
		Kind:    kind,
		Version: RecordEnvelopeVersion,
		Payload: json.RawMessage(bytes.Clone(payload)),
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(envelope)
}

func DecodeRecord(data []byte) (RecordEnvelope, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return RecordEnvelope{}, fmt.Errorf("decode journal record envelope: %w", err)
	}
	if _, hasKind := fields["kind"]; !hasKind {
		return RecordEnvelope{}, ErrNotEnvelope
	}

	var envelope RecordEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return RecordEnvelope{}, fmt.Errorf("decode journal record envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		return RecordEnvelope{}, err
	}
	return envelope, nil
}

func (e RecordEnvelope) Validate() error {
	if e.Kind == "" {
		return fmt.Errorf("journal record kind is required")
	}
	if e.Version == 0 {
		return fmt.Errorf("journal record version is required")
	}
	if e.Version > RecordEnvelopeVersion {
		return fmt.Errorf("journal record version %d is unsupported", e.Version)
	}
	if len(e.Payload) == 0 || bytes.Equal(e.Payload, []byte("null")) {
		return fmt.Errorf("journal record payload is required")
	}
	return nil
}
