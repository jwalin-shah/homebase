# TICKET 203: Axioms Integration - Phase 1 Implementation

**Date:** 2026-07-26  
**Phase:** 1 (Implementation)  
**Status:** IN PROGRESS  
**Focus:** Full Neo4j querying capability

---

## Task 1: Implement Query by Axiom Endpoint

**Endpoint:** `GET /api/v1/decisions?axiom=AX-001`

```go
// handlers.go - Add new handler
func (h *Handler) QueryDecisionsByAxiom(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
  }
  
  axiom := r.URL.Query().Get("axiom")
  if axiom == "" {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(ErrorResponse{Error: "axiom parameter required", Status: 400})
    return
  }
  
  // Query Neo4j
  decisions, err := h.validator.QueryDecisionsByAxiom(axiom)
  if err != nil {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("query failed: %v", err), Status: 500})
    return
  }
  
  response := map[string]interface{}{
    "axiom": axiom,
    "decisions": decisions,
    "count": len(decisions),
  }
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(response)
}
```

**Register Route:** `server.go`
```go
mux.HandleFunc("/api/v1/decisions", func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Query().Get("axiom") != "" {
    h.QueryDecisionsByAxiom(w, r)
  } else {
    // existing ListDecisions
  }
})
```

---

## Task 2: Implement Axiom Filtering by Domain

**Endpoint:** `GET /api/v1/axioms?domain=DECISION`

```go
// handlers.go
func (h *Handler) FilterAxiomsByDomain(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
  }
  
  domain := r.URL.Query().Get("domain")
  if domain == "" {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusBadRequest)
    json.NewEncoder(w).Encode(ErrorResponse{Error: "domain parameter required", Status: 400})
    return
  }
  
  // Query Neo4j by domain
  axioms, err := h.validator.FilterAxiomsByDomain(domain)
  if err != nil {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("filter failed: %v", err), Status: 500})
    return
  }
  
  response := map[string]interface{}{
    "domain": domain,
    "axioms": axioms,
    "count": len(axioms),
  }
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(response)
}
```

**Register Route:** `server.go`
```go
mux.HandleFunc("/api/v1/axioms", func(w http.ResponseWriter, r *http.Request) {
  if r.URL.Query().Get("domain") != "" {
    h.FilterAxiomsByDomain(w, r)
  } else if r.Method == http.MethodPost && r.URL.Path == "/api/v1/axioms/ingest" {
    h.IngestAxiom(w, r)
  }
})
```

---

## Task 3: Cache Rebuild Endpoint

**Endpoint:** `POST /api/v1/axioms/rebuild-cache`

```go
// handlers.go
func (h *Handler) RebuildAxiomCache(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodPost {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
  }
  
  start := time.Now()
  
  // Read all decisions from ledger
  decisions := h.ledger.List(100000)
  
  // Track statistics
  axiomSet := make(map[string]bool)
  relationshipCount := 0
  
  // Rebuild cache in Neo4j
  for _, decision := range decisions {
    // Create Decision node
    err := h.validator.CreateDecisionNode(decision)
    if err != nil {
      w.Header().Set("Content-Type", "application/json")
      w.WriteHeader(http.StatusInternalServerError)
      json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("cache rebuild failed: %v", err), Status: 500})
      return
    }
    
    // Create CITES relationships
    for _, axiom := range decision.Axioms {
      axiomSet[axiom] = true
      err := h.validator.CreateCitesRelationship(decision.ID, axiom)
      if err != nil {
        // Log and continue (partial rebuild is OK)
        continue
      }
      relationshipCount++
    }
  }
  
  // Create indices
  h.validator.CreateIndices()
  
  duration := time.Since(start).Milliseconds()
  
  response := map[string]interface{}{
    "status": "cache_rebuilt",
    "decisions_processed": len(decisions),
    "axioms_indexed": len(axiomSet),
    "relationships_created": relationshipCount,
    "duration_ms": duration,
  }
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(response)
}
```

**Register Route:** `server.go`
```go
mux.HandleFunc("/api/v1/axioms/rebuild-cache", h.RebuildAxiomCache)
```

---

## Task 4: Implement Neo4j Query Methods

**In validation.go or new axioms.go:**

```go
// Query decisions by axiom
func (v *Validator) QueryDecisionsByAxiom(axiom string) ([]Decision, error) {
  if v.cacheClient == nil {
    return nil, fmt.Errorf("neo4j unavailable")
  }
  
  query := `
    MATCH (d:Decision)-[:CITES]->(a:Axiom {id: $axiom})
    RETURN d.id, d.decision_text, d.recorded_at
    ORDER BY d.recorded_at DESC
  `
  
  results, err := v.cacheClient.Query(query, map[string]interface{}{
    "axiom": axiom,
  })
  
  if err != nil {
    return nil, err
  }
  
  decisions := make([]Decision, 0)
  for _, result := range results {
    decisions = append(decisions, parseDecisionNode(result))
  }
  
  return decisions, nil
}

// Filter axioms by domain
func (v *Validator) FilterAxiomsByDomain(domain string) ([]Axiom, error) {
  if v.cacheClient == nil {
    return nil, fmt.Errorf("neo4j unavailable")
  }
  
  query := `
    MATCH (a:Axiom)
    WHERE a.domain = $domain
    RETURN a.id, a.principle, a.domain, a.category
    ORDER BY a.id
  `
  
  results, err := v.cacheClient.Query(query, map[string]interface{}{
    "domain": domain,
  })
  
  if err != nil {
    return nil, err
  }
  
  axioms := make([]Axiom, 0)
  for _, result := range results {
    axioms = append(axioms, parseAxiomNode(result))
  }
  
  return axioms, nil
}

// Create indices for performance
func (v *Validator) CreateIndices() error {
  if v.cacheClient == nil {
    return nil  // Graceful degradation
  }
  
  queries := []string{
    "CREATE INDEX ON :Decision(id)",
    "CREATE INDEX ON :Axiom(id)",
    "CREATE INDEX ON :Axiom(domain)",
  }
  
  for _, q := range queries {
    v.cacheClient.Execute(q)
  }
  
  return nil
}
```

---

## Implementation Checklist

- [ ] Task 1: Query by axiom endpoint (1 hour)
- [ ] Task 2: Filter by domain endpoint (1 hour)
- [ ] Task 3: Cache rebuild endpoint (1 hour)
- [ ] Task 4: Neo4j query methods (2 hours)
- [ ] Error handling (30 min)
- [ ] Route registration (30 min)
- [ ] Build verification (15 min)
- [ ] Phase 1 finalization (15 min)

**Estimated Phase 1 Time:** 6-7 hours

---

## Success Criteria

- [ ] All 4 endpoints implemented
- [ ] Graceful degradation (works without Neo4j)
- [ ] All error paths handled
- [ ] Binary builds without errors
- [ ] Performance targets met (< 100ms queries)

---

**Phase 1 Status:** STARTING NOW

Next: Implement Task 1 (Query by axiom)
