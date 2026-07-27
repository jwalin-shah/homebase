package validation

import (
	"context"
	"fmt"
	"time"

	"homebase/internal/cache"
	"homebase/internal/ledger"
	"homebase/internal/types"
)

// Validator handles decision validation
type Validator struct {
	cache   *cache.Client
	ledger  *ledger.Store
	timeout time.Duration
}

// NewValidator creates a new validator
func NewValidator(cacheClient *cache.Client, ledgerStore *ledger.Store) *Validator {
	return &Validator{
		cache:   cacheClient,
		ledger:  ledgerStore,
		timeout: 5 * time.Second,
	}
}



// VerifyAxiomsExist checks if all cited axioms exist in Neo4j.
// This physically implements the AxiomFirewall interface for the S0: PLAN state.
func (v *Validator) VerifyAxiomsExist(ctx context.Context, axioms []types.AxiomID) error {
	// Graceful degradation: if cache is nil, skip axiom validation
	if v.cache == nil {
		return nil
	}

	for _, axiom := range axioms {
		exists, err := v.cache.AxiomExists(ctx, axiom.String())
		if err != nil {
			// If Neo4j is down, continue (graceful degradation)
			// Log warning but don't fail
			fmt.Printf("WARNING: axiom check skipped for %s (Neo4j unavailable)\n", axiom.String())
			continue
		}

		if !exists {
			return fmt.Errorf("CRITICAL FIREWALL BLOCK: axiom not found in graph: %s", axiom.String())
		}
	}

	return nil
}



// QueryDecisionsByAxiom retrieves all decisions citing a specific axiom from Neo4j
func (v *Validator) QueryDecisionsByAxiom(axiom string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	if v.cache == nil {
		return nil, fmt.Errorf("neo4j unavailable")
	}

	decisions, err := v.cache.QueryDecisionsByAxiom(ctx, axiom)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	return decisions, nil
}

// FilterAxiomsByDomain retrieves axioms matching a specific domain from Neo4j
func (v *Validator) FilterAxiomsByDomain(domain string) ([]map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	if v.cache == nil {
		return nil, fmt.Errorf("neo4j unavailable")
	}

	axioms, err := v.cache.FilterAxiomsByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("filter failed: %w", err)
	}

	return axioms, nil
}

// CreateDecisionNode creates a Decision node in Neo4j
func (v *Validator) CreateDecisionNode(decision *ledger.Decision) error {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	if v.cache == nil {
		// Graceful degradation: continue without Neo4j
		return nil
	}

	return v.cache.CreateDecisionNode(ctx, decision.ID, decision.Decision, decision.RecordedAt)
}

// CreateCitesRelationship creates a CITES relationship between a Decision and Axiom
func (v *Validator) CreateCitesRelationship(decisionID, axiomID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	if v.cache == nil {
		// Graceful degradation: continue without Neo4j
		return nil
	}

	return v.cache.CreateCitesRelationship(ctx, decisionID, axiomID)
}

// CreateIndices creates performance indices on Decision and Axiom nodes
func (v *Validator) CreateIndices() error {
	ctx, cancel := context.WithTimeout(context.Background(), v.timeout)
	defer cancel()

	if v.cache == nil {
		// Graceful degradation: continue without Neo4j
		return nil
	}

	return v.cache.CreateIndices(ctx)
}



// CheckNeo4jHealth checks if Neo4j is available and healthy
func (v *Validator) CheckNeo4jHealth(ctx context.Context) error {
	if v.cache == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	return v.cache.Health(ctx)
}
