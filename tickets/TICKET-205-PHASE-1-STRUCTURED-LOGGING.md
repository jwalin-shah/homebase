# TICKET 205: Observability - Phase 1 Implementation

**Date:** 2026-07-26  
**Phase:** 1 (Implementation)  
**Status:** IN PROGRESS  
**Focus:** Structured logging + metrics foundation

---

## Task 1: Add Structured Logger

**Add to main.go:**

```go
import "log/slog"

func main() {
  // Initialize structured logger
  logHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
  })
  logger := slog.New(logHandler)
  
  // Set global logger
  slog.SetDefault(logger)
  
  // Log startup
  slog.Info("homebase_starting",
    "version", "1.0",
    "listen", *listenAddr,
    "ledger", *ledgerPath,
  )
}
```

---

## Task 2: Add Logging to Handlers

**In handlers.go - CreateEscalation:**

```go
func (h *Handler) CreateEscalation(w http.ResponseWriter, r *http.Request) {
  correlationID := r.Header.Get("X-Correlation-ID")
  if correlationID == "" {
    correlationID = generateCorrelationID()
  }
  
  logger := slog.With("correlation_id", correlationID)
  
  start := time.Now()
  
  // ... existing request parsing ...
  
  logger.Info("escalation_creating",
    "decision_id", req.DecisionID,
    "spawn_type", req.SpawnType,
    "system", req.System,
  )
  
  // ... existing business logic ...
  
  if err := h.ledger.Append(&escalationDecision); err != nil {
    logger.Error("escalation_create_failed",
      "error", err.Error(),
      "decision_id", req.DecisionID,
      "duration_ms", time.Since(start).Milliseconds(),
    )
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusInternalServerError)
    json.NewEncoder(w).Encode(ErrorResponse{Error: fmt.Sprintf("failed to create escalation: %v", err), Status: 500})
    return
  }
  
  logger.Info("escalation_created",
    "escalation_id", escalationID,
    "decision_id", req.DecisionID,
    "status", "PENDING",
    "duration_ms", time.Since(start).Milliseconds(),
  )
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusCreated)
  json.NewEncoder(w).Encode(escalation)
}
```

**Apply same pattern to:**
- GetEscalation
- ApproveEscalation
- BridgeCallback
- RecordDecision
- VerifyDecision

---

## Task 3: Correlation ID Tracking

**Add helper function:**

```go
func generateCorrelationID() string {
  timestamp := time.Now().Unix()
  randomBytes := make([]byte, 8)
  rand.Read(randomBytes)
  randomHex := hex.EncodeToString(randomBytes)
  return fmt.Sprintf("corr-%d-%s", timestamp, randomHex)
}
```

**Middleware to propagate:**

```go
// In server.go
mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
  correlationID := r.Header.Get("X-Correlation-ID")
  if correlationID == "" {
    correlationID = generateCorrelationID()
  }
  
  // Add to response header
  w.Header().Set("X-Correlation-ID", correlationID)
  
  // Add to context for handlers
  ctx := context.WithValue(r.Context(), "correlation_id", correlationID)
  
  // Next handler
  next(w, r.WithContext(ctx))
})
```

---

## Task 4: Metrics Collection

**Add metrics package:**

```go
// metrics.go
package api

import (
  "sync"
)

type Metrics struct {
  mu sync.RWMutex
  
  // Request metrics
  RequestsTotal       map[string]int  // by status
  RequestDurationMs   []int           // histogram
  
  // Decision metrics
  DecisionsCreated    int
  DecisionsFailed     int
  AxiomsCited         map[string]int  // by axiom
  
  // Escalation metrics
  EscalationsTotal    map[string]int  // by status
  EscalationDuration  []int           // histogram
  BridgeCallbackTime  []int           // histogram
  
  // System metrics
  LedgerSizeBytes     int64
  DecisionsCount      int
}

func (m *Metrics) RecordRequest(status string, durationMs int) {
  m.mu.Lock()
  defer m.mu.Unlock()
  
  if m.RequestsTotal == nil {
    m.RequestsTotal = make(map[string]int)
  }
  m.RequestsTotal[status]++
  m.RequestDurationMs = append(m.RequestDurationMs, durationMs)
}

func (m *Metrics) RecordDecisionCreated() {
  m.mu.Lock()
  defer m.mu.Unlock()
  m.DecisionsCreated++
}

func (m *Metrics) RecordEscalation(status string) {
  m.mu.Lock()
  defer m.mu.Unlock()
  
  if m.EscalationsTotal == nil {
    m.EscalationsTotal = make(map[string]int)
  }
  m.EscalationsTotal[status]++
}
```

**Update Handler:**

```go
type Handler struct {
  ledger    *ledger.Store
  validator *validation.Validator
  signer    *signing.Signer
  verifier  *signing.Verifier
  metrics   *Metrics  // Add this
}

// In handlers
func (h *Handler) CreateEscalation(w http.ResponseWriter, r *http.Request) {
  // ... logging ...
  
  h.metrics.RecordEscalation("PENDING")
  
  // ... rest of code ...
}
```

---

## Task 5: Metrics Endpoint

**Endpoint:** `GET /api/v1/metrics`

```go
func (h *Handler) GetMetrics(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
  }
  
  h.metrics.mu.RLock()
  defer h.metrics.mu.RUnlock()
  
  metrics := map[string]interface{}{
    "requests_total": h.metrics.RequestsTotal,
    "decisions_created": h.metrics.DecisionsCreated,
    "decisions_failed": h.metrics.DecisionsFailed,
    "escalations_total": h.metrics.EscalationsTotal,
    "ledger_size_bytes": h.metrics.LedgerSizeBytes,
    "decisions_count": h.metrics.DecisionsCount,
  }
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(metrics)
}
```

**Register:** `server.go`
```go
mux.HandleFunc("/api/v1/metrics", h.GetMetrics)
```

---

## Task 6: Enhanced Health Check

**Update health endpoint:**

```go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
  if r.Method != http.MethodGet {
    http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    return
  }
  
  status := "healthy"
  
  // Check ledger writable
  testDecision := &ledger.Decision{
    ID:         fmt.Sprintf("health-check-%d", time.Now().UnixNano()),
    Decision:   "health check",
    Axioms:     []string{"AX-HEALTH"},
    Evidence:   "system check",
    DecidedBy:  "system",
    Status:     "HEALTH_CHECK",
    RiskLevel:  "trivial",
    RecordedAt: time.Now(),
  }
  
  if err := h.ledger.Append(testDecision); err != nil {
    status = "degraded"
    slog.Error("health_check_ledger_failed", "error", err.Error())
  }
  
  // Check Neo4j
  neoStatus := "healthy"
  neoLatency := 0
  if h.validator.cacheClient != nil {
    start := time.Now()
    _, err := h.validator.cacheClient.CheckConnection()
    neoLatency = int(time.Since(start).Milliseconds())
    if err != nil {
      neoStatus = "unavailable"
      slog.Warn("health_check_neo4j_unavailable")
    }
  } else {
    neoStatus = "disabled"
  }
  
  response := map[string]interface{}{
    "status": status,
    "timestamp": time.Now().Format(time.RFC3339),
    "components": map[string]interface{}{
      "ledger": map[string]interface{}{
        "status": "healthy",
        "writable": true,
      },
      "neo4j": map[string]interface{}{
        "status": neoStatus,
        "latency_ms": neoLatency,
      },
      "signing": map[string]interface{}{
        "status": "healthy",
      },
    },
  }
  
  w.Header().Set("Content-Type", "application/json")
  w.WriteHeader(http.StatusOK)
  json.NewEncoder(w).Encode(response)
}
```

---

## Implementation Checklist

- [ ] Task 1: Add structured logger (30 min)
- [ ] Task 2: Add logging to all handlers (2 hours)
- [ ] Task 3: Correlation ID tracking (30 min)
- [ ] Task 4: Metrics collection (1 hour)
- [ ] Task 5: Metrics endpoint (30 min)
- [ ] Task 6: Enhanced health check (1 hour)
- [ ] Build verification (15 min)
- [ ] Phase 1 finalization (15 min)

**Estimated Phase 1 Time:** 6-7 hours

---

## Success Criteria

- [ ] All handlers emit structured JSON logs
- [ ] Correlation IDs track requests end-to-end
- [ ] Metrics collected accurately
- [ ] Metrics endpoint working
- [ ] Health check returns detailed status
- [ ] Performance impact < 5%
- [ ] Binary builds without errors

---

**Phase 1 Status:** STARTING NOW

Next: Implement Task 1 (structured logger)
