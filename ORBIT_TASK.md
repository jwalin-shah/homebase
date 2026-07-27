# HomeBase: Unified Decision + Evidence System

**Status:** ORBIT_TASK Authorization  
**Authority:** HB-001 Specification + Captain Approved  
**Task Type:** Implementation (Phase 2-4)

---

## What

Build HomeBase — a unified decision + evidence system with:
- Axiom-grounded decisions (cite proven principles)
- Immutable ledger (JSONL append-only)
- Neo4j integration (knowledge engine)
- Tool circuit breakers (resilience)
- Offline capability
- Graceful degradation

## Tickets

- **201:** Portfolio Redesign (Master Control Plane)
- **202:** Bridge Redesign (AX-SPAWN-001 Implementation)
- **203:** Axioms Integration (Knowledge Engine)

## Stack

- **Language:** Go (primary) + Python (Neo4j seam)
- **Go 1.26** single module
- **Neo4j 5.x** knowledge store
- **JSONL** ledger (immutable, durable)

## Axioms Driving Design

- **AX-GO-002:** Error checking on deferred Close()
- **AX-ORACLE-CORRECT-014:** Design by Contract
- **AX-ORACLE-CORRECT-029:** State Invariant
- **AX-GO-001:** Goroutine leak prevention
- **AX-SAIP-010-026:** Performance axioms

---

Authorization granted for implementation.
