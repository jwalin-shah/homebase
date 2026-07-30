# Code generation policy

## Decision

Use typed source contracts and deterministic generators for every mechanical
artifact. Do not ask an LLM to repeatedly hand-write code that can be derived
from a reviewed contract.

## Source-of-truth lanes

| Concern | Canonical source | Generated/derived output | Acceptance gate |
|---|---|---|---|
| Shared record shape | `running-machine-contracts` schema | Go/TypeScript/Python types and validators | schema conformance + generated diff is empty |
| Wire/API protocol | versioned IDL (Protobuf or equivalent after an explicit decision) | clients, server bindings, serialization | compatibility test + generated diff is empty |
| HomeBase reducer | small Dafny model | verified target-language core | Dafny proof + generated artifact build + refinement fixtures |
| Code relationships | compiler-backed SCIP index | commit-keyed `.scip` index | indexer succeeds and symbol queries resolve |
| Context bundle | typed provenance records | bounded agent context view | source/hash/freshness/authority checks |
| Query projection | Knowledge Engine-owned projection schema | rebuildable index/graph | replay from authoritative records matches projection |

## What is generated and when

Generation runs:

1. whenever a source contract changes;
2. on every clean CI build;
3. before a release or runtime activation;
4. during the controlled task harness before execution.

The generator version, source hash, input contract hash, output hash, and
command are recorded as evidence. CI fails if regeneration produces a diff.
Generated files are marked and protected from manual edits. A generator bug is
handled as a toolchain defect with its own fixture and review; it is not fixed
by hand-editing the output.

## What remains handwritten

Only the parts that cannot be safely derived: the contract itself, proof
lemmas, policy decisions, bounded orchestration, adapter code, performance
choices, and failure handling. Those changes require a rationale, proof
obligations, tests, and adversarial review.

## Current implementation status

The running-machine slice currently has handwritten Go mirrors for the shared
record envelope and transcript-promotion case. They are not being called
generated: the contracts repository is the source of truth, and Python
conformance plus Go fixture-parity tests are the current guard against drift.
The next safe generator is a pinned schema-to-types tool selected through the
tool inventory, followed by a checked-in generated diff and a mutation suite.
Until that exists, changing the schema requires changing the Go mirror and its
tests in the same reviewed change.

## SCIP boundary

SCIP is a code-intelligence index. It is generated from the language-aware
indexer after the code compiles and is keyed to an exact commit. Agents may use
it to answer “which definitions, references, implementations, and tests are
connected to this symbol?” They may not use it as evidence that a behavior is
correct, authorized, current, or safe. The agent context must include the
SCIP query result plus the source files, contracts, proof results, and runtime
evidence that justify the proposed change.

## Non-negotiable anti-drift checks

- A generated artifact cannot be accepted when its source contract hash is
  missing or different from the checked-in contract.
- A proof artifact cannot certify code from a different source revision.
- A SCIP index cannot be used when its commit digest differs from the task
  base commit.
- A stale or partial index is a diagnostic limitation, never positive evidence.
- A generated client cannot mint authority; only the owning service can admit
  the corresponding record or effect.
