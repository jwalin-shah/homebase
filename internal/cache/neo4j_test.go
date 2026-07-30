package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewClientRejectsMissingPassword(t *testing.T) {
	_, err := NewClient("neo4j://localhost:7687", "neo4j", "   ")
	if err == nil {
		t.Fatal("expected missing password to fail closed")
	}
	if !strings.Contains(err.Error(), "refusing an implicit credential") {
		t.Fatalf("error = %q, want explicit fail-closed error", err)
	}
}

// TestClient_NewClient_Nil tests creating client with nil driver
func TestClient_CreateDecisionNode_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
		uri:    "neo4j://localhost:7474",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.CreateDecisionNode(ctx, "test-id", "Test decision", time.Now())
	if err == nil {
		t.Errorf("Expected error for nil driver, got nil")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_CreateCitesRelationship_NilClient tests relationship creation with nil driver
func TestClient_CreateCitesRelationship_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.CreateCitesRelationship(ctx, "dec-1", "AX-001")
	if err == nil {
		t.Errorf("Expected error for nil driver, got nil")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_CreateIndices_NilClient tests index creation with nil driver
func TestClient_CreateIndices_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.CreateIndices(ctx)
	if err == nil {
		t.Errorf("Expected error for nil driver, got nil")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_QueryDecisionsByAxiom_NilClient tests axiom query with nil driver
func TestClient_QueryDecisionsByAxiom_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := client.QueryDecisionsByAxiom(ctx, "AX-001")
	if err == nil || results != nil {
		t.Errorf("Expected error and nil results for nil driver")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_FilterAxiomsByDomain_NilClient tests domain filter with nil driver
func TestClient_FilterAxiomsByDomain_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	results, err := client.FilterAxiomsByDomain(ctx, "DECISION")
	if err == nil || results != nil {
		t.Errorf("Expected error and nil results for nil driver")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_ClearCache_NilClient tests cache clearing with nil driver
func TestClient_ClearCache_NilClient(t *testing.T) {
	// Create a client with no driver
	client := &Client{
		driver: nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := client.ClearCache(ctx)
	if err == nil {
		t.Errorf("Expected error for nil driver, got nil")
	}

	if err.Error() != "neo4j client not initialized" {
		t.Errorf("Expected 'neo4j client not initialized', got '%s'", err.Error())
	}
}

// TestClient_Contains_Helper tests the contains helper function
func TestContains_Helper(t *testing.T) {
	tests := []struct {
		str    string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello", "hello", true},
		{"hello", "x", false},
		{"", "", false},
		{"x", "", false},
		{"already exists", "already", true},
		{"index already exists", "already exists", true},
	}

	for _, tt := range tests {
		got := contains(tt.str, tt.substr)
		if got != tt.want {
			t.Errorf("contains(%q, %q) = %v, want %v", tt.str, tt.substr, got, tt.want)
		}
	}
}
