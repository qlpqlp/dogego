// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"dogego/wire"
)

// Pool stores accepted raw transactions keyed by RPC display txid.
// Script, fee, package-limit, and orphan policy are enforced in consensus before Add (see node relay and RPC admission).
type Pool struct {
	mu  sync.Mutex
	max int
	maxBytes  int   // serialized tx byte cap (0 = DefaultMaxMempoolMB)
	expirySec int64 // max age in pool seconds (0 = DefaultMempoolExpiryHours)
	raw          map[string][]byte
	addedAt      map[string]int64 // RPC display txid -> unix seconds when accepted
	addedAtHeight map[string]int64 // RPC display txid -> header tip height when accepted
	tipHeightFn  func() int64     // optional; records addedAtHeight on Add
	onRemove     func(displayTxid string)
	onAdd        func(raw []byte) // after a new tx is stored (not on duplicate id)
	paused       bool             // when true, Add rejects new transactions (relay paused)
	// Rolling minimum relay feerate after eviction (Core rollingMinimumFeeRate).
	rollingMinFeePerKB           float64
	lastRollingFeeUpdate         int64
	blockSinceLastRollingFeeBump bool
	feeDelta                     map[string]int64 // prioritisetransaction virtual deltas (koinu)
	descendantFeeBoost           map[string]int64 // ancestor mining score: boosts from prioritised descendants
	ancestorFeeBoost             map[string]int64 // descendant mining score: boosts from prioritised ancestors
	feeDeltaProp                 map[string]feeDeltaPropagation
	changeSeq                    uint64           // bumps on pool mutations (Core GetTransactionsUpdated analogue)
	incrementalRelayFeePerKB     uint64           // Core incrementalRelayFee (BIP125 / rolling min after eviction)
}

const defaultIncrementalRelayFeePerKB = 100_000

func New(max int) *Pool {
	p := &Pool{max: max, raw: make(map[string][]byte), incrementalRelayFeePerKB: defaultIncrementalRelayFeePerKB}
	p.SetPolicy(0, 0)
	return p
}

// SetIncrementalRelayFeePerKB sets the incremental relay rate used for rolling minimum fee after eviction.
func (p *Pool) SetIncrementalRelayFeePerKB(koinuPerKB uint64) {
	if p == nil || koinuPerKB < 1 {
		return
	}
	p.mu.Lock()
	p.incrementalRelayFeePerKB = koinuPerKB
	p.mu.Unlock()
}

// IncrementalRelayFeePerKB returns the configured incremental relay rate (koinu per kB).
func (p *Pool) IncrementalRelayFeePerKB() uint64 {
	return p.incrementalRelayPerKB()
}

func (p *Pool) incrementalRelayPerKB() uint64 {
	if p == nil {
		return defaultIncrementalRelayFeePerKB
	}
	p.mu.Lock()
	v := p.incrementalRelayFeePerKB
	p.mu.Unlock()
	if v < 1 {
		return defaultIncrementalRelayFeePerKB
	}
	return v
}

// SetOnAdd calls fn when a new tx is accepted (not when the same blob is already present).
func (p *Pool) SetOnAdd(fn func(raw []byte)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.onAdd = fn
	p.mu.Unlock()
}

// SetOnRemove calls fn when a tx leaves the pool (eviction, expiry, block inclusion).
func (p *Pool) SetOnRemove(fn func(displayTxid string)) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.onRemove = fn
	p.mu.Unlock()
}

// SetTipHeightFn records the header tip height when txs are accepted (fee estimator confirmation delay).
func (p *Pool) SetTipHeightFn(fn func() int64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.tipHeightFn = fn
	p.mu.Unlock()
}

// SetPolicy configures Core-style -maxmempool (MB) and -mempoolexpiry (hours); 0 uses defaults.
func (p *Pool) SetPolicy(maxMempoolMB, expiryHours int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.maxBytes = MaxMempoolBytes(maxMempoolMB)
	p.expirySec = MempoolExpirySeconds(expiryHours)
	p.mu.Unlock()
}

// SetRollingMinFeePerKBForTest sets the post-eviction rolling minimum (tests only).
func (p *Pool) SetRollingMinFeePerKBForTest(koinuPerKB uint64) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.rollingMinFeePerKB = float64(koinuPerKB)
	p.blockSinceLastRollingFeeBump = true
	p.mu.Unlock()
}

// ExpiryHours returns configured mempool max age in hours (Core -mempoolexpiry).
func (p *Pool) ExpiryHours() int {
	if p == nil {
		return DefaultMempoolExpiryHours
	}
	p.mu.Lock()
	sec := p.expirySec
	p.mu.Unlock()
	if sec <= 0 {
		return DefaultMempoolExpiryHours
	}
	h := int(sec / 3600)
	if h < 1 {
		return 1
	}
	return h
}

// MaxMempoolLimitBytes returns the configured byte cap for getmempoolinfo maxmempool.
func (p *Pool) MaxMempoolLimitBytes() int {
	if p == nil {
		return MaxMempoolBytes(0)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.maxBytes > 0 {
		return p.maxBytes
	}
	return MaxMempoolBytes(0)
}

// Paused reports whether new transactions are rejected.
func (p *Pool) Paused() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.paused
}

// SetPaused toggles admission of new transactions (existing entries remain).
func (p *Pool) SetPaused(paused bool) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.paused = paused
	p.mu.Unlock()
}

// Clear removes all transactions from the pool.
func (p *Pool) Clear() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	n := len(p.raw)
	p.raw = make(map[string][]byte)
	p.addedAt = nil
	p.addedAtHeight = nil
	p.clearAllFeeDeltasLocked()
	p.bumpChangeSeqLocked()
	p.mu.Unlock()
	return n
}

// ChangeSequence returns a monotonic counter bumped when mempool contents or fee deltas change.
func (p *Pool) ChangeSequence() uint64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.changeSeq
}

func (p *Pool) bumpChangeSeqLocked() {
	p.changeSeq++
}

// Add stores a raw transaction blob. Id is hex(SHA256(tx)[:16]) for dedup only.
func (p *Pool) Add(rawTx []byte) error {
	if len(rawTx) == 0 {
		return fmt.Errorf("empty tx")
	}
	p.mu.Lock()
	if p.paused {
		p.mu.Unlock()
		return fmt.Errorf("mempool relay paused")
	}
	if p.needsEvictionLocked(len(rawTx)) {
		p.mu.Unlock()
		return fmt.Errorf("mempool full (%d)", p.max)
	}
	h := sha256.Sum256(rawTx)
	id := fmt.Sprintf("%x", h[:16])
	if _, ok := p.raw[id]; ok {
		p.mu.Unlock()
		return nil
	}
	p.raw[id] = append([]byte(nil), rawTx...)
	var rpcID string
	if tx, err := wire.DeserializeTx(rawTx); err == nil {
		rpcID = txidDisplayHex(tx.TxHash())
		if p.addedAt == nil {
			p.addedAt = make(map[string]int64)
		}
		if _, ok := p.addedAt[rpcID]; !ok {
			p.addedAt[rpcID] = time.Now().Unix()
		}
		if p.tipHeightFn != nil {
			if h := p.tipHeightFn(); h >= 0 {
				if p.addedAtHeight == nil {
					p.addedAtHeight = make(map[string]int64)
				}
				if _, ok := p.addedAtHeight[rpcID]; !ok {
					p.addedAtHeight[rpcID] = h
				}
			}
		}
	}
	p.bumpChangeSeqLocked()
	p.mu.Unlock()
	if p.onAdd != nil {
		rawCopy := append([]byte(nil), rawTx...)
		fn := p.onAdd
		go fn(rawCopy)
	}
	if rpcID != "" {
		p.applyPendingFeeDeltaAfterAdd(rpcID)
	}
	return nil
}

// EntryTime returns when a tx was accepted into the pool (unix seconds), or 0 if unknown.
func (p *Pool) EntryTime(rpcTxid string) int64 {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.addedAt[strings.TrimSpace(strings.ToLower(rpcTxid))]
}

// MaxEntries returns the configured maximum pooled tx count (0 = unlimited).
func (p *Pool) MaxEntries() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.max
}

// RawBlobs returns copies of all pooled serialized transactions (for fee estimation).
func (p *Pool) RawBlobs() [][]byte {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([][]byte, 0, len(p.raw))
	for _, b := range p.raw {
		out = append(out, append([]byte(nil), b...))
	}
	return out
}

// Count returns the number of pooled transactions.
func (p *Pool) Count() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.raw)
}

// GetRawByTxID returns serialized tx bytes for an RPC-display txid (64 hex), or an error if not in this pool.
func (p *Pool) GetRawByTxID(rpcTxid string) ([]byte, error) {
	rpcTxid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(rpcTxid), "0x"))
	if len(rpcTxid) != 64 {
		return nil, fmt.Errorf("txid must be 64 hex characters")
	}
	for _, c := range rpcTxid {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return nil, fmt.Errorf("txid must be hex")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, raw := range p.raw {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		if txidDisplayHex(tx.TxHash()) == rpcTxid {
			return raw, nil
		}
	}
	return nil, fmt.Errorf("transaction not in mempool")
}

// ContainsTxID reports whether a transaction with this RPC display txid is already in the pool.
func (p *Pool) ContainsTxID(rpcTxid string) bool {
	_, err := p.GetRawByTxID(rpcTxid)
	return err == nil
}

// IsFull reports whether the pool has reached its configured maximum entry count or byte cap.
func (p *Pool) IsFull() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.needsEvictionLocked(0)
}

func (p *Pool) needsEvictionLocked(extraBytes int) bool {
	if p.max > 0 && len(p.raw) >= p.max {
		return true
	}
	if p.maxBytes > 0 && p.totalBytesLocked()+extraBytes > p.maxBytes {
		return true
	}
	return false
}

// TotalBytes returns the sum of raw serialized transaction sizes in the pool.
func (p *Pool) TotalBytes() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.totalBytesLocked()
}

func (p *Pool) totalBytesLocked() int {
	n := 0
	for _, b := range p.raw {
		n += len(b)
	}
	return n
}

func txidDisplayHex(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return hex.EncodeToString(b)
}

// TxIDDisplayHex returns the RPC-style display txid for a legacy-serialization tx hash.
func TxIDDisplayHex(h [32]byte) string {
	return txidDisplayHex(h)
}

// MempoolRawByDisplayTxid implements consensus.MempoolRawLookup.
func (p *Pool) MempoolRawByDisplayTxid(rpcTxid string) ([]byte, bool) {
	raw, err := p.GetRawByTxID(rpcTxid)
	return raw, err == nil
}

// EntryHeight returns the chain height when the tx was admitted (0 if unknown).
func (p *Pool) EntryHeight(displayTxid string) int64 {
	if p == nil {
		return 0
	}
	displayTxid = strings.TrimSpace(strings.ToLower(displayTxid))
	p.mu.Lock()
	h := p.entryHeightLocked(displayTxid)
	p.mu.Unlock()
	return h
}

// BlocksWaitedAtConfirm returns blocks from pool admission height to confirmHeight (minimum 1).
func (p *Pool) BlocksWaitedAtConfirm(displayTxid string, confirmHeight int64) int {
	if p == nil {
		return 1
	}
	if confirmHeight < 0 {
		return 1
	}
	displayTxid = strings.TrimSpace(strings.ToLower(displayTxid))
	p.mu.Lock()
	admit := p.addedAtHeight[displayTxid]
	p.mu.Unlock()
	if admit < 0 {
		return 1
	}
	w := int(confirmHeight - admit + 1)
	if w < 1 {
		return 1
	}
	return w
}

type sortedTx struct {
	txid string
	raw  []byte
}

// MemPoolVerboseEntry is one mempool row for getrawmempool verbose=1 (subset of Core fields).
type MemPoolVerboseEntry struct {
	TxID    string
	Size    int
	VSize   int
	Time    int64
	Height  int64  // chain height when accepted (0 if unknown; Core getrawmempool verbose height)
	Depends []string // direct in-mempool parents (sorted); empty if none
}

func (p *Pool) entryHeightLocked(displayTxid string) int64 {
	if p == nil || p.addedAtHeight == nil {
		return 0
	}
	return p.addedAtHeight[displayTxid]
}

func (p *Pool) sortedTxEntries() ([]sortedTx, error) {
	p.mu.Lock()
	blobs := make([][]byte, 0, len(p.raw))
	for _, v := range p.raw {
		blobs = append(blobs, v)
	}
	p.mu.Unlock()
	out := make([]sortedTx, 0, len(blobs))
	for _, raw := range blobs {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			return nil, fmt.Errorf("mempool: corrupt entry: %w", err)
		}
		out = append(out, sortedTx{txid: txidDisplayHex(tx.TxHash()), raw: raw})
	}
	slices.SortFunc(out, func(a, b sortedTx) int { return cmp.Compare(a.txid, b.txid) })
	return out, nil
}

// SortedTransaction is one mempool tx in stable RPC txid order.
type SortedTransaction struct {
	TxID string
	Raw  []byte
}

// SortedTransactions returns all pooled transactions in stable RPC txid order.
func (p *Pool) SortedTransactions() ([]SortedTransaction, error) {
	entries, err := p.sortedTxEntries()
	if err != nil {
		return nil, err
	}
	out := make([]SortedTransaction, len(entries))
	for i, e := range entries {
		out[i] = SortedTransaction{TxID: e.txid, Raw: e.raw}
	}
	return out, nil
}

// RawMemPoolTxIDs returns pooled transaction ids in RPC display order, sorted lexicographically.
func (p *Pool) RawMemPoolTxIDs() ([]string, error) {
	entries, err := p.sortedTxEntries()
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(entries))
	for i := range entries {
		ids[i] = entries[i].txid
	}
	return ids, nil
}

// SortedMemPoolVerbose returns mempool entries sorted by txid (RPC display hex).
func (p *Pool) SortedMemPoolVerbose() ([]MemPoolVerboseEntry, error) {
	entries, err := p.sortedTxEntries()
	if err != nil {
		return nil, err
	}
	_, parents, _, err := p.spendEdges()
	if err != nil {
		return nil, err
	}
	p.mu.Lock()
	out := make([]MemPoolVerboseEntry, len(entries))
	for i := range entries {
		sz, vs := txSizeVSize(entries[i].raw)
		id := entries[i].txid
		deps := append([]string(nil), parents[id]...)
		slices.Sort(deps)
		at := p.addedAt[id]
		out[i] = MemPoolVerboseEntry{TxID: id, Size: sz, VSize: vs, Time: at, Height: p.entryHeightLocked(id), Depends: deps}
	}
	p.mu.Unlock()
	return out, nil
}

// MemPoolVerboseEntryForTxID returns a getrawmempool-verbose-shaped row for one pooled txid.
func (p *Pool) MemPoolVerboseEntryForTxID(rpcTxid string) (MemPoolVerboseEntry, error) {
	raw, err := p.GetRawByTxID(rpcTxid)
	if err != nil {
		return MemPoolVerboseEntry{}, err
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return MemPoolVerboseEntry{}, err
	}
	sz, vs := txSizeVSize(raw)
	id := txidDisplayHex(tx.TxHash())
	_, parents, _, err := p.spendEdges()
	if err != nil {
		return MemPoolVerboseEntry{}, err
	}
	deps := append([]string(nil), parents[id]...)
	slices.Sort(deps)
	p.mu.Lock()
	at := int64(0)
	if p.addedAt != nil {
		at = p.addedAt[id]
	}
	h := p.entryHeightLocked(id)
	p.mu.Unlock()
	return MemPoolVerboseEntry{TxID: id, Size: sz, VSize: vs, Time: at, Height: h, Depends: deps}, nil
}

func txSizeVSize(raw []byte) (size, vsize int) {
	size = len(raw)
	vsize = size
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return size, vsize
	}
	if total, err := wire.TransactionTotalSize(tx); err == nil {
		size = total
	}
	if vs, err := wire.TransactionVirtualSize(tx); err == nil {
		vsize = vs
	}
	return size, vsize
}
