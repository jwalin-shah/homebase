package records

import (
	"errors"
	"strings"
	"testing"

	"homebase/internal/journal"
)

func TestSpecificationDecisionCommitIsAtomicIdempotentAndRebuildable(t *testing.T) {
	path := t.TempDir() + "/records.journal"
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	specification := validSpecification(t)
	decision := validApprovalDecision(t)
	first, err := store.AppendSpecificationAndDecision(specification, decision)
	if err != nil {
		t.Fatalf("first specification/decision commit: %v", err)
	}
	if first.Existing || first.Sequence != 1 || first.Specification.ID != "spec:homebase:test:v1" || first.Decision.ID != "decision:spec-homebase-test" {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("record count = %d, want 2", got)
	}

	second, err := store.AppendSpecificationAndDecision(specification, decision)
	if err != nil {
		t.Fatalf("duplicate specification/decision commit: %v", err)
	}
	if !second.Existing || second.Sequence != first.Sequence {
		t.Fatalf("duplicate specification/decision commit was not idempotent: %+v", second)
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("duplicate commit changed record count to %d", got)
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
	if got := len(reopened.List()); got != 2 {
		t.Fatalf("replayed record count = %d, want 2", got)
	}
	if _, err := reopened.Get("spec:homebase:test:v1"); err != nil {
		t.Fatalf("replayed specification missing: %v", err)
	}
	if _, err := reopened.Get("decision:spec-homebase-test"); err != nil {
		t.Fatalf("replayed decision missing: %v", err)
	}
}

func TestSpecificationDecisionCommitRejectsConflictingReplayAtomically(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendSpecificationAndDecision(validSpecification(t), validApprovalDecision(t)); err != nil {
		t.Fatal(err)
	}

	conflicting := decodeObject(t, validApprovalDecision(t))
	payload := conflicting["payload"].(map[string]any)
	payload["decision"] = "approve specification spec:homebase:test:v1 (revised)"
	conflicting["content_hash"] = payloadHash(t, payload)
	if _, err := store.AppendSpecificationAndDecision(validSpecification(t), mustJSON(t, conflicting)); !errors.Is(err, ErrConflict) {
		t.Fatalf("conflicting decision error = %v, want ErrConflict", err)
	}
	if got := len(store.List()); got != 2 {
		t.Fatalf("rejected conflicting commit changed record count to %d, want unchanged 2", got)
	}
}

func TestSpecificationDecisionCommitRejectsMissingSpecificationReference(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	decision := decodeObject(t, validApprovalDecision(t))
	delete(decision["payload"].(map[string]any), "specification_ref")
	decision["content_hash"] = payloadHash(t, decision["payload"])
	if _, err := store.AppendSpecificationAndDecision(validSpecification(t), mustJSON(t, decision)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("missing specification_ref error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("rejected commit left %d records, want 0", got)
	}
}

func TestSpecificationDecisionCommitRejectsMismatchedDigest(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	decision := decodeObject(t, validApprovalDecision(t))
	decision["payload"].(map[string]any)["specification_ref"].(map[string]any)["content_hash"] = strings.Repeat("f", 64)
	decision["content_hash"] = payloadHash(t, decision["payload"])
	if _, err := store.AppendSpecificationAndDecision(validSpecification(t), mustJSON(t, decision)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("mismatched digest error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("rejected commit left %d records, want 0", got)
	}
}

func TestSpecificationDecisionCommitRejectsProposedSpecification(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	specification := decodeObject(t, validSpecification(t))
	specification["status"] = "proposed"
	specification["authority_class"] = AuthorityAgentProposal
	specification["source"] = map[string]any{"id": "knowledge-engine", "role": "knowledge_engine"}
	delete(specification["payload"].(map[string]any), "approval_ref")
	specification["content_hash"] = payloadHash(t, specification["payload"])
	if _, err := store.AppendSpecificationAndDecision(mustJSON(t, specification), validApprovalDecision(t)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("proposed specification error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("rejected commit left %d records, want 0", got)
	}
}

func TestSpecificationDecisionCommitRejectsWrongAuthoritySource(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	decision := decodeObject(t, validApprovalDecision(t))
	decision["source"] = map[string]any{"id": "portfolio", "role": "portfolio"}
	decision["payload"].(map[string]any)["decided_by"] = "portfolio"
	decision["content_hash"] = payloadHash(t, decision["payload"])
	if _, err := store.AppendSpecificationAndDecision(validSpecification(t), mustJSON(t, decision)); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("non-captain decision error = %v, want ErrInvalidRecord", err)
	}
	if got := len(store.List()); got != 0 {
		t.Fatalf("rejected commit left %d records, want 0", got)
	}
}
