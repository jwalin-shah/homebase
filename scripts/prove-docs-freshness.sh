#!/usr/bin/env bash
# prove-docs-freshness.sh — repo-local doc/linkage gate for receipt-readback worktree.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
FAIL=0
ok() { echo "OK: $*"; }
fail() { echo "FAIL: $*" >&2; FAIL=1; }

README="${ROOT}/README.md"
MAKEFILE="${ROOT}/Makefile"

for needle in \
  '/api/v1/verifications/receipts/read' \
  'prove-receipt-readback' \
  'HandleReadVerificationReceipt'; do
  if rg -q --fixed-strings "$needle" "$README"; then
    ok "README documents ${needle}"
  else
    fail "README.md missing ${needle}"
  fi
done

if rg -q 'prove-receipt-readback-deployment' "$MAKEFILE"; then
  ok "Makefile declares prove-receipt-readback-deployment"
else
  fail "Makefile missing prove-receipt-readback-deployment target"
fi

if [[ -x "${ROOT}/scripts/prove-receipt-readback-deployment.sh" ]]; then
  ok "deployment gate script is executable"
else
  fail "scripts/prove-receipt-readback-deployment.sh missing or not executable"
fi

if [[ "$FAIL" -ne 0 ]]; then
  echo "prove-docs-freshness: FAILED" >&2
  exit 1
fi
echo "prove-docs-freshness: PASS"
exit 0
