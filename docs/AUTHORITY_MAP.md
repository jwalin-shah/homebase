# HomeBase Authority Map

| Component | Authoritative Source | Executable Artifact | Assurance |
|-----------|----------------------|---------------------|-----------|
| **Reducer** | `Reducer.dfy` | Generated Go | Dafny verification |
| **Event Schema** | `requirements/schema AST` | Generated Go codecs/types | Schema validation |
| **Journal** | Restricted handwritten Go | Same Go | Goose/Perennial proof |
| **Repository Adapter** | Handwritten Go | Same Go | Contract + concurrency tests |
| **Effect Dispatcher** | Protocol spec + Go adapter | Handwritten Go | Model, idempotency, and fault tests |

## Status Classification
- **Current Reducer Go:** Prototype/reference. Authority is temporary. Target is Dafny-generated Go.
- **Current Journal Go:** Candidate executable implementation. Authority is handwritten Goose-compatible source. Target is Goose + Perennial verification.
