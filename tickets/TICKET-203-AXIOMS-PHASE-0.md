# TICKET 203: Axioms Integration - Phase 0 Specification

**Date:** 2026-07-26  
**Phase:** 0 (Specification Review)  
**Status:** IN PROGRESS  
**Decision ID:** DEC-2026-07-26-002-AXIOMS  
**Depends On:** Ticket 202 (Bridge) ✅ COMPLETE

---

## Overview

Ticket 203 enhances axiom handling beyond basic validation. While Ticket 201 implemented basic axiom checking (verify axiom exists), Ticket 203 adds full Neo4j integration for querying, filtering, and relationship navigation.

---

## Existing State (From Ticket 201)

✅ **What works:**
- Basic axiom validation (axiom exists check)
- Graceful degradation (continues without Neo4j)
- Axiom citations on decisions

❌ **What's missing:**
- Decision querying by axiom
- Axiom filtering by domain
- Axiom relationship navigation
- Cache invalidation/rebuild
- Axiom statistics

---

## Phase 0 Requirements

### 1. Decision Query by Axiom

**Endpoint:** `GET /api/v1/decisions?axiom=AX-001`

**Response:**
```json
{
  "axiom": "AX-001",
  "decisions": [
    { "id": "dec-001", "decision": "...", "recorded_at": "..." },
    { "id": "dec-002", "decision": "...", "recorded_at": "..." }
  ],
  "count": 2
}
```

**Query Path:**
1. Parse axiom parameter
2. Query Neo4j: `MATCH (d:Decision)-[:CITES]->(a:Axiom {id: $axiom}) RETURN d`
3. Return decisions that cite this axiom

---

### 2. Axiom Filtering by Domain

**Endpoint:** `GET /api/v1/axioms?domain=DECISION&filter=CORE`

**Response:**
```json
{
  "domain": "DECISION",
  "filter": "CORE",
  "axioms": [
    { "id": "AX-001", "principle": "...", "category": "CORE" },
    { "id": "AX-002", "principle": "...", "category": "CORE" }
  ],
  "count": 2
}
```

**Query Path:**
1. Parse domain and filter parameters
2. Query Neo4j: `MATCH (a:Axiom) WHERE a.domain = $domain AND a.category = $filter RETURN a`
3. Return axioms matching criteria

---

### 3. Axiom Cache Rebuild

**Endpoint:** `POST /api/v1/axioms/rebuild-cache`

**Purpose:** Rebuild Neo4j cache from ledger

**Process:**
1. Read all decisions from ledger
2. Extract all axiom citations
3. For each decision:
   - Create Decision node
   - Create CITES relationship to Axiom node
4. Create indices for performance
5. Return summary

**Response:**
```json
{
  "status": "cache_rebuilt",
  "decisions_processed": 1234,
  "axioms_indexed": 456,
  "relationships_created": 5678,
  "duration_ms": 1234
}
```

---

### 4. Axiom Relationship Navigation

**Capability:** Query axiom relationships (dependencies, related axioms, etc.)

**Endpoint:** `GET /api/v1/axioms/AX-001/related`

**Response:**
```json
{
  "axiom": "AX-001",
  "related": [
    { "id": "AX-002", "relationship": "depends_on" },
    { "id": "AX-003", "relationship": "reinforces" }
  ],
  "decision_count": 123
}
```

---

## Neo4j Schema

### Nodes

**Decision**
- Properties: id, decision_text, recorded_at, risk_level, status
- Index: id (unique)

**Axiom**
- Properties: id, principle, domain, category, source
- Index: id (unique)

**User**
- Properties: id, email
- Index: id (unique)

### Relationships

**Decision -[CITES]-> Axiom**
- Represents: Decision cites this axiom

**Decision -[DECIDED_BY]-> User**
- Represents: Who made this decision

**Axiom -[DEPENDS_ON]-> Axiom**
- Represents: Axiom dependencies

**Axiom -[REINFORCES]-> Axiom**
- Represents: Related axioms

---

## Error Scenarios

| Scenario | Status | Response |
|----------|--------|----------|
| Invalid axiom ID | 400 | "axiom_id required" |
| Axiom not found | 404 | "axiom not found: AX-999" |
| Invalid domain filter | 400 | "invalid domain filter" |
| Neo4j unavailable | 503 | "neo4j currently unavailable" |
| Cache rebuild failure | 500 | "cache rebuild failed: {error}" |

---

## Performance Targets

- Query by axiom: < 100ms
- Filter by domain: < 50ms
- Cache rebuild: < 5 seconds (for 10k decisions)
- Relationships query: < 100ms

---

## Acceptance Criteria

- [ ] All 4 endpoints specified (query by axiom, filter by domain, rebuild cache, related axioms)
- [ ] Neo4j schema defined (nodes, relationships, indices)
- [ ] All error scenarios documented
- [ ] Performance targets identified
- [ ] Graceful degradation defined (no Neo4j = skip enhanced queries)
- [ ] Database schema reviewed
- [ ] Architecture team approved

---

## Dependencies

- Neo4j 4.0+ (already optional from Ticket 201)
- Decision ledger (Ticket 201) ✅
- Bridge integration (Ticket 202) ✅

---

## Timeline

- Phase 0: Specification (THIS WEEK) ✅ IN PROGRESS
- Phase 1: Implementation (NEXT WEEK)
- Phase 2: Unit tests (NEXT WEEK)
- Phase 3: Integration tests (NEXT WEEK)
- Phase 4: Independent review (NEXT WEEK)
- Phase 5: Production ready (END OF WEEK)

---

**Phase 0 Status:** READY FOR ARCHITECTURE TEAM REVIEW
