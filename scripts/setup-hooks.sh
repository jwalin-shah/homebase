#!/bin/bash
# Setup git hooks for local enforcement
#
# Usage: ./scripts/setup-hooks.sh
# This installs git hooks that enforce the phase workflow locally

set -e

echo "📋 Setting up HomeBase enforcement hooks..."

# Create .git/hooks if it doesn't exist
mkdir -p .git/hooks

# Install pre-commit hook
echo "  Installing pre-commit hook..."
ln -sf ../../.githooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
echo "    ✓ pre-commit installed"

# Install commit-msg hook
echo "  Installing commit-msg hook..."
ln -sf ../../.githooks/commit-msg .git/hooks/commit-msg
chmod +x .git/hooks/commit-msg
echo "    ✓ commit-msg installed"

# Install phase-gate hook
echo "  Installing phase-gate-local hook..."
ln -sf ../../.githooks/phase-gate-local .git/hooks/phase-gate-local
chmod +x .git/hooks/phase-gate-local
echo "    ✓ phase-gate-local installed"

# Make common-bugs-check available
if [ ! -d "scripts" ]; then
  mkdir -p scripts
fi

echo "  Creating common-bugs-check script..."
cat > scripts/common-bugs-check.sh << 'EOF'
#!/bin/bash
# Common bugs check (can be run manually or in hooks)

set -e

echo "🔍 Scanning for common bugs..."

# Bug 1.1: Unverified signatures
UNVERIFIED=$(grep -r "\.Signature = " *.go 2>/dev/null | grep -v "Verify\|validate" | wc -l || echo 0)
if [ $UNVERIFIED -gt 0 ]; then
  echo "⚠️  $UNVERIFIED signature assignments found without verification"
fi

# Bug 2.1: Unhandled errors
UNHANDLED=$(grep -n "_ = " *.go 2>/dev/null | grep -v "for _" | grep -v range | wc -l || echo 0)
if [ $UNHANDLED -gt 0 ]; then
  echo "⚠️  $UNHANDLED potential unhandled errors found"
  echo "   Review: grep -n '_ = ' *.go"
fi

# Bug 3.1: Unbounded queries
UNBOUNDED=$(grep -r "SELECT\|MATCH" *.go 2>/dev/null | grep -v "LIMIT" | wc -l || echo 0)
if [ $UNBOUNDED -gt 0 ]; then
  echo "⚠️  $UNBOUNDED queries without LIMIT found"
fi

# Bug 6.1: Test isolation
SHARED_MEM=$(grep ":memory:" *_test.go 2>/dev/null | wc -l || echo 0)
if [ $SHARED_MEM -gt 0 ]; then
  echo "❌ FAIL: $SHARED_MEM :memory: databases in test code"
  echo "   Use t.TempDir() for test isolation"
  exit 1
fi

echo "✓ Common bugs check passed"
EOF

chmod +x scripts/common-bugs-check.sh
echo "    ✓ common-bugs-check.sh created"

echo ""
echo "✅ Hooks installed successfully!"
echo ""
echo "How enforcement works:"
echo ""
echo "1. PRE-COMMIT HOOK (.git/hooks/pre-commit)"
echo "   Runs before you commit. Checks:"
echo "   • No unhandled errors"
echo "   • No hardcoded config"
echo "   • No :memory: in production code"
echo "   • Code is formatted (gofmt)"
echo ""
echo "2. COMMIT-MSG HOOK (.git/hooks/commit-msg)"
echo "   Validates commit message. Checks:"
echo "   • References TICKET-XXX or Phase N"
echo "   • Sign-off line if claiming completion"
echo "   • Warns if phase gate might fail"
echo ""
echo "3. PHASE-GATE-LOCAL HOOK (.git/hooks/phase-gate-local)"
echo "   Run manually before pushing: ./.githooks/phase-gate-local"
echo "   Checks:"
echo "   • Prior phase is complete (all signatures)"
echo "   • Audit findings resolved (if Phase 5)"
echo ""
echo "BYPASS (DISCOURAGED):"
echo "   git commit --no-verify        # Skip pre-commit hook"
echo "   git push --no-verify          # Not applicable (local only)"
echo ""
echo "NEXT STEP:"
echo "   git commit -m 'TICKET-202: Setup enforcement (Phase 0)'"
echo ""
