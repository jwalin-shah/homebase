package effect

import (
	"testing"
	"time"

	"homebase/internal/domain"
)

func TestEffect_UnknownOutcomeRouting(t *testing.T) {
	tests := []struct {
		name          string
		caps          domain.CapabilityDescriptor
		expectedEvent interface{}
		expectedPhase domain.EffectPhase
	}{
		{
			name: "None_None_Yields_ManualResolution",
			caps: domain.CapabilityDescriptor{
				Idempotency:    domain.IdempotencyNone,
				ResultLookup:   domain.ResultLookupNone,
				UnknownOutcome: domain.UnknownRequiresManualResolution,
				Status:         domain.StatusVerified,
			},
			expectedEvent: domain.EventManualResolutionRequired{},
			expectedPhase: domain.EffectManualResolutionRequired,
		},
		{
			name: "StableKey_None_Yields_Retry",
			caps: domain.CapabilityDescriptor{
				Idempotency:    domain.IdempotencyStableKey,
				ResultLookup:   domain.ResultLookupNone,
				UnknownOutcome: domain.UnknownCanSafelyRetry,
				Status:         domain.StatusVerified,
			},
			expectedEvent: domain.EventRetryAuthorized{},
			expectedPhase: domain.EffectPending, // EventRetryAuthorized resets to pending
		},
		{
			name: "None_Supported_Yields_Reconcile",
			caps: domain.CapabilityDescriptor{
				Idempotency:    domain.IdempotencyNone,
				ResultLookup:   domain.ResultLookupByIdempotencyKey,
				UnknownOutcome: domain.UnknownCanReconcile,
				Status:         domain.StatusVerified,
			},
			expectedEvent: domain.EventReconciliationRequired{},
			expectedPhase: domain.EffectReconciliationRequired,
		},
		{
			name: "StableKey_Supported_Yields_Reconcile",
			caps: domain.CapabilityDescriptor{
				Idempotency:    domain.IdempotencyStableKey,
				ResultLookup:   domain.ResultLookupByIdempotencyKey,
				UnknownOutcome: domain.UnknownCanReconcile,
				Status:         domain.StatusVerified,
			},
			expectedEvent: domain.EventReconciliationRequired{},
			expectedPhase: domain.EffectReconciliationRequired,
		},
		{
			name: "Unverified_Yields_ManualResolution",
			caps: domain.CapabilityDescriptor{
				Idempotency:    domain.IdempotencyStableKey,
				ResultLookup:   domain.ResultLookupByIdempotencyKey,
				UnknownOutcome: domain.UnknownCanReconcile,
				Status:         domain.StatusProposed, // Unverified
			},
			expectedEvent: domain.EventManualResolutionRequired{},
			expectedPhase: domain.EffectManualResolutionRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewFakeEffectRepository()
			effectID := domain.EffectID("eff-cap-" + tt.name)
			currentTime := int(time.Now().Unix())

			// Claim
			state, version := repo.Load(effectID)
			cmdClaim := domain.CommandClaimEffect{EffectID: effectID, WorkerID: "worker-1", ExpectedVersion: int(version), LeaseUntil: currentTime + 60, CurrentTime: currentTime}
			decision := domain.DecideEffect(state, cmdClaim)
			repo.Append(effectID, version, decision.Events)

			// Observe Unknown
			state, version = repo.Load(effectID)
			cmdObs := domain.CommandObserveEffect{EffectID: effectID, WorkerID: "worker-1", ClaimEpoch: state.ClaimEpoch, Outcome: domain.OutcomeUnknown}
			decision = domain.DecideEffect(state, cmdObs)
			repo.Append(effectID, version, decision.Events)

			// Resolve Unknown based on capabilities
			state, version = repo.Load(effectID)
			cmdResolve := domain.CommandResolveUnknown{EffectID: effectID, Capabilities: tt.caps}
			decision = domain.DecideEffect(state, cmdResolve)
			if decision.Status != domain.DecisionAccepted {
				t.Fatalf("expected capability routing to be accepted")
			}
			repo.Append(effectID, version, decision.Events)

			finalState, _ := repo.Load(effectID)
			if finalState.Phase != tt.expectedPhase {
				t.Errorf("expected phase %v, got %v", tt.expectedPhase, finalState.Phase)
			}

			// Validate emitted event
			if len(decision.Events) != 1 {
				t.Fatalf("expected exactly 1 event")
			}
			eventMatched := false
			switch decision.Events[0].(type) {
			case domain.EventManualResolutionRequired:
				_, eventMatched = tt.expectedEvent.(domain.EventManualResolutionRequired)
			case domain.EventReconciliationRequired:
				_, eventMatched = tt.expectedEvent.(domain.EventReconciliationRequired)
			case domain.EventRetryAuthorized:
				_, eventMatched = tt.expectedEvent.(domain.EventRetryAuthorized)
			}
			if !eventMatched {
				t.Errorf("expected event %T, got %T", tt.expectedEvent, decision.Events[0])
			}

			emitEvidence(t, Evidence{
				ID:         "EVD-M5B-" + tt.name,
				ClaimIDs:   []string{"CLM-M5B-001", "CLM-M5B-002", "CLM-M5B-003", "CLM-M5B-004"},
				TestName:   "TestEffect_UnknownOutcomeRouting/" + tt.name,
				Inputs:     map[string]interface{}{"capabilities": tt.caps},
				Assertions: map[string]interface{}{"expected_phase": tt.expectedPhase, "event_type_matched": true},
			})
		})
	}
}
