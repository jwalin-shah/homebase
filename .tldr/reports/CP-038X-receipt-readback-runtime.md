# CP-038X: Compiled Runtime Receipt Read-Back Verification

**Worktree:** `/Users/jwalinshah/orca/workspaces/homebase/homebase-receipt-readback-v01`  
**Date:** 2026-09-01  
**Scope:** Isolated verification only — no live port 9102, no `~/.local/bin/homebase`, no Portfolio/Drive/credentials.

## Route Under Test

`POST /api/v1/verifications/receipts/read` mounted in `cmd/homebase/main.go` → `api.Server.HandleReadVerificationReceipt`.

## Isolation Guarantees

| Constraint | How enforced |
|---|---|
| Temporary storage | `t.TempDir()` journal + ledger files; `HOMEBASE_RECORD_JOURNAL` env |
| Ephemeral port | `net.Listen("tcp", "127.0.0.1:0")` → `PORT=<ephemeral>` |
| Generated keys | `ed25519.GenerateKey` → `HOMEBASE_BRIDGE_PUBLIC_KEY_HEX` |
| Compiled binary | `go build -o <tmpdir>/homebase .` from `cmd/homebase` (not installed binary) |

## Evidence Commands (all exit 0)

```bash
cd /Users/jwalinshah/orca/workspaces/homebase/homebase-receipt-readback-v01
go build ./...
go vet ./...
go test -race ./...
make prove-receipt-readback
go test -v -count=1 -race ./cmd/homebase -run TestCompiledReceiptReadbackRoute
git diff --check
```

## Runtime Proof (`TestCompiledReceiptReadbackRoute`)

Harness: `cmd/homebase/receipt_readback_runtime_test.go`

1. Seed prerequisite Decision/Specification/Contract/CapabilityGrant (plus a Contract stored under a receipt-shaped ID for wrong-kind probing) into a temp journal.
2. Build and start compiled `cmd/homebase` against temp storage + generated Bridge public key.
3. `POST /api/v1/verifications/bridge` — append VerificationReceipt → **201 Created**
4. `POST /api/v1/verifications/receipts/read` with valid Bridge read signature → **200 OK**, body equals canonical stored bytes + trailing newline (verified after server stop by reopening journal).
5. Fail-closed cases while server running:
   - Missing receipt ID → **404** `verification receipt not found`
   - Wrong-kind ID (receipt-shaped ID pointing at Contract record) → **404** (kind boundary hidden as not found)
   - Bad read signature → **401** `Bridge verification read signature failed`
   - Missing auth header → **401**

## Handler-Level Proof (existing)

`api/handlers_test.go::TestHandleReadVerificationReceiptReturnsCanonicalStoredReceipt` covers the same happy path + missing/malformed-id/bad-signature cases via `httptest` (no subprocess).

## Limits / Not Proved

- No restart durability test (journal replay after process crash) in this harness.
- No live service on port 9102 or installed `~/.local/bin/homebase`.
- No `prove-docs-freshness` target present in this worktree.
- Wrong-kind at HTTP layer returns 404 (not 400) by design — matches `GetVerificationReceiptCanonical` hiding non-VerificationReceipt IDs.

## Files Added

- `cmd/homebase/receipt_readback_runtime_test.go` — isolated compiled-runtime harness
