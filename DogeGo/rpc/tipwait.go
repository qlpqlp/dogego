// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"dogego/store"
)

// TipWaiter broadcasts chainActive tip changes for waitfor* RPCs (node NotifyRPCTip).
type TipWaiter struct {
	mu     sync.Mutex
	height int64
	hash   string
	cond   *sync.Cond
}

// NewTipWaiter returns a waiter with no tip yet.
func NewTipWaiter() *TipWaiter {
	t := &TipWaiter{height: -1}
	t.cond = sync.NewCond(&t.mu)
	return t
}

// Notify records a new best tip (hash is display hex).
func (t *TipWaiter) Notify(height int64, hash string) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.height = height
	t.hash = strings.TrimSpace(strings.ToLower(hash))
	t.cond.Broadcast()
	t.mu.Unlock()
}

// Snapshot returns the current tip height and hash.
func (t *TipWaiter) Snapshot() (height int64, hash string) {
	if t == nil {
		return -1, ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.height, t.hash
}

func (t *TipWaiter) waitUntil(pred func(height int64, hash string) bool, timeoutSec int64) (map[string]interface{}, bool) {
	if t == nil {
		return nil, true
	}
	var deadline time.Time
	if timeoutSec > 0 {
		deadline = time.Now().Add(time.Duration(timeoutSec) * time.Second)
	}
	for {
		t.mu.Lock()
		h, hash := t.height, t.hash
		t.mu.Unlock()
		if pred(h, hash) {
			return map[string]interface{}{
				"height": h,
				"hash":   hash,
			}, false
		}
		if timeoutSec == 0 {
			t.mu.Lock()
			t.cond.Wait()
			t.mu.Unlock()
			continue
		}
		if time.Now().After(deadline) {
			return nil, true
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func tipWaiterFrom(paths *DataPaths) *TipWaiter {
	if paths != nil && paths.TipWaiter != nil {
		return paths.TipWaiter
	}
	return nil
}

func parseOptionalTimeout(params []json.RawMessage, idx int, method string) (int64, int, string) {
	if len(params) <= idx || strings.TrimSpace(string(params[idx])) == "null" {
		return 0, 0, ""
	}
	var to float64
	if err := json.Unmarshal(params[idx], &to); err != nil {
		return 0, -8, method + ": timeout must be an integer"
	}
	if to < 0 || to != float64(int64(to)) {
		return 0, -8, method + ": timeout must be non-negative"
	}
	return int64(to), 0, ""
}

// execWaitForNewBlock waits until chainActive advances from the height/hash at call time.
func execWaitForNewBlock(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) > 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	timeout, code, msg := parseOptionalTimeout(params, 0, "waitfornewblock")
	if code != 0 {
		return nil, code, msg
	}
	tw := tipWaiterFrom(paths)
	if tw == nil {
		return nil, -1, "waitfornewblock: tip notifications not available"
	}
	var startH int64
	var startHash string
	if j != nil {
		var err error
		startH, startHash, err = ChainActiveTip(j, raw, paths)
		if err != nil {
			return nil, -1, "waitfornewblock: " + err.Error()
		}
	} else {
		startH, startHash = tw.Snapshot()
	}
	res, timedOut := tw.waitUntil(func(waiterH int64, waiterHash string) bool {
		if j == nil {
			return waiterH > startH || (waiterH == startH && waiterHash != "" && waiterHash != startHash)
		}
		curH, curHash, err := ChainActiveTip(j, raw, paths)
		if err != nil {
			return false
		}
		return curH > startH || (curH == startH && curHash != "" && curHash != startHash)
	}, timeout)
	if timedOut {
		return nil, -1, "waitfornewblock: timeout"
	}
	if j != nil {
		curH, curHash, err := ChainActiveTip(j, raw, paths)
		if err != nil {
			return nil, -1, "waitfornewblock: " + err.Error()
		}
		return map[string]interface{}{"height": curH, "hash": curHash}, 0, ""
	}
	return res, 0, ""
}

// execWaitForBlock waits until the given block hash is the chainActive tip.
func execWaitForBlock(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	want, code, msg := parseOneBlockHashParam(params, "waitforblock")
	if code != 0 {
		return nil, code, msg
	}
	timeout, code, msg := parseOptionalTimeout(params, 1, "waitforblock")
	if code != 0 {
		return nil, code, msg
	}
	tw := tipWaiterFrom(paths)
	if tw == nil {
		return nil, -1, "waitforblock: tip notifications not available"
	}
	if j != nil {
		curH, curHash, err := ChainActiveTip(j, raw, paths)
		if err != nil {
			return nil, -1, "waitforblock: " + err.Error()
		}
		if curHash == want {
			return map[string]interface{}{"height": curH, "hash": curHash}, 0, ""
		}
	}
	res, timedOut := tw.waitUntil(func(_ int64, waiterHash string) bool {
		if j == nil {
			return waiterHash == want
		}
		_, curHash, err := ChainActiveTip(j, raw, paths)
		return err == nil && curHash == want
	}, timeout)
	if timedOut {
		return nil, -1, "waitforblock: timeout"
	}
	if j != nil {
		curH, curHash, err := ChainActiveTip(j, raw, paths)
		if err != nil {
			return nil, -1, "waitforblock: " + err.Error()
		}
		return map[string]interface{}{"height": curH, "hash": curHash}, 0, ""
	}
	return res, 0, ""
}

// execWaitForBlockHeight waits until chainActive height is at least the target.
func execWaitForBlockHeight(paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var h float64
	if err := json.Unmarshal(params[0], &h); err != nil {
		return nil, -8, "waitforblockheight: height must be a number"
	}
	if h < 0 || h != float64(int64(h)) {
		return nil, -8, "waitforblockheight: height must be a non-negative integer"
	}
	target := int64(h)
	timeout, code, msg := parseOptionalTimeout(params, 1, "waitforblockheight")
	if code != 0 {
		return nil, code, msg
	}
	tw := tipWaiterFrom(paths)
	if tw == nil {
		return nil, -1, "waitforblockheight: tip notifications not available"
	}
	if j != nil {
		curH, _, _ := activeChainFromJournal(j, raw, paths)
		if curH >= target {
			hash, err := blockHashHexAt(j, curH)
			if err != nil {
				return nil, -1, "waitforblockheight: " + err.Error()
			}
			return map[string]interface{}{"height": curH, "hash": hash}, 0, ""
		}
	} else if h, _ := tw.Snapshot(); h >= target {
		_, hash := tw.Snapshot()
		return map[string]interface{}{"height": h, "hash": hash}, 0, ""
	}
	res, timedOut := tw.waitUntil(func(waiterH int64, _ string) bool {
		if j == nil {
			return waiterH >= target
		}
		curH, _, _ := activeChainFromJournal(j, raw, paths)
		return curH >= target
	}, timeout)
	if timedOut {
		return nil, -1, "waitforblockheight: timeout"
	}
	if j != nil {
		curH, _, _ := activeChainFromJournal(j, raw, paths)
		hash, err := blockHashHexAt(j, curH)
		if err != nil {
			return nil, -1, "waitforblockheight: " + err.Error()
		}
		return map[string]interface{}{"height": curH, "hash": hash}, 0, ""
	}
	return res, 0, ""
}
