package ledger

import (
	"time"
)

// Decision represents an architectural or implementation decision
type Decision struct {
	ID               string    `json:"id"`
	Decision         string    `json:"decision"`
	Axioms           []string  `json:"axioms"` // Required: must cite axioms
	Evidence         string    `json:"evidence"`
	DecidedBy        string    `json:"decided_by"`
	Approver         string    `json:"approver"`
	Status           string    `json:"status"` // PENDING | APPROVED
	Tags             []string  `json:"tags"`
	RiskLevel        string    `json:"risk_level"` // trivial | minor | major | critical
	AffectedSystems  []string  `json:"affected_systems"`
	RelatedDecisions []string  `json:"related_decisions"`
	RecordedAt       time.Time `json:"recorded_at"`
	Signature        string    `json:"signature"`      // Ed25519 hex
	LedgerLine       int       `json:"ledger_line"`    // For audit trail
	SchemaVersion    string    `json:"schema_version"` // For backward compatibility
	PreviousHash     string    `json:"previous_hash"`  // Hash of previous entry (chain)
}

// Validate checks if decision has all required fields
func (d *Decision) Validate() error {
	if d.ID == "" {
		return ErrMissingID
	}
	if d.Decision == "" {
		return ErrMissingDecision
	}
	if len(d.Axioms) == 0 {
		return ErrMissingAxioms
	}
	if d.DecidedBy == "" {
		return ErrMissingDecidedBy
	}
	if d.RiskLevel == "" {
		return ErrMissingRiskLevel
	}
	if !isValidRiskLevel(d.RiskLevel) {
		return ErrInvalidRiskLevel
	}
	return nil
}

func isValidRiskLevel(level string) bool {
	valid := map[string]bool{
		"trivial":  true,
		"minor":    true,
		"major":    true,
		"critical": true,
	}
	return valid[level]
}
