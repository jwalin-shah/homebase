# TICKET 205: Observability - Phase 0 Specification

**Date:** 2026-07-26  
**Phase:** 0 (Specification Review)  
**Status:** IN PROGRESS  
**Decision ID:** DEC-2026-07-26-004-OBSERVABILITY  
**Depends On:** Ticket 201 (Core System) ✅ COMPLETE

---

## Overview

Ticket 205 adds comprehensive observability: structured logging, metrics collection, health monitoring, and alerting infrastructure.

**Why it matters:** Without logging and metrics, production issues are invisible and debugging is guesswork.

---

## Current State

❌ **What's Missing:**
- No structured logging (just basic startup messages)
- No request correlation tracking
- No performance metrics
- No health monitoring
- No alerting infrastructure
- No dashboard visibility

---

## Phase 0 Requirements

### 1. Structured Logging

**Format:** JSON lines (one JSON object per log line)

**Fields per log entry:**
```json
{
  "timestamp": "2026-07-26T...",
  "level": "INFO|WARN|ERROR|DEBUG",
  "logger": "homebase.api",
  "message": "decision_created",
  "correlation_id": "corr-123",
  "duration_ms": 5,
  "user_id": "captain",
  "decision_id": "dec-001",
  "status": "success|error",
  "error": "...",
  "metadata": {...}
}
```

**Log Levels:**
- **INFO:** Normal operations (decision created, escalation sent, etc.)
- **WARN:** Graceful degradation (Neo4j unavailable, etc.)
- **ERROR:** Failed operations (ledger write failed, etc.)
- **DEBUG:** Detailed flow (signature verification steps, validation checks, etc.)

**Endpoints to Log:**
- POST /decisions (decision created)
- POST /escalations (escalation created, decision linked)
- GET /escalations/{id} (escalation queried, status returned)
- POST /escalations/{id}/approve (escalation approved/rejected)
- POST /bridge/callback (Bridge response received, verified)

---

### 2. Correlation Tracking

**Purpose:** Link all related operations across services

**Correlation ID Format:** `corr-<timestamp>-<random>`

**Implementation:**
```go
// Extract from request header or generate
correlationID := r.Header.Get("X-Correlation-ID")
if correlationID == "" {
  correlationID = generateCorrelationID()
}

// Pass through all operations
logger.WithCorrelationID(correlationID).Info("decision_created", ...)
```

**Usage:**
- Client sends `X-Correlation-ID` header
- HomeBase passes to Bridge
- Bridge sends back in callback
- All logs tagged with same ID
- Can trace request end-to-end

---

### 3. Metrics Collection

**Metrics to track:**

**Request Metrics:**
- `homebase_requests_total` (counter by method, path, status)
- `homebase_request_duration_ms` (histogram: p50, p95, p99)
- `homebase_request_size_bytes` (histogram)
- `homebase_response_size_bytes` (histogram)

**Decision Metrics:**
- `homebase_decisions_total` (counter by status)
- `homebase_decisions_creation_duration_ms` (histogram)
- `homebase_axioms_cited_total` (counter by axiom)

**Escalation Metrics:**
- `homebase_escalations_total` (counter by status: PENDING, APPROVED, REJECTED)
- `homebase_escalation_duration_ms` (histogram: time from creation to approval)
- `homebase_bridge_callback_latency_ms` (histogram: Bridge response time)

**System Metrics:**
- `homebase_ledger_size_bytes` (gauge)
- `homebase_decisions_count` (gauge: total in ledger)
- `homebase_neo4j_available` (gauge: 1 if connected, 0 if not)
- `homebase_neo4j_query_latency_ms` (histogram)

**Format:** Prometheus-compatible format (can be scraped)

---

### 4. Health Monitoring

**Health Check Endpoint:** `/api/v1/health`

**Response:**
```json
{
  "status": "healthy|degraded|down",
  "timestamp": "2026-07-26T...",
  "components": {
    "ledger": {
      "status": "healthy",
      "writable": true,
      "last_write_ms": 12
    },
    "neo4j": {
      "status": "healthy",
      "connected": true,
      "latency_ms": 45
    },
    "signing": {
      "status": "healthy",
      "keys_available": true
    }
  },
  "details": {
    "decisions_total": 1234,
    "ledger_size_bytes": 5678900,
    "uptime_seconds": 3600
  }
}
```

**Health Checks:**
- Ledger writable (try append, verify)
- Neo4j connected (ping, measure latency)
- Signing keys available
- No disk full errors
- No memory pressure

---

### 5. Alerting Rules

**Critical (page immediately):**
- Ledger write failures (5+ in 1 minute)
- Neo4j unavailable for > 5 minutes
- Signing failures
- Disk full (< 1GB remaining)
- Error rate > 5%

**High (alert in 1 hour):**
- Bridge callback failures (> 10)
- Escalation timeout rate > 10%
- Request latency p99 > 500ms
- Ledger size > 10GB

**Medium (review in daily standup):**
- Neo4j latency > 100ms
- Request latency p95 > 200ms
- Axiom validation cache misses > 20%

---

### 6. Dashboard Metrics

**Grafana Dashboard Panels:**

1. **Request Rate** (per endpoint)
2. **Request Latency** (p50, p95, p99)
3. **Error Rate** (by endpoint)
4. **Escalation Status** (pie chart: PENDING, APPROVED, REJECTED)
5. **Bridge Response Time** (histogram)
6. **System Health** (status indicators)
7. **Ledger Growth** (bytes over time)
8. **Neo4j Latency** (if connected)

---

## Error Scenarios

| Scenario | Log Level | Action |
|----------|-----------|--------|
| Ledger write fails | ERROR | Log error, increment counter, return 500 |
| Neo4j unavailable | WARN | Log warning, continue without axiom check |
| Signature fails | ERROR | Log error with decision ID, return 500 |
| Bridge timeout | WARN | Log timeout, increment counter |

---

## Acceptance Criteria

- [ ] Structured JSON logging implemented
- [ ] All endpoints log with correlation IDs
- [ ] Request/response metrics collected
- [ ] Prometheus-format metrics exposed
- [ ] Health check endpoint working
- [ ] Alerting rules defined
- [ ] Grafana dashboard designed
- [ ] Log retention policy documented
- [ ] Performance impact < 5%

---

## Implementation Strategy

**Phase 1 (Code):**
1. Add structured logger (stdlib or slog)
2. Add logging to all handlers
3. Implement correlation ID tracking
4. Expose Prometheus metrics endpoint
5. Implement health check logic
6. Document metric meanings

**Phase 2 (Unit Tests):**
- Test logger output format
- Test metric collection
- Test health check scenarios
- 80%+ coverage

**Phase 3 (Integration Tests):**
- End-to-end request logging
- Full metric lifecycle
- Health check accuracy

**Phase 4 (Independent Review):**
- Log quality/usefulness
- Metric completeness
- Performance impact
- Alert threshold appropriateness

---

## Dependencies

- Ticket 201 (Core system) ✅
- Go stdlib (log/slog)
- Prometheus client library (optional, for metrics export)

---

## Performance Impact

- Structured logging: ~1-2% overhead
- Metrics collection: ~1% overhead
- Health checks: ~1% overhead
- **Total: < 5% impact expected**

---

## Timeline

- Phase 0: Specification (THIS WEEK) ✅ IN PROGRESS
- Phase 1: Implementation (3-4 DAYS)
- Phase 2: Unit tests (1 DAY)
- Phase 3: Integration tests (1 DAY)
- Phase 4: Independent review (1 DAY)
- Phase 5: Production ready (NEXT WEEK)

---

**Phase 0 Status:** READY FOR ARCHITECTURE TEAM REVIEW
