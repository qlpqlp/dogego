// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"dogego/pow"
)

// HeaderJournal appends raw 80-byte non-auxpow headers sequentially (monolith headers.bin or segment files under headers/seg/).
type HeaderJournal struct {
	path     string // monolith path; empty when using segments
	chainDir string
	seg      *headerSegmentLayout
	mu       sync.RWMutex
	// cachedCount/cachedTip avoid Stat on every dashboard poll (-1 = refresh from disk).
	cachedCount atomic.Int64
	cachedTip   atomic.Int64
}

// OpenHeaderJournal opens or creates the journal; if empty, writes genesis80 as the first record.
func OpenHeaderJournal(path string, genesis80 []byte) (*HeaderJournal, error) {
	if len(genesis80) != 80 {
		return nil, fmt.Errorf("genesis header must be 80 bytes")
	}
	j := &HeaderJournal{path: path, chainDir: filepath.Dir(path)}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() == 0 {
		if _, err := f.Write(genesis80[:]); err != nil {
			return nil, err
		}
		if err := f.Sync(); err != nil {
			return nil, err
		}
	} else if st.Size()%80 != 0 {
		if err := repairPartialHeaderJournalTail(f, st.Size()); err != nil {
			return nil, err
		}
	}
	j.refreshCountCache()
	return j, nil
}

func (j *HeaderJournal) invalidateCountCache() {
	j.cachedCount.Store(-1)
	j.cachedTip.Store(-1)
}

func (j *HeaderJournal) refreshCountCache() {
	if j == nil {
		return
	}
	j.mu.RLock()
	j.reconcileCountCacheLocked()
	j.mu.RUnlock()
}

// reconcileCountCacheLocked updates cachedCount/cachedTip from on-disk size.
// Caller must hold j.mu (read or write).
func (j *HeaderJournal) reconcileCountCacheLocked() {
	if j == nil {
		return
	}
	if j.seg != nil {
		if m, ok := readSegmentManifestFile(j.chainDir); ok {
			n := m.TipHeight + 1
			j.cachedCount.Store(n)
			j.cachedTip.Store(m.TipHeight)
			return
		}
		j.seg.mu.RLock()
		n := j.seg.countLocked()
		j.seg.mu.RUnlock()
		j.cachedCount.Store(n)
		if n > 0 {
			j.cachedTip.Store(n - 1)
		} else {
			j.cachedTip.Store(-1)
		}
		return
	}
	st, err := os.Stat(j.path)
	if err != nil || st.Size()%80 != 0 {
		j.invalidateCountCache()
		return
	}
	n := st.Size() / 80
	j.cachedCount.Store(n)
	j.cachedTip.Store(n - 1)
}

// ReconcileCountCacheFromDisk refreshes the tip/count cache from headers.bin (used before truncate/prune).
func (j *HeaderJournal) ReconcileCountCacheFromDisk() {
	if j == nil {
		return
	}
	j.mu.RLock()
	j.reconcileCountCacheLocked()
	j.mu.RUnlock()
}

// repairPartialHeaderJournalTail drops a torn trailing write (common after force-kill during append).
func repairPartialHeaderJournalTail(f *os.File, size int64) error {
	keep := (size / 80) * 80
	if keep < 80 {
		return fmt.Errorf("corrupt header journal size %d (less than one header)", size)
	}
	if err := f.Truncate(keep); err != nil {
		return fmt.Errorf("corrupt header journal size %d: truncate: %w", size, err)
	}
	if err := f.Sync(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "DogeGo: repaired headers.bin (dropped %d-byte partial record at end; tip height now %d). "+
		"Sync will continue from the last complete header.\n", size-keep, keep/80-1)
	return nil
}

// Count returns the number of stored headers (including genesis).
func (j *HeaderJournal) Count() (int64, error) {
	if j == nil {
		return 0, fmt.Errorf("nil journal")
	}
	if j.seg != nil {
		j.seg.mu.RLock()
		c := j.seg.countLocked()
		j.seg.mu.RUnlock()
		j.cachedCount.Store(c)
		if c > 0 {
			j.cachedTip.Store(c - 1)
		} else {
			j.cachedTip.Store(-1)
		}
		return c, nil
	}
	st, err := os.Stat(j.path)
	if err != nil {
		return 0, err
	}
	if st.Size()%80 != 0 {
		return 0, fmt.Errorf("bad journal size %d", st.Size())
	}
	disk := st.Size() / 80
	if disk == 0 {
		return 0, nil
	}
	n := j.cachedCount.Load()
	if n >= 0 && n == disk {
		return n, nil
	}
	j.mu.Lock()
	j.reconcileCountCacheLocked()
	n = j.cachedCount.Load()
	j.mu.Unlock()
	return n, nil
}

// DiskTip returns the header tip from on-disk segment manifest without journal locks.
func (j *HeaderJournal) DiskTip() (int64, bool) {
	if j == nil {
		return -1, false
	}
	if j.HeaderLayout() == headerLayoutSegments {
		return ReadSegmentManifestTip(j.ChainDir())
	}
	tip, _, err := j.SyncTipFromDisk()
	return tip, err == nil && tip >= 0
}

// SyncTipFromDisk refreshes cached tip/count from on-disk header storage.
// Call before dashboard/RPC reads while dedicated or background header sync appends on other goroutines.
func (j *HeaderJournal) SyncTipFromDisk() (tip int64, count int64, err error) {
	if j == nil {
		return -1, 0, fmt.Errorf("nil journal")
	}
	if j.seg != nil {
		if err := j.seg.reloadManifestFromDisk(); err != nil {
			return -1, 0, err
		}
		if cp, cpErr := LoadHeaderSyncCheckpoint(j.chainDir); cpErr == nil && cp.HeaderCount > 0 {
			j.seg.mu.RLock()
			cur := j.seg.countLocked()
			j.seg.mu.RUnlock()
			if cur < cp.HeaderCount {
				j.seg.mu.Lock()
				j.seg.manifest.TipHeight = cp.TipHeight
				if cp.TipHashHex != "" {
					j.seg.manifest.TipHashHex = cp.TipHashHex
				}
				j.seg.mu.Unlock()
			}
		}
	}
	count, err = j.Count()
	if err != nil {
		return -1, 0, err
	}
	if count == 0 {
		return -1, 0, nil
	}
	return count - 1, count, nil
}

// LastTipHash returns the block hash of the last stored header (LE uint256 bytes).
func (j *HeaderJournal) LastTipHash() ([32]byte, error) {
	if j.seg != nil {
		j.seg.mu.RLock()
		defer j.seg.mu.RUnlock()
		tip := j.seg.manifest.TipHeight
		if tip < 0 {
			return [32]byte{}, fmt.Errorf("journal too small")
		}
		h80, err := j.seg.readAtUnlocked(tip)
		if err != nil {
			return [32]byte{}, err
		}
		return pow.BlockHashLE(h80), nil
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	f, err := os.Open(j.path)
	if err != nil {
		return [32]byte{}, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return [32]byte{}, err
	}
	if st.Size() < 80 {
		return [32]byte{}, fmt.Errorf("journal too small")
	}
	if _, err := f.Seek(st.Size()-80, io.SeekStart); err != nil {
		return [32]byte{}, err
	}
	var buf [80]byte
	if _, err := io.ReadFull(f, buf[:]); err != nil {
		return [32]byte{}, err
	}
	return pow.BlockHashLE(buf[:]), nil
}

// AppendHeaders appends validated 80-byte headers after the current tip (one syscall + sync per batch).
func (j *HeaderJournal) AppendHeaders(headers [][]byte) error {
	if len(headers) == 0 {
		return nil
	}
	buf := make([]byte, 0, len(headers)*80)
	for i, h := range headers {
		if len(h) != 80 {
			return fmt.Errorf("bad header len %d at index %d", len(h), i)
		}
		buf = append(buf, h...)
	}
	return j.appendHeaderBytes(buf)
}

// AppendWireHeaderBatch appends a contiguous wire80×N buffer (one write+sync per batch).
func (j *HeaderJournal) AppendWireHeaderBatch(buf []byte) error {
	return j.appendHeaderBytes(buf)
}

func (j *HeaderJournal) appendHeaderBytes(b []byte) error {
	if len(b) == 0 || len(b)%80 != 0 {
		return fmt.Errorf("bad header batch size %d", len(b))
	}
	if j.seg != nil {
		if err := j.seg.appendBatch(b); err != nil {
			return err
		}
		j.refreshCountCache()
		return nil
	}
	j.mu.Lock()
	f, err := os.OpenFile(j.path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		j.mu.Unlock()
		return err
	}
	_, werr := f.Write(b)
	if serr := f.Sync(); serr != nil && werr == nil {
		werr = serr
	}
	if cerr := f.Close(); cerr != nil && werr == nil {
		werr = cerr
	}
	if werr == nil {
		j.reconcileCountCacheLocked()
	}
	j.mu.Unlock()
	if werr == nil && j.chainDir != "" {
		count := j.cachedCount.Load()
		if count > 0 {
			last := b[len(b)-80:]
			_ = SaveHeaderSyncCheckpoint(j.chainDir, HeaderSyncCheckpoint{
				Layout:       "monolith",
				TipHeight:    count - 1,
				HeaderCount:  count,
				JournalBytes: count * 80,
				TipHashHex:   pow.BlockHashHex(last),
			})
		}
	}
	return werr
}

// ReadAll returns every stored header (including genesis), in height order.
func (j *HeaderJournal) ReadAll() ([][]byte, error) {
	if j.seg != nil {
		return j.seg.readAllHeaders()
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	b, err := os.ReadFile(j.path)
	if err != nil {
		return nil, err
	}
	if len(b)%80 != 0 {
		return nil, fmt.Errorf("bad journal size %d", len(b))
	}
	n := len(b) / 80
	out := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		slice := b[i*80 : (i+1)*80]
		cp := make([]byte, 80)
		copy(cp, slice)
		out = append(out, cp)
	}
	return out, nil
}

// GenesisHashHex returns display hex of genesis block id from first journal record.
func (j *HeaderJournal) GenesisHashHex() (string, error) {
	h80, err := j.ReadHeaderAt(0)
	if err != nil {
		return "", err
	}
	return pow.BlockHashHex(h80), nil
}

// TipHeight is Count()-1.
func (j *HeaderJournal) TipHeight() (int64, error) {
	c, err := j.Count()
	if err != nil {
		return 0, err
	}
	return c - 1, nil
}

// BestBlockHashHex returns display hex of tip hash.
func (j *HeaderJournal) BestBlockHashHex() (string, error) {
	if j.seg != nil {
		j.seg.mu.RLock()
		h := j.seg.manifest.TipHashHex
		j.seg.mu.RUnlock()
		if h != "" {
			return h, nil
		}
	}
	tip, err := j.LastTipHash()
	if err != nil {
		return "", err
	}
	// display reverse of internal LE
	var rev [32]byte
	for i := 0; i < 32; i++ {
		rev[i] = tip[31-i]
	}
	return fmt.Sprintf("%x", rev), nil
}

// Path returns the backing monolith path or headers/manifest.json for segment layout.
func (j *HeaderJournal) Path() string {
	if j == nil {
		return ""
	}
	if j.seg != nil {
		return headerManifestPath(j.chainDir)
	}
	return j.path
}

// ReadHeadersThrough returns a contiguous snapshot of raw headers from height 0 through through (inclusive).
// The slice length is (through+1)*80. The journal mutex is held for the duration of the read so the view
// is consistent with a single on-disk size (no torn records mid-append).
func (j *HeaderJournal) ReadHeadersThrough(through int64) ([]byte, error) {
	if through < 0 {
		return nil, fmt.Errorf("negative through height %d", through)
	}
	if j.seg != nil {
		return j.seg.readHeadersThrough(through)
	}
	need := (through + 1) * 80
	j.mu.RLock()
	defer j.mu.RUnlock()
	st, err := os.Stat(j.path)
	if err != nil {
		return nil, err
	}
	if st.Size()%80 != 0 {
		return nil, fmt.Errorf("corrupt header journal size %d", st.Size())
	}
	if need > st.Size() {
		return nil, fmt.Errorf("height %d beyond journal (size %d bytes)", through, st.Size())
	}
	f, err := os.Open(j.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, need)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// ReadHeaderAt returns the 80-byte header at chain height (0 = genesis).
func (j *HeaderJournal) ReadHeaderAt(height int64) ([]byte, error) {
	if j.seg != nil {
		return j.seg.readAt(height)
	}
	if height < 0 {
		return nil, fmt.Errorf("negative height %d", height)
	}
	off := height * 80
	j.mu.RLock()
	defer j.mu.RUnlock()
	f, err := os.Open(j.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if off+80 > st.Size() {
		return nil, fmt.Errorf("height %d out of range (size %d)", height, st.Size())
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, err
	}
	buf := make([]byte, 80)
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// BuildBlockLocator returns a Bitcoin-style block locator (newest first), up to max entries (max 101).
func (j *HeaderJournal) BuildBlockLocator(max int) ([][32]byte, error) {
	if max < 1 || max > 101 {
		max = 101
	}
	all, err := j.ReadAll()
	if err != nil {
		return nil, err
	}
	n := len(all)
	if n == 0 {
		return nil, fmt.Errorf("empty chain")
	}
	curH := int64(n - 1)
	step := 1
	var out [][32]byte
	for len(out) < max {
		buf := all[curH]
		h := pow.BlockHashLE(buf)
		out = append(out, h)
		if curH == 0 {
			break
		}
		nHeight := curH - int64(step)
		if nHeight < 0 {
			nHeight = 0
		}
		for curH > nHeight {
			curH--
		}
		if len(out) > 10 {
			step *= 2
		}
	}
	return out, nil
}

// BuildBlockLocatorFromHeight returns a block locator from fromHeight down toward genesis (newest first).
func (j *HeaderJournal) BuildBlockLocatorFromHeight(fromHeight int64, max int) ([][32]byte, error) {
	if max < 1 || max > 101 {
		max = 101
	}
	tipH, err := j.TipHeight()
	if err != nil {
		return nil, err
	}
	if fromHeight > tipH {
		fromHeight = tipH
	}
	if fromHeight < 0 {
		fromHeight = 0
	}
	all, err := j.ReadAll()
	if err != nil {
		return nil, err
	}
	n := len(all)
	if n == 0 {
		return nil, fmt.Errorf("empty chain")
	}
	curH := fromHeight
	step := int64(1)
	var out [][32]byte
	for len(out) < max {
		if curH >= int64(n) {
			return nil, fmt.Errorf("locator height %d out of range (count %d)", curH, n)
		}
		buf := all[curH]
		h := pow.BlockHashLE(buf)
		out = append(out, h)
		if curH == 0 {
			break
		}
		nHeight := curH - step
		if nHeight < 0 {
			nHeight = 0
		}
		for curH > nHeight {
			curH--
		}
		if len(out) > 10 {
			step *= 2
		}
	}
	return out, nil
}

// HeightByBlockHashLE scans the journal for a header whose block hash equals hashLE (internal LE uint256).
func (j *HeaderJournal) HeightByBlockHashLE(hashLE [32]byte) (int64, error) {
	if j.seg != nil {
		return j.seg.heightByHashLE(hashLE)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	st, err := os.Stat(j.path)
	if err != nil {
		return -1, err
	}
	n := st.Size() / 80
	f, err := os.Open(j.path)
	if err != nil {
		return -1, err
	}
	defer f.Close()
	buf := make([]byte, 80)
	for h := int64(0); h < n; h++ {
		if _, err := io.ReadFull(f, buf); err != nil {
			return -1, err
		}
		if pow.BlockHashLE(buf) == hashLE {
			return h, nil
		}
	}
	return -1, fmt.Errorf("block hash not in header journal")
}

// TruncateToHeight removes headers above inclusiveHeight, keeping records 0..inclusiveHeight.
func (j *HeaderJournal) TruncateToHeight(inclusiveHeight int64) error {
	if j.seg != nil {
		if err := j.seg.truncateTo(inclusiveHeight); err != nil {
			return err
		}
		j.refreshCountCache()
		return nil
	}
	if inclusiveHeight < 0 {
		return fmt.Errorf("negative truncate height %d", inclusiveHeight)
	}
	wantSize := (inclusiveHeight + 1) * 80
	j.mu.Lock()
	defer j.mu.Unlock()
	st, err := os.Stat(j.path)
	if err != nil {
		return err
	}
	if st.Size()%80 != 0 {
		return fmt.Errorf("corrupt header journal size %d", st.Size())
	}
	if st.Size() == wantSize {
		return nil
	}
	if st.Size() < wantSize {
		return fmt.Errorf("truncate height %d beyond journal tip %d", inclusiveHeight, st.Size()/80-1)
	}
	f, err := os.OpenFile(j.path, os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := f.Truncate(wantSize); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	j.reconcileCountCacheLocked()
	return nil
}

// HeightByDisplayHash returns the chain height whose block hash matches display hex (case-insensitive), or an error if not found.
func (j *HeaderJournal) HeightByDisplayHash(displayHex string) (int64, error) {
	displayHex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(displayHex), "0x"))
	if len(displayHex) != 64 {
		return -1, fmt.Errorf("block hash must be 64 hex chars")
	}
	if j.seg != nil {
		return j.seg.heightByDisplayHash(displayHex)
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	st, err := os.Stat(j.path)
	if err != nil {
		return -1, err
	}
	n := st.Size() / 80
	f, err := os.Open(j.path)
	if err != nil {
		return -1, err
	}
	defer f.Close()
	buf := make([]byte, 80)
	for h := int64(0); h < n; h++ {
		if _, err := io.ReadFull(f, buf); err != nil {
			return -1, err
		}
		if strings.EqualFold(pow.BlockHashHex(buf), displayHex) {
			return h, nil
		}
	}
	return -1, fmt.Errorf("block not in header journal")
}
