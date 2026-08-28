package promotion

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"homebase/internal/journal"
	"homebase/internal/records"
)

const defaultContractFixtureRoot = "/Users/jwalinshah/projects/running-machine-contracts"

func contractFixtureRoot() string {
	if root := os.Getenv("RUNNING_MACHINE_CONTRACTS_ROOT"); root != "" {
		return root
	}
	candidates := []string{
		defaultContractFixtureRoot,
		filepath.Join(os.Getenv("HOME"), "Projects", "running-machine-contracts"),
		filepath.Join(os.Getenv("HOME"), "projects", "running-machine-contracts"),
		filepath.Join("..", "running-machine-contracts"),
	}
	for _, root := range candidates {
		if root == "" {
			continue
		}
		if st, err := os.Stat(filepath.Join(root, "fixtures", "transcript-promotion")); err == nil && st.IsDir() {
			return root
		}
	}
	return defaultContractFixtureRoot
}

func requirePromotionFixtures(t *testing.T) string {
	t.Helper()
	root := contractFixtureRoot()
	dir := filepath.Join(root, "fixtures", "transcript-promotion")
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Skipf("running-machine-contracts fixtures unavailable at %s (set RUNNING_MACHINE_CONTRACTS_ROOT)", dir)
	}
	return root
}

func TestPromoteFixtureDurableAndRestart(t *testing.T) {
	raw := loadPromotionFixture(t, "valid-explicit-approval.json")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) }

	journalPath := filepath.Join(t.TempDir(), "records.journal")
	j, err := journal.OpenBinaryJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	store, authorities, err := records.NewStoreWithAuthorities(j)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewServiceWithAuthority(store, authorities.Promotion, fixtureVerifier(raw, public), private, now)
	if err != nil {
		t.Fatal(err)
	}
	signature := fixtureSignature(t, raw, private)
	first, err := service.Promote(context.Background(), raw, signature)
	if err != nil {
		t.Fatalf("first promotion: %v (%+v)", err, first)
	}
	if !first.Accepted || first.Existing || first.Receipt == nil {
		t.Fatalf("unexpected first outcome: %+v", first)
	}
	if len(store.List()) != 2 || len(store.ListPromotionCommits()) != 1 {
		t.Fatalf("promotion did not atomically expose two records and one bundle: records=%d bundles=%d", len(store.List()), len(store.ListPromotionCommits()))
	}

	second, err := service.Promote(context.Background(), raw, signature)
	if err != nil || !second.Accepted || !second.Existing || second.Receipt == nil {
		t.Fatalf("duplicate promotion was not idempotent: outcome=%+v err=%v", second, err)
	}
	if len(store.ListPromotionCommits()) != 1 {
		t.Fatalf("duplicate promotion appended another bundle")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := journal.OpenBinaryJournal(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	reopenedStore, reopenedAuthorities, err := records.NewStoreWithAuthorities(j2)
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := NewServiceWithAuthority(reopenedStore, reopenedAuthorities.Promotion, fixtureVerifier(raw, public), private, now)
	if err != nil {
		t.Fatalf("rebuild promotion service: %v", err)
	}
	afterRestart, err := reopened.Promote(context.Background(), raw, signature)
	if err != nil || !afterRestart.Accepted || !afterRestart.Existing {
		t.Fatalf("restart replay did not preserve idempotency: outcome=%+v err=%v", afterRestart, err)
	}
}

func TestContractFixturesMatchGoBoundaryAcceptance(t *testing.T) {
	root := requirePromotionFixtures(t)
	paths, err := filepath.Glob(filepath.Join(root, "fixtures", "transcript-promotion", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 {
		t.Fatal("no transcript promotion fixtures found")
	}
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var fixture struct {
				Expected struct {
					Valid bool `json:"valid"`
				} `json:"expected"`
			}
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatal(err)
			}
			_, found := validateCase(raw, now)
			if (len(found) == 0) != fixture.Expected.Valid {
				t.Fatalf("Go acceptance=%v expected=%v errors=%v", len(found) == 0, fixture.Expected.Valid, found)
			}
		})
	}
}

func TestPromoteRejectsBadAuthenticationWithoutAppend(t *testing.T) {
	raw := loadPromotionFixture(t, "valid-explicit-approval.json")
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	j, err := journal.OpenBinaryJournal(filepath.Join(t.TempDir(), "records.journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := records.NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, fixtureVerifier(raw, public), private, func() time.Time {
		return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.Promote(context.Background(), raw, []byte("wrong-signature"))
	if !errors.Is(err, ErrUnauthenticated) || outcome.Accepted {
		t.Fatalf("bad authentication was accepted: outcome=%+v err=%v", outcome, err)
	}
	if len(store.List()) != 0 || len(store.ListPromotionCommits()) != 0 {
		t.Fatalf("bad authentication changed durable state")
	}
}

func TestPromoteRejectsTamperedTranscriptBeforeAuthentication(t *testing.T) {
	raw := loadPromotionFixture(t, "valid-explicit-approval.json")
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	transcript := value["transcript"].(map[string]any)
	transcript["content_hash"] = "0000000000000000000000000000000000000000000000000000000000000000"
	tampered, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	j, err := journal.OpenBinaryJournal(filepath.Join(t.TempDir(), "records.journal"))
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	store, err := records.NewStore(j)
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewService(store, func(context.Context, string, string, []byte) error {
		t.Fatal("authentication must not run for an invalid case")
		return nil
	}, private, func() time.Time { return time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := service.Promote(context.Background(), tampered, nil)
	if !errors.Is(err, ErrInvalidPromotion) || outcome.Accepted {
		t.Fatalf("tampered transcript was accepted: outcome=%+v err=%v", outcome, err)
	}
	if len(store.List()) != 0 {
		t.Fatalf("tampered transcript changed durable state")
	}
}

func loadPromotionFixture(t *testing.T, name string) []byte {
	t.Helper()
	root := requirePromotionFixtures(t)
	raw, err := os.ReadFile(filepath.Join(root, "fixtures", "transcript-promotion", name))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func fixtureSignature(t *testing.T, raw []byte, private ed25519.PrivateKey) []byte {
	t.Helper()
	var value Case
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatal(err)
	}
	digest, err := digestValue(transcriptDigestValue(value.Transcript))
	if err != nil {
		t.Fatal(err)
	}
	return ed25519.Sign(private, []byte(digest))
}

func fixtureVerifier(raw []byte, public ed25519.PublicKey) VerifyFunc {
	return func(_ context.Context, _ string, digest string, signature []byte) error {
		if !ed25519.Verify(public, []byte(digest), signature) {
			return errors.New("fixture signature mismatch")
		}
		return nil
	}
}
