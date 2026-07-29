package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
)

var (
	MagicBytes      = []byte("JRNL")
	FormatVersion   = uint16(1)
	ErrInvalidMagic = errors.New("invalid magic bytes")
	ErrCorruptData  = errors.New("checksum mismatch")
	ErrTruncated    = errors.New("truncated record")
	ErrOutOfOrder   = errors.New("append sequence out of order")
)

type BinaryJournal struct {
	mu           sync.Mutex
	file         *os.File
	nextSeq      uint64
	previousHash [32]byte
	path         string
}

func OpenBinaryJournal(path string) (*BinaryJournal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}

	j := &BinaryJournal{
		file: f,
		path: path,
	}

	if err := j.recover(); err != nil {
		f.Close()
		return nil, err
	}

	return j, nil
}

func (j *BinaryJournal) recover() error {
	_, err := j.file.Seek(0, 0)
	if err != nil {
		return err
	}

	var lastSeq uint64 = 0
	var lastHash [32]byte
	lastGoodOffset, err := j.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	for {
		header := make([]byte, 4+2+8+4+32) // Magic(4) + Version(2) + Seq(8) + Len(4) + PrevHash(32)
		_, err := io.ReadFull(j.file, header)
		if err == io.EOF {
			break // Clean end of file
		}
		if err != nil {
			// If we read a partial header, it's a truncated tail. We stop here and truncate.
			if err == io.ErrUnexpectedEOF {
				if err := j.file.Truncate(lastGoodOffset); err != nil {
					return err
				}
				break
			}
			return err
		}

		if !bytes.Equal(header[0:4], MagicBytes) {
			return ErrInvalidMagic
		}
		if binary.BigEndian.Uint16(header[4:6]) != FormatVersion {
			return fmt.Errorf("unsupported journal format version %d", binary.BigEndian.Uint16(header[4:6]))
		}

		seq := binary.BigEndian.Uint64(header[6:14])
		length := binary.BigEndian.Uint32(header[14:18])
		var prevHash [32]byte
		copy(prevHash[:], header[18:50])

		if lastSeq == 0 {
			if seq != 1 || prevHash != ([32]byte{}) {
				return ErrOutOfOrder
			}
		} else {
			if seq != lastSeq+1 {
				return ErrOutOfOrder
			}
			if prevHash != lastHash {
				return fmt.Errorf("hash chain broken at seq %d", seq)
			}
		}

		payload := make([]byte, length)
		_, err = io.ReadFull(j.file, payload)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if err := j.file.Truncate(lastGoodOffset); err != nil {
					return err
				}
				if err := j.file.Sync(); err != nil {
					return err
				}
				break
			}
			return err
		}

		var checksum [32]byte
		_, err = io.ReadFull(j.file, checksum[:])
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				if err := j.file.Truncate(lastGoodOffset); err != nil {
					return err
				}
				if err := j.file.Sync(); err != nil {
					return err
				}
				break
			}
			return err
		}

		// Verify checksum
		h := sha256.New()
		h.Write(header)
		h.Write(payload)
		var computed [32]byte
		copy(computed[:], h.Sum(nil))

		if checksum != computed {
			// Interior corruption or torn write at the end.
			// The safest bet for torn write is truncation if it's the absolute end,
			// but for interior corruption it should fail hard.
			// Let's assume ErrCorruptData fails hard for now.
			return ErrCorruptData
		}

		lastSeq = seq
		lastHash = computed
		lastGoodOffset, err = j.file.Seek(0, io.SeekCurrent)
		if err != nil {
			return err
		}
	}

	j.nextSeq = lastSeq + 1
	j.previousHash = lastHash
	_, err = j.file.Seek(0, io.SeekEnd)
	return err
}

func (j *BinaryJournal) truncateToCurrentOffset() error {
	offset, err := j.file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	// Truncate to discard partial record
	return j.file.Truncate(offset)
}

func (j *BinaryJournal) Append(payload []byte) (uint64, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	seq := j.nextSeq
	length := uint32(len(payload))

	header := make([]byte, 50)
	copy(header[0:4], MagicBytes)
	binary.BigEndian.PutUint16(header[4:6], FormatVersion)
	binary.BigEndian.PutUint64(header[6:14], seq)
	binary.BigEndian.PutUint32(header[14:18], length)
	copy(header[18:50], j.previousHash[:])

	h := sha256.New()
	h.Write(header)
	h.Write(payload)
	var checksum [32]byte
	copy(checksum[:], h.Sum(nil))

	// Write everything as a single batch to avoid partial writes at OS level
	// (though OS can still partial write, we fsync afterwards)
	record := make([]byte, 0, 50+length+32)
	record = append(record, header...)
	record = append(record, payload...)
	record = append(record, checksum[:]...)

	for len(record) > 0 {
		written, err := j.file.Write(record)
		if err != nil {
			return 0, err
		}
		if written == 0 {
			return 0, io.ErrShortWrite
		}
		record = record[written:]
	}

	if err := j.file.Sync(); err != nil {
		return 0, err
	}

	j.nextSeq++
	j.previousHash = checksum
	return seq, nil
}

func (j *BinaryJournal) Replay(handler func(seq uint64, payload []byte) error) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	f, err := os.Open(j.path)
	if err != nil {
		return err
	}
	defer f.Close()

	var lastSeq uint64
	var lastHash [32]byte
	for {
		header := make([]byte, 50)
		_, err := io.ReadFull(f, header)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(header[0:4], MagicBytes) {
			return ErrInvalidMagic
		}
		if binary.BigEndian.Uint16(header[4:6]) != FormatVersion {
			return fmt.Errorf("unsupported journal format version %d", binary.BigEndian.Uint16(header[4:6]))
		}
		seq := binary.BigEndian.Uint64(header[6:14])
		var previousHash [32]byte
		copy(previousHash[:], header[18:50])
		if lastSeq == 0 {
			if seq != 1 || previousHash != ([32]byte{}) {
				return ErrOutOfOrder
			}
		} else {
			if seq != lastSeq+1 {
				return ErrOutOfOrder
			}
			if previousHash != lastHash {
				return fmt.Errorf("hash chain broken at seq %d", seq)
			}
		}

		length := binary.BigEndian.Uint32(header[14:18])
		payload := make([]byte, length)
		if _, err := io.ReadFull(f, payload); err != nil {
			return err
		}

		var checksum [32]byte
		if _, err := io.ReadFull(f, checksum[:]); err != nil {
			return err
		}

		h := sha256.New()
		h.Write(header)
		h.Write(payload)
		var computed [32]byte
		copy(computed[:], h.Sum(nil))

		if checksum != computed {
			return ErrCorruptData
		}

		if err := handler(seq, payload); err != nil {
			return err
		}
		lastSeq = seq
		lastHash = computed
	}
	return nil
}

func (j *BinaryJournal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.file.Sync()
	return j.file.Close()
}
