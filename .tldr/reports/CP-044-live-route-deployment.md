# CP-044: Live LaunchAgent Deployment Report

**Worktree:** `/Users/jwalinshah/orca/workspaces/homebase/homebase-receipt-readback-v01`
**Deployed:** 2026-09-01T23:15:08Z (UTC)
**Scope:** Controlled binary swap + LaunchAgent restart only. No Specification/Decision
created, no `POST /api/v1/specifications/decisions` call made, no Google Drive/Portfolio
primary/Bridge primary/credential change, no commit or push.

## Pre-deployment gates (all PASS)

| Gate | Result |
|---|---|
| `gofmt -l .` | clean (no unformatted files) |
| `go vet ./...` | PASS |
| `go build ./...` | PASS |
| `go test -v -race ./...` | PASS — 12 packages, 0 failures |
| `git diff --check` | clean (no whitespace errors) |
| `scripts/prove-docs-freshness.sh` | PASS |
| `scripts/prove-receipt-readback-deployment.sh` (pre-deploy) | staged proofs all PASS; live dry-run showed expected pre-deploy hash **drift** (installed binary predates this build) — the one and only failure, and it is the deployment trigger, not a blocker |

No prerequisite failure occurred, so deployment proceeded.

## Deployment steps performed

1. **Backup** — copied the running installed binary before any change:
   - `cp -p /Users/jwalinshah/.local/bin/homebase /Users/jwalinshah/.local/bin/homebase.bak.20260901T231512Z`
   - Backup SHA256: `b92cbb06a7330040dbc649e89ec701e13284832aea27c3e9918ba915f9046253` (matches pre-deploy installed binary exactly)
2. **Build** — `go build -o <scratchpad>/homebase-cp044-deploy ./cmd/homebase` from this worktree at commit `01aefed` + uncommitted working-tree changes (the CP-040 authority route, CP-041 status route, CP-039 receipt read-back). Staged SHA256: `1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2`.
3. **Install** — atomic replace: copied to `/Users/jwalinshah/.local/bin/homebase.new`, `chmod 755`, `mv -f` onto `/Users/jwalinshah/.local/bin/homebase`.
4. **Restart** — `launchctl kickstart -k gui/501/org.nixos.org.nixos.com.jwalinshah.homebase`.

## Post-deployment proof

| Check | Result |
|---|---|
| Old PID (37477) exits | confirmed — `ps -p 37477` returns no such process after kickstart |
| New PID serves `GET /v1/status` → 200 | confirmed — PID `89188`, `{"status":"ok","service":"homebase","ledger_ready":true,"record_store_ready":true}` |
| Unsigned `POST /api/v1/verifications/receipts/read` remains 401 | confirmed — `401 Bridge verification read signature failed` |
| New authority route present, **not exercised** | confirmed via static binary marker check only (`rg -F "/api/v1/specifications/decisions"` on the installed binary → PRESENT); **no HTTP call was made to this route** |
| Installed SHA256 == staged SHA256 | confirmed — both `1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2` |
| `scripts/prove-receipt-readback-deployment.sh` (post-deploy, read-only) | **PASS** — "LIVE VERDICT: READY", staged/installed hash match, both routes mounted |
| `git status` / `git diff --check` | understood: same pre-existing uncommitted working-tree changes as before deployment (CP-030/039/040/041 implementation + evidence-artifact files that `go test` regenerates as a side effect of running the suite); `git diff --check` clean; **no commit or push performed** |

## Artifacts

- Installed binary: `/Users/jwalinshah/.local/bin/homebase` (10,257,042 bytes, mode 755, SHA256 `1336ca62e59f82ef53c238f60e9476d464c425b0b908869fa1330d2d3145b8a2`)
- Backup: `/Users/jwalinshah/.local/bin/homebase.bak.20260901T231512Z` (10,223,106 bytes, SHA256 `b92cbb06a7330040dbc649e89ec701e13284832aea27c3e9918ba915f9046253`) — restore path if rollback is needed: `cp -p homebase.bak.20260901T231512Z homebase && launchctl kickstart -k gui/501/org.nixos.org.nixos.com.jwalinshah.homebase`
- LaunchAgent: `org.nixos.org.nixos.com.jwalinshah.homebase`, plist `/Users/jwalinshah/Library/LaunchAgents/org.nixos.org.nixos.com.jwalinshah.homebase.plist`, domain `gui/501`
- Old PID: `37477` (exited) → New PID: `89188` (running)
- Log: `/Users/jwalinshah/.local/state/homebase/homebase.log` — clean startup, no crash loop, `WARNING: NEO4J_URI not set; running WITHOUT the Axiom Firewall (local-only mode)` (expected, unrelated to this deploy)
- Companion read-only gate report: `.tldr/reports/CP-039-receipt-readback-deployment-readiness.md` (now shows OVERALL PASS / LIVE VERDICT: READY)

## Explicitly not done

- No `POST /api/v1/specifications/decisions` call (route confirmed present by static binary inspection only).
- No Specification or Decision record created on the live instance.
- No Google Drive, Portfolio primary, Bridge primary, or credential file touched.
- No other worktree touched.
- No git commit or push.
