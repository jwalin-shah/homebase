# Phase 0 Checklist: Specification

**Goal:** Prevent implementation of unclear or incomplete requirements.

**Owner:** Discovery Lead (writes), Architect (reviews)

**Blocking:** No Phase 1 starts until this checklist is 100% complete and both roles sign off.

---

## Discovery Lead Checklist

### Requirements Clarity
- [ ] What problem are we solving? (one sentence)
- [ ] Who is the user/customer? (be specific: "internal tool for X team" vs "external API")
- [ ] What are the success criteria? (measurable)
- [ ] What are the failure modes we care about? (what's unacceptable?)

**Verify Each:**
```
Problem: [one sentence]
User: [role + constraints]
Success: [measurable metric or behavior]
Failure: [what breaks the system]
```

### Use Cases
- [ ] Write 3-5 main use cases in "As a [role], I want to [action], so that [outcome]" format
- [ ] Include one edge case or error scenario per use case
- [ ] Each use case has a happy path AND an error path

**Example for HomeBase:**
```
Use Case 1 (Happy Path):
  As an Architect, I want to record a decision with evidence,
  so that future teams understand why we chose Redis.
  
Use Case 1 (Error Path):
  As an Architect, I want to know if the system is down,
  so that I don't think my decision was recorded when it wasn't.
```

### Constraints & Non-Functional Requirements
- [ ] Performance: What's acceptable latency? (e.g., <100ms for decision recording)
- [ ] Scale: How many decisions/day? Year-1 goal? Year-2 goal?
- [ ] Security: What data is sensitive? Who should see it?
- [ ] Compliance: Are there legal/audit requirements? (e.g., immutable audit trail, signed decisions)
- [ ] Availability: Can the system degrade, or must it always work?
- [ ] Operational: Who runs this? What's their expertise? (On-prem? Containerized?)

**Verify Each:**
```
Performance: [latency target + measurement method]
Scale: [transactions/day, year-1/2 goals]
Security: [data classification + access control needs]
Compliance: [regulations or internal requirements]
Availability: [SLA + degradation strategy]
Operations: [who runs it, how is it deployed]
```

### Risks & Assumptions
- [ ] List 5-10 key assumptions (e.g., "Neo4j will always be available," "Humans will approve decisions within 1 hour")
- [ ] For each assumption, ask: "What if this breaks?"
- [ ] Identify 3-5 major risks (e.g., "Bridge LLM analysis is unreliable," "Ledger corruption is unrecoverable")
- [ ] For each risk, note impact (critical/high/medium)

**Verify Each:**
```
Assumption: [specific claim]
If it breaks: [consequence]
Mitigation: [how we detect or work around it]

Risk: [specific failure mode]
Impact: [CRITICAL/HIGH/MEDIUM]
Detection: [how we know when it happens]
Response: [what we do about it]
```

### Specification Document
- [ ] Spec file exists: `TICKET-XXX-PHASE-0-SPECIFICATION.md`
- [ ] All sections above are written out (not just checkboxes)
- [ ] Spec is 2-5 pages (long enough to be clear, short enough to read)
- [ ] Spec includes one diagram (data flow, system boundary, or use case)

---

## Architect Checklist (Phase 0 Review)

**Goal:** Ensure spec is buildable and has thought through the hard parts.

### Specification Completeness
- [ ] Read entire spec start-to-finish (takes 20 mins, don't skip)
- [ ] Spec answers: "What are we building?" and "Why?"
- [ ] Spec identifies at least one integration point (how does this connect to existing systems?)
- [ ] Spec explicitly says what's OUT OF SCOPE (what are we NOT doing?)

**Questions to Ask if Unclear:**
```
"What problem are we solving?" → Spec should answer clearly
"Who benefits?" → Spec should name the user/role
"How do we know we succeeded?" → Spec should have metrics
"What breaks the system?" → Spec should name failure modes
"How does this integrate with [other system]?" → Spec should diagram it
```

### Buildability Assessment
- [ ] I can sketch the high-level architecture (modules/services/data flow) in 30 mins
- [ ] I can identify which team/person will build each part
- [ ] I can estimate Phase 1 implementation time (rough order of magnitude)
- [ ] I don't see blocking unknowns (things we'd need to decide during Phase 1)

**Red Flags:**
```
❌ "We'll figure out the architecture during implementation"
❌ "We don't know how this will integrate yet"
❌ "The team has never built this before" (without resources)
❌ "We might need to change the database/framework mid-way"
```

### Risk & Assumption Review
- [ ] I've read and agree with the risk assessment
- [ ] For each risk marked CRITICAL, I understand the mitigation
- [ ] For each assumption, I've thought: "What if it's wrong?" and see a plan
- [ ] I've noted which risks should be surfaced to leadership (business risk, not just tech risk)

**Questions to Ask:**
```
"What breaks the system?" → Risk list should address
"How do we work around [service] being down?" → Should say "graceful degradation" or "circuit breaker"
"What if [assumption] is wrong?" → Should have a plan or note to revisit in Phase 1
"Is leadership aware of the risk?" → Should know before we start
```

### Phase 1 Planning
- [ ] Spec identifies the critical path (what must we build first?)
- [ ] Spec identifies dependencies (which parts must be done together?)
- [ ] Spec identifies unknowns (what do we need to learn/verify in Phase 1?)
- [ ] I can write a rough Phase 1 implementation plan (3-5 bullet points)

**Example Phase 1 Plan (from spec):**
```
1. Set up ledger storage + persistence (JSONL, Ed25519 signing)
2. Build API handlers (record decision, fetch decision, escalate)
3. Integrate with Neo4j (index axioms, query by axiom)
4. Add correlation ID tracking through request flow
5. End-to-end test: record decision → query by axiom → read from ledger
```

---

## Joint Sign-Off (Discovery Lead + Architect)

Before Phase 0 is complete:

### Discovery Lead
- [ ] I have written and reviewed the specification
- [ ] The spec is complete and I'm confident we can build this
- [ ] Risks are identified and leadership is aware
- [ ] I'm ready for Phase 1 to start

**Signature:** _____________________ **Date:** _________

### Architect
- [ ] I have read the spec and believe it's buildable
- [ ] I've sketched the high-level architecture
- [ ] I've identified integration points and dependencies
- [ ] I've thought through major risks and see mitigation paths
- [ ] I'm confident the team can execute Phase 1-4 as scoped

**Signature:** _____________________ **Date:** _________

---

## Common Phase 0 Failures

**❌ "We're not clear on the data model yet"**  
→ Phase 0 should define data model (at least high-level)

**❌ "We don't know which database to use"**  
→ Phase 0 should make that choice (or call it out as Phase 1 risk)

**❌ "Performance requirements are vague"**  
→ Phase 0 should say "sub-100ms for 99th percentile" or "best effort, no SLA"

**❌ "Team has built something similar but with different tech"**  
→ Phase 0 should note this explicitly and plan for Phase 1 learning

**❌ "Spec doesn't mention failure modes"**  
→ Red flag. Go back and think harder about what could go wrong.

---

## Success Criteria

Phase 0 is DONE when:

1. ✓ Specification document is 2-5 pages and well-written
2. ✓ Requirements are clear (use cases, constraints, risks)
3. ✓ High-level architecture is sketched
4. ✓ Discovery Lead signed off
5. ✓ Architect signed off
6. ✓ Zero blockers for Phase 1
7. ✓ Ticket is marked "Phase 0 Complete"

---

**Phase 0 typically takes 2-4 hours.**  
If it's taking longer, you might be over-designing. Focus on clarity over completeness.
