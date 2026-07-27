package ledger

import "errors"

var (
	// Validation errors
	ErrMissingID        = errors.New("decision id required")
	ErrMissingDecision  = errors.New("decision text required")
	ErrMissingAxioms    = errors.New("at least one axiom citation required")
	ErrMissingDecidedBy = errors.New("decided_by required")
	ErrMissingRiskLevel = errors.New("risk_level required")
	ErrInvalidRiskLevel = errors.New("risk_level must be: trivial, minor, major, or critical")
	ErrDuplicateID      = errors.New("decision with this id already exists")

	// Ledger errors
	ErrLedgerNotFound    = errors.New("ledger file not found")
	ErrLedgerReadFailed  = errors.New("failed to read ledger")
	ErrLedgerWriteFailed = errors.New("failed to write to ledger")
	ErrAppendOnly        = errors.New("ledger is append-only, modifications not allowed")
)
