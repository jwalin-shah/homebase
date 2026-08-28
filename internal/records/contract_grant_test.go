package records

import (
	"errors"
	"strings"
	"testing"

	"homebase/internal/journal"
)

func TestContractGrantCommitIsAtomicIdempotentAndRebuildable(t *testing.T) {
	store, j, path := newStoreWithHistoricalRecords(t, validApprovalDecision(t))
	specification := validSpecification(t)
	contract := validContract(t, "contract-1")
	grant := validGrant(t, "grant-1", "contract-1", "idem-1")
	first, err := store.AppendContractAndGrant(specification, contract, grant)
	if err != nil {
		t.Fatalf("first contract/grant commit: %v", err)
	}
	if first.Existing || first.Sequence != 2 || first.Contract.ID != "contract-1" || first.Grant.ID != "grant-1" || first.Specification.ID != "spec:homebase:test:v1" {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second, err := store.AppendContractAndGrant(specification, contract, grant)
	if err != nil {
		t.Fatalf("duplicate contract/grant commit: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate contract/grant commit was not idempotent: %+v", second)
	}
	if got := len(store.List()); got != 4 {
		t.Fatalf("record count = %d, want 4", got)
	}
	if _, err := store.AppendContractAndGrant(specification, contract, validGrant(t, "grant-2", "contract-1", "idem-2")); err == nil {
		t.Fatal("conflicting grant replaced the committed pair")
	}
	if got := len(store.List()); got != 4 {
		t.Fatalf("conflicting pair changed record count to %d", got)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	reopened, err := NewStore(j2)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	if got := len(reopened.List()); got != 4 {
		t.Fatalf("replayed record count = %d, want 4", got)
	}
	if _, err := reopened.Get("spec:homebase:test:v1"); err != nil {
		t.Fatalf("replayed specification missing: %v", err)
	}
	if _, err := reopened.Get("contract-1"); err != nil {
		t.Fatalf("replayed contract missing: %v", err)
	}
	if _, err := reopened.Get("grant-1"); err != nil {
		t.Fatalf("replayed grant missing: %v", err)
	}
}

func TestLegacyContractGrantCommitFailsClosedWithMigrationDiagnostic(t *testing.T) {
	legacy := mustJSON(t, map[string]any{
		"contract": map[string]any{"kind": "Contract", "version": "1"},
		"grant":    map[string]any{"kind": "CapabilityGrant", "version": "1"},
	})
	if _, err := decodeContractGrantCommit(legacy); err == nil || !strings.Contains(err.Error(), "migrate the journal before startup") {
		t.Fatalf("legacy commit error = %v, want explicit migration diagnostic", err)
	}
}

func TestContractGrantCommitRejectsMissingContractLineageAtomically(t *testing.T) {
	store, j, _ := newStoreWithHistoricalRecords(t, validApprovalDecision(t))
	defer j.Close()
	grant := validGrant(t, "grant-1", "missing-contract", "idem-1")
	if _, err := store.AppendContractAndGrant(validSpecification(t), validContract(t, "contract-1"), grant); err == nil {
		t.Fatal("contract/grant commit accepted mismatched grant lineage")
	}
	if got := len(store.List()); got != 1 {
		t.Fatalf("failed contract/grant commit changed store beyond approval decision: %d", got)
	}
}

func TestContractGrantCommitRejectsUnapprovedSpecification(t *testing.T) {
	store, j, _ := newStoreWithHistoricalRecords(t, validApprovalDecision(t))
	defer j.Close()
	specification := decodeObject(t, validSpecification(t))
	specification["status"] = "proposed"
	specification["authority_class"] = AuthorityAgentProposal
	specification["source"] = map[string]any{"id": "knowledge-engine", "role": "knowledge_engine"}
	delete(specification["payload"].(map[string]any), "approval_ref")
	specification["content_hash"] = payloadHash(t, specification["payload"])
	specificationRaw := mustJSON(t, specification)
	contractRaw, grantRaw := bindSpecification(t, specificationRaw)
	if _, err := store.AppendContractAndGrant(specificationRaw, contractRaw, grantRaw); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("unapproved specification error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 1 {
		t.Fatalf("unapproved specification changed store beyond approval decision: %d", got)
	}
}

func TestContractGrantCommitRejectsMismatchedSpecificationDigest(t *testing.T) {
	store, j, _ := newStoreWithHistoricalRecords(t, validApprovalDecision(t))
	defer j.Close()
	specification := validSpecification(t)
	contractRaw, grantRaw := bindSpecification(t, specification)
	contract := decodeObject(t, contractRaw)
	contractPayload := contract["payload"].(map[string]any)
	contractPayload["specification_digest"] = strings.Repeat("f", 64)
	contract["content_hash"] = payloadHash(t, contractPayload)
	for _, rawRef := range contract["source_refs"].([]any) {
		ref := rawRef.(map[string]any)
		if ref["kind"] == "specification" {
			ref["content_hash"] = strings.Repeat("f", 64)
		}
	}
	grant := decodeObject(t, grantRaw)
	grantPayload := grant["payload"].(map[string]any)
	grantPayload["specification_digest"] = strings.Repeat("f", 64)
	grant["content_hash"] = payloadHash(t, grantPayload)
	if _, err := store.AppendContractAndGrant(specification, mustJSON(t, contract), mustJSON(t, grant)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("mismatched specification digest error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 1 {
		t.Fatalf("mismatched digest changed store beyond approval decision: %d", got)
	}
}

func TestApprovedSpecificationMustMatchDecisionIdentityAndDigest(t *testing.T) {
	for name, mutate := range map[string]func(map[string]any){
		"different_id": func(spec map[string]any) {
			spec["id"] = "spec:homebase:other:v1"
		},
		"different_content": func(spec map[string]any) {
			spec["payload"].(map[string]any)["purpose"] = "unreviewed replacement"
		},
	} {
		t.Run(name, func(t *testing.T) {
			store, j, _ := newStoreWithHistoricalRecords(t, validApprovalDecision(t))
			defer j.Close()
			specification := decodeObject(t, validSpecification(t))
			mutate(specification)
			specification["content_hash"] = payloadHash(t, specification["payload"])
			specificationRaw := mustJSON(t, specification)
			contractRaw, grantRaw := bindSpecification(t, specificationRaw)
			if _, err := store.AppendContractAndGrant(specificationRaw, contractRaw, grantRaw); !errors.Is(err, ErrInvalidRecord) {
				t.Fatalf("mismatched approved specification error = %v, want ErrInvalidRecord", err)
			}
			if got := len(store.List()); got != 1 {
				t.Fatalf("mismatched approved specification changed store beyond decision: %d", got)
			}
		})
	}
}

func bindSpecification(t *testing.T, specificationRaw []byte) ([]byte, []byte) {
	t.Helper()
	specification := decodeObject(t, specificationRaw)
	specificationID := specification["id"].(string)
	specificationDigest := specification["content_hash"].(string)
	contract := decodeObject(t, validContract(t, "contract-bound"))
	contractPayload := contract["payload"].(map[string]any)
	contractPayload["specification_id"] = specificationID
	contractPayload["specification_digest"] = specificationDigest
	for _, rawRef := range contract["source_refs"].([]any) {
		ref := rawRef.(map[string]any)
		if ref["kind"] == "specification" {
			ref["id"] = specificationID
			ref["content_hash"] = specificationDigest
		}
	}
	contract["content_hash"] = payloadHash(t, contractPayload)
	grant := decodeObject(t, validGrant(t, "grant-bound", "contract-bound", "idem-bound"))
	grantPayload := grant["payload"].(map[string]any)
	grantPayload["specification_id"] = specificationID
	grantPayload["specification_digest"] = specificationDigest
	grant["content_hash"] = payloadHash(t, grantPayload)
	return mustJSON(t, contract), mustJSON(t, grant)
}
