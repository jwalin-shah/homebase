# HomeBase Ledger Contract

**Status:** Under audit (not locked)  
**Version:** 1.0  

The ledger is SQLite. It stores PersistedEventEnvelopes in append-only order.

---

## 1. Persisted Event Envelope

```
PersistedEventEnvelope {
    event_id: EventID                        // Assigned by ledger (UUID or sequential)
    global_sequence: uint64                  // Total order across all events
    aggregate_id: TaskID                     // Which task
    aggregate_type: String                   // Always "Task" (for future extensibility)
    aggregate_version: uint64                // Version of the aggregate before applying this event
    event_type: String                       // E.g., "ContractLocked", "AttemptCreated"
    schema_version: uint32                   // Event payload version (for migration)
    
    // Origin metadata (from CommandOrigin)
    command_id: CommandID
    command_fingerprint: Hash                // Digest of command body
    authority_principal_id: AuthorityID
    authority_role: String
    correlation_id: CorrelationID
    causation_id: Option[CommandID | EventID]
    issued_at: Timestamp                     // When command was issued
    
    // Domain event payload
    payload: String                          // RFC 8785 canonical JSON
    metadata: String                         // RFC 8785 canonical JSON (from domain event)
    
    // Ledger fields
    recorded_at: Timestamp                   // When written to SQLite
    previous_hash: Hash                      // SHA-256 of prior envelope
    event_hash: Hash                         // SHA-256 of this envelope (see Hash Calculation)
}
```

---

## 2. SQLite Schema

```sql
CREATE TABLE events (
    global_sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    event_id         TEXT NOT NULL UNIQUE,
    aggregate_id     TEXT NOT NULL,
    aggregate_type   TEXT NOT NULL,
    aggregate_version INTEGER NOT NULL,
    event_type       TEXT NOT NULL,
    schema_version   INTEGER NOT NULL,
    
    command_id       TEXT NOT NULL,
    command_fingerprint BLOB NOT NULL,
    authority_principal_id TEXT NOT NULL,
    authority_role   TEXT NOT NULL,
    correlation_id   TEXT NOT NULL,
    causation_id     TEXT,
    issued_at        TEXT NOT NULL,
    
    payload          TEXT NOT NULL,
    metadata         TEXT,
    
    recorded_at      TEXT NOT NULL,
    previous_hash    BLOB,
    event_hash       BLOB NOT NULL UNIQUE,
    
    UNIQUE (aggregate_id, aggregate_version)
);

CREATE INDEX events_by_aggregate
ON events (aggregate_id, aggregate_version);

CREATE INDEX events_by_correlation
ON events (correlation_id);
```

**Constraints:**

* `event_id` is globally unique (no duplicates across time)
* `(aggregate_id, aggregate_version)` is unique (each version exists exactly once)
* `event_hash` is globally unique (detects chain corruption)
* `global_sequence` is monotonically increasing (total order)

---

## 3. Compare-and-Append Transaction

The ledger commit operation is:

```
Commit(
    task_id: TaskID,
    expected_version: uint64,
    recorded_events: Seq[RecordedDomainEvent]
) returns (error)
```

The atomicity guarantee:

```sql
BEGIN TRANSACTION;

  -- Verify optimistic concurrency
  SELECT aggregate_version FROM events
  WHERE aggregate_id = ? 
  ORDER BY aggregate_version DESC LIMIT 1
  INTO current_version;
  
  IF current_version != expected_version THEN
    ROLLBACK;
    RETURN ERROR(STALE_VERSION);
  END IF;

  -- Assign event IDs and hashes (see section 5)
  FOR EACH event IN recorded_events:
    event.event_id := generateEventID();
    event.recorded_at := now();
    event.global_sequence := (SELECT MAX(global_sequence) + 1);
    event.previous_hash := (SELECT event_hash FROM events
                            WHERE aggregate_id = ?
                            ORDER BY aggregate_version DESC LIMIT 1);
    event.event_hash := sha256(canonicalBytes(event));
  END FOR;

  -- Atomic batch insert
  INSERT INTO events (...) VALUES (...);

COMMIT;
```

**Guarantees:**

* If the current aggregate_version does not match expected_version, the entire batch is rejected.
* If the batch is accepted, all events are written in one transaction with assigned sequence numbers.
* The hash chain is continuous (no gaps or reordering).

---

## 4. Canonical Event Encoding

Hash calculation requires exact byte-level reproducibility.

```
event_hash = SHA-256(
    domain_separator ||
    previous_hash ||
    canonical_payload
)

domain_separator = "homebase.event.v1" (18 bytes, UTF-8)

canonical_payload = RFC 8785 (JSON canonicalization) of {
    "aggregate_id": aggregate_id,
    "aggregate_type": aggregate_type,
    "aggregate_version": aggregate_version,
    "event_type": event_type,
    "schema_version": schema_version,
    "command_id": command_id,
    "command_fingerprint": command_fingerprint (hex string),
    "authority_principal_id": authority_principal_id,
    "authority_role": authority_role,
    "correlation_id": correlation_id,
    "causation_id": causation_id (null if missing),
    "issued_at": issued_at (RFC 3339),
    "payload": payload (already canonicalized as JSON),
    "metadata": metadata (already canonicalized as JSON),
    "recorded_at": recorded_at (RFC 3339)
}
```

**RFC 8785 rules:**

* Keys are sorted lexicographically.
* No whitespace outside strings.
* Unicode escapes use lowercase hex.
* Duplicate keys are not allowed.

**Test vectors:** See testdata/ledger/canonical_event_v1.json.

---

## 5. Hash Chain Semantics

The chain detects:

* **Internal corruption**: A byte-level change to any event is detected (hash mismatch).
* **Deletion**: If an event is removed, the next event's previous_hash no longer matches the prior event's event_hash.
* **Reordering**: Events depend on sequence, so reordering breaks the chain.
* **Mutation**: Changing any field invalidates the hash.

The chain does NOT protect against:

* **Complete database rewrite**: An attacker with write access to SQLite can rewrite the entire ledger and recalculate every hash.
* **Truncation without external verification**: Without a trusted checkpoint (e.g., signed ledger head, external log), the absence of evidence is not evidence of absence.

**Trusted checkpoints** are deferred. For now, the hash chain detects accidental corruption relative to a trusted head hash retained elsewhere (e.g., in a separate append-only log).

---

## 6. Recovery and Verification

**On restart:**

```
VerifyLedger(aggregate_id: TaskID) returns (state, errors)
```

1. Read all events for the aggregate in sequence order.
2. Compute expected hash for each event.
3. Compare against stored event_hash.
4. If any hash mismatches, report corruption (but do not repair).
5. Fold events to reconstruct current state.

**On command resubmission after crash:**

```
CommandFingerprintMatch(
    command_id: CommandID,
    command_fingerprint: Hash
) returns (receipt: CommandReceipt, error)
```

1. Query command_receipts for matching (command_id, command_fingerprint).
2. If exact match: return the prior result (idempotent).
3. If command_id exists with different fingerprint: return COMMAND_ID_CONFLICT.
4. If command_id not found: command was never processed (safe to retry or reject).

---

## 7. Durability Settings

```
PRAGMA journal_mode = WAL;          -- Write-Ahead Logging
PRAGMA synchronous = FULL;          -- Full fsync after each transaction
PRAGMA foreign_keys = ON;           -- (not used yet, for safety)
PRAGMA busy_timeout = 30000;        -- 30s lock timeout
```

**WAL guarantees:** If the process crashes mid-transaction, the transaction is either fully committed (both WAL and main database updated) or fully rolled back (no half-written records).

---

## 8. Ledger Fixtures

Conformance fixtures verify:

1. **canonical_event_v1.json**: Canonical byte encoding of domain events.
2. **hash_chain_v1.json**: Correct hash chain computation and verification.
3. **reordered_event.json**: Reordering breaks the chain.
4. **mutated_payload.json**: Payload mutation invalidates hash.

See testdata/ledger/ for examples.

---

## 9. Deferred (Not in First Slice)

* External append-only log (e.g., GCS, S3, or signed ledger heads).
* Ledger snapshots for performance.
* Event compaction or archival.
* Multi-ledger consensus or verification.
* Time-based retention policies.
