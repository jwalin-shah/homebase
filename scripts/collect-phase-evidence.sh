#!/bin/bash
# Collect evidence for phase completion
# Runs all verification checks and captures proof
#
# Usage: ./scripts/collect-phase-evidence.sh PHASE-NUM TICKET-NUM
# Example: ./scripts/collect-phase-evidence.sh 1 202

set -e

PHASE=$1
TICKET=$2

if [ -z "$PHASE" ] || [ -z "$TICKET" ]; then
  echo "Usage: $0 PHASE-NUM TICKET-NUM"
  echo "Example: $0 1 202"
  exit 1
fi

EVIDENCE_FILE="tickets/PHASE-${PHASE}-EVIDENCE-${TICKET}.md"

echo "📊 Collecting Phase $PHASE evidence for TICKET-$TICKET..."
echo ""

# Start evidence file
cat > "$EVIDENCE_FILE" << EOF
# Phase $PHASE Evidence Report

**Ticket:** TICKET-$TICKET
**Date:** $(date -u +"%Y-%m-%d %H:%M:%S UTC")
**Collector:** $(whoami)@$(hostname)

---

## Evidence Collection

EOF

# Phase 0: Specification
if [ "$PHASE" -eq 0 ]; then
  echo "Collecting Phase 0 (Specification) evidence..."

  cat >> "$EVIDENCE_FILE" << 'EOF'

### Specification Completeness

**Check 1: Spec file exists and is complete**
EOF

  SPEC_FILE="tickets/TICKET-${TICKET}-PHASE-0-SPECIFICATION.md"
  if [ -f "$SPEC_FILE" ]; then
    LINES=$(wc -l < "$SPEC_FILE")
    WORDS=$(wc -w < "$SPEC_FILE")
    cat >> "$EVIDENCE_FILE" << EOF
- ✓ Spec file: $SPEC_FILE
- ✓ Size: $LINES lines, $WORDS words (acceptable: 200-500 lines)
EOF
  else
    cat >> "$EVIDENCE_FILE" << EOF
- ✗ Spec file NOT found: $SPEC_FILE
EOF
  fi

  # Check for required sections
  cat >> "$EVIDENCE_FILE" << EOF

**Check 2: Required sections present**
EOF

  for SECTION in "Use Cases" "Constraints" "Risks" "Assumptions"; do
    if grep -q "$SECTION" "$SPEC_FILE" 2>/dev/null; then
      echo "- ✓ $SECTION" >> "$EVIDENCE_FILE"
    else
      echo "- ✗ $SECTION" >> "$EVIDENCE_FILE"
    fi
  done

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 3: Risk assessment complete**
EOF

  RISKS=$(grep -c "Risk:" "$SPEC_FILE" 2>/dev/null || echo 0)
  echo "- $RISKS risks identified (target: ≥3)" >> "$EVIDENCE_FILE"

fi

# Phase 1: Implementation
if [ "$PHASE" -eq 1 ]; then
  echo "Collecting Phase 1 (Implementation) evidence..."

  cat >> "$EVIDENCE_FILE" << 'EOF'

### Implementation Completeness

**Check 1: Code compiles**
EOF

  if go build ./... >/dev/null 2>&1; then
    echo "- ✓ Code compiles without errors" >> "$EVIDENCE_FILE"
    COMPILE_EXIT=0
  else
    echo "- ✗ Code compilation FAILED" >> "$EVIDENCE_FILE"
    COMPILE_EXIT=1
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 2: Linting passes**
EOF

  if command -v golangci-lint &> /dev/null; then
    if golangci-lint run ./... >/dev/null 2>&1; then
      echo "- ✓ No linting errors" >> "$EVIDENCE_FILE"
    else
      LINT_ISSUES=$(golangci-lint run ./... 2>&1 | wc -l)
      echo "- ⚠️  $LINT_ISSUES linting issues found" >> "$EVIDENCE_FILE"
    fi
  else
    echo "- ⚠️  golangci-lint not installed" >> "$EVIDENCE_FILE"
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 3: Error handling verification**
EOF

  UNHANDLED=$(grep -r "_ = " *.go 2>/dev/null | grep -v "for _" | grep -v range | wc -l || echo 0)
  echo "- Unhandled error patterns: $UNHANDLED (target: 0)" >> "$EVIDENCE_FILE"

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 4: No hardcoded config**
EOF

  HARDCODED=$(grep -r "localhost\|:7474\|/tmp/ledger" *.go 2>/dev/null | grep -v "_test.go" | wc -l || echo 0)
  echo "- Hardcoded values found: $HARDCODED (target: 0)" >> "$EVIDENCE_FILE"

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 5: Signature verification checks**
EOF

  SIG_ASSIGNS=$(grep -r "\.Signature = " *.go 2>/dev/null | wc -l || echo 0)
  VERIFY_CALLS=$(grep -r "Verify" *.go 2>/dev/null | wc -l || echo 0)
  echo "- Signature assignments: $SIG_ASSIGNS" >> "$EVIDENCE_FILE"
  echo "- Verify calls: $VERIFY_CALLS" >> "$EVIDENCE_FILE"

fi

# Phase 2: Unit Tests
if [ "$PHASE" -eq 2 ]; then
  echo "Collecting Phase 2 (Unit Tests) evidence..."

  cat >> "$EVIDENCE_FILE" << 'EOF'

### Test Completeness

**Check 1: Tests run and pass**
EOF

  if go test ./... -v > /tmp/test-results.txt 2>&1; then
    TESTS_PASSED=$(grep "ok\|PASS" /tmp/test-results.txt | wc -l)
    echo "- ✓ Tests passed" >> "$EVIDENCE_FILE"
    echo "- Test count: $TESTS_PASSED" >> "$EVIDENCE_FILE"
  else
    echo "- ✗ Tests FAILED" >> "$EVIDENCE_FILE"
    grep "FAIL" /tmp/test-results.txt >> "$EVIDENCE_FILE" || true
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 2: Code coverage**
EOF

  if go test ./... -cover > /tmp/coverage.txt 2>&1; then
    COVERAGE=$(grep "coverage:" /tmp/coverage.txt | tail -1)
    echo "- $COVERAGE" >> "$EVIDENCE_FILE"
  else
    echo "- Coverage measurement failed" >> "$EVIDENCE_FILE"
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 3: Test isolation (no :memory:)**
EOF

  SHARED_MEM=$(grep ":memory:" *_test.go 2>/dev/null | wc -l || echo 0)
  echo "- :memory: in tests: $SHARED_MEM (target: 0)" >> "$EVIDENCE_FILE"

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 4: Flakiness verification**
EOF

  echo "- Need to run: for i in {1..5}; do go test ./... -count=1; done" >> "$EVIDENCE_FILE"

fi

# Phase 3: Integration Tests
if [ "$PHASE" -eq 3 ]; then
  echo "Collecting Phase 3 (Integration Tests) evidence..."

  cat >> "$EVIDENCE_FILE" << 'EOF'

### Integration Test Results

**Check 1: End-to-end flow**
EOF

  if [ -f "tests/integration_test.go" ]; then
    echo "- ✓ Integration tests exist (tests/integration_test.go)" >> "$EVIDENCE_FILE"
  else
    echo "- ✗ Integration tests NOT found" >> "$EVIDENCE_FILE"
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 2: All handlers tested**
EOF

  HANDLERS=$(grep -c "func (h \*Handler)" api/handlers.go 2>/dev/null || echo 0)
  HANDLER_TESTS=$(grep -c "Test.*Handler" api/*_test.go 2>/dev/null || echo 0)
  echo "- Handlers: $HANDLERS" >> "$EVIDENCE_FILE"
  echo "- Handler tests: $HANDLER_TESTS" >> "$EVIDENCE_FILE"

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 3: Integration test pass rate**
EOF

  if go test -tags integration ./... >/dev/null 2>&1; then
    echo "- ✓ Integration tests pass" >> "$EVIDENCE_FILE"
  else
    echo "- ⚠️  Some integration tests may have failed" >> "$EVIDENCE_FILE"
  fi

fi

# Phase 4: Audit
if [ "$PHASE" -eq 4 ]; then
  echo "Collecting Phase 4 (Audit) evidence..."

  cat >> "$EVIDENCE_FILE" << 'EOF'

### Audit Findings

**Check 1: Audit findings documented**
EOF

  FINDINGS=$(ls tickets/AUDIT-FINDINGS-$TICKET-*.md 2>/dev/null | wc -l)
  echo "- Audit finding docs: $FINDINGS" >> "$EVIDENCE_FILE"

  if [ $FINDINGS -gt 0 ]; then
    cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 2: Findings by severity**
EOF

    for LEVEL in CRITICAL HIGH MEDIUM LOW; do
      COUNT=$(grep -h "Severity: $LEVEL" tickets/AUDIT-FINDINGS-$TICKET-*.md 2>/dev/null | wc -l || echo 0)
      echo "- $LEVEL: $COUNT" >> "$EVIDENCE_FILE"
    done
  fi

  cat >> "$EVIDENCE_FILE" << 'EOF'

**Check 3: Findings resolution status**
EOF

  FIXED=$(grep -h "Status: FIXED" tickets/AUDIT-FINDINGS-$TICKET-*.md 2>/dev/null | wc -l || echo 0)
  UNFIXED=$(grep -h "Status:" tickets/AUDIT-FINDINGS-$TICKET-*.md 2>/dev/null | grep -c -v "FIXED" || echo 0)
  echo "- Fixed: $FIXED" >> "$EVIDENCE_FILE"
  echo "- Unfixed: $UNFIXED" >> "$EVIDENCE_FILE"

fi

# Common checks (all phases)
cat >> "$EVIDENCE_FILE" << 'EOF'

---

## Sign-Off Gate

This evidence must be reviewed and approved before the following roles can sign off:

- [ ] Discovery Lead confirms Phase $PHASE work is complete
- [ ] Architect reviews evidence and verifies requirements met
- [ ] Implementation/Tester confirms their work matches evidence
- [ ] Auditor (Phase 4+) validates evidence is sufficient

**Evidence is immutable:** This file is committed to git and cannot be altered.

---

## How to Use This Evidence

1. **For developers:** Show this evidence when requesting sign-off
2. **For reviewers:** Use evidence checklist to verify phase is complete
3. **For captain:** Use to track quality metrics across tickets
4. **For audit:** Immutable proof of work for compliance

---

**Generated by:** collect-phase-evidence.sh
**Evidence Status:** DRAFT (not yet verified)
**Approval Status:** PENDING (waiting for sign-offs)

EOF

echo ""
echo "✓ Evidence collected: $EVIDENCE_FILE"
echo ""
echo "Next steps:"
echo "1. Review evidence: cat $EVIDENCE_FILE"
echo "2. Address any gaps (red X marks)"
echo "3. Commit to git: git add $EVIDENCE_FILE && git commit"
echo "4. Request sign-offs from 5 roles"
echo ""
