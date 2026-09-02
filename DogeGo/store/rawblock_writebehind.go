// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Per-file IBD used to WriteFile+Rename on the P2P read goroutine. NTFS create/rename per
// tiny Dogecoin body serialized getdata (~80 blk/min live). Stage in RAM, mark the body
// present immediately, and drain to disk on a worker pool (Core keeps blocks in memory
// too; its blk*.dat append is just cheaper than one file per height).
const (
	ibdWriteBehindMaxBytes = 512 << 20 // 512 MiB RAM ingest buffer during per-file IBD
	ibdWriteBehindWorkers  = 16
	ibdWriteBehindQueue    = 65536
)

type ibdWriteJob struct {
	hash [32]byte
	raw  []byte
}

type ibdWriteBehind struct {
	s        *RawBlockStore
	bundled  bool
	mu       sync.Mutex
	cond     *sync.Cond
	staged   map[[32]byte][]byte
	bytes    int64
	q        chan ibdWriteJob
	inflight atomic.Int64
	stop     chan struct{}
	wg       sync.WaitGroup
}

func newIBDWriteBehind(s *RawBlockStore) *ibdWriteBehind {
	workers := ibdWriteBehindWorkers
	bundled := s != nil && s.opts.Layout == BlockLayoutBundled
	if bundled {
		// Append offsets are serialized by bundledAppendMu inside putBundled. Multiple
		// flushers still help: locator WriteFile was the NTFS bottleneck (~50 loc/min with
		// one worker) while blk*.dat append stayed nearly idle. Keep a modest pool so
		// locator creates run in parallel after each append.
		workers = 8
	}
	wb := &ibdWriteBehind{
		s:       s,
		bundled: bundled,
		staged:  make(map[[32]byte][]byte),
		q:       make(chan ibdWriteJob, ibdWriteBehindQueue),
		stop:    make(chan struct{}),
	}
	wb.cond = sync.NewCond(&wb.mu)
	for i := 0; i < workers; i++ {
		wb.wg.Add(1)
		go wb.worker()
	}
	return wb
}

func (wb *ibdWriteBehind) has(hash [32]byte) bool {
	if wb == nil {
		return false
	}
	wb.mu.Lock()
	_, ok := wb.staged[hash]
	wb.mu.Unlock()
	return ok
}

func (wb *ibdWriteBehind) queuedBytes() int64 {
	if wb == nil {
		return 0
	}
	wb.mu.Lock()
	n := wb.bytes
	wb.mu.Unlock()
	if n < 0 {
		return 0
	}
	return n
}

// nearCapacity reports whether staged RAM bytes are at or above frac of the IBD buffer.
func (wb *ibdWriteBehind) nearCapacity(frac float64) bool {
	if wb == nil || frac <= 0 {
		return false
	}
	if frac > 1 {
		frac = 1
	}
	return float64(wb.queuedBytes()) >= float64(ibdWriteBehindMaxBytes)*frac
}

func (wb *ibdWriteBehind) size(hash [32]byte) (int, bool) {
	if wb == nil {
		return 0, false
	}
	wb.mu.Lock()
	raw, ok := wb.staged[hash]
	wb.mu.Unlock()
	if !ok {
		return 0, false
	}
	return len(raw), true
}

func (wb *ibdWriteBehind) get(hash [32]byte) ([]byte, bool) {
	if wb == nil {
		return nil, false
	}
	wb.mu.Lock()
	raw, ok := wb.staged[hash]
	wb.mu.Unlock()
	if !ok {
		return nil, false
	}
	return append([]byte(nil), raw...), true
}

func (wb *ibdWriteBehind) drop(hash [32]byte) {
	if wb == nil {
		return
	}
	wb.mu.Lock()
	if raw, ok := wb.staged[hash]; ok {
		wb.bytes -= int64(len(raw))
		if wb.bytes < 0 {
			wb.bytes = 0
		}
		delete(wb.staged, hash)
		wb.cond.Broadcast()
	}
	wb.mu.Unlock()
}

func writeBehindTestHooksActive() bool {
	return abortBeforeRawPutRename || stallAfterRawPutTmpWrite > 0
}

func writeBehindGiveUp(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "cannot find the path") || strings.Contains(msg, "The system cannot find the path")
}

func (wb *ibdWriteBehind) stage(hash [32]byte, raw []byte) error {
	if wb == nil {
		return fmt.Errorf("write-behind not started")
	}
	cp := append([]byte(nil), raw...)
	for {
		wb.mu.Lock()
		if wb.bytes+int64(len(cp)) <= ibdWriteBehindMaxBytes {
			break
		}
		// Do not hold wb.mu while waiting — that stalled every concurrent Put/Get
		// when the 512 MiB buffer filled during multi-peer IBD.
		wb.mu.Unlock()
		select {
		case <-wb.stop:
			return fmt.Errorf("write-behind stopped")
		case <-time.After(5 * time.Millisecond):
		}
	}
	if _, ok := wb.staged[hash]; ok {
		wb.mu.Unlock()
		return nil
	}
	wb.staged[hash] = cp
	wb.bytes += int64(len(cp))
	wb.mu.Unlock()
	select {
	case wb.q <- ibdWriteJob{hash: hash, raw: cp}:
		return nil
	case <-wb.stop:
		return fmt.Errorf("write-behind stopped")
	}
}

func (wb *ibdWriteBehind) worker() {
	defer wb.wg.Done()
	for {
		select {
		case <-wb.stop:
			return
		case job, ok := <-wb.q:
			if !ok {
				return
			}
			wb.inflight.Add(1)
			var err error
			if wb.bundled {
				// putBundled takes bundledAppendMu itself — never hold RawBlockStore.mu
				// across NTFS writes (claim/Get would freeze for the whole Put).
				err = wb.s.putBundled(job.hash, job.raw)
			} else {
				err = wb.s.putPerFile(job.hash, job.raw)
			}
			wb.inflight.Add(-1)
			if err != nil {
				if writeBehindGiveUp(err) {
					wb.drop(job.hash)
					continue
				}
				fmt.Fprintf(os.Stderr, "raw block write-behind %x: %v\n", job.hash[:4], err)
				time.Sleep(10 * time.Millisecond)
				select {
				case wb.q <- job:
				case <-wb.stop:
					return
				}
				continue
			}
			wb.drop(job.hash)
		}
	}
}

// Flush waits until every staged body has been written to disk (or the wait is empty).
func (wb *ibdWriteBehind) Flush() {
	if wb == nil {
		return
	}
	deadline := time.Now().Add(ShutdownFlushWait)
	for time.Now().Before(deadline) {
		wb.mu.Lock()
		n := len(wb.staged)
		wb.mu.Unlock()
		if n == 0 && wb.inflight.Load() == 0 && len(wb.q) == 0 {
			if wb.bundled && wb.s != nil {
				_ = wb.s.Close()
			}
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// ShutdownFlushWait is how long Flush will wait for the disk pool during tests/shutdown.
const ShutdownFlushWait = 15 * time.Second
