package cache

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"homebase/internal/types"
)

// Client manages Neo4j connection and queries
type Client struct {
	driver neo4j.DriverWithContext
	uri    string
	user   string
	pass   string
}

// NewClient creates a new Neo4j client
func NewClient(uri, user, pass string) (*Client, error) {
	if strings.TrimSpace(pass) == "" {
		return nil, fmt.Errorf("neo4j password is required; refusing an implicit credential")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, pass, ""))
	if err != nil {
		return nil, fmt.Errorf("failed to create driver: %w", err)
	}

	// Test connection
	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, fmt.Errorf("failed to connect to Neo4j: %w", err)
	}

	return &Client{
		driver: driver,
		uri:    uri,
		user:   user,
		pass:   pass,
	}, nil
}

// AxiomExists checks if an axiom exists in Neo4j
func (c *Client) AxiomExists(ctx context.Context, axiomID string) (bool, error) {
	if c.driver == nil {
		return false, fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (a:Axiom {id: $axiom_id}) RETURN a.id AS id",
		map[string]interface{}{"axiom_id": axiomID},
	)
	if err != nil {
		return false, fmt.Errorf("query failed: %w", err)
	}

	return result.Next(ctx), nil
}

// VerifyAxiomsExist implements the AxiomFirewall interface.
// It mathematically enforces Invariant 6 (Axiom Grounding) by ensuring
// every single axiom cited by the LLM physically exists in the Code Graph.
func (c *Client) VerifyAxiomsExist(ctx context.Context, axioms []types.AxiomID) error {
	for _, ax := range axioms {
		exists, err := c.AxiomExists(ctx, ax.String())
		if err != nil {
			return fmt.Errorf("failed to query neo4j for axiom %s: %w", ax.String(), err)
		}
		if !exists {
			return fmt.Errorf("AXIOM FIREWALL BLOCK: LLM cited axiom %s, but it does not exist in the Neo4j ontology", ax.String())
		}
	}
	return nil
}

// GetAxiomsByDomain retrieves axioms for a given domain
func (c *Client) GetAxiomsByDomain(ctx context.Context, domain string) ([]map[string]interface{}, error) {
	if c.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (a:Axiom) WHERE toLower(a.domain) = toLower($domain) RETURN a.id AS id, a.equation AS equation ORDER BY a.id LIMIT $limit",
		map[string]interface{}{"domain": domain, "limit": int64(10000)},
	)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var axioms []map[string]interface{}
	for result.Next(ctx) {
		axioms = append(axioms, result.Record().AsMap())
	}

	return axioms, result.Err()
}

// GetDecisionCount returns total number of decisions in cache
func (c *Client) GetDecisionCount(ctx context.Context) (int, error) {
	if c.driver == nil {
		return 0, fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	result, err := session.Run(ctx,
		"MATCH (d:Decision) RETURN count(d) AS count",
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("query failed: %w", err)
	}

	if !result.Next(ctx) {
		return 0, fmt.Errorf("no result from count query")
	}

	count, ok := result.Record().Get("count")
	if !ok {
		return 0, fmt.Errorf("count not found in result")
	}

	return int(count.(int64)), nil
}

// Close closes the Neo4j connection
func (c *Client) Close(ctx context.Context) error {
	if c.driver != nil {
		return c.driver.Close(ctx)
	}
	return nil
}

// Health checks if Neo4j is available
func (c *Client) Health(ctx context.Context) error {
	if c.driver == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	return c.driver.VerifyConnectivity(ctx)
}

// QueryDecisionsByAxiom retrieves all decisions citing a specific axiom
func (c *Client) QueryDecisionsByAxiom(ctx context.Context, axiomID string) ([]map[string]interface{}, error) {
	if c.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	query := `
		MATCH (d:Decision)-[:CITES]->(a:Axiom)
		WHERE a.id = $axiom
		RETURN d.id AS id, d.decision_text AS decision_text, d.recorded_at AS recorded_at
		ORDER BY d.recorded_at DESC
		LIMIT $limit
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"axiom": axiomID,
		"limit": int64(10000),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var decisions []map[string]interface{}
	for result.Next(ctx) {
		decisions = append(decisions, result.Record().AsMap())
	}

	return decisions, result.Err()
}

// FilterAxiomsByDomain retrieves axioms matching a specific domain
func (c *Client) FilterAxiomsByDomain(ctx context.Context, domain string) ([]map[string]interface{}, error) {
	if c.driver == nil {
		return nil, fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeRead,
	})
	defer session.Close(ctx)

	query := `
		MATCH (a:Axiom)
		WHERE toLower(a.domain) = toLower($domain)
		RETURN a.id AS id, a.principle AS principle, a.domain AS domain, a.category AS category
		ORDER BY a.id
		LIMIT $limit
	`

	result, err := session.Run(ctx, query, map[string]interface{}{
		"domain": domain,
		"limit":  int64(10000),
	})
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}

	var axioms []map[string]interface{}
	for result.Next(ctx) {
		axioms = append(axioms, result.Record().AsMap())
	}

	return axioms, result.Err()
}

// CreateDecisionNode creates or updates a Decision node in Neo4j
func (c *Client) CreateDecisionNode(ctx context.Context, id, decisionText string, recordedAt time.Time) error {
	if c.driver == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	query := `
		MERGE (d:Decision {id: $id})
		SET d.decision_text = $decision_text, d.recorded_at = $recorded_at
	`

	_, err := session.Run(ctx, query, map[string]interface{}{
		"id":            id,
		"decision_text": decisionText,
		"recorded_at":   recordedAt,
	})

	return err
}

// CreateCitesRelationship creates a CITES relationship between a Decision and Axiom
func (c *Client) CreateCitesRelationship(ctx context.Context, decisionID, axiomID string) error {
	if c.driver == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	query := `
		MATCH (d:Decision {id: $decision_id}), (a:Axiom {id: $axiom_id})
		MERGE (d)-[:CITES]->(a)
	`

	_, err := session.Run(ctx, query, map[string]interface{}{
		"decision_id": decisionID,
		"axiom_id":    axiomID,
	})

	return err
}

// CreateIndices creates performance indices on Decision and Axiom nodes
func (c *Client) CreateIndices(ctx context.Context) error {
	if c.driver == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	indexQueries := []string{
		"CREATE INDEX IF NOT EXISTS FOR (d:Decision) ON (d.id)",
		"CREATE INDEX IF NOT EXISTS FOR (a:Axiom) ON (a.id)",
		"CREATE INDEX IF NOT EXISTS FOR (a:Axiom) ON (a.domain)",
	}

	for _, query := range indexQueries {
		if _, err := session.Run(ctx, query, nil); err != nil {
			// Ignore index already exists errors
			if !contains(err.Error(), "already exists") {
				return fmt.Errorf("index creation failed: %w", err)
			}
		}
	}

	return nil
}

// ClearCache removes all Decision and CITES relationships from Neo4j
func (c *Client) ClearCache(ctx context.Context) error {
	if c.driver == nil {
		return fmt.Errorf("neo4j client not initialized")
	}

	session := c.driver.NewSession(ctx, neo4j.SessionConfig{
		AccessMode: neo4j.AccessModeWrite,
	})
	defer session.Close(ctx)

	query := `
		MATCH (d:Decision)
		DETACH DELETE d
	`

	_, err := session.Run(ctx, query, nil)
	return err
}

// contains checks if string contains substring (helper)
func contains(s, substr string) bool {
	if substr == "" {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
