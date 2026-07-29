# Live transcript-promotion design

## Boundary

The contracts repository proves the shape of a transcript-promotion case, but
its fixture command deliberately never writes HomeBase. This package is the
runtime boundary that closes that gap.

The authority equation is:

```text
accepted(case, now) <=>
  strict_shape(case)
  AND transcript_hash_matches(case)
  AND complete_capture(case)
  AND authenticated_transport(case)
  AND candidate_spans_bind(case)
  AND explicit_approval(case)
  AND approval_is_fresh(case, now)
  AND nonce_is_new(case)
  AND durable_commit(decision, evidence, receipt)
```

The negative equation is equally important:

```text
rejected(case) => no_decision_record_is_appended
```

The caller-supplied `auth.verified` field is metadata only. The transport must
also carry a detached signature over the canonical transcript digest, and the
configured HomeBase authenticator must verify it for the claimed principal.

## Commit shape

An accepted case writes one `PromotionCommit` journal entry containing:

1. an untrusted `Evidence` record for the authenticated transcript digest;
2. a human-authoritative `Decision` record bound to that evidence; and
3. a HomeBase-signed promotion receipt binding the decision, evidence,
   transcript, approval, nonce, and hashes.

The three objects are encoded in one journal payload so a crash cannot expose
the decision without its evidence and receipt. Replay rebuilds the decision and
evidence indexes and rejects an invalid or orphaned promotion commit.

## Pseudocode

```text
parse strict typed case
verify transcript and turn hashes, ordering, spans, and completeness
verify detached transcript signature for transcript.auth.principal
verify candidate proposition/context hashes
verify exactly one user approval and explicit approval wording
verify approval/candidate/source-span/confirmation bindings and freshness
lock promotion store
if nonce exists:
  return the prior identical receipt, or reject a conflicting reuse
build Evidence and Decision records
sign receipt over canonical receipt fields
append one PromotionCommit and fsync it
return the durable receipt
```

This is still a runtime refinement slice, not a claim that every possible
natural-language intent is understood correctly. The verifier proves exact
bindings and freshness; it does not infer intent beyond the explicit approval
text.
