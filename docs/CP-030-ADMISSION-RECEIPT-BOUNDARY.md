# CP-030: Admission-check receipt boundary

**Status:** BLOCKED — fail closed pending schema and authority decision  
**Scope:** successful `POST /api/v1/contracts/grants/check` responses only  
**Evidence date:** 2026-09-01  

## Decision

Do not add an admission-check read-back endpoint or persist a successful check
as an existing HomeBase record. The current response is an authenticated,
read-only lease check, but it is not a durable HomeBase record: the response
body and response signature are produced at request time and are not written to
the typed journal.

Do not encode the response as `VerificationReceipt`. That record kind means
verifier-owned completed verification evidence and requires a verifier source,
tree/check/evidence fields, and references to the worker claim and proofs. A
successful authority check does not provide those facts. Do not encode it as
`Observation` either: observations are worker-owned effect reports, not
HomeBase-issued authority. The existing journal therefore has no honest owner
or record kind for this artifact.

The correct current result is an explicit fail-closed boundary, not a
best-effort read-back seam.

```json
{
  "decision_record_type": "homebase.boundary-decision",
  "schema_version": "1",
  "id": "CP-030",
  "status": "blocked",
  "decision": "do_not_implement_admission_check_readback",
  "reason": "existing_record_model_has_no_admission_receipt_kind_or_approved_issuer",
  "transient_success_is_not": ["VerificationReceipt", "Observation"],
  "required_authority_decision": "name_the_durable_issuer_and_key_owner_for_admission_check_receipts",
  "required_schema": "homebase.admission-check-receipt.v1"
}
```

This JSON is a decision artifact, not an accepted v1 journal record. It is not
submitted through `/api/v1/records` and does not authorize work.

## Current signed response contract

`api/handlers.go:232-375` currently performs the following sequence:

| Boundary | Current behavior | Durable? |
|---|---|---|
| Request body | Strict JSON check containing `contract_id`, `grant_id`, `task_id`, `worker_id`, repository/tree/scope fields, `context_hash`, specification identity, verifier identity, and `idempotency_key` | No |
| Request authentication | `X-Bridge-Contract-Check-Signature` verifies the canonical request with the configured Bridge public key | No journal record |
| Authority lookup | HomeBase loads the exact `Contract` and `CapabilityGrant`, requires approved/active status, matching scope, and live freshness | Existing records only |
| Request binding | SHA-256 of the canonical request is emitted as `request_hash` | Hash is emitted, not stored |
| Success body | `contract_id`, `grant_id`, `task_id`, `worker_id`, `context_hash`, `request_hash`, and the minimum `valid_until` of context/grant expiry | Emitted only |
| Response authentication | `X-HomeBase-Admission-Signature` signs canonical response bytes with the HomeBase admission key | Header is emitted only |
| Wire bytes | Handler writes the marshaled response followed by a newline | Not preserved |
| Failure behavior | Unknown/mismatched/expired/unsigned requests fail before success response | No failed receipt is created |

The `Idempotency-Key` header is not used by this handler. The request's
`idempotency_key` is only a contract/grant scope field. Repeating an otherwise
valid check repeats a transient check and can produce another signed response;
there is no journal identity or conflict check for the response.

## Existing record and authority limits

The running record validator (`internal/records/store.go:921-1017`) allows the
known v1 kinds but not `AdmissionCheckReceipt`. Its authority rules establish:

- `Contract` and `Decision`: `human_decision` from captain/portfolio/HomeBase;
- `CapabilityGrant`: `authoritative_fact` from Bridge;
- `Observation`: `worker_observation` from worker/agent;
- `Proof` and `VerificationReceipt`: `verified_evidence`, with
  `VerificationReceipt` restricted to a verifier source.

The `VerificationReceipt` payload (`internal/records/store.go:1041-1052` and
`internal/records/store.go:1149-1266`) requires a completed tree verification
shape. Reusing it for an admission response would silently change the meaning
of both the authority class and receipt lifecycle. Adding a new kind without an
approved issuer would instead invent authority, so neither change is made in
this worktree.

## Bridge and router expectations

The Bridge client (`pkg/homebaseclient/client.go:175-235`) currently:

1. canonicalizes and signs the check request;
2. verifies the HomeBase response signature;
3. recomputes and checks `request_hash`;
4. checks exact `contract_id`, `grant_id`, `task_id`, `worker_id`, and
   `context_hash` bindings; and
5. checks that `valid_until` is still in the future, returning only
   `time.Time`.

It discards the response body, response signature, and request hash after
returning. The spawn-pipeline and local-executor interfaces therefore consume
only `valid_until` (`internal/spawnpipeline/config.go:49-75` and
`internal/localexecutor/production_verifier.go:220-273` in the Bridge checkout).
No Bridge client read-back method, receipt sink, or router join currently
exists for an admission-check response.

The already-mounted HomeBase read route,
`POST /api/v1/verifications/receipts/read`, is intentionally limited to exact,
durable `VerificationReceipt` records. It cannot be used for CP-030 without
violating the record-kind boundary.

## Required future contract before implementation

An approved authority owner must first decide whether HomeBase itself is the
issuer of this artifact, or whether another named authority issues it. The
Bridge transport key authenticates the requester; it must not be silently
treated as the issuer of HomeBase authority. The decision must also name the
key ID and rotation/revocation owner for the signing key.

After that decision, a separately versioned typed schema is required. The
minimum proposed shape below is a contract for review, not an implemented
schema:

```text
homebase.admission-check-receipt.v1
  kind: AdmissionCheckReceipt
  id: deterministic identity chosen by the authority decision
  issuer: named HomeBase authority and signing key ID
  request:
    canonical_bytes: exact canonical request bytes
    wire_bytes: exact request bytes received, if the contract requires them
    request_hash: SHA-256(canonical_bytes)
    bridge_signature: exact X-Bridge-Contract-Check-Signature bytes
  response:
    wire_bytes: exact successful response bytes, including the emitted newline
    canonical_bytes: exact bytes covered by the HomeBase signature
    response_signature: exact X-HomeBase-Admission-Signature bytes
  bindings:
    contract_id
    grant_id
    task_id
    worker_id
    context_hash
    valid_until
  captured_at: HomeBase UTC time
```

The final schema must define exact identity, including whether identity is
derived from `(request_hash, response bytes)` or from a separate authority-
issued receipt ID. It must define whether historical read-back after
`valid_until` is allowed as evidence while consumption is rejected, or whether
the read route itself returns an expiry failure. It must also define
idempotent replay (same identity and bytes returns the same receipt), conflict
behavior (same identity with changed bytes/bindings is rejected), and the
failed/unknown/unsigned/mismatched request behavior (no receipt is created).

Only after those decisions can the implementation safely add a dedicated
journal kind, replay/index state, HomeBase-owned append path, and a separate
Bridge-signed read route. That implementation must include restart/read-back
tests and exact byte/signature/hash/identity/freshness assertions.

## Live wiring still absent

The following remain intentionally absent and were not modified:

- no durable admission-check record or journal kind;
- no HomeBase append/read API for such a record;
- no Bridge client return type carrying response bytes/signature/receipt ID;
- no Bridge/router receipt sink or read-back call;
- no primary HomeBase binary restart or live-service mutation;
- no Bridge, Portfolio, Drive, credential, or external-service changes.

The current live proof is limited to authenticated, scope-bound, signed
admission at check time and the existing Bridge lease expiry enforcement. It is
not proof of durable admission-receipt persistence or read-back.

