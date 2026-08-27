package records

import (
	"errors"
	"homebase/internal/journal"
	"testing"
)

// TASK-021: generic Store.Append is not an authenticated authority boundary.
// Schema-valid records carrying authoritative classes must be rejected before
// any journal mutation and routed through a typed, authority-bearing path.
func TestRawAppendRejectsVerifiedEvidenceWithoutAuthority(t *testing.T) {
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

	// validEvidence is intentionally a low-authority untrusted_text fixture.
	// Promote only its authority class here so this red gate tests the actual
	// complete-mediation boundary rather than accidentally banning low-authority
	// Evidence from the generic append path.
	value := decodeObject(t, validEvidence(t, "raw-authoritative-evidence"))
	value["authority_class"] = AuthorityVerifiedEvidence
	raw := mustJSON(t, value)

	if _, err := store.Append(raw); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("raw authoritative append error = %v, want ErrAuthorityRequired", err)
	}
	if after := len(store.List()); after != before {
		t.Fatalf("rejected authoritative append changed store: before=%d after=%d", before, after)
	}
}

// Low-authority records remain usable through the generic append path. This is
// the control for the complete-mediation gate above: TASK-021 narrows durable
// authority creation; it must not turn historical/replay/explicitly
// low-authority evidence into an authenticated-authority requirement.
func TestRawAppendPreservesExplicitLowAuthorityEvidence(t *testing.T) {
	j, err := journal.OpenBinaryJournal(t.TempDir() + "/records.journal")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()

	store, err := NewStore(j)
	if err != nil {
		t.Fatal(err)
	}

	result, err := store.Append(validEvidence(t, "raw-low-authority-evidence"))
	if err != nil {
		t.Fatalf("low-authority evidence append failed: %v", err)
	}
	if result.Existing {
		t.Fatal("first low-authority append unexpectedly reported Existing")
	}
	got, err := store.Get("raw-low-authority-evidence")
	if err != nil {
		t.Fatal(err)
	}
	if got.AuthorityClass != AuthorityUntrustedText {
		t.Fatalf("authority_class = %q, want %q", got.AuthorityClass, AuthorityUntrustedText)
	}
}
