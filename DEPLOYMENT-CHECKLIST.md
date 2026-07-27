# Staging Deployment Checklist - Ticket 201

**Date:** 2026-07-26  
**Target:** Staging environment  
**Binary:** homebase (from `cmd/homebase/main.go`)

---

## Pre-Deployment

- [ ] Verify Neo4j staging cluster is accessible
  ```bash
  curl neo4j://staging.internal:7474
  ```

- [ ] Prepare ledger storage location
  ```bash
  mkdir -p /data
  chmod 755 /data
  ```

- [ ] Generate signing keypair (or use existing)
  ```bash
  mkdir -p .keys
  # Binary will auto-generate on first run if missing
  ```

- [ ] Build binary
  ```bash
  cd /Users/jwalinshah/projects/homebase
  go build -o homebase cmd/homebase/main.go
  chmod +x homebase
  ```

- [ ] Verify binary works locally
  ```bash
  ./homebase -listen :9000 &
  sleep 1
  curl http://localhost:9000/api/v1/health
  kill %1
  ```

---

## Staging Deployment

- [ ] Copy binary to staging
  ```bash
  scp homebase staging-server:/opt/homebase/
  ```

- [ ] Copy configuration (if any)
  ```bash
  scp config.yaml staging-server:/opt/homebase/
  ```

- [ ] Create systemd service (if needed)
  ```bash
  cat > /etc/systemd/system/homebase.service << 'EOF'
  [Unit]
  Description=HomeBase Decision Ledger
  After=network.target
  
  [Service]
  Type=simple
  User=homebase
  WorkingDirectory=/opt/homebase
  ExecStart=/opt/homebase/homebase \
    -ledger /data/ledger.jsonl \
    -neo4j-uri neo4j://staging.internal:7474 \
    -neo4j-user neo4j \
    -neo4j-pass <password> \
    -listen :8080 \
    -private-key .keys/private.key \
    -public-key .keys/public.key
  Restart=on-failure
  RestartSec=10
  
  [Install]
  WantedBy=multi-user.target
  EOF
  
  systemctl daemon-reload
  systemctl enable homebase
  ```

- [ ] Start service
  ```bash
  systemctl start homebase
  ```

- [ ] Verify running
  ```bash
  systemctl status homebase
  curl http://staging:8080/api/v1/health
  # Expected: {"status":"full","ledger":"healthy"}
  ```

---

## Phase 2 Testing

- [ ] Record first decision
  ```bash
  curl -X POST http://staging:8080/api/v1/decisions \
    -H "Content-Type: application/json" \
    -d '{
      "id": "dec-test-001",
      "decision": "Test decision",
      "axioms": ["AX-001"],
      "evidence": "Test evidence",
      "decided_by": "test-agent",
      "risk_level": "minor"
    }'
  # Expected: 201 Created with signature
  ```

- [ ] Query decision back
  ```bash
  curl http://staging:8080/api/v1/decisions/dec-test-001
  # Expected: 200 with full decision
  ```

- [ ] List all decisions
  ```bash
  curl http://staging:8080/api/v1/decisions
  # Expected: 200 with array
  ```

- [ ] Verify signature
  ```bash
  curl -X POST http://staging:8080/api/v1/decisions/dec-test-001/verify \
    -H "Content-Type: application/json" \
    -d '{"signature":"<sig-from-record>"}'
  # Expected: {"valid":true}
  ```

- [ ] Test Neo4j graceful degradation
  ```bash
  # Stop Neo4j
  systemctl stop neo4j
  
  # Record decision (should succeed with WARNING)
  curl -X POST http://staging:8080/api/v1/decisions \
    -H "Content-Type: application/json" \
    -d '{
      "id": "dec-test-offline",
      "decision": "Test offline",
      "axioms": ["AX-001"],
      "evidence": "Test",
      "decided_by": "test",
      "risk_level": "minor"
    }'
  # Expected: 201 Created, logs show "axiom check skipped"
  
  # Restart Neo4j
  systemctl start neo4j
  ```

- [ ] Verify ledger durability
  ```bash
  # Stop service
  systemctl stop homebase
  
  # Restart
  systemctl start homebase
  
  # Query decisions
  curl http://staging:8080/api/v1/decisions
  # Expected: all decisions still present
  ```

---

## Monitoring & Logs

- [ ] Check logs
  ```bash
  journalctl -u homebase -f
  ```

- [ ] Monitor performance
  ```bash
  # Record 100 decisions, measure latency
  # Check memory usage
  ps aux | grep homebase
  
  # Check disk usage
  du -sh /data/ledger.jsonl
  ```

- [ ] Verify Neo4j integration
  ```bash
  # Check if axioms are cached
  # Query Neo4j directly for decision count
  ```

---

## Smoke Tests

- [ ] API responds to requests (latency <100ms)
- [ ] Health check returns 200
- [ ] Decisions recorded and queryable
- [ ] Signatures verify correctly
- [ ] Ledger persists across restarts
- [ ] Neo4j unavailability doesn't crash system
- [ ] No errors in logs

---

## Go/No-Go Decision

**Proceed to Phase 2 if:**
- ✅ All smoke tests pass
- ✅ No crashes in logs
- ✅ Performance acceptable

**Hold if:**
- ❌ Any failing test
- ❌ High error rate in logs
- ❌ Performance degraded

---

## Rollback Plan

If issues discovered:

```bash
# Stop service
systemctl stop homebase

# Restore previous version (if applicable)
cp /opt/homebase/backup/homebase /opt/homebase/homebase

# Restart
systemctl start homebase

# Verify rollback
curl http://staging:8080/api/v1/health
```

---

## Contact

**On-Call:** jwalinshah13@gmail.com  
**Escalation:** Architecture team  
**Neo4j Admin:** staging-admin@company.com

---

**Status:** Ready to Deploy  
**Next:** Execute checklist and proceed to Phase 2 testing
