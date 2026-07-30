package application

import (
	"context"
	"encoding/json"
	"homebase/internal/domain"
	"homebase/internal/journal"
	"path/filepath"
	"testing"
)

func TestJournalRepo_MixedEnvelopeAndLegacyReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mixed.journal")
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	repo, err := NewJournalAttemptRepository(j)
	if err != nil {
		t.Fatal(err)
	}
	aid, _ := domain.ParseAttemptID("mixed-records")
	if _, err := repo.Append(context.Background(), aid, 0, []domain.Event{}); err != nil {
		t.Fatalf("append enveloped event batch: %v", err)
	}

	legacy, err := json.Marshal(EventBatch{AttemptID: aid.String(), Version: 1, Events: []TypedEvent{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(legacy); err != nil {
		t.Fatalf("append legacy event batch: %v", err)
	}
	decision, err := journal.EncodeRecord(journal.RecordKindDecisionRecord, []byte(`{"id":"decision-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append(decision); err != nil {
		t.Fatalf("append decision record: %v", err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	repo2, err := NewJournalAttemptRepository(j2)
	if err != nil {
		t.Fatalf("mixed-format reopen rejected valid records: %v", err)
	}
	_, version, err := repo2.Load(context.Background(), aid)
	if err != nil {
		t.Fatal(err)
	}
	if version != 2 {
		t.Fatalf("replayed version = %d, want 2 (enveloped + legacy EventBatch)", version)
	}
}

func setupRepo(t *testing.T, path string) (*JournalAttemptRepository, func()) {
	j, err := journal.OpenBinaryJournal(path)
	if err != nil {
		t.Fatalf("failed to open journal: %v", err)
	}
	repo, err := NewJournalAttemptRepository(j)
	if err != nil {
		t.Fatalf("failed to create repo: %v", err)
	}
	return repo, func() {
		j.Close()
	}
}

// 1. Command round trip & 4. Multi-event atomicity
func TestJournalRepo_RoundTripAndAtomicity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repo.journal")

	repo, cleanup := setupRepo(t, path)

	aid, _ := domain.ParseAttemptID("test-roundtrip")
	ctx := context.Background()

	state, version, err := repo.Load(ctx, aid)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if version != 0 {
		t.Fatalf("expected version 0, got %d", version)
	}

	cmd := domain.CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req1", Version: version}
	decision := domain.Decide(state, cmd)

	newVer, err := repo.Append(ctx, aid, version, decision.Events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if newVer != 1 {
		t.Fatalf("expected version 1, got %d", newVer)
	}

	cleanup()

	// Restart
	repo2, cleanup2 := setupRepo(t, path)
	defer cleanup2()

	state2, version2, err := repo2.Load(ctx, aid)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if version2 != 1 {
		t.Fatalf("expected version 1 after restart, got %d", version2)
	}
	if state2.RecoveryDispatches != 1 {
		t.Fatalf("expected 1 recovery dispatch, got %d", state2.RecoveryDispatches)
	}
}

func TestJournalRepo_ReopensConcludedAttempt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "concluded.journal")
	repo, cleanup := setupRepo(t, path)
	aid, _ := domain.ParseAttemptID("test-concluded")
	ctx := context.Background()

	if _, err := repo.Append(ctx, aid, 0, []domain.Event{
		domain.EventRecoveryDispatched{AttemptID: aid, EffectID: "effect-1", Ordinal: 1, IdempotencyKey: "req-1"},
	}); err != nil {
		t.Fatalf("append recovery event: %v", err)
	}
	state, version, err := repo.Load(ctx, aid)
	if err != nil {
		t.Fatalf("load before conclude: %v", err)
	}
	decision := domain.Decide(state, domain.CommandConclude{AttemptID: aid})
	if decision.Status != domain.DecisionAccepted || len(decision.Events) != 1 {
		t.Fatalf("conclude decision = %+v, want one accepted event", decision)
	}
	if _, err := repo.Append(ctx, aid, version, decision.Events); err != nil {
		t.Fatalf("append conclude event: %v", err)
	}
	cleanup()

	repo2, cleanup2 := setupRepo(t, path)
	defer cleanup2()
	reopened, reopenedVersion, err := repo2.Load(ctx, aid)
	if err != nil {
		t.Fatalf("reopen concluded attempt: %v", err)
	}
	if reopened.Phase != domain.AttemptConcluded || reopenedVersion != 2 {
		t.Fatalf("reopened state = %+v version=%d, want concluded version 2", reopened, reopenedVersion)
	}
}

// 2. Duplicate command after restart
func TestJournalRepo_DuplicateCommandAfterRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repo-dup.journal")
	repo, cleanup := setupRepo(t, path)

	aid, _ := domain.ParseAttemptID("test-dup")
	ctx := context.Background()
	state, ver, _ := repo.Load(ctx, aid)
	cmd := domain.CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req1", Version: ver}
	decision := domain.Decide(state, cmd)
	_, _ = repo.Append(ctx, aid, ver, decision.Events)
	cleanup()

	// Restart
	repo2, cleanup2 := setupRepo(t, path)
	defer cleanup2()

	state2, ver2, _ := repo2.Load(ctx, aid)
	cmd2 := domain.CommandProposeRecovery{AttemptID: aid, IdempotencyKey: "req1", Version: ver2}
	decision2 := domain.Decide(state2, cmd2)

	// Decision should be NoOp because IdempotencyKey "req1" is already processed.
	if decision2.Status != domain.DecisionNoOp {
		t.Fatalf("expected NoOp for duplicate command, got %v", decision2.Status)
	}
}

// 3. Stale concurrent write
func TestJournalRepo_StaleConcurrentWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repo-concurrent.journal")
	repo, cleanup := setupRepo(t, path)
	defer cleanup()

	aid, _ := domain.ParseAttemptID("test-concurrent")
	ctx := context.Background()

	// Two requests load version 0
	_, verA, _ := repo.Load(ctx, aid)
	_, verB, _ := repo.Load(ctx, aid)

	// A appends successfully
	_, errA := repo.Append(ctx, aid, verA, []domain.Event{})
	if errA != nil {
		t.Fatalf("A append failed: %v", errA)
	}

	// B attempts to append with stale version
	_, errB := repo.Append(ctx, aid, verB, []domain.Event{})
	if errB != ErrVersionConflict {
		t.Fatalf("expected ErrVersionConflict for B, got %v", errB)
	}
}

// 8. Unknown event version / unsupported event type
func TestJournalRepo_UnsupportedEvent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "repo-unsupported.journal")
	repo, cleanup := setupRepo(t, path)
	defer cleanup()

	aid, _ := domain.ParseAttemptID("test-unsupported")
	ctx := context.Background()

	_, err := repo.Append(ctx, aid, 0, []domain.Event{nil})
	if err == nil || err.Error() != "unsupported event type" {
		t.Fatalf("expected unsupported event type error, got %v", err)
	}
}
