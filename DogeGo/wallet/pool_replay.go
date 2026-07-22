// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"sort"
	"strings"

	"dogego/wallet/corewallet"
)

// HDKeypoolCoreIndexEntry maps a BIP44 receive index to a Core wallet.dat pool index.
type HDKeypoolCoreIndexEntry struct {
	ReceiveIndex uint32 `json:"receive_index"`
	CoreIndex    int64  `json:"core_index"`
}

// HDKeypoolCoreIndexEntries returns receive→Core pool index rows from wallet.json (sorted by receive index).
func (w *Disk) HDKeypoolCoreIndexEntries() []HDKeypoolCoreIndexEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.hdKeypoolCoreIdx) == 0 {
		return nil
	}
	recv := make([]uint32, 0, len(w.hdKeypoolCoreIdx))
	for k := range w.hdKeypoolCoreIdx {
		recv = append(recv, k)
	}
	sort.Slice(recv, func(i, j int) bool { return recv[i] < recv[j] })
	out := make([]HDKeypoolCoreIndexEntry, len(recv))
	for i, r := range recv {
		out[i] = HDKeypoolCoreIndexEntry{ReceiveIndex: r, CoreIndex: w.hdKeypoolCoreIdx[r]}
	}
	return out
}

// IsReceiveInKeypool reports whether addr is an unused HD receive keypool entry.
func (w *Disk) IsReceiveInKeypool(addr string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return false
	}
	idx, ok := w.receiveIndexForAddressLocked(addr)
	if !ok {
		return false
	}
	return w.receiveIndexInPoolLocked(idx)
}

// IsChangeInKeypool reports whether addr is an unused HD change keypool entry.
func (w *Disk) IsChangeInKeypool(addr string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return false
	}
	idx, ok := w.changeIndexForAddressLocked(addr)
	if !ok {
		return false
	}
	return w.changeIndexInPoolLocked(idx)
}

func (w *Disk) receiveIndexInPoolLocked(idx uint32) bool {
	for _, k := range w.hdKeypool {
		if k == idx {
			return true
		}
	}
	return false
}

func (w *Disk) changeIndexInPoolLocked(idx uint32) bool {
	for _, k := range w.hdChangeKeypool {
		if k == idx {
			return true
		}
	}
	return false
}

// receiveIndexIssuedLocked reports whether a BIP44 receive index was already
// consumed (default index 0, or allocated then removed from the unused pool).
func (w *Disk) receiveIndexIssuedLocked(idx uint32) bool {
	if idx == 0 {
		return true
	}
	if w.receiveIndexInPoolLocked(idx) {
		return false
	}
	return idx < w.hdExternalNext
}

func (w *Disk) changeIndexForAddressLocked(addr string) (uint32, bool) {
	addr = strings.TrimSpace(addr)
	scanMax := w.hdMaxChangeIndexLocked()
	for i := uint32(0); i <= scanMax; i++ {
		d, err := w.deriveChange(i)
		if err == nil && d.Addr == addr {
			return i, true
		}
	}
	return 0, false
}

// CorePoolIndexForAddress returns the Core wallet.dat pool index stored for addr.
func (w *Disk) CorePoolIndexForAddress(addr string) (int64, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || len(w.hdKeypoolCoreIdx) == 0 {
		return 0, false
	}
	idx, ok := w.receiveIndexForAddressLocked(addr)
	if !ok {
		return 0, false
	}
	core, ok := w.hdKeypoolCoreIdx[idx]
	return core, ok
}

const poolReplayScanCap = 2000

// PoolReplayScanCap is the maximum BIP44 receive index scanned when matching Core wallet.dat pool pubkeys.
const PoolReplayScanCap = poolReplayScanCap

// poolReplayScanMax returns the highest BIP44 receive index to scan when matching Core pool pubkeys.
func (w *Disk) poolReplayScanMaxLocked(entries []corewallet.PoolEntry) uint32 {
	want := w.hdMaxReceiveIndexLocked()
	if len(entries) == 0 {
		return want
	}
	const deepFloor = 256
	if want < deepFloor {
		want = deepFloor
	}
	want += defaultKeypoolSize
	if want > poolReplayScanCap {
		want = poolReplayScanCap
	}
	return want
}

func (w *Disk) receiveIndexForAddressLocked(addr string) (uint32, bool) {
	addr = strings.TrimSpace(addr)
	scanMax := w.hdMaxReceiveIndexLocked()
	for i := uint32(0); i <= scanMax; i++ {
		d, err := w.deriveReceive(i)
		if err == nil && d.Addr == addr {
			return i, true
		}
	}
	return 0, false
}

// PoolReplayResult summarizes Core BDB pool → DogeGo HD keypool replay.
type PoolReplayResult struct {
	IndicesReplayed   bool
	Matched           int
	Reserved          int
	Skipped           int
	CoreIndicesStored int
}

// ReplayCorePoolIntoHDKeypool reserves BIP44 receive indices whose compressed
// pubkey appears in Core pool entries. Matched Core pool index numbers are stored
// in wallet.json hd_keypool_core_index. Already-issued receive indices (default
// index 0 or consumed via getnewaddress) keep their core index mapping but are
// not re-queued into hd_keypool.
func (w *Disk) ReplayCorePoolIntoHDKeypool(entries []corewallet.PoolEntry) (PoolReplayResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	var res PoolReplayResult
	if !w.hdEnabled() || w.priv == nil || len(entries) == 0 {
		res.Skipped = len(entries)
		return res, nil
	}
	w.syncReceiveNextFromPoolLocked()
	inPool := make(map[uint32]struct{}, len(w.hdKeypool))
	for _, idx := range w.hdKeypool {
		inPool[idx] = struct{}{}
	}
	for _, e := range entries {
		pubHex := strings.ToLower(strings.TrimSpace(e.PubKeyHex))
		if pubHex == "" {
			res.Skipped++
			continue
		}
		idx, ok := w.findReceiveIndexByPubkeyHexLocked(pubHex, w.poolReplayScanMaxLocked(entries))
		if !ok {
			res.Skipped++
			continue
		}
		res.Matched++
		if w.hdKeypoolCoreIdx == nil {
			w.hdKeypoolCoreIdx = make(map[uint32]int64)
		}
		if _, have := w.hdKeypoolCoreIdx[idx]; !have {
			w.hdKeypoolCoreIdx[idx] = e.Index
			res.CoreIndicesStored++
		}
		if _, dup := inPool[idx]; dup {
			continue
		}
		if w.receiveIndexIssuedLocked(idx) {
			continue
		}
		w.hdKeypool = append(w.hdKeypool, idx)
		inPool[idx] = struct{}{}
		if idx >= w.hdExternalNext {
			w.hdExternalNext = idx + 1
		}
		res.Reserved++
	}
	res.IndicesReplayed = res.Reserved > 0
	if res.Reserved > 0 || res.CoreIndicesStored > 0 {
		if err := w.saveLocked(); err != nil {
			return res, err
		}
	}
	return res, nil
}

func (w *Disk) findReceiveIndexByPubkeyHexLocked(pubHex string, scanMax uint32) (uint32, bool) {
	if scanMax > poolReplayScanCap {
		scanMax = poolReplayScanCap
	}
	for i := uint32(0); i <= scanMax; i++ {
		d, err := w.deriveReceive(i)
		if err != nil || d.Priv == nil {
			continue
		}
		if hex.EncodeToString(d.Priv.PubKey().SerializeCompressed()) == pubHex {
			return i, true
		}
	}
	return 0, false
}
