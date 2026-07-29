package journal

import (
	"bytes"
	"encoding/binary"
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
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(50+len([]byte(`"event1"`))+32) {
		t.Fatalf("recovery left truncated bytes in journal: size=%d", info.Size())
	}
	j2.Close()
}

func TestBinaryJournal_TruncatedPayloadTail(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trunc-payload.journal")
	j, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append([]byte(`"event1"`)); err != nil {
		t.Fatal(err)
	}
	j.Close()

	valid, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	header := make([]byte, 50)
	copy(header, MagicBytes)
	binary.BigEndian.PutUint16(header[4:6], FormatVersion)
	binary.BigEndian.PutUint64(header[6:14], 2)
	binary.BigEndian.PutUint32(header[14:18], 4)
	copy(header[18:50], valid[len(valid)-32:])
	if _, err := f.Write(append(header, []byte("x")...)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	j2, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatalf("payload-tail recovery failed: %v", err)
	}
	defer j2.Close()
	if j2.nextSeq != 2 {
		t.Fatalf("expected nextSeq 2 after payload-tail recovery, got %d", j2.nextSeq)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != int64(50+len([]byte(`"event1"`))+32) {
		t.Fatalf("payload-tail recovery left truncated bytes: size=%d", info.Size())
	}
}

func TestBinaryJournal_RejectsVersionAndSequenceGaps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid-header.journal")
	j, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append([]byte(`"event1"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append([]byte(`"event2"`)); err != nil {
		t.Fatal(err)
	}
	j.Close()

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secondOffset := 50 + len([]byte(`"event1"`)) + 32
	binary.BigEndian.PutUint64(content[secondOffset+6:secondOffset+14], 3)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBinaryJournal(path); err != ErrOutOfOrder {
		t.Fatalf("sequence gap open error = %v, want %v", err, ErrOutOfOrder)
	}

	versionPath := filepath.Join(dir, "invalid-version.journal")
	j2, err := OpenBinaryJournal(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j2.Append([]byte(`"event1"`)); err != nil {
		t.Fatal(err)
	}
	j2.Close()
	versionContent, err := os.ReadFile(versionPath)
	if err != nil {
		t.Fatal(err)
	}
	binary.BigEndian.PutUint16(versionContent[4:6], FormatVersion+1)
	if err := os.WriteFile(versionPath, versionContent, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenBinaryJournal(versionPath); err == nil {
		t.Fatal("unsupported journal version was accepted")
	}
}

func TestBinaryJournal_ReplayChecksHashChain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "replay-chain.journal")
	j, err := OpenBinaryJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append([]byte(`"event1"`)); err != nil {
		t.Fatal(err)
	}
	if _, err := j.Append([]byte(`"event2"`)); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	secondOffset := 50 + len([]byte(`"event1"`)) + 32
	content[secondOffset+18] ^= 0xff
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if err := j.Replay(func(seq uint64, payload []byte) error { return nil }); err == nil {
		t.Fatal("replay accepted a broken hash chain")
	}
	j.Close()
}
