# HomeBase: Contract/Grant Admission and Durable Signed Receipt Authority

**Status:** Implemented — local HTTP service on 127.0.0.1:9102

---

## What it does

HomeBase is the **authority ledger** of the machine. It records the irreversible
decisions that authorize work and the signed receipts that prove it happened.
It is *not* an agent runner, supervisor, or terminal — it is the durable,
cryptographically-signed record of authority.

Implemented components:

- **Append-only JSONL ledger** (`homebase_ledger.jsonl`) — every record is
  appended and fsynced; existing records are never mutated.
- **Binary typed-record journal** (`HOMEBASE_RECORD_JOURNAL`) — typed records
  (Contract, CapabilityGrant, Claim, Proof, VerificationReceipt, decision)
  are replayed into an in-memory store on startup.
- **Neo4j validation (Axiom Firewall)** — optional; when `NEO4J_URI` is set,
  records are validated against the Neo4j axiom corpus. Local-only builds run
  without it (the validator degrades gracefully when the cache client is nil).
- **Contract/Grant endpoints** — owner-signed admission of a captain-approved
  Specification + Contract + scoped CapabilityGrant as one journal commit.
- **Specification/Decision endpoint** — owner-signed admission of a
  captain-approved Specification together with its approving Decision, as one
  atomic, idempotent journal commit. This is the smallest authority path that
  can create the pair Bridge's task-contract admission requires by exact ID
  and content-hash reference; it does not create, modify, or reference any
  Contract or CapabilityGrant. **Status: implemented, not yet exercised
  against a live authority record** — this worktree adds the endpoint,
  store operation, and tests only; no real Specification/Decision has been
  submitted through it in production.
- **Bridge verification receipt endpoints** — Bridge-signed transport receipts
  that HomeBase derives into Claim/Proof/VerificationReceipt records and
  commits atomically against the pre-existing Contract and CapabilityGrant.
- **Configured authority keys** — captain, Bridge transport, admission
  response, verifier, and receipt keys loaded from `~/.local/state/homebase/keys/`.

## Endpoints (mounted routes)

All routes bind to `127.0.0.1` only. Every route is `POST` except the
`GET`-only `/v1/status` health-check route. Verified in `cmd/homebase/main.go`:

| Route | Handler | Purpose |
|---|---|---|
| `/api/v1/records` | `HandleAppendExternalRecord` | Public ingress for Trajectory and other untrusted producers; validates the envelope, verifies the payload hash, fsyncs before returning success |
| `/api/v1/promotions/transcript` | `HandlePromoteTranscript` | Admission of a transcript-derived decision through the authenticated promotion service; evidence, decision, and signed receipt committed together |
| `/api/v1/contracts/grants` | `HandleAppendContractGrant` | Owner-signed admission of Specification + Contract + scoped CapabilityGrant as one journal commit |
| `/api/v1/contracts/grants/check` | `HandleCheckContractGrant` | Read-only Bridge admission check: proves an approved Contract + active CapabilityGrant exist with exact scope match, freshness, and expiry; Bridge cannot mint or extend authority |
| `/api/v1/specifications/decisions` | `HandleAppendSpecificationDecision` | Owner-signed admission of a captain-approved Specification + its approving Decision as one atomic, idempotent journal commit; no Contract or CapabilityGrant involved. Reuses the same captain/contract signing key as `/api/v1/contracts/grants` |
| `/api/v1/verifications/bridge` | `HandleAppendBridgeVerification` | Accepts a Bridge-signed transport receipt; derives Claim/Proof/VerificationReceipt records, checks them against the pre-existing Contract and CapabilityGrant, commits atomically |
| `/api/v1/verifications/receipts/read` | `HandleReadVerificationReceipt` | Read-only Bridge-signed lookup of a durable VerificationReceipt by exact `receipt_id`; returns canonical stored bytes without mutation or personal-data expansion |
| `/v1/status` | `HandleStatus` | `GET`-only, unauthenticated LaunchAgent health-check route (CP-041). Reports only `status`, `service`, `ledger_ready`, `record_store_ready` — no keys, records, journal contents, or paths. Non-`GET` methods return 405 |

The legacy decision endpoint is intentionally **not** mounted — it accepted
caller-controlled decisions signed with a process-local key, which is not an
authenticated authority boundary.

### Specification/Decision authority pair (`/api/v1/specifications/decisions`)

**Status: implemented and tested in this worktree; not verified against a
live HomeBase authority record.** No real Specification/Decision has been
submitted through this route; the endpoint, store operation, and client/CLI
support exist but have only been exercised by the test suite below.

Request body (`Content-Type: application/json`, header
`X-HomeBase-Specification-Signature: <hex ed25519 signature>` over the
canonical JSON of the exact request body, signed with the captain/contract
private key — the same key that signs `/api/v1/contracts/grants`):

```json
{
  "specification": { "kind": "Specification", "status": "approved", "...": "..." },
  "decision": { "kind": "Decision", "status": "approved", "source": {"role": "captain"}, "...": "..." }
}
```

Requirements enforced by `internal/records.Store.AppendSpecificationAndDecision`
(`internal/records/specification_decision.go`):

- `specification.status` must be `approved` (a `proposed` Specification is
  rejected; those remain reachable only through `AppendExternal` as an
  `agent_proposal`).
- `decision.status` must be `approved` with `source.role == "captain"`.
- `decision.payload.specification_ref` must be `{"kind":"specification","id","content_hash"}`
  and must exactly match the Specification's `id` and `content_hash` — the
  same exact-ID/exact-digest binding Bridge's task-contract admission checks
  against Contract/CapabilityGrant.
- The pair commits in one journal entry (`SpecificationDecisionCommit`): both
  records durably exist or neither does.
- Resubmitting byte-identical `specification`/`decision` is idempotent
  (`200` with `"existing": true`, unchanged sequence). Resubmitting the same
  Specification ID with different Decision bytes is a `409 Conflict` and
  leaves the journal/store unchanged.
- `AppendExternal` continues to reject `Specification`/`Decision` records
  outright (`403`); this endpoint is the only path that can create either.

Response body: `{"specification_id", "decision_id", "existing", "sequence"}`.
Errors: `401` (bad/missing signature), `422` (schema/authority violation,
e.g. wrong status or missing/mismatched `specification_ref`), `409`
(conflicting resubmission), `503` (endpoint not configured — no
`contractKey`).

Tests: `internal/records/specification_decision_test.go` (atomicity,
idempotent replay, conflict, missing reference, mismatched digest, proposed
specification, non-captain decision — each proving the rejected case leaves
the store unchanged) and `api/handlers_test.go`
(`TestHandleAppendSpecificationDecision*`, covering accepted, duplicate,
conflict, bad signature, missing signature, wrong-authority signer, wrong
status, and missing reference over HTTP).

## Key ownership

Keys live in `~/.local/state/homebase/keys/` (mode `0600`; the server refuses
group/world-readable key files):

| File | Role |
|---|---|
| `captain.pub` | Owner (captain) public key — authenticates contract/transcript promotion |
| `bridge.pub` | Bridge transport public key — authenticates Bridge admission checks and verification receipts |
| `admission.priv` | HomeBase admission response signing key — signs the response to Bridge's read-only authority check |
| `verifier.pub` | Verifier public key (with `HOMEBASE_VERIFIER_KEY_ID`) — authenticates verifier-owned receipts |
| `receipt.priv` | Receipt signing key — signs promotion receipts |

Environment variables (`HOMEBASE_CAPTAIN_PUBLIC_KEY_FILE`,
`HOMEBASE_BRIDGE_PUBLIC_KEY_FILE`, `HOMEBASE_ADMISSION_PRIVATE_KEY_FILE`,
`HOMEBASE_VERIFIER_PUBLIC_KEY_FILE`, `HOMEBASE_VERIFIER_KEY_ID`,
`HOMEBASE_RECEIPT_PRIVATE_KEY_FILE`) point at these files. Keys are provisioned
once (see `dotfiles/bin/provision-authority-keys.sh`) and persisted; they are
never generated per-launch.

## How Bridge integrates

Bridge is the caller of the authority chain:

- `BRIDGE_HOMEBASE_URL=http://127.0.0.1:9102` — HomeBase's HTTP address.
- Bridge signs contract/grant checks with **its private key**
  (`~/.local/state/homebase/keys/bridge.priv`); HomeBase verifies with
  `bridge.pub`.
- HomeBase signs admission responses with `admission.priv`; Bridge verifies
  with `admission.pub`.
- Bridge submits verification receipts to `/api/v1/verifications/bridge`;
  HomeBase commits them against the pre-existing Contract and CapabilityGrant.
- Bridge can read back a committed verification receipt with a signed POST to
  `/api/v1/verifications/receipts/read` (`X-Bridge-Verification-Read-Signature`).
  Admission check responses are signed at check time and are not durably stored;
  only journal-backed VerificationReceipt records are readable through this seam.
  CP-030 records the fail-closed boundary and the schema/authority decision
  required before admission-check read-back can be added.
- The captain/authority operator can submit a Specification + approving
  Decision pair via `bridge/pkg/homebaseclient.Client.SubmitSpecificationDecision`
  or `homebase-submit -specification <path> -decision <path> -url ... -key-file ...`.
  This uses the same `X-HomeBase-Specification-Signature`-over-canonical-JSON
  scheme as the other owner-authenticated routes; the private key is read
  from `-key-file` and never appears in argv, logs, or the request body.

The flow: captain approves a Contract → HomeBase records it →
Bridge checks the grant before creating a worktree → Bridge verifies the
worker result → HomeBase commits the verification receipt → authorized
delivery.

## Research (not implemented)

The following is **design/speculation — not the live behavior of this server**.
It is preserved for reference only.

- **`SYSTEM-DESIGN.md`** — the formal graph-structured system design
  (graph states, 5-state transitions, 6 provable invariants I1–I6, TLA-style
  model checking). Status: **design, not implemented**.
- **`AGENTS.md` / `CLAUDE.md` / `tickets/PHASE-ENFORCEMENT-FRAMEWORK.md`** —
  the implementation blueprint and phase-gate process documents.
- **Dafny/formal-assurance work** under `verification/` — assurance-case
  scaffolding, not part of the running service.
- **Older planning docs** (`PHASE-2-PLAN.md`, `SYSTEM-AUDIT.md`,
  `TICKET-202-*.md` at the repo root) — superseded, slated for deletion.

The live service is the HTTP API described above. Any gap between the design
documents and the running code: **the code is the truth**.

## LaunchAgent health contract (CP-041)

The live LaunchAgent plist (`org.nixos.org.nixos.com.jwalinshah.homebase.plist`)
sets `DAEMON_HEALTH_URL=http://127.0.0.1:9102/v1/status`. Before CP-041, that
route was unmounted and returned HTTP 404 even while the process was healthy
and serving `/api/v1/*` routes. `HandleStatus` closes that gap with the
smallest possible surface: `GET /v1/status` returns HTTP 200 and a JSON body
containing only `status`, `service`, `ledger_ready`, and `record_store_ready`.
It is unauthenticated because the daemon-wrapper health check has no signing
capability and the response carries no secret or record data; every other
route remains `POST`-only and, where applicable, signature-authenticated.
Non-`GET` requests to `/v1/status` return 405.

## Verification gates (receipt read-back + health status)

| Command | Proof class |
|---|---|
| `make prove-receipt-readback` | Handler-level receipt read-back seam |
| `go test ./api -run TestHandleStatus` | Handler-level health-status seam (CP-041) |
| `go test ./cmd/homebase -run TestCompiledStatusRoute` | Compiled-runtime health-status seam (CP-041) |
| `make prove-receipt-readback-deployment` | CP-039 + CP-041 staged artifact + live dry-run deployment readiness |
| `bash scripts/prove-docs-freshness.sh` | Doc/linkage freshness for this worktree |

`prove-receipt-readback-deployment` distinguishes **staged artifact proof** (hermetic
build + tests in the worktree) from **live deployment proof** (read-only plist/binary/key
inspection of `~/.local/bin/homebase` on port 9102, plus a live `GET /v1/status` and
`POST /api/v1/verifications/receipts/read` HTTP check). It fail-closes when the installed
binary hash or either route marker does not match the staged candidate, or when the live
HTTP seam for either route is unmounted (404).
