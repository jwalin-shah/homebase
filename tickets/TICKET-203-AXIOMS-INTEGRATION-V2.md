# TICKET 203: Axioms Integration

**ID:** 203  
**Title:** Wire Axioms as Live Knowledge Engine in Portfolio + Bridge  
**Status:** Ready to Implement (Design-Backed)  
**Authority:** HB-001 Specification + DESIGN.md + Execution Plan  
**Timeline:** 1 week (Phase 4)  
**Blocked by:** Tickets 201, 202 (portfolio + bridge must exist)  
**Blocks:** None (final integration step)  

**Design Reference:** `/Users/jwalinshah/projects/homebase/docs/DESIGN.md`
- Part 1: System Overview
- Part 2: Data Flow 3 (Axiom Extraction)
- Part 3: Axiom Ingestion Endpoint (10)
- Part 4: Axioms Tests (A1T1-A1T10)

---

## What

Make axioms (Neo4j corpus of 2231+ proven principles) the **live knowledge engine** that drives portfolio + bridge decisions.

Every decision:
- Portfolio: queries axioms before approving
- Bridge: loads axioms before spawning
- Orbit: verifies against axioms
- System: learns from patterns, feeds back new axioms

**Result:** System becomes self-maintaining. Knowledge loops close.

---

## From

- `/Users/jwalinshah/projects/axioms/` (Neo4j corpus)
- `/Users/jwalinshah/projects/homebase/HB-001-COMPLETE-SPECIFICATION.md` (Section 6, Defenses 6, 12)
- `/Users/jwalinshah/projects/homebase/docs/DESIGN.md` (Part 2 Flow 3, Part 3 Endpoint 10, Part 4 Tests A1T1-A1T10)

---

## Tasks

### Task 1: Portfolio Queries Axioms (Before Decisions)
**What:** Portfolio queries Neo4j for axioms relevant to a decision before approving it.

**Files to Create:**
- `internal/cache/query.go` — axiom query patterns
- `internal/cache/query_test.go` — tests (A1T1, A1T5)

**Implementation (from DESIGN.md Part 2, Flow 3, before decision approval):**
```go
func GetRelevantAxioms(decision Decision) ([]*Axiom, error) {
    // Query Neo4j for axioms in the same domain/tags
    
    query := `
    MATCH (a:Axiom) 
    WHERE a.domain IN $domains
    RETURN a
    ORDER BY a.id
    `
    
    // Map decision tags/domains to axiom domains
    domains := MapDecisionDomainsToAxiomDomains(decision.Tags)
    
    axioms, err := neo4j.Query(query, map[string]interface{}{
        "domains": domains,
    })
    
    return axioms, err
}
```

**Workflow (from DESIGN.md Part 2, Flow 3, Step 1):**
```
Decision received:
  1. Extract tags + domain from decision
  2. Query Neo4j: axioms in same domain
  3. Return relevant axioms
  4. Portfolio shows axioms to captain (for context)
  5. Captain cites axioms in decision approval
```

**Success Criteria:**
- ☑ Portfolio queries axioms by domain
- ☑ Returns relevant axioms (security, authentication, systems, etc.)
- ☑ Query latency P99 < 100ms (indexed)
- ☑ Test A1T1 passes (axiom query by domain)
- ☑ Test A1T5 passes (portfolio queries axioms before decisions)

---

### Task 2: Bridge Loads Axioms Before Spawn
**What:** Bridge queries axioms before spawning, verifies compliance with AX-SPAWN-001.

**Files to Create:**
- `internal/spawn/axiom_check.go` — already started in Ticket 202 Task 5
- Extend: `internal/spawn/axiom_check_test.go` — tests (A1T6)

**Implementation (from DESIGN.md Part 2, Flow 2, Step 3):**
```go
func CheckAxiomCompliance(ticket Ticket) error {
    // Query portfolio for axioms
    axioms, err := portfolioClient.GetAxioms([]string{
        "AX-SPAWN-001",
        "AX-SECURITY-004",
        "AX-SYSTEMS-012",
    })
    if err != nil {
        // Portfolio down: continue anyway (graceful degradation)
        // Log: "axioms unavailable, spawn proceeding without axiom check"
        return nil
    }
    
    // Verify AX-SPAWN-001 conditions
    for _, axiom := range axioms {
        if axiom.ID == "AX-SPAWN-001" {
            // Verify all 4 conditions will be met:
            // 1. preconditions_verified ✓ (Task 202 Task 1)
            // 2. tool_calls_observable ✓ (Task 202 Task 2)
            // 3. failures_explicit ✓ (Task 202 Task 3)
            // 4. escalation_path_exists ✓ (Task 202 Task 4)
            
            // Log: "AX-SPAWN-001 conditions verified"
        }
    }
    
    return nil
}
```

**Success Criteria:**
- ☑ Bridge queries axioms before spawn (not during/after)
- ☑ Checks AX-SPAWN-001 + security axioms
- ☑ Logs axiom check in spawn decision
- ☑ Gracefully handles portfolio unavailable
- ☑ Test A1T6 passes (bridge checks axioms before spawn)

---

### Task 3: Orbit Verifies Against Axioms
**What:** Orbit test gates check if implementation satisfies axiom equations.

**Files to Create:**
- `internal/orbit/gate_axiom.go` — axiom verification gates
- `internal/orbit/gate_axiom_test.go` — tests (A1T7)

**Implementation (from DESIGN.md Part 4, A1T7):**
```go
func RunAxiomGate(ticket Ticket) error {
    // Read decision from portfolio
    decision, err := portfolioClient.GetDecision(ticket.ID)
    if err != nil {
        return err
    }
    
    // Get axioms cited in decision
    axioms, err := portfolioClient.GetAxioms(decision.Axioms)
    if err != nil {
        return err
    }
    
    // Run verification gate for each axiom
    for _, axiom := range axioms {
        if axiom.ID == "AX-SPAWN-001" {
            // Test: Does implementation satisfy all 4 conditions?
            err := VerifyAXSPAWN001Conditions(ticket)
            if err != nil {
                return fmt.Errorf("axiom %s violated: %v", axiom.ID, err)
            }
        }
        
        if axiom.ID == "AX-SECURITY-004" {
            // Test: Does implementation follow deny-by-default?
            err := VerifyDenyByDefault(ticket)
            if err != nil {
                return fmt.Errorf("axiom %s violated: %v", axiom.ID, err)
            }
        }
    }
    
    // Log gate results to portfolio
    portfolioClient.LogGateResult(ticket.ID, "axiom_verification", "pass")
    return nil
}
```

**Success Criteria:**
- ☑ Orbit reads portfolio decisions
- ☑ Loads axioms cited in decisions
- ☑ Verification gates check axiom satisfaction
- ☑ Gate results logged to portfolio
- ☑ Test A1T7 passes

---

### Task 4: Knowledge Extraction (Pattern → Axiom → Ingest)
**What:** Periodically scan ledger for patterns, formalize as axioms, ingest to Neo4j.

**Files to Create:**
- `cmd/axiom-extractor/main.go` — periodic extractor
- `internal/axioms/extractor.go` — pattern detection + formalization
- `internal/axioms/extractor_test.go` — tests (A1T2, A1T3, A1T4)

**Implementation (from DESIGN.md Part 2, Flow 3, Steps 1-5):**

**Step 1: Periodically Scan Ledger**
```go
func ScanLedgerForPatterns(hours int) error {
    // Query ledger: last N decisions
    decisions, err := portfolioClient.GetDecisions(
        Limit: 100,
        Since: time.Now().Add(-time.Duration(hours) * time.Hour),
    )
    if err != nil {
        return err
    }
    
    return nil
}
```

**Step 2: Detect Pattern (>10 similar decisions)**
```go
func DetectPattern(decisions []*Decision) *Pattern {
    // Group by {tags, risk_level}
    groups := GroupByTagsAndRisk(decisions)
    
    for group, items := range groups {
        if len(items) > 10 {
            // Pattern found!
            return &Pattern{
                Tag: group.Tag,
                Risk: group.Risk,
                Count: len(items),
                Examples: items[:3],
            }
        }
    }
    
    return nil  // No pattern detected
}
```

**Step 3: Formalize as Axiom Equation**
```go
func FormalizeAxiom(pattern *Pattern) *Axiom {
    // Example: pattern has {tag: "auth", risk: "critical"} (12 decisions)
    // Formalize: ∀d: tag[auth] ∧ risk[critical] → escalation_required(d)
    
    axiom := &Axiom{
        ID: fmt.Sprintf("AX-%s-CRITICAL-001", strings.ToUpper(pattern.Tag)),
        Equation: fmt.Sprintf(
            "∀d ∈ decisions: tag[%s] ∧ risk[critical] → escalation_required(d)",
            pattern.Tag,
        ),
        Domain: pattern.Tag,
        DiscoverySource: "portfolio-ledger-analysis",
        DiscoveryDate: time.Now(),
        DiscoveryContext: fmt.Sprintf("%d %s decisions observed", pattern.Count, pattern.Tag),
        Verdict: "PROPOSED",  // Not VERIFIED yet
    }
    
    return axiom
}
```

**Step 4: Validate New Axiom**
```go
func ValidateAxiom(axiom *Axiom) error {
    // Check if new axiom contradicts existing axioms
    existingAxioms, err := neo4j.Query(`
        MATCH (a:Axiom) WHERE a.domain = $domain RETURN a
    `, map[string]interface{}{"domain": axiom.Domain})
    if err != nil {
        return err
    }
    
    // (Simple check: just verify it doesn't duplicate existing)
    for _, existing := range existingAxioms {
        if existing.ID == axiom.ID {
            return fmt.Errorf("axiom already exists: %s", axiom.ID)
        }
    }
    
    return nil
}
```

**Step 5: Ingest to Neo4j**
```go
func IngestAxiom(axiom *Axiom) error {
    // Call portfolio API endpoint (from DESIGN.md Part 3, Endpoint 10)
    return portfolioClient.IngestAxiom(axiom)
}

// Portfolio backend (Ticket 201) handles:
// - Create Neo4j node: CREATE (a:Axiom {...})
// - Link to HomeBase: CREATE (hb:System)-[:DISCOVERED]->(a)
// - Mark as PROPOSED: verdict: "PROPOSED"
// - Return: axiom_id, queryable_at
```

**Scheduling:**
```go
func StartExtractor() {
    // Run weekly
    ticker := time.NewTicker(7 * 24 * time.Hour)
    defer ticker.Stop()
    
    for range ticker.C {
        decisions, _ := portfolioClient.GetDecisions(Limit: 100)
        pattern := DetectPattern(decisions)
        if pattern != nil {
            axiom := FormalizeAxiom(pattern)
            ValidateAxiom(axiom)
            IngestAxiom(axiom)
        }
    }
}
```

**Success Criteria:**
- ☑ Extractor runs periodically (weekly or on-demand)
- ☑ Detects patterns (>10 similar decisions)
- ☑ Formalizes as axiom equation (valid logic)
- ☑ Validates against existing axioms (no duplicates)
- ☑ Ingests to Neo4j via portfolio API
- ☑ Marks axiom as PROPOSED (not verified)
- ☑ Tests A1T2, A1T3, A1T4 pass

---

### Task 5: Knowledge Loop Closes (Next Decision Uses New Axiom)
**What:** After axiom ingested, next decision can cite it.

**Implementation:**
```
Timeline:
  T1: Ledger has 12 critical auth decisions
  T2: Extractor detects pattern
  T3: Formalize axiom AX-AUTH-CRITICAL-001
  T4: Ingest to Neo4j
  T5: Captain makes new auth decision (tag: auth, risk: critical)
  T6: Portfolio queries axioms (now includes AX-AUTH-CRITICAL-001)
  T7: Captain cites AX-AUTH-CRITICAL-001 in decision
  T8: Portfolio logs decision with axiom citation
  T9: Next decisions benefit from learned pattern
```

**Success Criteria:**
- ☑ New axiom queryable after ingestion
- ☑ Portfolio sees new axiom when querying
- ☑ Next decision can cite new axiom
- ☑ Test A1T8 passes (full knowledge loop)

---

### Task 6: Error Handling & Recovery
**What:** Handle failures gracefully (Neo4j down, extraction fails, etc.)

**Scenarios to Handle (from DESIGN.md Part 5):**
- Neo4j down during extraction → retry on reconnect
- Axiom ingestion fails → mark as FAILED, don't ingest
- Pattern detection timeout → continue with next batch

**Success Criteria:**
- ☑ Extractor resilient to Neo4j unavailability
- ☑ Axiom ingestion failures logged (not silent)
- ☑ System continues even if extraction fails
- ☑ Tests A1T9, A1T10 pass (chaos scenarios)

---

## Acceptance Criteria

Axioms integration is DONE when:

**Task 1 (Portfolio Queries):**
- ☑ Portfolio queries Neo4j for axioms by domain
- ☑ Relevant axioms returned before decisions approved
- ☑ Query latency P99 < 100ms

**Task 2 (Bridge Checks):**
- ☑ Bridge loads axioms before spawn
- ☑ Checks AX-SPAWN-001 + security axioms
- ☑ Logs axiom check in spawn decision

**Task 3 (Orbit Verifies):**
- ☑ Orbit reads portfolio decisions
- ☑ Loads axioms cited in decisions
- ☑ Gate results logged to portfolio

**Task 4 (Extraction):**
- ☑ Extractor runs periodically (weekly)
- ☑ Detects patterns (>10 similar decisions)
- ☑ Formalizes as axiom equations
- ☑ Ingests to Neo4j via portfolio API
- ☑ Axioms marked PROPOSED (not verified)

**Task 5 (Knowledge Loop):**
- ☑ New axioms queryable after ingestion
- ☑ Next decisions can cite new axioms
- ☑ System learns from patterns

**Task 6 (Resilience):**
- ☑ Extractor resilient to Neo4j failures
- ☑ Axiom ingestion failures logged
- ☑ System continues on errors

**All A1 Tests Pass (10/10):**
- A1T1: Axiom query by domain
- A1T2: Pattern detection (>10 similar)
- A1T3: Axiom formalization (valid equation)
- A1T4: Axiom ingestion to Neo4j
- A1T5: Portfolio queries axioms before decisions
- A1T6: Bridge checks axioms before spawn
- A1T7: Orbit verifies against axioms
- A1T8: Full knowledge loop (pattern → axiom → decision)
- A1T9: Extraction resilient to Neo4j down
- A1T10: Axiom contradiction detection

---

## Owned by

Knowledge engine team (or captain if solo)

---

## Next

System is now self-maintaining:
- Portfolio grounds decisions in axioms
- Bridge follows axiom-driven spawn rules
- Orbit verifies axiom compliance
- New patterns discovered and fed back
- Axiom corpus grows with experience
- Loop closes: decisions → patterns → axioms → next decisions
