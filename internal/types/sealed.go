package types

import "errors"

type Signature struct {
	value string
}

func (s Signature) String() string { return s.value }
func (s Signature) IsEmpty() bool  { return s.value == "" }

func InternalNewSignature(val string) Signature {
	return Signature{value: val}
}

type AxiomID struct {
	value string
}

func (a AxiomID) String() string { return a.value }

func InternalNewAxiomID(val string) AxiomID {
	return AxiomID{value: val}
}

type BridgeSignature struct {
	value string
}

func (b BridgeSignature) String() string { return b.value }

func InternalNewBridgeSignature(val string) BridgeSignature {
	return BridgeSignature{value: val}
}

type EscalationStatus string

const (
	StatusPending   EscalationStatus = "PENDING"
	StatusApproved  EscalationStatus = "APPROVED"
	StatusRejected  EscalationStatus = "REJECTED"
	StatusExpired   EscalationStatus = "EXPIRED"
	StatusEscalated EscalationStatus = "ESCALATED"
)

var ErrInvalidTransition = errors.New("invalid escalation status transition")

// Escalation tracks the state of a human intervention.
// FIX: Unexported fields to prevent malicious mutation.
type Escalation struct {
	id     string
	status EscalationStatus
}

// NewEscalation safely initializes an escalation record.
func NewEscalation(id string) Escalation {
	return Escalation{id: id, status: StatusPending}
}

func (e *Escalation) ID() string               { return e.id }
func (e *Escalation) Status() EscalationStatus { return e.status }

// Approve safely transitions an escalation to APPROVED.
func (e *Escalation) Approve() error {
	if e.status != StatusPending {
		return ErrInvalidTransition
	}
	e.status = StatusApproved
	return nil
}

// Reject safely transitions an escalation to REJECTED.
func (e *Escalation) Reject() error {
	if e.status != StatusPending {
		return ErrInvalidTransition
	}
	e.status = StatusRejected
	return nil
}
