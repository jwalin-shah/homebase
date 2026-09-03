package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const receiptReadbackRoute = "/api/v1/verifications/receipts/read"
const healthStatusRoute = "/v1/status"

// TestStagedDeploymentReadinessContract proves the hermetic staged binary
// carries the receipt read-back route marker expected by CP-039 and the
// LaunchAgent health-check route marker (DAEMON_HEALTH_URL) expected by the
// plist contract.
func TestStagedDeploymentReadinessContract(t *testing.T) {
	t.Parallel()

	mainGo, err := filepath.Abs(filepath.Join("main.go"))
	if err != nil {
		t.Fatal(err)
	}
	source, err := os.ReadFile(mainGo)
	if err != nil {
		t.Fatalf("read main.go: %v", err)
	}
	if !strings.Contains(string(source), receiptReadbackRoute) {
		t.Fatalf("main.go missing route mount %q", receiptReadbackRoute)
	}
	if !strings.Contains(string(source), healthStatusRoute) {
		t.Fatalf("main.go missing route mount %q", healthStatusRoute)
	}

	workDir := t.TempDir()
	binary := filepath.Join(workDir, "homebase")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/homebase: %v\n%s", err, out)
	}

	stringsOut, err := exec.Command("strings", binary).CombinedOutput()
	if err != nil {
		t.Fatalf("strings staged binary: %v", err)
	}
	if !strings.Contains(string(stringsOut), receiptReadbackRoute) {
		t.Fatalf("staged binary missing route marker %q", receiptReadbackRoute)
	}
	if !strings.Contains(string(stringsOut), healthStatusRoute) {
		t.Fatalf("staged binary missing route marker %q", healthStatusRoute)
	}
}
