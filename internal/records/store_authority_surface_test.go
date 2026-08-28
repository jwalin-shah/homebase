package records

import (
	"reflect"
	"testing"
)

// A0b requires complete mediation at the Store API surface, not only a gated
// alternative. Keeping either legacy exported specialized write method would
// leave an internal caller able to bypass the Store-bound domain capability.
func TestStoreAuthoritySurfaceHasNoUngatedSpecializedWriteBypass(t *testing.T) {
	storeType := reflect.TypeOf((*Store)(nil))
	for _, methodName := range []string{
		"AppendPromotionCommit",
		"AppendContractAndGrant",
	} {
		if _, ok := storeType.MethodByName(methodName); ok {
			t.Fatalf("exported %s remains an ungated authoritative Store bypass", methodName)
		}
	}
}
