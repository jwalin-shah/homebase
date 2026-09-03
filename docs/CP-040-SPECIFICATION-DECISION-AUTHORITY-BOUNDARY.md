# CP-040: Specification + Decision authority-submission boundary

**Status:** Implemented and tested in this worktree. Not verified against a
live HomeBase authority record — no real Specification/Decision has been
submitted through this route, and the live authority endpoint was never
called while building it.
**Scope:** `POST /api/v1/specifications/decisions` only.
**Evidence date:** 2026-09-01

## Problem

`AppendExternal` (`internal/records/store.go`) intentionally rejects
`human_decision` records — Decisions, Contracts, and approved Specifications
must arrive through an authenticated owner-specific path, not the generic
external ingress. The existing owner-authenticated atomic commit,
`AppendContractAndGrant` (`internal/records/contract_grant.go`), already
covers Specification + Contract + CapabilityGrant, but it requires the
approving Decision to already exist in the store (its tests seed it with a
direct, non-HTTP `Store.Append` call). Nothing in the deployed HTTP surface,
and no Bridge client method or CLI flag, could create that Decision — or the
Specification it approves — in the first place.

The only prior tool that could seed this pair was `cmd/drive-admit
seed-decision`: a local CLI that opens the production journal file directly
and writes a hard-coded sandbox specification payload
(`portfolio one-surface sandbox drive atom A`), bypassing HTTP authentication
entirely. That tool is a fixture for a specific sandbox exercise, not a
production authority path, and this change does not extend it.

Bridge task-contract admission (`HandleCheckContractGrant`,
`internal/records/store.go` Contract/CapabilityGrant validation) requires the
exact Specification ID and content digest to be bound throughout the chain.
Without an owner-authenticated way to create the approving Decision and its
Specification, there was no production-safe way to establish that root of
the chain.

## Decision

Add the smallest owner-authenticated, atomic, idempotent path for exactly
this pair — nothing else:

- `internal/records/specification_decision.go`:
  `Store.AppendSpecificationAndDecision(specificationRaw, decisionRaw []byte)`.
  Mirrors `AppendContractAndGrant`'s shape: a dedicated journal record kind
  (`SpecificationDecisionCommit`), a `prepare`/`apply`/`replay` split, and a
  `specificationDecisions` index keyed by Specification ID for
  idempotent-duplicate and conflict detection.
- `api/handlers.go`: `Server.HandleAppendSpecificationDecision`, mounted at
  `POST /api/v1/specifications/decisions`
  (`cmd/homebase/main.go`). Verifies `X-HomeBase-Specification-Signature`
  against the same `contractKey` already wired for
  `/api/v1/contracts/grants` — no new key is introduced.
- `bridge/pkg/homebaseclient` (Bridge worktree
  `cp041-bridge-project-gates`): `Client.SubmitSpecificationDecision`.
- `bridge/cmd/homebase-submit`: `-specification`/`-decision` flags submit
  only this pair; combining `-decision` with `-contract`/`-grant` is
  rejected before any network call.

### What is enforced

- `specification.status` must be `"approved"`. A `"proposed"` Specification
  is rejected — those remain reachable only through `AppendExternal` as an
  `agent_proposal`, unchanged.
- `decision.status` must be `"approved"` with `source.role == "captain"`.
  (Generic `Record.validate()` already requires `authority_class ==
  human_decision` for any `Decision`; this endpoint additionally requires
  captain specifically, not portfolio/homebase.)
- `decision.payload.specification_ref` must be present and must equal
  `{"kind":"specification","id":<specification.id>,"content_hash":<specification.content_hash>}`
  exactly — the same exact-ID/exact-digest binding Contract/CapabilityGrant
  validation already requires elsewhere in the chain.
- The existing `validateSpecification`/`validateReferences` cross-checks
  still run (via a cloned candidate store, decision inserted before
  specification) so the full approval-chain shape validation is not
  duplicated or weakened.
- Both records commit in one journal entry: they durably exist together or
  neither does. A crash between validating and appending leaves the journal
  unchanged (`journal.Append` failure poisons the store rather than partially
  applying).
- Resubmitting byte-identical `specification`/`decision` is idempotent:
  `200 OK` with `"existing": true` and the original `sequence`, no new
  journal entry.
- Resubmitting the same Specification ID with different Decision bytes (or
  vice versa) is `409 Conflict` and leaves the journal/store unchanged.
- `AppendExternal` is untouched and still rejects `Specification`/`Decision`
  records from any caller other than this endpoint.

### What is explicitly unchanged

- `AppendContractAndGrant`, `AppendBridgeVerificationSubmission`, and their
  routes (`/api/v1/contracts/grants`, `/api/v1/contracts/grants/check`,
  `/api/v1/verifications/bridge`, `/api/v1/verifications/receipts/read`) are
  not modified.
- `bridge/pkg/homebaseclient.Client.SubmitContractGrant`,
  `CheckContractGrant(WithLease)`, and `Submit` (verifier receipts) are
  unmodified; `homebase-submit`'s existing contract/grant, receipt, and check
  modes behave identically to before this change.
- `cmd/drive-admit` is untouched.
- No new signing key, environment variable, or key file is introduced; the
  endpoint is unavailable (`503`) exactly when `/api/v1/contracts/grants`
  would also be unavailable (no `contractKey` configured).

## Request shape

```
POST /api/v1/specifications/decisions
Content-Type: application/json
X-HomeBase-Specification-Signature: <hex ed25519 signature over canonical JSON of the body>

{
  "specification": { "kind": "Specification", "version": "1", "status": "approved", "...": "..." },
  "decision": { "kind": "Decision", "version": "1", "status": "approved", "source": {"id": "captain", "role": "captain"}, "...": "..." }
}
```

Response (`201` on first commit, `200` on idempotent duplicate):

```json
{"specification_id": "...", "decision_id": "...", "existing": false, "sequence": 1}
```

Errors: `401` bad/missing signature; `422` schema or authority violation
(wrong status, missing/mismatched `specification_ref`, non-captain source);
`409` conflicting resubmission; `503` endpoint not configured.

## Proof

- `internal/records/specification_decision_test.go`: atomic commit +
  journal replay after reopen, idempotent duplicate resubmission,
  conflicting-replay rejection, missing `specification_ref`, mismatched
  digest, proposed (non-approved) Specification, and non-captain Decision —
  each rejection case asserts the store's record count is unchanged.
- `api/handlers_test.go`
  (`TestHandleAppendSpecificationDecisionAuthenticatesAndCommitsAtomically`,
  `TestHandleAppendSpecificationDecisionRejectsWrongStatusAndMissingReference`):
  accepted, duplicate-identical, conflict, bad signature, missing signature,
  wrong-authority signer, wrong status, and missing-reference paths over
  HTTP — each rejection case asserts the record store is unchanged.
- `bridge/pkg/homebaseclient/client_test.go`
  (`TestSubmitSpecificationDecisionSignsCanonicalEnvelope`,
  `TestSubmitSpecificationDecisionRejectsInvalidJSON`) and
  `bridge/cmd/homebase-submit/main_test.go`
  (`TestResolveSubmitMode*`): client signs the canonical envelope with the
  correct header and never sends a `contract`/`grant` field in this mode;
  the CLI's flag-mode resolution accepts the Specification+Decision pair,
  the full Contract/Grant bundle, the receipt, and the check modes, and
  rejects every invalid combination (bare `-decision`, bare `-specification`,
  `-decision` combined with `-contract`/`-grant`, mixed modes, no flags at
  all) before any network call — proving no private key material path was
  altered for the existing modes.
- Full-suite checks in this worktree: `go build ./...`, `go vet ./...`,
  `gofmt -l .` (empty), `git diff --check`, `go test -race ./...`,
  `scripts/prove-docs-freshness.sh`.
- Bridge worktree `cp041-bridge-project-gates`: `go build ./...`,
  `go vet ./pkg/homebaseclient/... ./cmd/homebase-submit/...`,
  `gofmt -l` (empty), `go test -race ./pkg/homebaseclient/...
  ./cmd/homebase-submit/...`.

## Live wiring status

The following remain intentionally absent from this change:

- no real CP-040 Specification/Decision was created or submitted;
- no call was made to any live HomeBase instance or its authority endpoint;
- no Google Drive, Portfolio primary, Bridge primary, or credential change;
- no other Bridge worktree, and no other file in `cp041-bridge-project-gates`
  beyond the client/CLI/test files listed above, was modified;
- no commit or push was made from either worktree.

This is proof of an implemented, tested, owner-authenticated boundary — not
proof of a live-issued Specification/Decision authority record.
