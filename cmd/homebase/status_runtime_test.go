package main_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// CP-041 proves the compiled cmd/homebase process serves the LaunchAgent
// health contract (DAEMON_HEALTH_URL=http://127.0.0.1:9102/v1/status) with
// HTTP 200 and no secret material, that non-GET methods fail safely, and
// that adding the route does not regress the existing CP-039 receipt
// read-back route.
func TestCompiledStatusRoute(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	journalPath := filepath.Join(workDir, "homebase_records.journal")
	if err := seedBridgePrerequisites(t, journalPath); err != nil {
		t.Fatal(err)
	}

	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	binary := filepath.Join(workDir, "homebase")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Dir = "."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/homebase: %v\n%s", err, out)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	cmd := exec.Command(binary)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("PORT=%d", port),
		"HOMEBASE_RECORD_JOURNAL="+journalPath,
		"HOMEBASE_BRIDGE_PUBLIC_KEY_HEX="+hex.EncodeToString(public),
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitForServer(baseURL, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	resp, err := http.Get(baseURL + "/v1/status")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/status = %d, body=%s", resp.StatusCode, body)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}
	lower := strings.ToLower(string(body))
	for _, needle := range []string{"priv", "secret", ".journal", "receipt.priv", "bridge.pub"} {
		if strings.Contains(lower, needle) {
			t.Fatalf("status body leaked forbidden term %q: %s", needle, body)
		}
	}

	postResp, err := http.Post(baseURL+"/v1/status", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(postResp.Body)
	_ = postResp.Body.Close()
	if postResp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST /v1/status = %d, want 405", postResp.StatusCode)
	}

	// The CP-039 receipt read-back route must remain mounted and unaffected
	// by the new health route: an unsigned POST must still fail with 401
	// (route mounted, signature check ran) rather than 404 (route absent).
	readResp, err := http.Post(baseURL+"/api/v1/verifications/receipts/read", "application/json", strings.NewReader(`{"receipt_id":"receipt:task-1:0123456789012345678901234567890123456789"}`))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(readResp.Body)
	_ = readResp.Body.Close()
	if readResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("receipt read-back route status = %d, want 401 (mounted, unsigned)", readResp.StatusCode)
	}

	_ = cmd.Process.Kill()
	_ = cmd.Wait()
}
