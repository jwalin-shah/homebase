package records

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestStoreVerificationSurfaceHasNoCallerSuppliedPolicyMutation(t *testing.T) {
	storeType := reflect.TypeOf((*Store)(nil))
	if _, ok := storeType.MethodByName("AppendBridgeVerificationSubmissionWithPolicy"); ok {
		t.Fatal("Store still exports a mutation seam that accepts caller-supplied VerifierPolicy")
	}
	if _, ok := storeType.MethodByName("AppendBridgeVerificationSubmissionAuthorized"); !ok {
		t.Fatal("Store is missing the opaque authority-gated verification admission seam")
	}
}

func TestBindStoreVerifierPolicyRequiresPrivilegedStoreCapability(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	policy, err := NewVerifierPolicy([]VerifierAuthority{{
		VerifierID: productionVerifierID,
		KeyID:      "verifier-key-1",
		PublicKey:  publicKey,
		ValidFrom:  time.Unix(1, 0),
		ValidUntil: time.Unix(4102444800, 0),
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := BindStoreVerifierPolicy(StoreAuthority{}, policy); !errors.Is(err, ErrAuthorityRequired) {
		t.Fatalf("BindStoreVerifierPolicy with zero authority error = %v, want ErrAuthorityRequired", err)
	}
}
