package journal

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func setupJournal(t *testing.T) (*FileJournal, func()) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.journal")
	j, err := OpenFileJournal(path)
	if err != nil {
		t.Fatalf("failed to open journal: %v", err)
	}
	return j, func() {
		j.Close()
	}
}

// Invariants 1 & 3: append only, monotonic sequence
func TestJournal_AppendAndMonotonic(t *testing.T) {
	j, cleanup := setupJournal(t)
	defer cleanup()

	seq1, err := j.Append([]byte(`{"cmd": "one"}`))
	if err != nil {
		t.Fatalf("append 1 failed: %v", err)
	}
	if seq1 != 1 {
		t.Fatalf("expected seq 1, got %d", seq1)
	}

	seq2, err := j.Append([]byte(`{"cmd": "two"}`))
	if err != nil {
		t.Fatalf("append 2 failed: %v", err)
	}
	if seq2 != 2 {
		t.Fatalf("expected seq 2, got %d", seq2)
	}
}

// Invariants 4 & 5: durable after fsync, deterministic replay
func TestJournal_Replay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replay.journal")
	
	j1, _ := OpenFileJournal(path)
	if _, err := j1.Append([]byte(`"event1"`)); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	if _, err := j1.Append([]byte(`"event2"`)); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	j1.Close()

	// Re-open, check sequence recovery
	j2, err := OpenFileJournal(path)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer j2.Close()

	if j2.nextSeq != 3 {
		data, _ := os.ReadFile(path)
		t.Fatalf("expected nextSeq 3 after recovery, got %d. File contents: %s", j2.nextSeq, string(data))
	}

	// Deterministic replay
	var history [][]byte
	err = j2.Replay(func(seq uint64, payload []byte) error {
		history = append(history, payload)
		return nil
	})
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	if len(history) != 2 {
		t.Fatalf("expected 2 items in replay, got %d", len(history))
	}
	if !bytes.Equal(history[0], []byte(`"event1"`)) {
		t.Errorf("expected \"event1\", got %s", history[0])
	}
}

// Invariant 6: corruption detection (tampering with payload or sequence)
func TestJournal_CorruptionDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.journal")
	
	j, _ := OpenFileJournal(path)
	if _, err := j.Append([]byte(`"valid_payload"`)); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	j.Close()

	// Tamper with the file
	content, _ := os.ReadFile(path)
	// Just mutate some bytes in the JSON (e.g. change "valid_payload" to "hacked_payload")
	content = bytes.Replace(content, []byte(`"valid_payload"`), []byte(`"hacked_payload"`), 1)
	os.WriteFile(path, content, 0600)

	// Opening should fail with corruption
	_, err := OpenFileJournal(path)
	if err == nil {
		t.Fatalf("expected error when opening corrupted journal")
	}
}

// Invariant 2: no rewrite
func TestJournal_NoRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "norewrite.journal")
	
	j, _ := OpenFileJournal(path)
	if _, err := j.Append([]byte(`"event1"`)); err != nil {
		t.Fatalf("append failed: %v", err)
	}
	j.Close()

	// Malicious actor tries to rewrite sequence 1 directly in the file
	// We will simulate it by writing a valid JSON line with sequence 1 at the end of the file
	hackedRec := Record{Sequence: 1, Payload: []byte(`"event1_hacked"`)}
	hackedRec.Checksum = HashRecord(1, hackedRec.Payload)
	data, _ := json.Marshal(hackedRec)
	
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.Write(data)
	f.Write([]byte("\n"))
	f.Close()

	// Recovery should fail because seq 1 follows seq 1 (out of order / not strictly monotonic)
	_, err := OpenFileJournal(path)
	if err == nil {
		t.Fatalf("expected ErrOutOfOrder when a previous sequence is rewritten at the end")
	}
}
