# CP-039: Receipt Read-Back Deployment Readiness

**Worktree:** `/Users/jwalinshah/orca/workspaces/homebase/homebase-receipt-readback-v01`
**Generated:** 2026-09-01T23:16:36Z
**Gate script:** `scripts/prove-receipt-readback-deployment.sh`

This gate also carries **CP-041: LaunchAgent health contract (`GET /v1/status`)**.
Both are proved together because they share the same staged artifact, the
same installed binary, and the same live process on port 9102.

## Proof classes

| Class | Meaning |
|---|---|
| **STAGED ARTIFACT PROOF** | Hermetic build + tests in this worktree. Proves the candidate binary carries the receipt read-back route. Does **not** prove live deployment. |
| **LIVE DEPLOYMENT DRY-RUN** | Read-only inspection of plist, installed binary, keys, and HTTP seam. Proves whether the machine is ready to accept the staged artifact. Does **not** install or restart anything. |

## STAGED ARTIFACT PROOF (hermetic)
- Build: `go build -o <tmpdir>/homebase ./cmd/homebase` — **PASS**
- Staged SHA256: `1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2`
- Route marker `/api/v1/verifications/receipts/read` in staged binary — **PRESENT**
- Route marker `/v1/status` in staged binary — **PRESENT**
- `go test ./api -run TestHandleReadVerificationReceipt` — **PASS**
- `go test ./api -run TestHandleStatus` — **PASS**
- `go test ./cmd/homebase -run TestCompiledReceiptReadbackRoute` — **PASS**
- `go test ./cmd/homebase -run TestCompiledStatusRoute` — **PASS**
- `go test ./cmd/homebase -run TestStagedDeploymentReadinessContract` — **PASS**

**STAGED VERDICT: PASS** — candidate artifact is verified in isolation.

## LIVE DEPLOYMENT DRY-RUN (read-only)
- Plist: `/Users/jwalinshah/Library/LaunchAgents/org.nixos.org.nixos.com.jwalinshah.homebase.plist` — **present**
- Label `org.nixos.org.nixos.com.jwalinshah.homebase` — **matches contract**
- PORT `9102` — **matches contract**
- ProgramArguments[0] `/Users/jwalinshah/.dotfiles/bin/daemon-wrapper` — **matches**
- ProgramArguments[1] `/Users/jwalinshah/.local/bin/homebase` — **matches install path**
- Installed binary `/Users/jwalinshah/.local/bin/homebase` — **present** (10257042 bytes, mtime 2026-09-01T16:15:08Z)
- Installed SHA256: `1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2`
- Route marker in live binary — **PRESENT**
- Health status route marker in live binary — **PRESENT**

### Authority key files (metadata only)
- `captain.pub` — present, mode `600`, size 65 (contents not read)
- `bridge.pub` — present, mode `600`, size 65 (contents not read)
- `admission.priv` — present, mode `600`, size 129 (contents not read)
- `verifier.pub` — present, mode `600`, size 65 (contents not read)
- `receipt.priv` — present, mode `600`, size 129 (contents not read)

### Staged vs live binary binding
- Staged SHA256 matches installed binary — **MATCH**

### Live HTTP seam (read-only)
- POST `http://127.0.0.1:9102/api/v1/verifications/receipts/read` (unsigned) → HTTP 401
  - **ROUTE MOUNTED** (401 Bridge verification read signature failed)

### Live health-status HTTP seam (read-only, DAEMON_HEALTH_URL)
- GET `http://127.0.0.1:9102/v1/status` → HTTP 200
  - **ROUTE MOUNTED** (200 OK)
  - Body: `{"status":"ok","service":"homebase","ledger_ready":true,"record_store_ready":true}`

## Operator commands (explicit; not run by this gate)

### PRE-DEPLOY (read-only baseline)
```bash
shasum -a 256 /Users/jwalinshah/.local/bin/homebase
strings /Users/jwalinshah/.local/bin/homebase | rg -F '/api/v1/verifications/receipts/read' || echo 'route absent (expected pre-deploy)'
strings /Users/jwalinshah/.local/bin/homebase | rg -F '/v1/status' || echo 'route absent (expected pre-deploy)'
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:9102/api/v1/verifications/receipts/read \
  -H 'Content-Type: application/json' \
  -d '{"receipt_id":"receipt:deploy-gate:0123456789012345678901234567890123456789"}'
# expect: 404 page not found (route absent) before deploy
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:9102/v1/status
# expect: 404 page not found (route absent) before deploy — this is CP-041's finding
```

### POST-DEPLOY (after operator runs: go install + launchctl kickstart)
```bash
shasum -a 256 /Users/jwalinshah/.local/bin/homebase    # must match staged: 1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2
strings /Users/jwalinshah/.local/bin/homebase | rg -F '/api/v1/verifications/receipts/read'
strings /Users/jwalinshah/.local/bin/homebase | rg -F '/v1/status'
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:9102/api/v1/verifications/receipts/read \
  -H 'Content-Type: application/json' \
  -d '{"receipt_id":"receipt:deploy-gate:0123456789012345678901234567890123456789"}'
# expect: 401 Bridge verification read signature failed (route mounted)
curl -s http://127.0.0.1:9102/v1/status
# expect: HTTP 200 with {"status":"ok",...} (route mounted, LaunchAgent health contract satisfied)
make -C /Users/jwalinshah/orca/workspaces/homebase/homebase-receipt-readback-v01 prove-receipt-readback-deployment
```

**LIVE VERDICT: READY** — installed binary matches staged artifact and live seam is mounted.

## Gate summary
- **OVERALL: PASS**
