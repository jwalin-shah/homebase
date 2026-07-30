// Model-only authority obligations for transcript promotion.
//
// This file deliberately models the finite admission state, not JSON parsing,
// Ed25519, Unicode spans, or filesystem durability. Those remain executable
// refinement obligations covered by the Go contract tests and journal tests.
module TranscriptPromotion {
  datatype Approval = Approval(
    principal: string,
    nonce: string,
    sourceIsBound: bool,
    approvalIsExplicit: bool,
    approvalIsFresh: bool,
    captureIsComplete: bool,
    transportAuthenticated: bool)

  datatype State = State(usedNonces: set<string>, decisions: set<string>)

  function NonceKey(principal: string, nonce: string): string {
    principal + ":" + nonce
  }

  predicate ValidApproval(a: Approval, principal: string) {
    a.principal == principal &&
    a.principal != "" &&
    a.nonce != "" &&
    a.sourceIsBound &&
    a.approvalIsExplicit &&
    a.approvalIsFresh &&
    a.captureIsComplete &&
    a.transportAuthenticated
  }

  predicate CanAccept(state: State, decisionID: string, a: Approval, principal: string) {
    decisionID != "" &&
    ValidApproval(a, principal) &&
    NonceKey(a.principal, a.nonce) !in state.usedNonces &&
    decisionID !in state.decisions
  }

  method Promote(state: State, decisionID: string, approval: Approval, principal: string)
    returns (next: State, accepted: bool)
    ensures accepted ==> CanAccept(state, decisionID, approval, principal)
    ensures accepted ==> next.usedNonces == state.usedNonces + {NonceKey(approval.principal, approval.nonce)}
    ensures accepted ==> next.decisions == state.decisions + {decisionID}
    ensures !accepted ==> next == state
    ensures !accepted ==> !CanAccept(state, decisionID, approval, principal)
  {
    if CanAccept(state, decisionID, approval, principal) {
      next := State(
        state.usedNonces + {NonceKey(approval.principal, approval.nonce)},
        state.decisions + {decisionID});
      accepted := true;
    } else {
      next := state;
      accepted := false;
    }
  }

  lemma AcceptedRequiresAllGates(state: State, decisionID: string, approval: Approval, principal: string)
    ensures CanAccept(state, decisionID, approval, principal) ==>
      ValidApproval(approval, principal) &&
      NonceKey(approval.principal, approval.nonce) !in state.usedNonces &&
      decisionID !in state.decisions
  {
  }
}
