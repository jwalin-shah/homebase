#!/usr/bin/env python3
"""
HomeBase Fixture Validator

Checks conformance of domain fixtures against the schema and internal consistency.
"""

import json
import sys
from pathlib import Path
from typing import List, Dict, Any, Set


# Closed sets of valid reason codes (from SEMANTICS.md)
REJECTION_REASONS = {
    "STALE_VERSION",
    "UNAUTHORIZED",
    "INVALID_STATUS",
    "ATTEMPT_LIMIT_REACHED",
    "ATTEMPT_NOT_FOUND",
    "ATTEMPT_NOT_ACTIVE",
    "EFFECT_NOT_FOUND",
    "EFFECT_KIND_NOT_ALLOWED",
    "CONFLICTING_EFFECT_ID",
    "OBSERVATION_NOT_FOUND",
    "CONFLICTING_OBSERVATION_ID",
    "OBSERVATION_NOT_SUCCESSFUL",
    "EVIDENCE_NOT_FOUND",
    "CONFLICTING_EVIDENCE_ID",
    "OBLIGATION_NOT_REQUIRED",
    "UNMET_OBLIGATIONS",
    "TERMINAL_STATE",
    "OUTCOME_UNKNOWN",
    "COMMAND_ID_CONFLICT",
}

NOOP_REASONS = {
    "COMMAND_ALREADY_APPLIED",
    "IDENTICAL_CONTRACT",
    "IDENTICAL_ATTEMPT",
    "IDENTICAL_EFFECT_INTENT",
    "IDENTICAL_OBSERVATION",
    "IDENTICAL_EVIDENCE",
    "OBLIGATION_ALREADY_SATISFIED",
    "ALREADY_COMPLETED",
    "ALREADY_ESCALATED",
}

AUTHORITY_ROLES = {
    "TaskInitiator",
    "Orchestrator",
    "BridgeAdapter",
    "Verifier",
    "RecoveryController",
}

OBSERVATION_OUTCOMES = {
    "NotStarted",
    "Running",
    "Succeeded",
    "Failed",
    "Unknown",
}

EVENT_TYPES = {
    "ContractLocked",
    "AttemptCreated",
    "EffectIntentCommitted",
    "EffectObserved",
    "EvidenceAccepted",
    "ObligationSatisfied",
    "TaskCompleted",
    "EscalationRequested",
}

SCENARIOS = {
    "happy_path",
    "recovery",
    "idempotency",
    "conflict",
    "completion",
    "escalation",
}

INVARIANTS = {
    "VersionMonotonicity",
    "ExpectedVersionSafety",
    "IntentBeforeObservation",
    "AttemptOwnership",
    "StableEffectIdentity",
    "EvidenceProvenance",
    "ObligationProvenance",
    "CompletionSoundness",
    "AttemptBoundSafety",
    "TerminalTrapping",
    "SingleTerminalOutcome",
    "DeterministicDecision",
}


class FixtureValidator:
    def __init__(self, fixture_path: Path):
        self.fixture_path = fixture_path
        self.errors: List[str] = []
        self.warnings: List[str] = []
        self.data: Dict[str, Any] = {}

    def load(self) -> bool:
        """Load fixture JSON."""
        try:
            with open(self.fixture_path) as f:
                self.data = json.load(f)
            return True
        except Exception as e:
            self.errors.append(f"Failed to load JSON: {e}")
            return False

    def validate(self) -> bool:
        """Run all validation checks."""
        if not self.load():
            return False

        self.check_required_fields()
        self.check_fixture_naming()
        self.check_command_decision_length()
        self.check_commands()
        self.check_decisions()
        self.check_version_progression()
        self.check_invariants()

        return len(self.errors) == 0

    def check_required_fields(self):
        """Verify all required top-level fields exist."""
        required = [
            "fixture_id", "fixture_version", "title", "description",
            "scenario", "initial_state", "commands", "expected_decisions"
        ]
        for field in required:
            if field not in self.data:
                self.errors.append(f"Missing required field: {field}")

    def check_fixture_naming(self):
        """Check fixture_id follows naming convention."""
        fixture_id = self.data.get("fixture_id", "")
        expected_name = self.fixture_path.stem
        if fixture_id != expected_name:
            self.errors.append(
                f"fixture_id '{fixture_id}' does not match filename '{expected_name}'"
            )

        # Check naming convention
        if not fixture_id.replace("_", "").isalnum():
            self.warnings.append(f"fixture_id contains unusual characters: {fixture_id}")

    def check_command_decision_length(self):
        """Verify commands and expected_decisions have same length."""
        commands = self.data.get("commands", [])
        decisions = self.data.get("expected_decisions", [])
        if len(commands) != len(decisions):
            self.errors.append(
                f"commands ({len(commands)}) and expected_decisions ({len(decisions)}) "
                f"have different lengths"
            )

    def check_commands(self):
        """Validate each command."""
        commands = self.data.get("commands", [])
        seen_ids = set()

        for i, cmd in enumerate(commands):
            cmd_id = cmd.get("command_id")
            task_id = cmd.get("task_id")
            expected_version = cmd.get("expected_version")
            authority = cmd.get("authority", {})

            # Check required command fields
            if not cmd_id:
                self.errors.append(f"Command {i}: missing command_id")
            if not task_id:
                self.errors.append(f"Command {i}: missing task_id")
            if expected_version is None:
                self.errors.append(f"Command {i}: missing expected_version")
            if not authority:
                self.errors.append(f"Command {i}: missing authority")

            # Check authority
            principal_id = authority.get("principal_id")
            role = authority.get("role")
            if not principal_id:
                self.errors.append(f"Command {i}: authority.principal_id missing")
            if role not in AUTHORITY_ROLES:
                self.errors.append(
                    f"Command {i}: invalid role '{role}'. "
                    f"Must be one of: {AUTHORITY_ROLES}"
                )

            # Check version is non-negative integer
            if isinstance(expected_version, int) and expected_version < 0:
                self.errors.append(f"Command {i}: expected_version must be >= 0")

            # Check command body exists
            if not cmd.get("body"):
                self.errors.append(f"Command {i}: missing body")

    def check_decisions(self):
        """Validate each decision."""
        decisions = self.data.get("expected_decisions", [])

        for i, decision in enumerate(decisions):
            decision_type = decision.get("decision_type")

            if decision_type == "Accepted":
                self.check_accepted_decision(i, decision)
            elif decision_type == "NoOp":
                self.check_noop_decision(i, decision)
            elif decision_type == "Rejected":
                self.check_rejected_decision(i, decision)
            else:
                self.errors.append(
                    f"Decision {i}: invalid decision_type '{decision_type}'. "
                    f"Must be Accepted, NoOp, or Rejected"
                )

    def check_accepted_decision(self, idx: int, decision: Dict):
        """Validate Accepted decision."""
        events = decision.get("events", [])
        if not events:
            self.errors.append(f"Decision {idx}: Accepted decision must have events")

        for i, event in enumerate(events):
            event_type = event.get("domain_event", {}).get("event_type")
            if event_type not in EVENT_TYPES:
                self.errors.append(
                    f"Decision {idx}, Event {i}: invalid event_type '{event_type}'. "
                    f"Must be one of: {EVENT_TYPES}"
                )

            aggregate_version = event.get("aggregate_version")
            if aggregate_version is None:
                self.errors.append(
                    f"Decision {idx}, Event {i}: missing aggregate_version"
                )

            origin = event.get("origin")
            if not origin:
                self.errors.append(f"Decision {idx}, Event {i}: missing origin")
            else:
                if not origin.get("command_id"):
                    self.errors.append(
                        f"Decision {idx}, Event {i}: origin.command_id missing"
                    )
                if not origin.get("command_fingerprint"):
                    self.errors.append(
                        f"Decision {idx}, Event {i}: origin.command_fingerprint missing"
                    )

    def check_noop_decision(self, idx: int, decision: Dict):
        """Validate NoOp decision."""
        reason_code = decision.get("reason_code")
        if reason_code not in NOOP_REASONS:
            self.errors.append(
                f"Decision {idx}: invalid NoOp reason_code '{reason_code}'. "
                f"Must be one of: {NOOP_REASONS}"
            )

    def check_rejected_decision(self, idx: int, decision: Dict):
        """Validate Rejected decision."""
        reason_code = decision.get("reason_code")
        if reason_code not in REJECTION_REASONS:
            self.errors.append(
                f"Decision {idx}: invalid Rejection reason_code '{reason_code}'. "
                f"Must be one of: {REJECTION_REASONS}"
            )

    def check_version_progression(self):
        """Verify version progression is monotonic."""
        commands = self.data.get("commands", [])
        decisions = self.data.get("expected_decisions", [])

        current_version = self.data.get("initial_state", {}).get("version", 0)

        for i, (cmd, decision) in enumerate(zip(commands, decisions)):
            expected_version = cmd.get("expected_version")

            # Command should match current version
            if expected_version != current_version:
                self.warnings.append(
                    f"Command {i}: expected_version is {expected_version} "
                    f"but previous state version was {current_version}"
                )

            # If Accepted, increment version by number of events
            if decision.get("decision_type") == "Accepted":
                events = decision.get("events", [])
                current_version += len(events)

    def check_invariants(self):
        """Verify invariants_verified references valid invariants."""
        invariants = self.data.get("invariants_verified", [])
        for inv in invariants:
            if inv not in INVARIANTS:
                self.warnings.append(
                    f"invariants_verified contains unknown invariant: {inv}"
                )

    def report(self):
        """Print validation results."""
        print(f"\n{'=' * 60}")
        print(f"Fixture: {self.fixture_path.name}")
        print(f"{'=' * 60}")

        if self.errors:
            print(f"\n❌ ERRORS ({len(self.errors)}):")
            for error in self.errors:
                print(f"   • {error}")

        if self.warnings:
            print(f"\n⚠️  WARNINGS ({len(self.warnings)}):")
            for warning in self.warnings:
                print(f"   • {warning}")

        if not self.errors and not self.warnings:
            print("\n✅ All checks passed")

        return len(self.errors) == 0


def main():
    fixture_dir = Path(__file__).parent.parent / "testdata" / "domain" / "fixtures"

    if not fixture_dir.exists():
        print(f"Fixture directory not found: {fixture_dir}")
        sys.exit(1)

    fixture_files = sorted(fixture_dir.glob("*.json"))
    if not fixture_files:
        print(f"No fixtures found in {fixture_dir}")
        sys.exit(1)

    all_passed = True
    for fixture_file in fixture_files:
        validator = FixtureValidator(fixture_file)
        if not validator.validate():
            all_passed = False
        validator.report()

    print(f"\n{'=' * 60}")
    if all_passed:
        print("✅ All fixtures validated successfully")
        sys.exit(0)
    else:
        print("❌ Some fixtures failed validation")
        sys.exit(1)


if __name__ == "__main__":
    main()
