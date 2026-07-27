# Assurance Case: HB-JOURNAL-001
**Status:** ENFORCED

## Claim Statement
> The ledger provides an append-only, monotonic sequence of events that  durably survives crashes (fsync) and reconstructs deterministic domain state.


## Architectural Boundaries
- filesystem boundary (fsync)
- application service boundary

## Hazard Disposition Ledger
| Hazard | Applicable | Disposition | Evidence |
|--------|------------|-------------|----------|
| `partial_write` | Yes | detected and ignored on replay via checksums | `tests/corruption_detection` |
| `out_of_order_append` | Yes | monotonic sequence enforcement | `tests/monotonic_sequence` |
| `silent_rewrite` | Yes | append-only file modes and immutability checks | `tests/no_rewrite` |
| `process_crash_during_append` | Yes | replay from last valid synced boundary | `tests/crash_recovery` |

## Environmental Assumptions
### Assumption 1: The underlying OS accurately honors fsync() barriers.
- **Compile-time Gate:** `none`
- **Test-time Gate:** `tests/simulated_power_loss`
- **Runtime Monitor:** `monitors/io_latency`

