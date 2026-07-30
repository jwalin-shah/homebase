package types

import "time"

// Subject represents an artifact being attested, matching the in-toto Statement subject.
type Subject struct {
	name   string
	digest map[string]string
}

// NewSubject creates a new Subject.
func NewSubject(name string, digest map[string]string) Subject {
	return Subject{
		name:   name,
		digest: digest,
	}
}

func (s Subject) Name() string { return s.name }
func (s Subject) Digest() map[string]string {
	cp := make(map[string]string, len(s.digest))
	for k, v := range s.digest {
		cp[k] = v
	}
	return cp
}

// AssuranceCase represents the formal payload of a decision (Claims-Arguments-Evidence)
// aligned with the in-toto Statement schema.
type AssuranceCase struct {
	statementType string    // _type, e.g., "https://in-toto.io/Statement/v1"
	subject       []Subject // subject
	predicateType string    // predicateType

	// Predicate fields
	id         string
	claim      string // The requirement being met
	model      string // e.g., "AWS_EBS_BOUNDED_RETRY"
	argument   string // Why the selected pattern satisfies the claim
	axioms     []AxiomID
	evidence   string // Gate outputs (Lean proofs, property tests, trace logs)
	recordedAt time.Time
	recordedBy string // The authorizer
}

func NewAssuranceCase(id string, subject []Subject, claim, model, argument string, axioms []AxiomID, evidence string, recordedBy string) AssuranceCase {
	axiomsCopy := make([]AxiomID, len(axioms))
	copy(axiomsCopy, axioms)

	subjectCopy := make([]Subject, len(subject))
	copy(subjectCopy, subject)

	return AssuranceCase{
		statementType: "https://in-toto.io/Statement/v1",
		subject:       subjectCopy,
		predicateType: "https://homebase.dev/assurance-case/v1",
		id:            id,
		claim:         claim,
		model:         model,
		argument:      argument,
		axioms:        axiomsCopy,
		evidence:      evidence,
		recordedAt:    time.Now(),
		recordedBy:    recordedBy,
	}
}

func (ac AssuranceCase) StatementType() string { return ac.statementType }
func (ac AssuranceCase) Subject() []Subject {
	cp := make([]Subject, len(ac.subject))
	copy(cp, ac.subject)
	return cp
}
func (ac AssuranceCase) PredicateType() string { return ac.predicateType }
func (ac AssuranceCase) ID() string            { return ac.id }
func (ac AssuranceCase) Claim() string         { return ac.claim }
func (ac AssuranceCase) Model() string         { return ac.model }
func (ac AssuranceCase) Argument() string      { return ac.argument }
func (ac AssuranceCase) Evidence() string      { return ac.evidence }
func (ac AssuranceCase) RecordedAt() time.Time { return ac.recordedAt }
func (ac AssuranceCase) RecordedBy() string    { return ac.recordedBy }

func (ac AssuranceCase) Axioms() []AxiomID {
	cp := make([]AxiomID, len(ac.axioms))
	copy(cp, ac.axioms)
	return cp
}

// DSSESignature represents a signature within a DSSEEnvelope.
type DSSESignature struct {
	keyID string
	sig   string // Base64 encoded signature
}

func NewDSSESignature(keyID string, sig string) DSSESignature {
	return DSSESignature{
		keyID: keyID,
		sig:   sig,
	}
}

func (s DSSESignature) KeyID() string { return s.keyID }
func (s DSSESignature) Sig() string   { return s.sig }

// DSSEEnvelope wraps a payload in an in-toto Dead Simple Signing Envelope.
type DSSEEnvelope struct {
	payloadType string
	payload     string // Base64 encoded payload
	signatures  []DSSESignature

	// Keep in-memory reference to decoded payload to avoid JSON unmarshaling everywhere
	decodedPayload AssuranceCase
}

// NewDSSEEnvelope safely constructs a signed decision envelope.
func NewDSSEEnvelope(base64Payload string, decoded AssuranceCase, sig DSSESignature) DSSEEnvelope {
	return DSSEEnvelope{
		payloadType:    "application/vnd.in-toto+json",
		payload:        base64Payload,
		signatures:     []DSSESignature{sig},
		decodedPayload: decoded,
	}
}

func (e DSSEEnvelope) PayloadType() string           { return e.payloadType }
func (e DSSEEnvelope) Payload() string               { return e.payload }
func (e DSSEEnvelope) DecodedPayload() AssuranceCase { return e.decodedPayload }
func (e DSSEEnvelope) Signatures() []DSSESignature {
	cp := make([]DSSESignature, len(e.signatures))
	copy(cp, e.signatures)
	return cp
}
