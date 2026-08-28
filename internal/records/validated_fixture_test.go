package records

import "testing"

// appendValidatedForCrossRecordTest exercises record parsing, reference checks,
// idempotency checks, and journal mutation below the authenticated-authority
// boundary. It exists only for tests whose target is lower-layer record
// semantics after authoritative prerequisites have been replayed as history.
//
// Never use this helper in authority-boundary tests: Store.Append and the typed
// authenticated submission APIs remain the only valid seams for proving that
// new authoritative state is admitted or rejected correctly.
func appendValidatedForCrossRecordTest(t *testing.T, store *Store, raw []byte) (AppendResult, error) {
	t.Helper()
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := store.ensureHealthy(); err != nil {
		return AppendResult{}, err
	}
	record, canonical, err := parseAndValidate(raw)
	if err != nil {
		return AppendResult{}, err
	}
	return store.appendValidated(record, canonical)
}
