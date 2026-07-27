# TICKET 201: Portfolio Redesign Implementation

**ID:** 201  
**Title:** Build Portfolio as Master Control Plane  
**Status:** Ready to Implement  
**Authority:** HB-001 Specification + Portfolio Redesign V2 + Execution Plan  
**Timeline:** 1 week (Phase 2)  
**Blocked by:** None (can start immediately)  
**Blocks:** Tickets 202, 203

---

## What

Implement Portfolio redesign as a live system. Portfolio is the **master control plane** for ALL decisions (architectural, implementation, policy).

Every decision gets logged with:
- Axiom citations (why chosen)
- Evidence (what backs it)
- Immutable ledger (JSONL append-only)
- Audit trail (who/when/what changed)

---

## From

- `/Users/jwalinshah/projects/portfolio/PORTFOLIO_REDESIGN_V2.md`
- `/Users/jwalinshah/projects/homebase/HB-001-COMPLETE-SPECIFICATION.md`
- `/Users/jwalinshah/projects/portfolio/wayfinder/system-redesign-axiom-first-2026-07-25/map.md`
- `/Users/jwalinshah/projects/portfolio/EXECUTION_PLAN.md`

---

## Tasks

### Task 1: Wayfinder Map Structure
Implement directory structure for active + completed maps:

```
portfolio/wayfinder/
├── active-maps/
│   ├── system-redesign-axiom-first-2026-07-25/
│   │   ├── map.md (plan + decisions + status)
│   │   ├── tickets/ (201, 202, 203 specs)
│   │   └── results/ (deliverables)
│   └── [other active]
├── completed-maps/ (archive)
└── template-map.md (for new maps)
```

**Success:** Directories created, existing map moved to correct location, template created.

---

### Task 2: Decision Ledger (JSONL)
Create immutable decision log at `portfolio/ledger.jsonl`

Each line (JSON):
```json
{
  "id": "dec-20260725-001",
  "date": "2026-07-25T10:00:00Z",
  "decision": "Portfolio is control plane",
  "axioms": ["AX-ARCHITECTURE-004", "AX-DECISION-001"],
  "evidence": "Documentation audit + systematic review",
  "decided_by": "captain",
  "approver": "captain",
  "status": "APPROVED",
  "tags": ["architecture"],
  "risk_level": "critical",
  "affected_systems": ["portfolio", "bridge", "orbit"],
  "related_decisions": []
}
```

Queryable by: ID, axiom, tag, date range, risk level.

**Success:** JSONL created, append-only enforced, queryable, schema validated.

---

### Task 3: Axiom Validation Gate
Wire portfolio to Neo4j. Before approving decision:
- Query: "Do cited axioms exist?"
- If axiom missing → reject decision
- Log axiom verification

**Success:** Neo4j integrated, validation gate works, all ledger decisions have verifiable axioms.

---

### Task 4: Coordination Layer
Bridge/Orbit/Axioms report decisions back to portfolio.

Bridge: "executed spawn ticket-202, result: success"  
Orbit: "verified portfolio-decision, tests: PASS"  
Axioms: "discovered new axiom AX-DECISION-002"

**Success:** All systems can log decisions back to portfolio.

---

### Task 5: Wayfinder Template
Create template for new maps (used for future projects).

**Success:** Template created, documented, ready for use.

---

## Acceptance Criteria

Portfolio redesign is DONE when:

- ☑ Wayfinder structure (active-maps, completed-maps, template)
- ☑ ledger.jsonl queryable (ID, axiom, tag, date, risk)
- ☑ Neo4j axiom validation gate
- ☑ Bridge can log decisions
- ☑ Orbit can log test results
- ☑ Axioms can log discoveries
- ☑ All existing decisions have axiom citations
- ☑ Wayfinder template created
- ☑ Portfolio queryable (CLI or HTTP API)
- ☑ Documentation complete

---

## Owned by

Portfolio team (or captain if solo)

---

## Next

Bridge can then read portfolio decisions (ticket 202 depends on this).
