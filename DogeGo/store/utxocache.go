// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sync"

	"dogego/pow"
	"dogego/wire"
)

// UtxoEntry is one unspent output tracked at the chain tip (Core CCoinsView subset).
type UtxoEntry struct {
	Value    int64
	PkScript []byte
	Height   int64
	Coinbase bool // Core CCoins::fCoinBase for the creating transaction
}

// UtxoCache maintains the UTXO set for connected blocks up to TipHeight.
// It is updated after successful ConnectBlock validation, not from headers alone.
type UtxoCache struct {
	mu        sync.RWMutex
	tipHeight int64
	coins     map[[36]byte]UtxoEntry
}

// NewUtxoCache returns an empty UTXO cache.
func NewUtxoCache() *UtxoCache {
	return &UtxoCache{tipHeight: -1, coins: make(map[[36]byte]UtxoEntry)}
}

// TipHeight is the height of the last applied block (-1 if empty).
func (u *UtxoCache) TipHeight() int64 {
	if u == nil {
		return -1
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.tipHeight
}

// SetTipHeightForTest sets chainActive height without applying blocks (unit tests only).
func (u *UtxoCache) SetTipHeightForTest(h int64) {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.tipHeight = h
	u.mu.Unlock()
}

// AddUtxoForTest inserts one coin (unit tests only).
func (u *UtxoCache) AddUtxoForTest(outpoint [36]byte, e UtxoEntry) {
	if u == nil {
		return
	}
	u.mu.Lock()
	if u.coins == nil {
		u.coins = make(map[[36]byte]UtxoEntry)
	}
	u.coins[outpoint] = e
	u.mu.Unlock()
}

// Count returns the number of unspent outputs in the cache.
func (u *UtxoCache) Count() int {
	if u == nil {
		return 0
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	return len(u.coins)
}

// ApproxMemoryBytes estimates in-RAM UTXO working-set size (map overhead + keys + scripts).
// Used for Core-style -dbcache flush budgeting; intentionally coarse.
func (u *UtxoCache) ApproxMemoryBytes() int64 {
	if u == nil {
		return 0
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	n := len(u.coins)
	if n == 0 {
		return 0
	}
	scriptBytes := 0
	// Sample up to 256 entries for average script length.
	limit := n
	if limit > 256 {
		limit = 256
	}
	i := 0
	for _, e := range u.coins {
		scriptBytes += len(e.PkScript)
		i++
		if i >= limit {
			break
		}
	}
	avgScript := scriptBytes / limit
	// ~48B map bucket + 36B key + 24B entry header + script + height/value.
	per := int64(48 + 36 + 24 + avgScript + 16)
	return int64(n) * per
}

// Reset clears the cache (after header reorg before replay).
func (u *UtxoCache) Reset() {
	if u == nil {
		return
	}
	u.mu.Lock()
	u.tipHeight = -1
	u.coins = make(map[[36]byte]UtxoEntry)
	u.mu.Unlock()
}

// Lookup returns a UTXO entry by display txid (RPC byte order) and vout.
func (u *UtxoCache) Lookup(rpcTxid string, vout uint32) (UtxoEntry, bool) {
	if u == nil {
		return UtxoEntry{}, false
	}
	var prev [32]byte
	if err := decodeDisplayTxid(rpcTxid, &prev); err != nil {
		return UtxoEntry{}, false
	}
	return u.LookupOutpoint(prev, vout)
}

// LookupOutpoint returns a UTXO by prevout hash and index.
func (u *UtxoCache) LookupOutpoint(prevHash [32]byte, vout uint32) (UtxoEntry, bool) {
	if u == nil {
		return UtxoEntry{}, false
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	e, ok := u.coins[outpointKey(prevHash, vout)]
	return e, ok
}

// UnspentHeight returns the height that created an unspent output (for sequence locks when txindex lags).
func (u *UtxoCache) UnspentHeight(prevHash [32]byte, vout uint32) (int64, bool) {
	e, ok := u.LookupOutpoint(prevHash, vout)
	if !ok {
		return 0, false
	}
	return e.Height, true
}

// ApplyBlock updates the UTXO set after block consensus checks at height.
// height must equal TipHeight+1 unless the cache is empty (genesis at 0).
func (u *UtxoCache) ApplyBlock(pb *wire.ParsedBlock, height int64) error {
	if u == nil || pb == nil {
		return nil
	}
	if height < 0 {
		return fmt.Errorf("utxo cache: negative height")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.tipHeight >= 0 && height <= u.tipHeight {
		return nil
	}
	if u.tipHeight >= 0 && height != u.tipHeight+1 {
		return fmt.Errorf("utxo cache: height %d not sequential (tip %d)", height, u.tipHeight)
	}
	for _, tx := range pb.Txs {
		u.applyTxLocked(tx, height)
	}
	u.tipHeight = height
	return nil
}

// ApplyBlockRaw updates the cache from a serialized block payload without retaining all txs.
func (u *UtxoCache) ApplyBlockRaw(raw []byte, height int64) error {
	if u == nil || len(raw) == 0 {
		return nil
	}
	if height < 0 {
		return fmt.Errorf("utxo cache: negative height")
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.tipHeight >= 0 && height <= u.tipHeight {
		return nil
	}
	if u.tipHeight >= 0 && height != u.tipHeight+1 {
		return fmt.Errorf("utxo cache: height %d not sequential (tip %d)", height, u.tipHeight)
	}
	if err := wire.ForEachBlockTx(raw, func(_ uint32, tx *wire.Tx) error {
		u.applyTxLocked(tx, height)
		return nil
	}); err != nil {
		return err
	}
	u.tipHeight = height
	return nil
}

func (u *UtxoCache) applyTxLocked(tx *wire.Tx, height int64) {
	coinbase := isCoinbaseTx(tx)
	if !coinbase {
		for _, in := range tx.Vin {
			if isNullOutpoint(&in) {
				continue
			}
			delete(u.coins, outpointKey(in.PrevHash, in.PrevIdx))
		}
	}
	h := tx.TxHash()
	for oi, o := range tx.Vout {
		u.coins[outpointKey(h, uint32(oi))] = UtxoEntry{
			Value:    o.Value,
			PkScript: append([]byte(nil), o.PkScript...),
			Height:   height,
			Coinbase: coinbase,
		}
	}
}

// UnspentEntry returns full coin metadata for ConnectBlock (Core AccessCoins).
func (u *UtxoCache) UnspentEntry(prevHash [32]byte, vout uint32) (height int64, coinbase bool, value int64, pkScript []byte, ok bool) {
	e, ok := u.LookupOutpoint(prevHash, vout)
	if !ok {
		return 0, false, 0, nil, false
	}
	return e.Height, e.Coinbase, e.Value, e.PkScript, true
}

// SerializedHash returns Core-compatible hash_serialized when the cache is non-empty.
// Prefer SerializedHashAtTip when the journal tip hash is available.
func (u *UtxoCache) SerializedHash() string {
	if u == nil {
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}
	return u.SerializedHashLE([32]byte{})
}

// Stats returns aggregate counts for gettxoutsetinfo-style RPC.
func (u *UtxoCache) Stats() (txouts int, totalKoinu int64, bytesSerialized int64) {
	if u == nil {
		return 0, 0, 0
	}
	u.mu.RLock()
	defer u.mu.RUnlock()
	txSet := make(map[[32]byte]struct{})
	for k, e := range u.coins {
		var txid [32]byte
		copy(txid[:], k[:32])
		txSet[txid] = struct{}{}
		totalKoinu += e.Value
		bytesSerialized += int64(36 + 8 + 4 + len(e.PkScript))
	}
	return len(u.coins), totalKoinu, bytesSerialized
}

// CatchUpFromChain applies blocks from max(0, tip+1) through toHeight without clearing existing state.
func (u *UtxoCache) CatchUpFromChain(j *HeaderJournal, raw *RawBlockStore, toHeight int64) error {
	if u == nil || j == nil || raw == nil {
		return fmt.Errorf("utxo catch-up: missing stores")
	}
	from := int64(0)
	if u.TipHeight() >= 0 {
		from = u.TipHeight() + 1
	}
	if from > toHeight {
		return nil
	}
	return u.applyRange(j, raw, from, toHeight)
}

// RebuildFromChain replays ApplyBlock for heights [fromHeight, toHeight] inclusive (resets first).
func (u *UtxoCache) RebuildFromChain(j *HeaderJournal, raw *RawBlockStore, fromHeight, toHeight int64) error {
	if u == nil || j == nil || raw == nil {
		return fmt.Errorf("utxo rebuild: missing stores")
	}
	if fromHeight < 0 || toHeight < fromHeight {
		return fmt.Errorf("utxo rebuild: invalid range %d..%d", fromHeight, toHeight)
	}
	u.Reset()
	return u.applyRange(j, raw, fromHeight, toHeight)
}

func (u *UtxoCache) applyRange(j *HeaderJournal, raw *RawBlockStore, fromHeight, toHeight int64) error {
	for h := fromHeight; h <= toHeight; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return err
		}
		id := pow.BlockHashLE(h80)
		if !raw.Has(id) {
			return fmt.Errorf("utxo rebuild: missing raw block at height %d", h)
		}
		payload, err := raw.Get(id)
		if err != nil {
			return err
		}
		if err := u.ApplyBlockRaw(payload, h); err != nil {
			return fmt.Errorf("height %d apply: %w", h, err)
		}
	}
	return nil
}

// UnspentOutpoint implements consensus.UtxoOutpointSource.
func (u *UtxoCache) UnspentOutpoint(prevHash [32]byte, vout uint32) (value int64, pkScript []byte, ok bool) {
	e, found := u.LookupOutpoint(prevHash, vout)
	if !found {
		return 0, nil, false
	}
	return e.Value, e.PkScript, true
}

func outpointKey(hash [32]byte, idx uint32) [36]byte {
	var k [36]byte
	copy(k[:32], hash[:])
	binary.LittleEndian.PutUint32(k[32:], idx)
	return k
}

func decodeDisplayTxid(display string, out *[32]byte) error {
	b, err := hex.DecodeString(display)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("decode txid: want 64 hex chars")
	}
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return nil
}

func isNullOutpoint(in *wire.TxIn) bool {
	var z [32]byte
	return in.PrevHash == z && in.PrevIdx == 0xffffffff
}

func isCoinbaseTx(tx *wire.Tx) bool {
	return len(tx.Vin) == 1 && isNullOutpoint(&tx.Vin[0])
}
