package effect

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type Evidence struct {
	ID         string
	ClaimIDs   []string
	TestName   string
	Inputs     map[string]interface{}
	Assertions map[string]interface{}
	Artifacts  map[string]string
}

func emitEvidence(t *testing.T, ev Evidence) {
	outDir := os.Getenv("HOMEBASE_EVIDENCE_DIR")
	if outDir == "" {
		outDir = filepath.Join("..", "..", "artifacts", "evidence", "local-run")
	}

	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		t.Fatalf("failed to create evidence dir: %v", err)
	}

	var sb strings.Builder
	sb.WriteString("evidence:\n")
	sb.WriteString(fmt.Sprintf("  id: %s\n", ev.ID))
	sb.WriteString("  status: candidate\n")
	sb.WriteString("  producer: test_harness\n")
	sb.WriteString("  verified_by: null\n")
	sb.WriteString("  claim_ids:\n")
	for _, c := range ev.ClaimIDs {
		sb.WriteString(fmt.Sprintf("    - %s\n", c))
	}
	sb.WriteString("  test:\n")
	sb.WriteString(fmt.Sprintf("    name: %s\n", ev.TestName))
	sb.WriteString(fmt.Sprintf("    command: go test -run %s\n", ev.TestName))
	sb.WriteString("    exit_code: 0\n")

	sb.WriteString("  environment:\n")
	sb.WriteString("    dafny_version: 4.11.0\n")
	sb.WriteString("    go_version: 1.22.0\n")

	sb.WriteString("  inputs:\n")
	for k, v := range ev.Inputs {
		sb.WriteString(fmt.Sprintf("    %s: %v\n", k, v))
	}

	sb.WriteString("  assertions:\n")
	for k, v := range ev.Assertions {
		sb.WriteString(fmt.Sprintf("    %s: %v\n", k, v))
	}

	if len(ev.Artifacts) > 0 {
		sb.WriteString("  artifacts:\n")
		for k, v := range ev.Artifacts {
			sb.WriteString(fmt.Sprintf("    %s: %v\n", k, v))
		}
	}

	filename := fmt.Sprintf("%s.yaml", ev.ID)
	outPath := filepath.Join(outDir, filename)
	err = os.WriteFile(outPath, []byte(sb.String()), 0644)
	if err != nil {
		t.Fatalf("failed to write evidence: %v", err)
	}
}

func getFingerprint(data string) [32]byte {
	return sha256.Sum256([]byte(data))
}
