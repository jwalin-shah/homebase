# HomeBase: Immutable Decision Ledger

**Status:** Phase 0 Design Complete, Ready for Phase 1

---

## Essential Files (Read These)

### 1. **SYSTEM-DESIGN.md** (Start Here)
**Location:** `/homebase/SYSTEM-DESIGN.md`  
**Size:** ~400 lines  
**Time to read:** 20 minutes  
**What it is:** Formal specification—the entire system in one document

**Contains:**
- Why graphs, not loops
- Three architectural commitments
- State machine (5 states + transitions)
- 6 provable invariants (I1-I6)
- Type system design
- Success criteria (testable)

**Use this to:** Understand what HomeBase does and how it works

---

### 2. **AGENTS.md** (Implementation Blueprint)
**Location:** `/homebase/AGENTS.md`  
**Size:** ~250 lines  
**Time to read:** 15 minutes  
**What it is:** How to implement SYSTEM-DESIGN.md

**Contains:**
- Recap of graph structure (5 states)
- 5-phase workflow (Phase 0-5)
- 4 parallel tickets (202-205)
- What each ticket builds
- File structure (where code goes)
- Commands (build, test, CI)

**Use this to:** Build code that matches SYSTEM-DESIGN.md

---

### 3. **CLAUDE.md** (Project Principles)
**Location:** `/homebase/CLAUDE.md`  
**Size:** ~300 lines  
**Time to read:** 15 minutes  
**What it is:** How to think about HomeBase problems

**Contains:**
- 5 principles specific to graph design
- Code review checklist (enforce graph structure)
- Common mistakes to avoid
- Alignment with global CLAUDE.md

**Use this to:** Write code that doesn't drift back to loops

---

### 4. **PHASE-ENFORCEMENT-FRAMEWORK.md** (Process)
**Location:** `/homebase/tickets/PHASE-ENFORCEMENT-FRAMEWORK.md`  
**Size:** ~200 lines  
**Time to read:** 10 minutes  
**What it is:** How to prevent partial work (the "no skip steps" rule)

**Contains:**
- 5 roles (Discovery Lead, Architect, Implementation, Tester, Auditor)
- Phase sign-off sheet template
- 3 enforcement layers (rule + checklist + harness)
- Quality metrics (track over time)

**Use this to:** Ensure every phase is complete before moving forward

---

## Optional Files (Reference Only)

### Deep Dives (Only Read If Implementing That Piece)

| File | Location | Size | When to Read |
|------|----------|------|--------------|
| COMMON-BUGS-CATALOG.md | tickets/ | 400 lines | When writing Phase 1 code (phase gate checks) |
| GROUNDED-REASONING-EVIDENCE.md | tickets/ | 200 lines | When collecting evidence (Phase 2-4) |
| ENFORCEMENT-HARNESS-LOCAL.md | tickets/ | 250 lines | When setting up git hooks |
| PHASE-0-CHECKLIST-SPECIFICATION.md | tickets/ | 150 lines | Phase 0 sign-off (what to verify) |
| PHASE-1-CHECKLIST-IMPLEMENTATION.md | tickets/ | 200 lines | Phase 1 code review (what to check) |
| PHASE-4-CHECKLIST-INDEPENDENT-AUDIT.md | tickets/ | 350 lines | Phase 4 audit (stress tests) |

---

## Files to DELETE (Old Work, Not Needed)

These were from earlier iterations and are now superseded:

```
/homebase/PHASE-2-PLAN.md                      ← DELETE (obsolete)
/homebase/SYSTEM-AUDIT.md                      ← DELETE (obsolete)
/homebase/TICKET-202-PHASE-*.md (all 5 files)  ← DELETE (superseded by tickets/TICKET-202-PHASE-*.md)
/homebase/tickets/TICKET-202-BRIDGE-SPECIFICATION-PHASE-0.md  ← DELETE
/homebase/tickets/TICKET-203-AXIOMS-PHASE-0.md  ← DELETE
/homebase/tickets/TICKET-204-INTEGRATION-TESTING-PHASE-0.md  ← DELETE
/homebase/tickets/TICKET-205-OBSERVABILITY-PHASE-0.md  ← DELETE
/homebase/tickets/TICKET-203-PHASE-1-AXIOM-QUERYING.md  ← DELETE
/homebase/tickets/TICKET-204-PHASE-1-TEST-ISOLATION-FIX.md  ← DELETE
/homebase/tickets/TICKET-205-PHASE-1-STRUCTURED-LOGGING.md  ← DELETE
```

---

## Quick Reference: What to Do Now

### If you're the Captain:
1. Read **SYSTEM-DESIGN.md** (20 min)
2. Read **AGENTS.md** (15 min)
3. Decide: Approve design, or request changes?
4. If approved: Phase 1 implementation starts

### If you're implementing (Phase 1):
1. Read **SYSTEM-DESIGN.md** (understand the graph)
2. Read **AGENTS.md** (understand your ticket's states)
3. Read **CLAUDE.md** (understand the principles)
4. Read **PHASE-1-CHECKLIST-IMPLEMENTATION.md** (before committing code)
5. Code your ticket (one of 202-205)

### If you're testing (Phase 2-3):
1. Read **SYSTEM-DESIGN.md** (what to verify)
2. Read **AGENTS.md** (the 5 states you're testing)
3. Read **COMMON-BUGS-CATALOG.md** (what can go wrong)
4. Write tests that verify the state machine works

### If you're auditing (Phase 4):
1. Read **SYSTEM-DESIGN.md** (formal spec)
2. Read **PHASE-4-CHECKLIST-INDEPENDENT-AUDIT.md** (what to stress-test)
3. Verify 6 invariants hold
4. Document findings in **AUDIT-FINDINGS-[TICKET]-PHASE-4.md**

---

## File Organization (Where Everything Lives)

```
/homebase/
├── README.md                          ← You are here
├── SYSTEM-DESIGN.md                   ← Formal spec (START)
├── AGENTS.md                          ← Implementation (START)
├── CLAUDE.md                          ← Principles (START)
│
├── cmd/homebase/
│   └── main.go                        ← Code entry point
│
├── internal/                          ← Implementation (per ticket)
│   ├── graph/                         ← NEW: States (PLAN, EXECUTE, etc.)
│   ├── ledger/                        ← JSONL store
│   ├── signing/                       ← Ed25519 (I4, I5)
│   ├── cache/                         ← Neo4j (I6)
│   └── validation/
│
├── scripts/
│   ├── setup-hooks.sh                 ← Install git hooks (Phase 1)
│   ├── collect-phase-evidence.sh      ← Collect evidence (each phase)
│   └── common-bugs-check.sh           ← Scan for known patterns
│
├── .githooks/
│   ├── pre-commit                     ← Catch bugs before commit
│   ├── commit-msg                     ← Validate ticket reference
│   └── phase-gate-local               ← Check phase progression
│
└── tickets/
    ├── PHASE-ENFORCEMENT-FRAMEWORK.md ← Process (how to enforce)
    │
    ├── PHASE-0-CHECKLIST-SPECIFICATION.md
    ├── PHASE-1-CHECKLIST-IMPLEMENTATION.md
    ├── PHASE-4-CHECKLIST-INDEPENDENT-AUDIT.md
    │
    ├── COMMON-BUGS-CATALOG.md         ← 11 bug categories + tests
    ├── GROUNDED-REASONING-EVIDENCE.md ← How to prove work
    ├── ENFORCEMENT-HARNESS-LOCAL.md   ← Git hooks explained
    │
    └── TICKET-[202-205]-PHASE-*.md    ← Work in progress (per ticket)
```

---

## Total Reading Time (Essential Only)

- **SYSTEM-DESIGN.md:** 20 min
- **AGENTS.md:** 15 min
- **CLAUDE.md:** 15 min
- **PHASE-ENFORCEMENT-FRAMEWORK.md:** 10 min

**Total: ~60 minutes to understand entire system.**

Optional reading (deeper dives): +2-3 hours if you need to implement specific pieces.

---

## Current Status

| Item | Status |
|------|--------|
| **Phase 0 Design** | ✓ COMPLETE |
| **System Design Formal Spec** | ✓ SYSTEM-DESIGN.md (locked) |
| **Implementation Blueprint** | ✓ AGENTS.md (ready) |
| **Project Principles** | ✓ CLAUDE.md (ready) |
| **Enforcement Rules** | ✓ PHASE-ENFORCEMENT-FRAMEWORK.md (ready) |
| **Phase 1 (Implementation)** | ⏳ PAUSED (waiting for captain approval) |
| **Phase 2-5** | ⏳ BLOCKED (depends on Phase 1) |

---

## Next Step

**Captain:** Approve SYSTEM-DESIGN.md? Yes / No / Needs Changes?

If yes → Phase 1 implementation starts (Tickets 202-205 parallel)
If no → What needs to change?

---

**That's it. Everything else is just details.**
