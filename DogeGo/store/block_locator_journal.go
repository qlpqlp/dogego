// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// Append-only locator journal replaces per-hash files during IBD.
// Core appends blk*.dat + batched LevelDB; NTFS create-per-hash was capping ~50–200 blk/min.
//
// Record layout (57 bytes):
//
//	magic u32 (0x4C4F434A "LOCJ") | hashLE[32] | locator[21]
const (
	locatorJournalMagic   = uint32(0x4C4F434A)
	locatorJournalRecLen  = 4 + 32 + blockLocatorLen
	locatorJournalFileName = "locators.jnl"
)

type locatorMem struct {
	mu   sync.RWMutex
	m    map[[32]byte]blockLocator
	file *os.File
	path string
}

func locatorJournalPath(locatorRoot string) string {
	return filepath.Join(locatorRoot, locatorJournalFileName)
}

func (s *RawBlockStore) ensureLocatorMem() *locatorMem {
	if s == nil {
		return nil
	}
	s.locMemOnce.Do(func() {
		s.locMem = &locatorMem{
			m:    make(map[[32]byte]blockLocator, 1024),
			path: locatorJournalPath(s.locatorRoot()),
		}
		_ = s.locMem.load()
	})
	return s.locMem
}

func (lm *locatorMem) load() error {
	if lm == nil {
		return nil
	}
	f, err := os.OpenFile(lm.path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	lm.file = f
	buf := make([]byte, locatorJournalRecLen)
	var goodEnd int64
	for {
		n, err := io.ReadFull(f, buf)
		if err == io.EOF {
			break
		}
		if err == io.ErrUnexpectedEOF {
			_ = f.Truncate(goodEnd)
			break
		}
		if err != nil {
			return err
		}
		if n < locatorJournalRecLen || binary.LittleEndian.Uint32(buf[0:4]) != locatorJournalMagic {
			_ = f.Truncate(goodEnd)
			break
		}
		var hash [32]byte
		copy(hash[:], buf[4:36])
		loc, err := decodeBlockLocator(buf[36 : 36+blockLocatorLen])
		if err != nil {
			_ = f.Truncate(goodEnd)
			break
		}
		lm.m[hash] = loc
		goodEnd += int64(locatorJournalRecLen)
	}
	if testing.Testing() {
		// Tests often omit Close(); keeping the handle open blocks TempDir cleanup on Windows.
		_ = f.Close()
		lm.file = nil
		return nil
	}
	_, _ = f.Seek(0, io.SeekEnd)
	return nil
}

func (lm *locatorMem) get(hash [32]byte) (blockLocator, bool) {
	if lm == nil {
		return blockLocator{}, false
	}
	lm.mu.RLock()
	loc, ok := lm.m[hash]
	lm.mu.RUnlock()
	return loc, ok
}

func (lm *locatorMem) put(hash [32]byte, loc blockLocator) error {
	if lm == nil {
		return fmt.Errorf("locator mem nil")
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.m[hash] = loc
	if lm.file == nil {
		f, err := os.OpenFile(lm.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			return err
		}
		lm.file = f
	}
	var rec [locatorJournalRecLen]byte
	binary.LittleEndian.PutUint32(rec[0:4], locatorJournalMagic)
	copy(rec[4:36], hash[:])
	enc := encodeBlockLocator(loc)
	copy(rec[36:], enc[:])
	_, err := lm.file.Write(rec[:])
	if testing.Testing() {
		_ = lm.file.Sync()
		_ = lm.file.Close()
		lm.file = nil
	}
	return err
}

func (lm *locatorMem) remove(hash [32]byte) {
	if lm == nil {
		return
	}
	lm.mu.Lock()
	delete(lm.m, hash)
	lm.mu.Unlock()
	// Journal is append-only; tombstones omitted (rare during IBD). Legacy file removed separately.
}

func (lm *locatorMem) close() error {
	if lm == nil {
		return nil
	}
	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.file == nil {
		return nil
	}
	err := lm.file.Sync()
	err2 := lm.file.Close()
	lm.file = nil
	if err != nil {
		return err
	}
	return err2
}

// commitBlockLocator stores a locator in the append journal + memory (bundled IBD fast path).
// Falls back to per-hash files only when the journal cannot be opened.
func (s *RawBlockStore) commitBlockLocator(hashLE [32]byte, loc blockLocator) error {
	if s == nil {
		return fmt.Errorf("nil store")
	}
	if err := os.MkdirAll(s.locatorRoot(), 0o700); err != nil {
		return err
	}
	lm := s.ensureLocatorMem()
	if lm != nil {
		if err := lm.put(hashLE, loc); err == nil {
			return nil
		}
	}
	return writeBlockLocator(s.locatorRoot(), hashLE, loc)
}

// lookupBlockLocator returns a locator from memory journal or legacy per-hash files.
func (s *RawBlockStore) lookupBlockLocator(hashLE [32]byte) (blockLocator, bool, error) {
	if s == nil {
		return blockLocator{}, false, nil
	}
	if lm := s.ensureLocatorMem(); lm != nil {
		if loc, ok := lm.get(hashLE); ok {
			return loc, true, nil
		}
	}
	return readBlockLocator(s.locatorRoot(), hashLE)
}
