package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"homebase/internal/journal"
	"homebase/internal/records"
)

func main() {
	if len(os.Args) != 5 {
		fatal("usage: drive-admit seed-decision <journal> <ticket> <outdir>")
	}
	if os.Args[1] != "seed-decision" {
		fatal("only seed-decision supported")
	}
	fatal("seed-decision disabled: authoritative Decision persistence must use an authenticated authority path")

	journalPath, ticketPath, outDir := os.Args[2], os.Args[3], os.Args[4]
	rawTicket, err := os.ReadFile(ticketPath)
	if err != nil {
		fatal(err)
	}
	var ticket map[string]any
	if err := json.Unmarshal(rawTicket, &ticket); err != nil {
		fatal(err)
	}
	specID, _ := ticket["specification_id"].(string)
	decisionID, _ := ticket["approval_decision_id"].(string)
	if specID == "" || decisionID == "" {
		fatal("ticket missing specification_id / approval_decision_id")
	}

	specPayload := map[string]any{
		"purpose":   "portfolio one-surface sandbox drive atom A",
		"scope":     map[string]any{"systems": []string{"Bridge", "HomeBase", "portfolio"}, "effects": []string{"admission", "spawn"}},
		"non_goals": []string{"production VM certification", "machine-wide LaunchAgent"},
		"requirements": []any{
			map[string]any{"id": "R1", "text": "Worker may write only under wayfinder/one-surface-system-2026-07-30/"},
			map[string]any{"id": "R2", "text": "Seatbelt deny-default for out-of-scope writes or attempt fails"},
		},
		"proof_obligations": []any{map[string]any{"id": "P1", "method": "TestAX", "text": "sandbox-drive-note exists and cites deny-default + go test"}},
		"golden_scenarios":  []any{},
		"context_sources":   []any{},
		"assumptions":       []any{},
		"admission_policy": map[string]any{
			"requires_human_approval":        true,
			"fail_closed_on_open_obligation": true,
			"worker_may_authorize":           false,
		},
		"approval_ref":    map[string]any{"kind": "decision", "id": decisionID},
		"revision_policy": "new ID and digest for every revision",
	}
	specHash, err := records.CanonicalContentHash(mustJSON(specPayload))
	if err != nil {
		fatal(err)
	}
	capturedAt := time.Now().UTC().Truncate(time.Second).Format("2006-01-02T15:04:05Z")
	specification := map[string]any{
		"kind": "Specification", "version": "1", "id": specID,
		"source_refs":     []any{map[string]any{"kind": "document", "id": "sandbox-drive-atom-a"}},
		"content_hash":    specHash,
		"captured_at":     capturedAt,
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil},
		"status":          "approved",
		"source":          map[string]any{"id": "captain", "role": "captain"},
		"payload":         specPayload,
	}

	decisionPayload := map[string]any{
		"decision":   "approve specification " + specID,
		"scope":      "running-machine contract admission",
		"decided_by": "captain",
		"specification_ref": map[string]any{
			"kind": "specification", "id": specID, "content_hash": specHash,
		},
	}
	decisionHash, err := records.CanonicalContentHash(mustJSON(decisionPayload))
	if err != nil {
		fatal(err)
	}
	decision := map[string]any{
		"kind": "Decision", "version": "1", "id": decisionID,
		"source_refs":     []any{map[string]any{"kind": "document", "id": "captain-approval-sandbox-drive"}},
		"content_hash":    decisionHash,
		"captured_at":     capturedAt,
		"authority_class": records.AuthorityHumanDecision,
		"freshness":       map[string]any{"mode": "immutable", "valid_until": nil},
		"status":          "approved",
		"source":          map[string]any{"id": "captain", "role": "captain"},
		"payload":         decisionPayload,
	}

	ticket["specification_digest"] = specHash
	ticket["approval_decision_digest"] = decisionHash
	updated := pretty(mustJSON(ticket))
	if err := os.WriteFile(ticketPath, append(updated, '\n'), 0o600); err != nil {
		fatal(err)
	}

	j, err := journal.OpenBinaryJournal(journalPath)
	if err != nil {
		fatal(err)
	}
	store, err := records.NewStore(j)
	if err != nil {
		j.Close()
		fatal(err)
	}
	if _, err := store.Append(mustJSON(decision)); err != nil {
		j.Close()
		fatal(fmt.Errorf("append decision: %w", err))
	}
	if err := j.Close(); err != nil {
		fatal(err)
	}

	mustWrite(filepath.Join(outDir, "specification.json"), mustJSON(specification))
	fmt.Printf("seeded decision %s digest=%s\n", decisionID, decisionHash)
	fmt.Printf("specification digest=%s written\n", specHash)
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		fatal(err)
	}
	return b
}
func pretty(v []byte) []byte {
	var obj any
	_ = json.Unmarshal(v, &obj)
	b, _ := json.MarshalIndent(obj, "", "  ")
	return b
}
func mustWrite(path string, data []byte) {
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		fatal(err)
	}
}
func fatal(err any) {
	fmt.Fprintln(os.Stderr, "drive-admit:", err)
	os.Exit(2)
}
