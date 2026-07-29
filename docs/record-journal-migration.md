# Record journal migration boundary

The running-machine authority model now requires every Contract and CapabilityGrant to carry an approved Specification ID and digest. The atomic journal commit also persists that Specification with the pair.

Older journals do not contain enough information to reconstruct this authority chain safely. HomeBase therefore fails closed during replay with an explicit migration diagnostic; it does not infer a Specification, upgrade a Decision, or silently preserve an old grant.

Before deployment, an operator must:

1. preserve the original journal as evidence;
2. inspect each legacy Contract/CapabilityGrant pair;
3. obtain a new captain-approved Specification and Decision binding;
4. issue a new Contract and CapabilityGrant through the authenticated atomic endpoint; and
5. reopen the new journal and run the full HomeBase and cross-project certification checks.

The replay boundary is covered by `TestLegacyContractGrantCommitFailsClosedWithMigrationDiagnostic`. A passing test proves the old format is rejected intentionally; it is not evidence that a legacy journal is deployable.
