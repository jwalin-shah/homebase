# HomeBase Architecture

## The Problem
Autonomous agents (like Orbit and Bridge) cannot be trusted with infinite loops, raw bash access, or vector-similarity hallucination. In past audits, agents silently bypassed errors, leaked goroutines, hallucinated rules, and tampered with logs.

## The Solution: A Neurosymbolic AI Factory
HomeBase is the **Symbolic Guardrail** around the Probabilistic LLM. 
It strips execution power from the agents and forces every decision through a mathematically bounded, cryptographically secured, and graph-verified state machine.

### 1. The Immutable Graph Engine (Bounded Execution)
Instead of an infinite `while(true)` loop, HomeBase uses a strict 5-move State Machine:
`PLAN → EXECUTE → RECOVER → ESCALATE → COMPLETE`

*   **Bounded Recovery (Invariant 2):** The engine is hardcoded to allow a maximum of 2 recovery attempts. If an execution fails twice, it mathematically halts and routes to `ESCALATE`. It cannot get stuck in an infinite loop.
*   **The Context Blast-Shield:** Every execution node is wrapped in a `context.WithTimeout`. This physically guarantees that no network call (to Neo4j or the sandbox) can hang forever. Goroutine leaks are impossible.
*   **Thread Safety:** The `ExecutionContext` is protected by a `sync.Mutex`. Parallel agents can use the engine simultaneously without data races.

### 2. The Axiom Firewall (Graph Grounding)
Vector databases fail because "similarity" is not a relationship. HomeBase uses **Neo4j** to enforce true relationships.
*   Before a plan is executed, the `PlanNode` hits the `AxiomFirewall` (`internal/cache/neo4j.go`).
*   It executes a Graph Traversal to verify that the Axioms cited by the agent physically exist and govern the code being touched.
*   If the agent hallucinates an axiom, the Firewall instantly aborts the execution.

### 3. Ed25519 Cryptography (Tamper-Proofing)
Agents are no longer allowed to execute code directly.
*   When a plan succeeds, HomeBase takes the original plan and the sandbox evidence, hashes them together, and signs it with an **Ed25519 Private Key**.
*   This proves that the exact plan approved was the exact code executed. The agent cannot silently alter the payload mid-flight.

### 4. The Physical Ledger (Durability)
The signed decision is written to `homebase_ledger.jsonl`.
*   The file descriptor is opened in `os.O_APPEND` mode. The OS kernel physically prevents overwriting past data.
*   We use `file.Sync()` (`fsync`) after every write. The data is durably written to the physical SSD before the HTTP request returns `200 OK`. Power failures cannot corrupt the ledger.

## Orbit Integration (What happens next)
Orbit (the pure reasoning agent) will no longer query markdown files or execute its own code.
1.  **Cocoindex** runs as a background daemon, parsing the ASTs of the code and pushing the exact physical relationships into **Neo4j**.
2.  **Orbit** connects to **Neo4j** directly and writes **Cypher** queries to traverse the graph (e.g., finding exactly what files a change impacts).
3.  **Orbit** drafts a `PLAN` and POSTs it to the HomeBase API.
4.  **HomeBase** verifies, executes, signs, and commits the decision.
