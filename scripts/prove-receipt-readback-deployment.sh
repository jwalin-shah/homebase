#!/usr/bin/env bash
# prove-receipt-readback-deployment.sh — CP-039 deployment-readiness gate.
#
# STAGED ARTIFACT PROOF (hermetic): builds cmd/homebase in a temp dir, checks the
# receipt read-back route marker, runs receipt-readback tests, records SHA256.
#
# LIVE DEPLOYMENT DRY-RUN (read-only): inspects LaunchAgent plist, installed
# binary metadata, and authority key-file presence/mode. Never reads key bytes,
# never installs/copies binaries, never reloads LaunchAgents, never contacts
# Portfolio/Drive/Neo4j/Bridge.
#
# Fail-closed: live deployment is NOT ready until staged hash matches the
# installed binary AND the live binary carries the route marker AND the live
# HTTP seam returns 401 (route mounted) instead of 404 (route absent).
#
# Usage:
#   prove-receipt-readback-deployment.sh            # staged + live dry-run
#   prove-receipt-readback-deployment.sh --staged   # staged artifact proof only
#   prove-receipt-readback-deployment.sh --live-dry-run
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$ROOT"

export PATH="${HOME}/.local/bin:${HOME}/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:${PATH:-}"

MODE="all"
case "${1:-}" in
  "") MODE="all" ;;
  --staged) MODE="staged" ;;
  --live-dry-run) MODE="live" ;;
  -h|--help)
    sed -n '1,20p' "$0"
    exit 0
    ;;
  *)
    echo "unknown argument: $1 (use --staged or --live-dry-run)" >&2
    exit 2
    ;;
esac

REPORT_DIR="${ROOT}/.tldr/reports"
REPORT="${REPORT_DIR}/CP-039-receipt-readback-deployment-readiness.md"
mkdir -p "$REPORT_DIR"

FAIL=0
STAGED_FAIL=0
LIVE_FAIL=0
ok() { echo "OK: $*"; }
fail() { echo "FAIL: $*" >&2; FAIL=1; }
fail_staged() { echo "FAIL[STAGED]: $*" >&2; FAIL=1; STAGED_FAIL=1; }
fail_live() { echo "FAIL[LIVE-DRY-RUN]: $*" >&2; FAIL=1; LIVE_FAIL=1; }
warn() { echo "WARN: $*" >&2; }

# --- deployment contract (read from live plist when present) ---
INSTALL_PATH="${HOMEBASE_INSTALL_PATH:-${HOME}/.local/bin/homebase}"
PLIST_PATH="${HOMEBASE_PLIST_PATH:-${HOME}/Library/LaunchAgents/org.nixos.org.nixos.com.jwalinshah.homebase.plist}"
KEY_DIR="${HOMEBASE_KEY_DIR:-${HOME}/.local/state/homebase/keys}"
ROUTE_MARKER="/api/v1/verifications/receipts/read"
STATUS_ROUTE_MARKER="/v1/status"
EXPECTED_PORT="${HOMEBASE_PORT:-9102}"
EXPECTED_LABEL="${HOMEBASE_LAUNCHAGENT_LABEL:-org.nixos.org.nixos.com.jwalinshah.homebase}"
DAEMON_WRAPPER="${HOMEBASE_DAEMON_WRAPPER:-${HOME}/.dotfiles/bin/daemon-wrapper}"
LIVE_URL="http://127.0.0.1:${EXPECTED_PORT}"

REQUIRED_KEY_FILES=(
  captain.pub
  bridge.pub
  admission.priv
  verifier.pub
  receipt.priv
)

STAGED_HASH=""
STAGED_BINARY=""
STAGED_ROUTE_PRESENT=0
STAGED_STATUS_ROUTE_PRESENT=0

report_begin() {
  cat >"$REPORT" <<EOF
# CP-039: Receipt Read-Back Deployment Readiness

**Worktree:** \`${ROOT}\`
**Generated:** $(date -u +"%Y-%m-%dT%H:%M:%SZ")
**Gate script:** \`scripts/prove-receipt-readback-deployment.sh\`

This gate also carries **CP-041: LaunchAgent health contract (\`GET /v1/status\`)**.
Both are proved together because they share the same staged artifact, the
same installed binary, and the same live process on port ${EXPECTED_PORT}.

## Proof classes

| Class | Meaning |
|---|---|
| **STAGED ARTIFACT PROOF** | Hermetic build + tests in this worktree. Proves the candidate binary carries the receipt read-back route. Does **not** prove live deployment. |
| **LIVE DEPLOYMENT DRY-RUN** | Read-only inspection of plist, installed binary, keys, and HTTP seam. Proves whether the machine is ready to accept the staged artifact. Does **not** install or restart anything. |

EOF
}

report_section() {
  printf '%s\n' "$1" >>"$REPORT"
}

prove_staged() {
  echo "=== STAGED ARTIFACT PROOF (hermetic) ==="
  report_section "## STAGED ARTIFACT PROOF (hermetic)"

  STAGED_DIR="$(mktemp -d "${TMPDIR:-/tmp}/homebase-staged.XXXXXX")"
  STAGED_BINARY="${STAGED_DIR}/homebase"
  trap 'rm -rf "$STAGED_DIR"' RETURN

  if go build -o "$STAGED_BINARY" ./cmd/homebase; then
    ok "staged build cmd/homebase → ${STAGED_BINARY}"
    report_section "- Build: \`go build -o <tmpdir>/homebase ./cmd/homebase\` — **PASS**"
  else
    fail_staged "staged build failed"
    report_section "- Build: **FAIL**"
    return
  fi

  STAGED_HASH="$(shasum -a 256 "$STAGED_BINARY" | awk '{print $1}')"
  ok "staged SHA256 ${STAGED_HASH}"
  report_section "- Staged SHA256: \`${STAGED_HASH}\`"

  if rg -Fq "$ROUTE_MARKER" "$STAGED_BINARY"; then
    STAGED_ROUTE_PRESENT=1
    ok "staged binary contains route marker"
    report_section "- Route marker \`${ROUTE_MARKER}\` in staged binary — **PRESENT**"
  else
    fail_staged "staged binary missing route marker ${ROUTE_MARKER}"
    report_section "- Route marker — **ABSENT**"
  fi

  if rg -Fq "$STATUS_ROUTE_MARKER" "$STAGED_BINARY"; then
    STAGED_STATUS_ROUTE_PRESENT=1
    ok "staged binary contains health status route marker"
    report_section "- Route marker \`${STATUS_ROUTE_MARKER}\` in staged binary — **PRESENT**"
  else
    fail_staged "staged binary missing route marker ${STATUS_ROUTE_MARKER}"
    report_section "- Health status route marker — **ABSENT**"
  fi

  if go test -count=1 ./api -run 'TestHandleReadVerificationReceipt' >/dev/null; then
    ok "handler receipt read-back tests"
    report_section "- \`go test ./api -run TestHandleReadVerificationReceipt\` — **PASS**"
  else
    fail_staged "handler receipt read-back tests failed"
    report_section "- Handler tests — **FAIL**"
  fi

  if go test -count=1 ./api -run 'TestHandleStatus' >/dev/null; then
    ok "handler status tests"
    report_section "- \`go test ./api -run TestHandleStatus\` — **PASS**"
  else
    fail_staged "handler status tests failed"
    report_section "- Handler status tests — **FAIL**"
  fi

  if go test -count=1 ./cmd/homebase -run 'TestCompiledReceiptReadbackRoute' >/dev/null; then
    ok "compiled runtime receipt read-back test"
    report_section "- \`go test ./cmd/homebase -run TestCompiledReceiptReadbackRoute\` — **PASS**"
  else
    fail_staged "compiled runtime receipt read-back test failed"
    report_section "- Compiled runtime test — **FAIL**"
  fi

  if go test -count=1 ./cmd/homebase -run 'TestCompiledStatusRoute' >/dev/null; then
    ok "compiled runtime status route test"
    report_section "- \`go test ./cmd/homebase -run TestCompiledStatusRoute\` — **PASS**"
  else
    fail_staged "compiled runtime status route test failed"
    report_section "- Compiled runtime status test — **FAIL**"
  fi

  if go test -count=1 ./cmd/homebase -run 'TestStagedDeploymentReadinessContract' >/dev/null; then
    ok "deployment readiness contract test"
    report_section "- \`go test ./cmd/homebase -run TestStagedDeploymentReadinessContract\` — **PASS**"
  else
    fail_staged "deployment readiness contract test failed"
    report_section "- Deployment readiness contract test — **FAIL**"
  fi

  if [[ "$STAGED_FAIL" -eq 0 ]]; then
    report_section ""
    report_section "**STAGED VERDICT: PASS** — candidate artifact is verified in isolation."
  else
    report_section ""
    report_section "**STAGED VERDICT: FAIL** — do not deploy until staged proofs pass."
  fi
}

prove_live_dry_run() {
  echo "=== LIVE DEPLOYMENT DRY-RUN (read-only) ==="
  report_section ""
  report_section "## LIVE DEPLOYMENT DRY-RUN (read-only)"

  # LaunchAgent plist metadata
  if [[ -f "$PLIST_PATH" ]]; then
    ok "LaunchAgent plist present: ${PLIST_PATH}"
    report_section "- Plist: \`${PLIST_PATH}\` — **present**"
  else
    fail_live "LaunchAgent plist missing: ${PLIST_PATH}"
    report_section "- Plist — **MISSING**"
  fi

  if [[ -f "$PLIST_PATH" ]]; then
    LABEL="$(/usr/libexec/PlistBuddy -c 'Print :Label' "$PLIST_PATH" 2>/dev/null || true)"
    PORT="$(/usr/libexec/PlistBuddy -c 'Print :EnvironmentVariables:PORT' "$PLIST_PATH" 2>/dev/null || true)"
    WRAPPER="$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments:0' "$PLIST_PATH" 2>/dev/null || true)"
    BINARY_ARG="$(/usr/libexec/PlistBuddy -c 'Print :ProgramArguments:1' "$PLIST_PATH" 2>/dev/null || true)"

    if [[ "$LABEL" == "$EXPECTED_LABEL" ]]; then
      ok "LaunchAgent label ${LABEL}"
      report_section "- Label \`${LABEL}\` — **matches contract**"
    else
      fail_live "LaunchAgent label mismatch: got=${LABEL:-<empty>} want=${EXPECTED_LABEL}"
      report_section "- Label — **MISMATCH** (got \`${LABEL:-<empty>}\`, want \`${EXPECTED_LABEL}\`)"
    fi

    if [[ "$PORT" == "$EXPECTED_PORT" ]]; then
      ok "LaunchAgent PORT=${PORT}"
      report_section "- PORT \`${PORT}\` — **matches contract**"
    else
      fail_live "LaunchAgent PORT mismatch: got=${PORT:-<empty>} want=${EXPECTED_PORT}"
      report_section "- PORT — **MISMATCH**"
    fi

    if [[ "$WRAPPER" == "$DAEMON_WRAPPER" ]]; then
      ok "ProgramArguments[0] daemon-wrapper"
      report_section "- ProgramArguments[0] \`${WRAPPER}\` — **matches**"
    else
      fail_live "daemon-wrapper mismatch: got=${WRAPPER:-<empty>} want=${DAEMON_WRAPPER}"
      report_section "- ProgramArguments[0] — **MISMATCH**"
    fi

    if [[ "$BINARY_ARG" == "$INSTALL_PATH" ]]; then
      ok "ProgramArguments[1] install path"
      report_section "- ProgramArguments[1] \`${BINARY_ARG}\` — **matches install path**"
    else
      fail_live "install path mismatch: got=${BINARY_ARG:-<empty>} want=${INSTALL_PATH}"
      report_section "- ProgramArguments[1] — **MISMATCH**"
    fi
  fi

  # Installed binary metadata (never copied/overwritten by this gate)
  LIVE_HASH=""
  LIVE_ROUTE_PRESENT=0
  LIVE_STATUS_ROUTE_PRESENT=0
  if [[ -f "$INSTALL_PATH" ]]; then
    LIVE_HASH="$(shasum -a 256 "$INSTALL_PATH" | awk '{print $1}')"
    LIVE_MTIME="$(stat -f '%Sm' -t '%Y-%m-%dT%H:%M:%SZ' "$INSTALL_PATH" 2>/dev/null || echo unknown)"
    LIVE_SIZE="$(stat -f '%z' "$INSTALL_PATH" 2>/dev/null || echo 0)"
    ok "installed binary present (${LIVE_SIZE} bytes, mtime ${LIVE_MTIME})"
    report_section "- Installed binary \`${INSTALL_PATH}\` — **present** (${LIVE_SIZE} bytes, mtime ${LIVE_MTIME})"
    report_section "- Installed SHA256: \`${LIVE_HASH}\`"

    if rg -Fq "$ROUTE_MARKER" "$INSTALL_PATH"; then
      LIVE_ROUTE_PRESENT=1
      ok "live binary contains route marker"
      report_section "- Route marker in live binary — **PRESENT**"
    else
      fail_live "live binary missing route marker (not deployed yet)"
      report_section "- Route marker in live binary — **ABSENT**"
    fi

    if rg -Fq "$STATUS_ROUTE_MARKER" "$INSTALL_PATH"; then
      LIVE_STATUS_ROUTE_PRESENT=1
      ok "live binary contains health status route marker"
      report_section "- Health status route marker in live binary — **PRESENT**"
    else
      fail_live "live binary missing health status route marker (not deployed yet)"
      report_section "- Health status route marker in live binary — **ABSENT**"
    fi
  else
    fail_live "installed binary missing: ${INSTALL_PATH}"
    report_section "- Installed binary — **MISSING**"
  fi

  # Authority key files: presence + mode metadata only (never read contents)
  report_section ""
  report_section "### Authority key files (metadata only)"
  for key in "${REQUIRED_KEY_FILES[@]}"; do
    f="${KEY_DIR}/${key}"
    if [[ -f "$f" ]]; then
      mode="$(stat -f '%Lp' "$f" 2>/dev/null || echo 000)"
      size="$(stat -f '%z' "$f" 2>/dev/null || echo 0)"
      if [[ "$mode" == "600" ]]; then
        ok "key ${key} present mode 600"
        report_section "- \`${key}\` — present, mode \`${mode}\`, size ${size} (contents not read)"
      else
        fail_live "key ${key} mode ${mode} (want 600)"
        report_section "- \`${key}\` — **BAD MODE ${mode}** (want 600)"
      fi
    else
      fail_live "missing key file ${f}"
      report_section "- \`${key}\` — **MISSING**"
    fi
  done

  # Staged vs live hash binding (fail-closed until operator deploys)
  report_section ""
  report_section "### Staged vs live binary binding"
  if [[ -n "$STAGED_HASH" && -n "$LIVE_HASH" ]]; then
    if [[ "$STAGED_HASH" == "$LIVE_HASH" ]]; then
      ok "staged hash matches installed binary"
      report_section "- Staged SHA256 matches installed binary — **MATCH**"
    else
      fail_live "staged hash != installed binary (deploy required)"
      report_section "- Staged SHA256 **≠** installed SHA256 — **DRIFT** (install blocked until operator deploys)"
      report_section "  - staged: \`${STAGED_HASH}\`"
      report_section "  - live:   \`${LIVE_HASH}\`"
    fi
  elif [[ -z "$STAGED_HASH" ]]; then
    warn "staged hash unavailable (run with --staged or default all mode)"
    report_section "- Staged hash unavailable — run full gate for binding check"
  fi

  # Live HTTP seam (read-only POST; distinguishes 404 absent vs 401 mounted)
  report_section ""
  report_section "### Live HTTP seam (read-only)"
  LIVE_HTTP_CODE="$(curl -s -o /tmp/homebase-cp039-body.$$ -w '%{http_code}' \
    -X POST "${LIVE_URL}${ROUTE_MARKER}" \
    -H 'Content-Type: application/json' \
    -d '{"receipt_id":"receipt:deploy-gate:0123456789012345678901234567890123456789"}' \
    2>/dev/null || echo 000)"
  LIVE_HTTP_BODY="$(head -c 120 /tmp/homebase-cp039-body.$$ 2>/dev/null || true)"
  rm -f /tmp/homebase-cp039-body.$$

  report_section "- POST \`${LIVE_URL}${ROUTE_MARKER}\` (unsigned) → HTTP ${LIVE_HTTP_CODE}"
  case "$LIVE_HTTP_CODE" in
    401)
      ok "live route mounted (401 unsigned read-back)"
      report_section "  - **ROUTE MOUNTED** (401 Bridge verification read signature failed)"
      ;;
    404)
      if [[ "$LIVE_HTTP_BODY" == *"page not found"* ]]; then
        fail_live "live route not mounted (404 page not found)"
        report_section "  - **ROUTE ABSENT** (Go default 404 — binary not deployed)"
      else
        warn "live returned 404 (receipt not found or route semantics differ)"
        report_section "  - 404 with body: \`${LIVE_HTTP_BODY}\`"
      fi
      ;;
    000)
      fail_live "HomeBase not reachable at ${LIVE_URL}"
      report_section "  - **UNREACHABLE**"
      ;;
    *)
      warn "unexpected live HTTP ${LIVE_HTTP_CODE}: ${LIVE_HTTP_BODY}"
      report_section "  - Unexpected response: \`${LIVE_HTTP_BODY}\`"
      ;;
  esac

  # Live health-status HTTP seam (read-only GET; distinguishes 404 absent vs
  # 200 mounted). This is the exact DAEMON_HEALTH_URL contract from the
  # LaunchAgent plist.
  report_section ""
  report_section "### Live health-status HTTP seam (read-only, DAEMON_HEALTH_URL)"
  STATUS_HTTP_CODE="$(curl -s -o /tmp/homebase-cp041-body.$$ -w '%{http_code}' \
    -X GET "${LIVE_URL}${STATUS_ROUTE_MARKER}" \
    2>/dev/null || echo 000)"
  STATUS_HTTP_BODY="$(head -c 200 /tmp/homebase-cp041-body.$$ 2>/dev/null || true)"
  rm -f /tmp/homebase-cp041-body.$$

  report_section "- GET \`${LIVE_URL}${STATUS_ROUTE_MARKER}\` → HTTP ${STATUS_HTTP_CODE}"
  case "$STATUS_HTTP_CODE" in
    200)
      ok "live health status route mounted (200)"
      report_section "  - **ROUTE MOUNTED** (200 OK)"
      report_section "  - Body: \`${STATUS_HTTP_BODY}\`"
      ;;
    404)
      fail_live "live health status route not mounted (404 — DAEMON_HEALTH_URL would fail)"
      report_section "  - **ROUTE ABSENT** (Go default 404 — binary not deployed; DAEMON_HEALTH_URL contract broken)"
      ;;
    000)
      fail_live "HomeBase not reachable at ${LIVE_URL}${STATUS_ROUTE_MARKER}"
      report_section "  - **UNREACHABLE**"
      ;;
    *)
      warn "unexpected live status HTTP ${STATUS_HTTP_CODE}: ${STATUS_HTTP_BODY}"
      report_section "  - Unexpected response: \`${STATUS_HTTP_BODY}\`"
      ;;
  esac

  # Explicit operator commands (documented; not executed mutatingly)
  report_section ""
  report_section "## Operator commands (explicit; not run by this gate)"
  report_section ""
  report_section "### PRE-DEPLOY (read-only baseline)"
  report_section '```bash'
  report_section "shasum -a 256 ${INSTALL_PATH}"
  report_section "strings ${INSTALL_PATH} | rg -F '${ROUTE_MARKER}' || echo 'route absent (expected pre-deploy)'"
  report_section "strings ${INSTALL_PATH} | rg -F '${STATUS_ROUTE_MARKER}' || echo 'route absent (expected pre-deploy)'"
  report_section "curl -s -o /dev/null -w '%{http_code}\\n' -X POST ${LIVE_URL}${ROUTE_MARKER} \\"
  report_section "  -H 'Content-Type: application/json' \\"
  report_section "  -d '{\"receipt_id\":\"receipt:deploy-gate:0123456789012345678901234567890123456789\"}'"
  report_section "# expect: 404 page not found (route absent) before deploy"
  report_section "curl -s -o /dev/null -w '%{http_code}\\n' ${LIVE_URL}${STATUS_ROUTE_MARKER}"
  report_section "# expect: 404 page not found (route absent) before deploy — this is CP-041's finding"
  report_section '```'
  report_section ""
  report_section "### POST-DEPLOY (after operator runs: go install + launchctl kickstart)"
  report_section '```bash'
  report_section "shasum -a 256 ${INSTALL_PATH}    # must match staged: ${STAGED_HASH:-<run staged gate first>}"
  report_section "strings ${INSTALL_PATH} | rg -F '${ROUTE_MARKER}'"
  report_section "strings ${INSTALL_PATH} | rg -F '${STATUS_ROUTE_MARKER}'"
  report_section "curl -s -o /dev/null -w '%{http_code}\\n' -X POST ${LIVE_URL}${ROUTE_MARKER} \\"
  report_section "  -H 'Content-Type: application/json' \\"
  report_section "  -d '{\"receipt_id\":\"receipt:deploy-gate:0123456789012345678901234567890123456789\"}'"
  report_section "# expect: 401 Bridge verification read signature failed (route mounted)"
  report_section "curl -s ${LIVE_URL}${STATUS_ROUTE_MARKER}"
  report_section "# expect: HTTP 200 with {\"status\":\"ok\",...} (route mounted, LaunchAgent health contract satisfied)"
  report_section "make -C ${ROOT} prove-receipt-readback-deployment"
  report_section '```'

  if [[ "$LIVE_FAIL" -eq 0 ]]; then
    report_section ""
    report_section "**LIVE VERDICT: READY** — installed binary matches staged artifact and live seam is mounted."
  else
    report_section ""
    report_section "**LIVE VERDICT: NOT READY** — operator must deploy staged artifact before live receipt read-back is available."
  fi
}

report_begin

case "$MODE" in
  staged)
    prove_staged
    ;;
  live)
    prove_live_dry_run
    ;;
  all)
    prove_staged
  prove_live_dry_run
    ;;
esac

report_section ""
report_section "## Gate summary"
if [[ "$FAIL" -eq 0 ]]; then
  report_section "- **OVERALL: PASS**"
  echo "prove-receipt-readback-deployment: PASS"
  echo "report: ${REPORT}"
  exit 0
fi

if [[ "$STAGED_FAIL" -ne 0 && "$LIVE_FAIL" -ne 0 ]]; then
  report_section "- **OVERALL: FAIL** (staged artifact and live deployment checks)"
elif [[ "$STAGED_FAIL" -ne 0 ]]; then
  report_section "- **OVERALL: FAIL** (staged artifact — do not deploy)"
elif [[ "$LIVE_FAIL" -ne 0 ]]; then
  report_section "- **OVERALL: FAIL** (live not ready — staged artifact may still be valid)"
fi

echo "prove-receipt-readback-deployment: FAILED" >&2
echo "report: ${REPORT}" >&2
exit 1
