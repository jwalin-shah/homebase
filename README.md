# HomeBase: Contract/Grant Admission and Durable Signed Receipt Authority

**Status:** Source binds to loopback; `PORT` defaults to 8080. The deployment's canonical caller URL must be read from its service configuration before Bridge activation.

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
- **Bridge verification receipt endpoints** — Bridge-signed transport receipts
  that HomeBase derives into Claim/Proof/VerificationReceipt records and
  commits atomically against the pre-existing Contract and CapabilityGrant.
- **Epistemic promotion** — authenticated research-run provenance, source
  Evidence, status-preserving Claims, and a signed acceptance Proof committed
  atomically. Acceptance attests to provenance intake, not claim truth or
  captain approval.
- **Configured authority keys** — captain, Bridge transport, admission
  response, verifier, and receipt keys loaded from `~/.local/state/homebase/keys/`.

## Endpoints (mounted routes)

All routes are `POST` and bind to `127.0.0.1` only. Verified in
`cmd/homebase/main.go`:

| Route | Handler | Purpose |
|---|---|---|
| `/api/v1/records` | `HandleAppendExternalRecord` | Public ingress for Trajectory and other untrusted producers; validates the envelope, verifies the payload hash, fsyncs before returning success |
| `/api/v1/promotions/transcript` | `HandlePromoteTranscript` | Admission of a transcript-derived decision through the authenticated promotion service; evidence, decision, and signed receipt committed together |
| `/api/v1/promotions/evidence` | `HandlePromoteEpistemic` | Authenticated ProviderResearchRun provenance, Evidence, epistemic-status Claims, and signed acceptance Proof in a dedicated atomic journal event; no transcript or captain-approval semantics |
| `/api/v1/contracts/grants` | `HandleAppendContractGrant` | Owner-signed admission of Specification + Contract + scoped CapabilityGrant as one journal commit |
| `/api/v1/specifications/decisions` | `HandleAppendSpecificationDecision` | Owner-signed admission of a captain-approved Specification + its approving Decision as one atomic, idempotent journal commit; reuses the captain/contract signing key |
| `/api/v1/contracts/grants/check` | `HandleCheckContractGrant` | Read-only Bridge admission check: proves an approved Contract + active CapabilityGrant exist with exact scope match, freshness, and expiry; Bridge cannot mint or extend authority |
| `/api/v1/verifications/bridge` | `HandleAppendBridgeVerification` | Accepts a Bridge-signed transport receipt; derives Claim/Proof/VerificationReceipt records, checks them against the pre-existing Contract and CapabilityGrant, commits atomically |

The legacy decision endpoint is intentionally **not** mounted — it accepted
caller-controlled decisions signed with a process-local key, which is not an
authenticated authority boundary.

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

The optional epistemic route requires `HOMEBASE_EPISTEMIC_PROMOTER_ID`,
`HOMEBASE_EPISTEMIC_PROMOTER_PUBLIC_KEY_FILE`,
`HOMEBASE_EPISTEMIC_RECEIPT_KEY_ID`, and
`HOMEBASE_EPISTEMIC_RECEIPT_PRIVATE_KEY_FILE`. It remains unavailable if any
reference is absent or invalid. Requests are authenticated with the detached
Ed25519 signature in `X-HomeBase-Epistemic-Signature` over canonical JSON.
`/api/v1/records` remains untrusted-only.

## How Bridge integrates

Bridge is the caller of the authority chain:

- `BRIDGE_HOMEBASE_URL=http://127.0.0.1:9102` is a historical configuration
  example, not a verified deployment value. This source defaults `PORT` to
  `8080`, and current OCI inventory observes a HomeBase listener on
  `127.0.0.1:8080`; the deployed Bridge caller URL still requires service
  configuration readback before activation.
- Bridge signs contract/grant checks with **its private key**
  (`~/.local/state/homebase/keys/bridge.priv`); HomeBase verifies with
  `bridge.pub`.
- HomeBase signs admission responses with `admission.priv`; Bridge verifies
  with `admission.pub`.
- Bridge submits verification receipts to `/api/v1/verifications/bridge`;
  HomeBase commits them against the pre-existing Contract and CapabilityGrant.

The flow: captain approves a Contract → HomeBase records it →
Bridge checks the grant before creating a worktree → Bridge verifies the
worker result → HomeBase commits the verification receipt → authorized
delivery.

## Research evidence promotion

`POST /api/v1/promotions/evidence` is a separate authenticated intake boundary.
It validates provider-run and source hashes, HTTPS source provenance, source
lineage, freshness, claim status, and Evidence references. The append-only
journal stores the complete bundle as one `EpistemicAcceptanceCommit` event.
Evidence and Claims retain `untrusted_text` authority; the signed Proof
attests that the submitted provenance bundle passed intake checks. Its receipt
sets `claims_asserted_true=false` and does not imply a captain-approved
Decision. Conflicting epistemic statuses remain explicit in the receipt.

This candidate implements local source behavior. It has no live key provisioning,
deployment, or HomeBase acceptance until a separately authorized runtime rollout.

The following remaining material is design/reference documentation, not live
behavior of this server:

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
