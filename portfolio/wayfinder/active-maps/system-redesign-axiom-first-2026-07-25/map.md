# Wayfinder Map: System Redesign (Axiom-First)

**Date Opened:** 2026-07-25  
**Destination:** Complete axiom-grounded redesign of portfolio → bridge → orbit integration  
**Authority:** Captain + systematic review  
**Status:** DESIGN PHASE → IMPLEMENTATION (via new bridge)  

---

## Destination

A complete system redesign where:
- **Portfolio** is the system-wide decision plane (EVERY decision gets logged here with axiom citations)
- **Bridge** reads portfolio decisions, implements axiom-driven spawn, logs results back
- **Orbit** reads portfolio decisions, verifies implementations, logs test results back
- **Axioms** are the source of truth for all engineering principles (queried by portfolio before deciding)
- **All projects feed decisions into portfolio** (bridge, axioms, orbit, trajectory, etc.)
- **Every system builds using the system** (bootstrap via bridge spawn, all decisions logged)

The system is self-documenting: read portfolio to understand why any decision was made.

---

## Decisions Made

### Decision 1: Portfolio Is Control Plane (AX-ARCHITECTURE-004)

**What:** Portfolio owns cross-project state and decision authority

**Axioms Cited:**
- AX-ARCHITECTURE-004: Single authority per fact family
- AX-DECISION-007: Narrowest owner wins
- AX-OBSERVABLE-003: All decisions visible

**Evidence:** Documentation audit showed spawn timeouts → root cause: preconditions not verified → portfolio as control plane would prevent this

**Status:** ✅ DECIDED

---

### Decision 2: Axioms Are Primary Source (AX-KNOWLEDGE-001)

**What:** Every decision must cite at least one axiom

**Axioms Cited:**
- AX-KNOWLEDGE-001: All reasoning grounded in proven principles
- AX-EVIDENCE-002: All claims require proof

**Evidence:** Current system fails because decisions are ad-hoc. Axiom-grounding prevents this.

**Status:** ✅ DECIDED

---

### Decision 3: New Axiom AX-SPAWN-001 (AX-SYSTEMS-IMPROVEMENT)

**What:** Formalize spawn system requirements as axiom

**Equation:**
```
∀worker_execution(W):
  preconditions_verified(W) ∧
  tool_calls_observable(W) ∧
  failures_explicit(W) ∧
  escalation_path_exists(W)
```

**Evidence:** Discovered through spawn timeout analysis — all 4 conditions violated

**Status:** ✅ DECIDED (axiom document created at axioms/NEW_AXIOM_AX_SPAWN_001.md)

---

### Decision 4: Bridge Redesign Must Follow AX-SPAWN-001 (AX-SYSTEMS-012)

**What:** Rebuild spawn system to satisfy all 4 conditions of AX-SPAWN-001

**Changes:**
1. Precondition check: verify sandbox + commands before spawning
2. Live pane watching: mintmux observability, not polling
3. Explicit errors: no timeouts, all failures have reasons
4. Escalation layer: tool interception + permission asks

**Evidence:** Current spawn times out with no information. AX-SPAWN-001 prevents this.

**Status:** ✅ DECIDED

---

## Active Work

### Ticket 201: Portfolio Redesign Implementation

**What:** Implement portfolio redesign document as live system

**Requirements:**
- Wayfinder maps directory structure
- Axiom-grounded decision format
- Ledger (immutable audit trail)
- Integration with bridge + orbit

**Spawned Via:** Bridge spawn (once new bridge ready)

**Acceptance Criteria:**
- [ ] Portfolio redesign document exists (PORTFOLIO_REDESIGN_V2.md)
- [ ] Wayfinder map structure implemented
- [ ] Decision ledger stores axiom citations
- [ ] Bridge can read portfolio decisions
- [ ] Orbit can verify portfolio against axioms

**Status:** 📋 PENDING (awaiting new bridge implementation)

---

### Ticket 202: Bridge Redesign (AX-SPAWN-001 Implementation)

**What:** Rebuild spawn system to satisfy AX-SPAWN-001

**Requirements:**
1. **Precondition check**
   - Read sandbox.sb
   - Read ticket.verification_commands
   - Cross-check: can all commands run?
   - Fail BEFORE spawning if mismatch

2. **Live observability**
   - Use mintmux to watch worker pane
   - Log all tool calls in real-time
   - Don't poll for manifest.json

3. **Explicit errors**
   - No timeouts → explicit reasons
   - Worker writes manifest with error type:
     * "denied: <permission>"
     * "tool-not-found: <tool>"
     * "crashed: <error message>"

4. **Escalation layer**
   - Intercept denied tool calls
   - Write manifest: "needs_escalation"
   - Bridge shows captain specific ask
   - Captain approves → Bridge retries with permission

**Spawned Via:** New bridge (once ticket design agreed)

**Acceptance Criteria:**
- [ ] Precondition check prevents timeout scenarios
- [ ] Live pane watching shows worker progress
- [ ] All failures have explicit reasons
- [ ] Permission denials escalate automatically
- [ ] All decisions cited to AX-SPAWN-001

**Status:** 📋 PENDING DESIGN

---

### Ticket 203: Axioms Integration

**What:** Integrate axioms as live knowledge engine in portfolio + bridge

**Requirements:**
- Portfolio queries axioms before major decisions
- Bridge references axioms in every spawn decision
- Orbit verifies implementations against axioms
- Every decision logs axiom citations

**Spawned Via:** New bridge

**Acceptance Criteria:**
- [ ] Portfolio queries Neo4j for axioms
- [ ] All portfolio decisions cite axiom IDs
- [ ] Bridge can load relevant axioms for spawn checks
- [ ] Orbit uses axioms for verification gates

**Status:** 📋 PENDING BRIDGE REDESIGN

---

## Not Yet Specified

1. **Wayfinder map template** — How exactly should maps be structured?
2. **Decision format** — JSON? Markdown? Both?
3. **Ledger queryability** — How should portfolio ledger be queried?
4. **Orbit verification** — What gates verify portfolio decisions?
5. **Escalation UI** — How does captain approve permissions?
6. **Axiom versioning** — How do we handle axiom updates?

---

## Out of Scope (For This Map)

- Redesigning individual projects (axioms, orbit, trajectory)
- Changing git workflows
- Modifying CI/CD (outside portfolio authority)
- Updating global CLAUDE.md (in other maps)

---

## Implementation Order

**Phase 1: Prepare (this week)**
- ✅ Portfolio redesign document (done)
- ✅ New axiom AX-SPAWN-001 (done)
- ⏳ Bridge redesign specification (in progress)

**Phase 2: Implement Bridge (new bridge spawns it)**
- Bridge redesign with AX-SPAWN-001
- Live pane watching via mintmux
- Explicit error handling
- Permission escalation layer

**Phase 3: Implement Portfolio**
- Portfolio redesign as live system
- Wayfinder maps for all work
- Axiom-grounded decisions
- Audit ledger

**Phase 4: Integrate Orbit**
- Orbit verifies bridge matches axioms
- Orbit verifies portfolio decisions
- Continuous verification gates

**Phase 5: Loop Back**
- New bridge spawns new portfolio
- New portfolio manages new bridge
- Axioms guide all decisions
- System is self-maintaining

---

## How Bridge Will Be Used to Build Bridge

The bootstrap process:

```
Current bridge (broken, has timeouts)
  ↓
Design new bridge following AX-SPAWN-001
  ↓
Create bridge redesign specification
  ↓
Current bridge spawns: "Implement new bridge" ticket
  ↓
(Even though current bridge is flaky, this task doesn't need network)
  ↓
New bridge is built
  ↓
New bridge spawns: "Implement portfolio" ticket
  ↓
Portfolio is built
  ↓
Portfolio spawns: "Verify new bridge against axioms" ticket
  ↓
Orbit tests → everything works
  ↓
System is now self-maintaining
```

This works because:
- The bridge redesign ticket doesn't need network (uses files/git)
- Current bridge, however broken, can handle local file work
- Once new bridge is built, it's used for everything else
- Portfolio controls decisions for both versions

---

## Success Criteria

Map is complete when:

- ✅ Portfolio redesign document created and approved
- ✅ AX-SPAWN-001 axiom formalized
- ✅ Bridge redesign specification matches axioms
- ✅ Bridge implementation ticket spawned and completed
- ✅ Portfolio implementation ticket spawned and completed
- ✅ All systems integrated and verified
- ✅ Every decision in portfolio cites axioms
- ✅ Orbit verifies everything
- ✅ System is self-maintaining

---

## Authority

**This map authorizes:**
1. Spending time on system redesign (rather than feature work)
2. Redesigning bridge despite it being functional
3. Making portfolio the control plane
4. Requiring axiom citations in all decisions
5. Using new bridge to build new bridge (bootstrap)

**Portfolio owns this work** (cross-project coordination).
**Bridge, orbit, axioms are execution tools** (follow portfolio's design).

---

## Next Action

1. **Captain reviews** portfolio redesign + AX-SPAWN-001
2. **Approve or request changes**
3. **Spawn bridge redesign ticket** (via current bridge if possible, else manual)
4. **Implement new bridge** following AX-SPAWN-001
5. **Use new bridge** to spawn everything else

---

**Map Status:** Ready for captain review  
**Decisions:** 4 (all cited to axioms)  
**Tickets:** 3 (pending implementation)  
**Authority Chain:** Captain → Portfolio → Bridge/Orbit/Axioms  
