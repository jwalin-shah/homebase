# Epistemic promotion v1

`POST /api/v1/promotions/evidence` admits an authenticated ProviderResearchRun
bundle independently of Bridge's Git-tree verification path and transcript
promotion.

## Authority boundary

- Request authentication uses detached Ed25519 over canonical request JSON,
  supplied in `X-HomeBase-Epistemic-Signature`.
- The configured promoter identity is recorded as the accepting authority.
- HomeBase signs an `EpistemicAcceptanceReceipt` with a separately configured
  receipt key. Receipt identity is bound to a request digest and submission ID.
- A duplicate submission with the same request digest returns the prior receipt;
  reusing its ID with different signed content conflicts.
- The receipt proves provenance intake and evaluation status. It sets
  `claims_asserted_true=false`; it does not authorize a Decision, approve a
  transcript, or assert that provider claims are true.
- `/api/v1/records` remains an untrusted ingress. The authenticated path is
  specific to research provenance and cannot accept arbitrary record bundles.

## Preserved provenance

Each Evidence payload binds a provider run ID, provider name, optional account
and route references, run input/output hashes, source artifact hash, source URI,
publisher identity, content digest, source lineage digest, source timestamp,
retrieval timestamp, freshness status, and whether source content was retained.
The lineage digest is recomputed from the canonical source URI using the
LifeOps lineage rule (lowercase scheme/host, trimmed trailing path slash,
tracking parameters removed, fragment omitted). Evidence retrieval must fall
inside the run's start/end interval.
Claims preserve normalized text, epistemic status, Evidence references,
contradiction references, and the source lineages used by the claim.

`CORROBORATED` requires at least two distinct canonical source lineages and
known fresh source timestamps under the request's bounded freshness policy.
`CONTESTED` requires a resolved contradiction reference. `STALE` requires cited
source evidence older than that policy. `UNVERIFIED` is retained as such.
`REJECTED` negative controls may have no Evidence references. These checks
validate the submitted classification and provenance; they do not independently
read source pages or establish claim truth.

## Atomic journal record

One `EpistemicAcceptanceCommit` contains all Evidence and Claim records and the
signed Proof record. The store checks unique IDs, evidence-to-claim content-hash
references, full Proof binding, receipt/submission binding, and journal replay
before accepting the bundle. A torn or invalid commit cannot expose only a
prefix of the Evidence/Claim/Proof set through the typed-record projection.

## Required configuration

- `HOMEBASE_EPISTEMIC_PROMOTER_ID`
- `HOMEBASE_EPISTEMIC_PROMOTER_PUBLIC_KEY_FILE`
- `HOMEBASE_EPISTEMIC_RECEIPT_KEY_ID`
- `HOMEBASE_EPISTEMIC_RECEIPT_PRIVATE_KEY_FILE`

Until these values are configured with owner-approved key material, the route
returns unavailable. This source candidate does not provision keys or deploy
the endpoint.
