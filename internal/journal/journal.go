package journal

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	ErrCorruptRecord = errors.New("corrupt record detected")
	ErrOutOfOrder    = errors.New("append sequence out of order")
	ErrRewrite       = errors.New("cannot rewrite existing sequence")
)

// Record is a single durable entry in the journal.
// It is deliberately completely agnostic to the domain types.
type Record struct {
	Sequence uint64          `json:"seq"`
	Payload  json.RawMessage `json:"payload"`
	Checksum string          `json:"checksum"` // sha256(seq + payload)
}

func (r *Record) Verify() error {
	expected := HashRecord(r.Sequence, r.Payload)
	if expected != r.Checksum {
		return ErrCorruptRecord
	}
	return nil
}

func HashRecord(seq uint64, payload []byte) string {
	h := sha256.New()
	h.Write([]byte(fmt.Sprintf("%d:", seq)))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// FileJournal provides an append-only, fsync-backed ledger.
type FileJournal struct {
	mu          sync.Mutex
	file        *os.File
	writer      *bufio.Writer
	nextSeq     uint64
	path        string
}

func OpenFileJournal(path string) (*FileJournal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	j := &FileJournal{
		file:    f,
		writer:  bufio.NewWriter(f),
		nextSeq: 1,
		path:    path,
	}

	// Replay to establish next sequence and verify integrity
	if err := j.recover(); err != nil {
		f.Close()
		return nil, err
	}

	return j, nil
}

func (j *FileJournal) recover() error {
	_, err := j.file.Seek(0, 0)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(j.file)
	var lastSeq uint64 = 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return fmt.Errorf("failed to parse JSON (%s): %w", string(line), err)
		}
		if err := rec.Verify(); err != nil {
			return fmt.Errorf("journal corruption at sequence %d: %w", rec.Sequence, err)
		}
		if rec.Sequence <= lastSeq {
			return ErrOutOfOrder
		}
		lastSeq = rec.Sequence
	}
	
	if err := scanner.Err(); err != nil {
		return err
	}

	j.nextSeq = lastSeq + 1
	
	// Seek back to end for future appends
	_, err = j.file.Seek(0, io.SeekEnd)
	return err
}

// Append writes a payload sequentially, fsyncing it to disk.
func (j *FileJournal) Append(payload []byte) (uint64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	seq := j.nextSeq
	rec := Record{
		Sequence: seq,
		Payload:  payload,
		Checksum: HashRecord(seq, payload),
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return 0, err
	}

	// Write line
	if _, err := j.writer.Write(data); err != nil {
		return 0, err
	}
	if _, err := j.writer.Write([]byte("\n")); err != nil {
		return 0, err
	}

	// Flush bufio
	if err := j.writer.Flush(); err != nil {
		return 0, err
	}

	// Fsync to disk
	if err := j.file.Sync(); err != nil {
		return 0, err
	}

	j.nextSeq++
	return seq, nil
}

// Replay reads all payloads sequentially.
func (j *FileJournal) Replay(handler func(seq uint64, payload []byte) error) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	f, err := os.Open(j.path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		// ignore empty lines that might result from trailing newlines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			return err
		}
		if err := rec.Verify(); err != nil {
			return err
		}
		if err := handler(rec.Sequence, rec.Payload); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func (j *FileJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.writer.Flush()
	j.file.Sync()
	return j.file.Close()
}
