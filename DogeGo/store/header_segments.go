// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"dogego/pow"
)

// HeaderSegmentSize matches P2P MAX_HEADERS_RESULTS - one atomic segment file per batch.
const HeaderSegmentSize = 2000

const (
	headerLayoutDir      = "headers"
	headerSegmentSubdir  = "seg"
	headerManifestName   = "manifest.json"
	headerLayoutSegments = "segments"
)

type headerManifest struct {
	Version     int    `json:"version"`
	SegmentSize int    `json:"segment_size"`
	TipHeight   int64  `json:"tip_height"`
	TipHashHex  string `json:"tip_hash_hex"`
}

// headerSegmentLayout stores headers as fixed-size segment files (like rawblocks/ per block).
type headerSegmentLayout struct {
	chainDir string
	segDir   string
	mu       sync.RWMutex
	manifest headerManifest
	// segCache avoids reopening the same segment file on every ReadHeaderAt (MTP uses up to 11 reads/block).
	segCacheMu    sync.Mutex
	segCacheStart int64
	segCacheBytes []byte
}

func headerManifestPath(chainDir string) string {
	return filepath.Join(chainDir, headerLayoutDir, headerManifestName)
}

func openHeaderSegmentLayout(chainDir string) (*headerSegmentLayout, error) {
	segDir := filepath.Join(chainDir, headerLayoutDir, headerSegmentSubdir)
	if err := os.MkdirAll(segDir, 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(headerManifestPath(chainDir) + ".tmp")
	l := &headerSegmentLayout{chainDir: chainDir, segDir: segDir}
	mp := headerManifestPath(chainDir)
	b, err := os.ReadFile(mp)
	if err != nil {
		if os.IsNotExist(err) {
			l.manifest = headerManifest{Version: 1, SegmentSize: HeaderSegmentSize, TipHeight: -1}
			return l, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(b, &l.manifest); err != nil {
		return nil, fmt.Errorf("header manifest: %w", err)
	}
	if l.manifest.SegmentSize <= 0 {
		l.manifest.SegmentSize = HeaderSegmentSize
	}
	if err := l.repairSegmentTailsOnOpen(); err != nil {
		return nil, err
	}
	return l, nil
}

// repairSegmentTailsOnOpen drops torn trailing bytes in the tip segment (force-kill during append).
func (l *headerSegmentLayout) repairSegmentTailsOnOpen() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.manifest.TipHeight < 0 {
		return nil
	}
	segSize := int64(l.manifest.SegmentSize)
	segStart := (l.manifest.TipHeight / segSize) * segSize
	path := l.segmentPath(segStart)
	st, err := os.Stat(path)
	if err != nil {
		return nil
	}
	size := st.Size()
	if size%80 == 0 {
		return nil
	}
	keep := (size / 80) * 80
	if keep > 0 {
		f, err := os.OpenFile(path, os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		if err := f.Truncate(keep); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
	} else {
		if err := os.Remove(path); err != nil {
			return err
		}
	}
	newTip := segStart + keep/80 - 1
	if newTip < 0 {
		l.manifest.TipHeight = -1
		l.manifest.TipHashHex = ""
	} else if newTip < l.manifest.TipHeight {
		l.manifest.TipHeight = newTip
		h80, err := l.readAtUnlocked(newTip)
		if err != nil {
			return err
		}
		l.manifest.TipHashHex = pow.BlockHashHex(h80)
	}
	return l.saveManifestLocked()
}

func (l *headerSegmentLayout) segmentPath(startHeight int64) string {
	return filepath.Join(l.segDir, fmt.Sprintf("%010d.bin", startHeight))
}

func (l *headerSegmentLayout) heightByHashLE(hashLE [32]byte) (int64, error) {
	if m, ok := readSegmentManifestFile(l.chainDir); ok {
		return l.heightByHashLEThrough(m.TipHeight, hashLE)
	}
	l.mu.RLock()
	tip := l.manifest.TipHeight
	l.mu.RUnlock()
	return l.heightByHashLEThrough(tip, hashLE)
}

func (l *headerSegmentLayout) heightByHashLEThrough(tip int64, hashLE [32]byte) (int64, error) {
	if tip < 0 {
		return -1, fmt.Errorf("block hash not in header journal")
	}
	segSize := int64(HeaderSegmentSize)
	for segStart := int64(0); segStart <= tip; segStart += segSize {
		path := l.segmentPath(segStart)
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return -1, err
		}
		for i := 0; i+80 <= len(b); i += 80 {
			h := segStart + int64(i/80)
			if h > tip {
				break
			}
			if pow.BlockHashLE(b[i:i+80]) == hashLE {
				return h, nil
			}
		}
	}
	return -1, fmt.Errorf("block hash not in header journal")
}

func (l *headerSegmentLayout) heightByDisplayHash(displayHex string) (int64, error) {
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	if len(displayHex) != 64 {
		return -1, fmt.Errorf("block hash must be 64 hex chars")
	}
	tip := int64(-1)
	if m, ok := readSegmentManifestFile(l.chainDir); ok {
		tip = m.TipHeight
	}
	if tip < 0 {
		l.mu.RLock()
		tip = l.manifest.TipHeight
		l.mu.RUnlock()
	}
	if tip < 0 {
		return -1, fmt.Errorf("block not in header journal")
	}
	segSize := int64(HeaderSegmentSize)
	for segStart := int64(0); segStart <= tip; segStart += segSize {
		path := l.segmentPath(segStart)
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return -1, err
		}
		for i := 0; i+80 <= len(b); i += 80 {
			h := segStart + int64(i/80)
			if h > tip {
				break
			}
			if strings.EqualFold(pow.BlockHashHex(b[i:i+80]), displayHex) {
				return h, nil
			}
		}
	}
	return -1, fmt.Errorf("block not in header journal")
}

func (l *headerSegmentLayout) readAllHeaders() ([][]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if l.manifest.TipHeight < 0 {
		return nil, nil
	}
	entries, err := os.ReadDir(l.segDir)
	if err != nil {
		return nil, err
	}
	type segFile struct {
		start int64
		name  string
	}
	var segs []segFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bin") || strings.HasSuffix(e.Name(), ".tmp") {
			continue
		}
		var start int64
		if _, err := fmt.Sscanf(e.Name(), "%010d.bin", &start); err != nil {
			continue
		}
		segs = append(segs, segFile{start: start, name: e.Name()})
	}
	for i := 0; i < len(segs); i++ {
		for j := i + 1; j < len(segs); j++ {
			if segs[j].start < segs[i].start {
				segs[i], segs[j] = segs[j], segs[i]
			}
		}
	}
	var out [][]byte
	for _, s := range segs {
		b, err := os.ReadFile(filepath.Join(l.segDir, s.name))
		if err != nil {
			return nil, err
		}
		for i := 0; i+80 <= len(b); i += 80 {
			cp := make([]byte, 80)
			copy(cp, b[i:i+80])
			out = append(out, cp)
		}
	}
	return out, nil
}

// readHeadersThrough returns raw header bytes for heights 0..through without loading the full journal.
func (l *headerSegmentLayout) readHeadersThrough(through int64) ([]byte, error) {
	if through < 0 {
		return nil, fmt.Errorf("negative through height %d", through)
	}
	l.mu.RLock()
	tip := l.manifest.TipHeight
	segSize := int64(l.manifest.SegmentSize)
	if segSize <= 0 {
		segSize = HeaderSegmentSize
	}
	l.mu.RUnlock()
	if tip >= 0 && through > tip {
		return nil, fmt.Errorf("height %d beyond journal tip %d", through, tip)
	}
	buf := make([]byte, (through+1)*80)
	for segStart := int64(0); segStart <= through; segStart += segSize {
		b, err := os.ReadFile(l.segmentPath(segStart))
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("missing header segment from height %d", segStart)
			}
			return nil, err
		}
		for i := 0; i+80 <= len(b); i += 80 {
			h := segStart + int64(i/80)
			if h > through {
				break
			}
			copy(buf[h*80:(h+1)*80], b[i:i+80])
		}
	}
	return buf, nil
}

func (l *headerSegmentLayout) countLocked() int64 {
	if l.manifest.TipHeight < 0 {
		return 0
	}
	return l.manifest.TipHeight + 1
}

func (l *headerSegmentLayout) recordCount() int64 {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.countLocked()
}

func (l *headerSegmentLayout) tipHeightLocked() int64 {
	return l.manifest.TipHeight
}

// reloadManifestFromDisk refreshes in-memory tip from headers/manifest.json (dashboard/RPC while sync appends).
func (l *headerSegmentLayout) reloadManifestFromDisk() error {
	mp := headerManifestPath(l.chainDir)
	b, err := os.ReadFile(mp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var m headerManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("header manifest: %w", err)
	}
	if m.SegmentSize <= 0 {
		m.SegmentSize = HeaderSegmentSize
	}
	l.mu.Lock()
	l.manifest = m
	l.mu.Unlock()
	return nil
}

func (l *headerSegmentLayout) saveManifestLocked() error {
	if l.manifest.SegmentSize <= 0 {
		l.manifest.SegmentSize = HeaderSegmentSize
	}
	b, err := json.Marshal(l.manifest)
	if err != nil {
		return err
	}
	mp := headerManifestPath(l.chainDir)
	return atomicWriteFile(mp, b, 0o600)
}

func (l *headerSegmentLayout) writeGenesis(genesis80 []byte) error {
	if len(genesis80) != 80 {
		return fmt.Errorf("genesis header must be 80 bytes")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.countLocked() > 0 {
		return nil
	}
	path := l.segmentPath(0)
	if err := atomicWriteFileStall(path, genesis80, 0o600, stallAfterHeaderSegTmpWrite); err != nil {
		return err
	}
	l.invalidateSegCache()
	l.manifest.TipHeight = 0
	l.manifest.TipHashHex = pow.BlockHashHex(genesis80)
	if err := l.saveManifestLocked(); err != nil {
		_ = l.reloadManifestFromDisk()
		return err
	}
	return SaveHeaderSyncCheckpoint(l.chainDir, HeaderSyncCheckpoint{
		Layout: headerLayoutSegments, TipHeight: 0, HeaderCount: 1, TipHashHex: l.manifest.TipHashHex,
	})
}

func (l *headerSegmentLayout) appendBatch(buf []byte) error {
	if len(buf) == 0 || len(buf)%80 != 0 {
		return fmt.Errorf("bad header batch size %d", len(buf))
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.countLocked() == 0 {
		l.mu.Unlock()
		if err := l.writeGenesis(buf[:80]); err != nil {
			return err
		}
		l.mu.Lock()
		buf = buf[80:]
		if len(buf) == 0 {
			return nil
		}
	}
	off := 0
	for off < len(buf) {
		nextH := l.manifest.TipHeight + 1
		segStart := (nextH / int64(l.manifest.SegmentSize)) * int64(l.manifest.SegmentSize)
		posInSeg := int(nextH - segStart)
		room := (l.manifest.SegmentSize - posInSeg) * 80
		if room <= 0 {
			return fmt.Errorf("segment math: height %d", nextH)
		}
		chunk := len(buf) - off
		if chunk > room {
			chunk = room
		}
		path := l.segmentPath(segStart)
		var existing []byte
		if posInSeg > 0 {
			var err error
			existing, err = os.ReadFile(path)
			if err != nil {
				return err
			}
		}
		out := append(existing, buf[off:off+chunk]...)
		if err := atomicWriteFileStall(path, out, 0o600, stallAfterHeaderSegTmpWrite); err != nil {
			return err
		}
		l.invalidateSegCache()
		off += chunk
		nHdr := chunk / 80
		l.manifest.TipHeight += int64(nHdr)
	}
	tipH80, err := l.readAtUnlocked(l.manifest.TipHeight)
	if err != nil {
		return err
	}
	l.manifest.TipHashHex = pow.BlockHashHex(tipH80)
	if err := l.saveManifestLocked(); err != nil {
		_ = l.reloadManifestFromDisk()
		return err
	}
	return SaveHeaderSyncCheckpoint(l.chainDir, HeaderSyncCheckpoint{
		Layout:      headerLayoutSegments,
		TipHeight:   l.manifest.TipHeight,
		HeaderCount: l.countLocked(),
		TipHashHex:  l.manifest.TipHashHex,
	})
}

func (l *headerSegmentLayout) readAt(height int64) ([]byte, error) {
	if height < 0 {
		return nil, fmt.Errorf("negative height %d", height)
	}
	tip := int64(-1)
	if m, ok := readSegmentManifestFile(l.chainDir); ok {
		tip = m.TipHeight
	}
	if tip < 0 {
		l.mu.RLock()
		tip = l.manifest.TipHeight
		l.mu.RUnlock()
	}
	if height > tip {
		return nil, fmt.Errorf("height %d out of range (tip %d)", height, tip)
	}
	return l.readAtUnlocked(height)
}

func (l *headerSegmentLayout) truncateTo(inclusiveHeight int64) error {
	if inclusiveHeight < 0 {
		return fmt.Errorf("negative truncate height %d", inclusiveHeight)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.manifest.TipHeight <= inclusiveHeight {
		return nil
	}
	segSize := int64(l.manifest.SegmentSize)
	keepStart := (inclusiveHeight / segSize) * segSize
	entries, err := os.ReadDir(l.segDir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".bin") {
			continue
		}
		var start int64
		if _, err := fmt.Sscanf(e.Name(), "%010d.bin", &start); err != nil {
			continue
		}
		if start > keepStart {
			_ = os.Remove(filepath.Join(l.segDir, e.Name()))
		}
	}
	lastPath := l.segmentPath(keepStart)
	wantBytes := int((inclusiveHeight - keepStart + 1) * 80)
	st, err := os.Stat(lastPath)
	if err != nil {
		return err
	}
	if int(st.Size()) > wantBytes {
		f, err := os.OpenFile(lastPath, os.O_RDWR, 0o600)
		if err != nil {
			return err
		}
		if err := f.Truncate(int64(wantBytes)); err != nil {
			_ = f.Close()
			return err
		}
		if err := f.Sync(); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
	}
	h80, err := l.readAtUnlocked(inclusiveHeight)
	if err != nil {
		return err
	}
	l.invalidateSegCache()
	l.manifest.TipHeight = inclusiveHeight
	l.manifest.TipHashHex = pow.BlockHashHex(h80)
	if err := l.saveManifestLocked(); err != nil {
		_ = l.reloadManifestFromDisk()
		return err
	}
	return SaveHeaderSyncCheckpoint(l.chainDir, HeaderSyncCheckpoint{
		Layout:      headerLayoutSegments,
		TipHeight:   inclusiveHeight,
		HeaderCount: inclusiveHeight + 1,
		TipHashHex:  l.manifest.TipHashHex,
	})
}

func (l *headerSegmentLayout) invalidateSegCache() {
	l.segCacheMu.Lock()
	l.segCacheStart = -1
	l.segCacheBytes = nil
	l.segCacheMu.Unlock()
}

func (l *headerSegmentLayout) readAtUnlocked(height int64) ([]byte, error) {
	segStart := (height / int64(l.manifest.SegmentSize)) * int64(l.manifest.SegmentSize)
	posInSeg := int(height - segStart)
	off := posInSeg * 80
	end := off + 80

	l.segCacheMu.Lock()
	if l.segCacheStart == segStart && len(l.segCacheBytes) >= end {
		buf := make([]byte, 80)
		copy(buf, l.segCacheBytes[off:end])
		l.segCacheMu.Unlock()
		return buf, nil
	}
	l.segCacheMu.Unlock()

	path := l.segmentPath(segStart)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < end {
		return nil, fmt.Errorf("segment %d short read at height %d", segStart, height)
	}
	l.segCacheMu.Lock()
	l.segCacheStart = segStart
	l.segCacheBytes = data
	buf := make([]byte, 80)
	copy(buf, data[off:end])
	l.segCacheMu.Unlock()
	return buf, nil
}

func (l *headerSegmentLayout) purgeStaleTemps() (int, error) {
	entries, err := os.ReadDir(l.segDir)
	if err != nil {
		return 0, err
	}
	var n int
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			if err := os.Remove(filepath.Join(l.segDir, e.Name())); err == nil {
				n++
			}
		}
	}
	return n, nil
}

func (l *headerSegmentLayout) repairFromCheckpoint() error {
	cp, err := LoadHeaderSyncCheckpoint(l.chainDir)
	if err != nil || cp.Layout != headerLayoutSegments || cp.TipHeight < 0 {
		return err
	}
	l.mu.RLock()
	tooHigh := l.manifest.TipHeight > cp.TipHeight
	l.mu.RUnlock()
	if !tooHigh {
		return nil
	}
	return l.truncateTo(cp.TipHeight)
}

// migrateMonolithToSegments copies headers.bin into segment files and renames the monolith to .legacy.
func migrateMonolithToSegments(chainDir, monolithPath string) (*headerSegmentLayout, error) {
	f, err := os.Open(monolithPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size()%80 != 0 {
		return nil, fmt.Errorf("headers.bin size %d not aligned", st.Size())
	}
	l, err := openHeaderSegmentLayout(chainDir)
	if err != nil {
		return nil, err
	}
	if n, _ := l.purgeStaleTemps(); n > 0 {
		fmt.Fprintf(os.Stderr, "DogeGo: removed %d stale header segment .tmp file(s)\n", n)
	}
	buf := make([]byte, HeaderSegmentSize*80)
	var total int64
	for {
		n, rerr := io.ReadFull(f, buf)
		if rerr == io.EOF || rerr == io.ErrUnexpectedEOF {
			if n == 0 {
				break
			}
			if n%80 != 0 {
				return nil, fmt.Errorf("partial header record at end of headers.bin")
			}
			if err := l.appendBatch(buf[:n]); err != nil {
				return nil, err
			}
			total += int64(n / 80)
			break
		}
		if rerr != nil {
			return nil, rerr
		}
		if err := l.appendBatch(buf); err != nil {
			return nil, err
		}
		total += HeaderSegmentSize
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	legacy := monolithPath + ".legacy"
	if err := os.Rename(monolithPath, legacy); err != nil {
		return nil, fmt.Errorf("migrate headers: rename legacy: %w", err)
	}
	fmt.Fprintf(os.Stderr, "DogeGo: migrated %d header(s) from headers.bin to %s/ (monolith → %s)\n",
		total, headerLayoutDir, filepath.Base(legacy))
	return l, nil
}
