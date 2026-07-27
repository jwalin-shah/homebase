package journal

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func setupJournal(t *testing.T) (*BinaryJournal, func()) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.journal")
	j, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatalf("failed to open journal: %v", err)
	}
	return j, func() {
		j.Close()
	}
}

// Invariants 1 & 3: append only, monotonic sequence
func TestBinaryJournal_AppendAndMonotonic(t *testing.T) {
	j, cleanup := setupJournal(t)
	defer cleanup()

	seq1, err := j.Append([]byte(`"event1"`))
	if err != nil {
		t.Fatalf("append 1 failed: %v", err)
	}
	if seq1 != 1 {
		t.Fatalf("expected seq 1, got %d", seq1)
	}

	seq2, err := j.Append([]byte(`"event2"`))
	if err != nil {
		t.Fatalf("append 2 failed: %v", err)
	}
	if seq2 != 2 {
		t.Fatalf("expected seq 2, got %d", seq2)
	}
}

// Invariants 4 & 5: durable after fsync, deterministic replay
func TestBinaryJournal_Replay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replay.journal")
	
	j1, _ := OpenBinaryJournal(path)
	j1.Append([]byte(`"event1"`))
	j1.Append([]byte(`"event2"`))
	j1.Close()

	j2, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatalf("failed to reopen: %v", err)
	}
	defer j2.Close()

	if j2.nextSeq != 3 {
		t.Fatalf("expected nextSeq 3 after recovery, got %d", j2.nextSeq)
	}

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
}

// Invariant 6: corruption detection (hash chain or payload mutation)
func TestBinaryJournal_CorruptionDetection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.journal")
	
	j, _ := OpenBinaryJournal(path)
	j.Append([]byte(`"valid_payload"`))
	j.Close()

	content, _ := os.ReadFile(path)
	content = bytes.Replace(content, []byte(`"valid_payload"`), []byte(`"hacked_payload"`), 1)
	os.WriteFile(path, content, 0600)

	_, err := OpenBinaryJournal(path)
	if err != ErrCorruptData {
		t.Fatalf("expected ErrCorruptData when opening corrupted journal, got %v", err)
	}
}

func TestBinaryJournal_TruncatedTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc.journal")
	
	j, _ := OpenBinaryJournal(path)
	j.Append([]byte(`"event1"`))
	j.Close()

	// Append a partial header (simulating a crash midway through writing)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	f.Write(MagicBytes)
	f.Write([]byte{0, 1}) // format version
	f.Close()

	// Should recover and truncate the tail
	j2, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatalf("expected successful recovery from truncated tail, got %v", err)
	}
	if j2.nextSeq != 2 {
		t.Fatalf("expected nextSeq 2, got %d", j2.nextSeq)
	}
	j2.Close()
}
