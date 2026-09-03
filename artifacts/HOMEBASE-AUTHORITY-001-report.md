# HOMEBASE-AUTHORITY-001 Implementation Report

**Worktree:** `/Users/jwalinshah/orca/workspaces/homebase/homebase-authority-001` (branch `jwalin-shah/homebase-authority-001`, clean base at `main` / `01aefed`)

## Summary

Implemented the authenticated captain path for atomic Specification+Decision journal commits, mirroring the sibling `homebase-receipt-readback-v01` implementation while preserving existing Contract/CapabilityGrant and legacy migration behavior.

## Changed files

| File | Change |
|---|---|
| `internal/journal/envelope.go` | Added `RecordKindSpecificationDecisionCommit` |
| `internal/records/store.go` | Added `specificationDecisions` index and replay hook |
| `internal/records/specification_decision.go` | **New** — `AppendSpecificationAndDecision`, prepare/apply/replay/decode |
| `internal/records/specification_decision_test.go` | **New** — atomicity, idempotency, conflict, schema/authority rejection |
| `api/handlers.go` | **New** `HandleAppendSpecificationDecision` with strict JSON + captain signature |
| `api/handlers_test.go` | HTTP tests: auth, idempotency, conflict, wrong status, missing ref |
| `cmd/homebase/main.go` | Mounted `POST /api/v1/specifications/decisions` |
| `README.md` | Endpoint table entry |
| `docs/record-journal-migration.md` | Migration step 3 references new endpoint |

## Authority boundary

- Requires `X-HomeBase-Specification-Signature` over canonical request JSON, verified with the same captain/contract key as `/api/v1/contracts/grants`.
- Specification must be `approved`; Decision must be captain-approved with hash-bound `specification_ref`.
- Atomic journal kind `SpecificationDecisionCommit`; byte-identical resubmit is idempotent; conflicting same-ID resubmit returns `409`.
- `AppendExternal` unchanged — still rejects Specification/Decision (`403`).
- Legacy `ContractGrantCommit` without specification still fails closed at replay.

## Evidence (all exit 0)

```text
gofmt -w <changed go files>
go test ./internal/records/ -run 'SpecificationDecision|ContractGrant|Legacy' -v
go test ./api/ -run 'SpecificationDecision|ContractGrant' -v
go build ./...
go vet ./...
go test ./...
git diff --check
```

## Not done (coordinator owns)

- No commit, push, live HomeBase interaction, or secret access.
