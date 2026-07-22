// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// BlockFilterIndex stores BIP158 basic filters under <chain>/filters/basic/ (one file per block).
type BlockFilterIndex struct {
	dir string
	mu  sync.Mutex
}

// OpenBlockFilterIndex creates <chainDataDir>/filters/basic/.
func OpenBlockFilterIndex(chainDataDir string) (*BlockFilterIndex, error) {
	d := filepath.Join(chainDataDir, "filters", "basic")
	if err := os.MkdirAll(d, 0o700); err != nil {
		return nil, err
	}
	return &BlockFilterIndex{dir: d}, nil
}

func (f *BlockFilterIndex) pathFor(hashLE [32]byte) string {
	return filepath.Join(f.dir, hex.EncodeToString(hashLE[:])+".dat")
}

// Put stores encoded filter bytes and the 32-byte filter header.
func (f *BlockFilterIndex) Put(hashLE [32]byte, encoded, header []byte) error {
	if f == nil {
		return nil
	}
	if len(header) != 32 {
		return fmt.Errorf("filter header must be 32 bytes")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	tmp := f.pathFor(hashLE) + ".tmp"
	payload := make([]byte, 32+len(encoded))
	copy(payload[:32], header)
	copy(payload[32:], encoded)
	if err := os.WriteFile(tmp, payload, 0o600); err != nil {
		return err
	}
	if stallAfterBlockFilterPutTmpWrite > 0 {
		time.Sleep(stallAfterBlockFilterPutTmpWrite)
	}
	return os.Rename(tmp, f.pathFor(hashLE))
}

// Get loads a stored filter and header.
func (f *BlockFilterIndex) Get(hashLE [32]byte) (encoded, header []byte, err error) {
	if f == nil {
		return nil, nil, fmt.Errorf("filter index disabled")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	b, err := os.ReadFile(f.pathFor(hashLE))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, fmt.Errorf("filter not found")
		}
		return nil, nil, err
	}
	if len(b) < 32 {
		return nil, nil, fmt.Errorf("corrupt filter file")
	}
	hdr := make([]byte, 32)
	copy(hdr, b[:32])
	return append([]byte(nil), b[32:]...), hdr, nil
}

// Has reports whether a filter file exists for this block.
func (f *BlockFilterIndex) Has(hashLE [32]byte) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	_, err := os.Stat(f.pathFor(hashLE))
	return err == nil
}

// Dir returns the filters/basic directory path.
func (f *BlockFilterIndex) Dir() string {
	if f == nil {
		return ""
	}
	return f.dir
}

// Count returns the number of stored filter files.
func (f *BlockFilterIndex) Count() (int, error) {
	if f == nil {
		return 0, nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ents, err := os.ReadDir(f.dir)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, e := range ents {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".dat" {
			n++
		}
	}
	return n, nil
}

// Remove deletes a stored filter (no error if absent).
func (f *BlockFilterIndex) Remove(hashLE [32]byte) error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	err := os.Remove(f.pathFor(hashLE))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
