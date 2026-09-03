package records

import (
	"errors"
	"homebase/internal/journal"
	"testing"
)

// TASK-021 A0 complete-mediation matrix. Generic Store.Append is the explicit
// low-authority/replay path; every authoritative record class must fail before
// appendValidated or any journal-backed state mutation. Cross-record lineage is
// intentionally not pre-seeded here: authority rejection must happen first.
func TestRawAppendRejectsEveryAuthoritativeRecordClassWithoutMutation(t *testing.T) {
	verifiedEvidence := func() []byte {
		value := decodeObject(t, validEvidence(t, "raw-matrix-evidence"))
		value["authority_class"] = AuthorityVerifiedEvidence
		return mustJSON(t, value)
	}

	cases := []struct {
		name string
		raw  func() []byte
	}{
		{name: "Specification/human_decision", raw: func() []byte { return validSpecification(t) }},
		{name: "Decision/human_decision", raw: func() []byte { return validApprovalDecision(t) }},
		{name: "Evidence/verified_evidence", raw: verifiedEvidence},
		{name: "Contract/human_decision", raw: func() []byte { return validContract(t, "raw-matrix-contract") }},
		{name: "CapabilityGrant/authoritative_fact", raw: func() []byte { return validGrant(t, "raw-matrix-grant", "raw-matrix-contract", "raw-matrix-idem") }},
		{name: "Proof/verified_evidence", raw: func() []byte { return validProof(t, "raw-matrix-proof") }},
		{name: "VerificationReceipt/verified_evidence", raw: func() []byte {
			const digest = "0000000000000000000000000000000000000000000000000000000000000000"
			return validVerificationReceipt(t, "raw-matrix-receipt", "verifier-1", "verifier-1", digest, digest)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
			if err != nil {
				t.Fatal(err)
			}
			defer j.Close()

			store, err := NewStore(j)
			if err != nil {
				t.Fatal(err)
			}
			before := len(store.List())

			if _, err := store.Append(tc.raw()); !errors.Is(err, ErrAuthorityRequired) {
				t.Fatalf("raw authoritative append error = %v, want ErrAuthorityRequired", err)
			}
			if after := len(store.List()); after != before {
				t.Fatalf("rejected authoritative append changed store: before=%d after=%d", before, after)
			}
		})
	}
}
