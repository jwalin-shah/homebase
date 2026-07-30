# Change rationale

Each code change in this integration must answer three questions: what broken
behavior existed, which contract/invariant requires the change, and what
evidence would falsify it.

## `internal/journal/envelope.go`

- **Reason:** The previous replay path used payload shape as a discriminator,
  which allows overlapping JSON shapes to be interpreted as the wrong record.
- **Contract:** Journal transport has an explicit kind and version; shared
  records are not attempt events.
- **Evidence:** Envelope tests and replay tests must reject missing/unsupported
  envelopes and keep shared records out of the attempt reducer.

## `internal/records/store.go`

- **Reason:** The projects had typed record schemas and fixture validators but
  no durable HomeBase admission path using them.
- **Contract:** Record v1 is strict, content-addressed, append-only, replayable,
  and authority-separated. External ingress is not allowed to mint
  human-authoritative or verified records.
- **Evidence:** The package tests cover hash forgery, unknown fields, duplicate
  identity, restart replay, authority rejection, and malformed nested JSON.
- **Limit:** This is runtime validation, not a mathematical proof that the Go
  implementation refines a Dafny model. That refinement is still open.

## `api/handlers.go`

- **Reason:** A durable store that is not reachable through the intended system
  path cannot exercise the real boundary.
- **Contract:** `/api/v1/records` accepts one bounded JSON record and returns
  success only after HomeBase has validated and fsynced it. Authoritative
  records require an authenticated owner-specific path.
- **Evidence:** Handler tests cover create, idempotent replay, and public
  authority rejection.

## `cmd/homebase/main.go`

- **Reason:** The runtime must open the same durable typed record store that the
  API uses; otherwise tests would only prove an in-process object.
- **Contract:** Journal path is explicit/configurable, replay happens before
  serving, and startup fails if the journal cannot be opened or replayed.
- **Limit:** The existing legacy decision endpoint still uses its old
  ephemeral-key/mock-attempt path. It must not be described as production
  authority until replaced and certified.

## What was deliberately not changed

- Original dirty repositories were not modified.
- No dotfiles were activated, deleted, or replaced.
- No GitHub publication, agent provider call, Neo4j mutation, or Knowledge
  Engine write was performed.
- No claim of full-system completion was made from these tests.
