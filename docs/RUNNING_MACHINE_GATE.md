# Running Machine gate

This file is the admission checklist for the running-machine integration. A
green compiler or test suite is evidence for one row only; it is not evidence
that the architecture is safe or complete.

## Required order

1. **Contract** — name the owner, record shape, authority class, allowed side
   effects, failure semantics, and compatibility rules.
2. **Proof obligations** — write the properties independently of the Go
   implementation. Each property must have a model, a falsifying mutation,
   and a refinement/conformance check where it is claimed to govern code.
3. **Adversarial review** — a fresh-context reviewer checks the contract,
   proof, code, and claims. Findings are blockers until fixed or explicitly
   accepted by the captain.
4. **Implementation** — write the smallest code that satisfies the admitted
   contract. Every non-trivial change gets a rationale and a named invariant.
5. **Break tests** — exercise malformed input, replay, races, torn writes,
   process restart, dependency loss, authority spoofing, and wrong-tree/wrong-
   commit execution.
6. **Certification** — run unit, integration, formal, end-to-end, and live
   operational checks separately. Record exact commands, commits, timestamps,
   and unknowns. No lower-level pass promotes a higher-level claim.

## Checklist

### Design and authority

- [ ] One owner is named for each decision, execution, provenance, graph
  projection, session/worktree, and user-facing routing concern.
- [ ] Every record has an explicit kind, version, source, source references,
  freshness, status, authority class, and content hash.
- [ ] Evidence, proposals, worker observations, decisions, contracts, grants,
  proofs, verification receipts, and challenges are distinct types.
- [ ] No transcript, agent output, test result, report, or graph projection is
  promoted to authority implicitly.
- [ ] Consequential authority requires an authenticated owner path; public
  ingestion cannot spoof a captain decision, grant, proof, or receipt.
- [ ] Duplicate IDs are idempotent only for byte-equivalent canonical records;
  conflicting reuse fails closed.

### Formal and executable proof

- [ ] The Dafny/Lean/Coq/TLA+ model states the exact property being claimed,
  including preconditions and adversarial transitions.
- [ ] The model has no `assume`, `axiom`, `sorry`, vacuous theorem, ignored
  result, or proof of a stronger-looking but different property.
- [ ] The verifier command is recorded and exits successfully from the pinned
  source revision.
- [ ] Generated/runtime code is tied to the model by a checked refinement or
  conformance fixture; a separate model pass is labeled model-only.
- [ ] A mutation suite proves that removing each important guard fails a
  check. Tests written after the implementation are not treated as proof by
  themselves.

### Persistence and recovery

- [ ] Every durable write is fsynced before acknowledgement.
- [ ] Partial header, payload, and checksum tails recover only to the last
  valid boundary; interior corruption fails closed.
- [ ] Replay uses explicit record envelopes and rejects unknown versions/kinds.
- [ ] Shared records cannot enter the attempt reducer as executable events.
- [ ] Reopen, duplicate append, concurrent append, and crash-restart tests pass.
- [ ] A trusted external checkpoint or equivalent operational recovery plan is
  defined; a local hash chain alone does not prove history was not rewritten.

### Agent task execution

- [ ] Orbit is a routing/client surface, not an authority or hidden executor.
- [ ] HomeBase admits a typed contract and capability grant before Bridge can
  execute.
- [ ] Bridge enforces base commit, allowed/forbidden paths, sandbox, command,
  idempotency, and publication gates.
- [ ] Worker output is observation only until an independent verifier creates a
  verification receipt.
- [ ] Completion requires terminal outcome, tree digest, verifier evidence, and
  no violations; degraded or missing evidence is review/failure.
- [ ] Task context is provenance-labeled and bounded; stale/unsupported
  context is visible rather than silently used.

### Knowledge and context

- [ ] Trajectory captures raw interactions without authority promotion.
- [ ] HomeBase is the durable decision/approval owner.
- [ ] Bridge owns operational execution and verification evidence.
- [ ] Knowledge Engine is a rebuildable projection/retrieval layer, never the
  decision authority.
- [ ] Code graph and semantic index capabilities are inventoried from their
  native interfaces before custom indexing is added.
- [ ] Every context item has source, timestamp, hash, freshness, authority,
  and inclusion reason visible to the agent and reviewer.

### Performance and operability

- [ ] Benchmarks measure the real hot path and state hardware/configuration.
- [ ] Bounds exist for record size, context size, retries, timeouts, queues,
  and subprocess lifetime.
- [ ] Logs contain IDs and evidence references but never secrets or unbounded
  transcript text.
- [ ] Restart, dependency outage, stuck worker, duplicate delivery, and
  operator cancellation have explicit outcomes.
- [ ] A one-command controlled harness can run a task in a disposable
  worktree, capture evidence, inject failures, and leave the original repos
  untouched.

## Current HomeBase slice

The current branch implements the first executable persistence boundary and a
runtime transcript-promotion slice:

- `internal/records` validates the shared v1 envelope, canonical payload hash,
  authority class, kind-specific fields, duplicate identity, and durable replay.
- `internal/journal` gives that store an explicit `SharedRecord` transport kind.
- `api/handlers.go` exposes external evidence/observation/proposal ingress and
  rejects authoritative kinds at that public boundary. It also exposes the
  authenticated transcript-promotion endpoint when persistent captain and
  receipt keys are configured.
- `internal/promotion` matches the transcript-promotion-v1 shape, verifies the
  detached Ed25519 transcript signature, enforces source spans/approval
  freshness/nonce binding, signs the receipt, and uses one `PromotionCommit`
  journal entry for evidence plus decision plus receipt.
- `cmd/homebase/main.go` opens the typed journal and mounts both endpoints;
  promotion remains explicitly unavailable when its persistent keys are absent.
- Tests cover durable reopen, idempotent replay, conflicting IDs, malformed and
  forged records, authority spoofing, and reducer separation.

The promotion slice has executable fixture parity, durable restart/idempotency
tests, invalid-authentication tests, and a model-only Dafny state proof. It is
still not an end-to-end Orbit/Trajectory/Knowledge-Engine/Bridge certification.
The Go implementation has not yet been refined from generated Dafny code, and
the legacy decision signer in `cmd/homebase` remains ephemeral; those are
separate production blockers.

This slice is **not yet proven end-to-end**. It does not establish
HomeBase-to-Knowledge-Engine projection, real Orbit/Bridge task execution,
generated-code refinement, or crash testing against a killed process. Those
remain explicit gates above.
