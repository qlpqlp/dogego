// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"sync"

	"dogego/applog"
	"dogego/bloom"
	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

// SPVBloomClient tracks BIP37 filterload state for SPV (headers + filtered blocks) mode.
type SPVBloomClient struct {
	mu            sync.Mutex
	filter        *bloom.Filter
	active        bool
	pendingHeight int64
	pendingMatch  map[[32]byte]struct{}
	wallet        *wallet.Disk
	params        chain.Params
	journal       *store.HeaderJournal
}

// NewSPVBloomClient builds a client bound to wallet + header journal (height lookup).
func NewSPVBloomClient(w *wallet.Disk, p chain.Params, j *store.HeaderJournal) *SPVBloomClient {
	c := &SPVBloomClient{wallet: w, params: p, journal: j, pendingMatch: make(map[[32]byte]struct{})}
	if w != nil {
		_ = c.RebuildFromWallet()
	}
	return c
}

// RebuildFromWallet recreates the bloom filter from current wallet scripts.
func (c *SPVBloomClient) RebuildFromWallet() error {
	if c == nil || c.wallet == nil {
		return fmt.Errorf("spv bloom: nil")
	}
	scripts := c.wallet.BloomScripts(c.params.PubkeyHashAddrID, c.params.ScriptHashAddrID)
	n := uint32(len(scripts) + 10)
	if n < 20 {
		n = 20
	}
	f, err := bloom.NewEmpty(n, 0.0001, 0, bloom.UpdateAll)
	if err != nil {
		return err
	}
	for _, pk := range scripts {
		f.Insert(pk)
	}
	c.mu.Lock()
	c.filter = f
	c.active = !f.IsEmpty()
	c.mu.Unlock()
	return nil
}

// Active reports whether a non-empty filter is ready.
func (c *SPVBloomClient) Active() bool {
	if c == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active && c.filter != nil && !c.filter.IsEmpty()
}

// SendFilterLoad sends filterload to a peer that advertises NODE_BLOOM.
func (c *SPVBloomClient) SendFilterLoad(mw *MsgWriter, peerServices uint64) error {
	if c == nil || mw == nil || !c.Active() {
		return nil
	}
	if peerServices&chain.ServiceBloom == 0 {
		return nil
	}
	c.mu.Lock()
	f := c.filter
	c.mu.Unlock()
	pl, err := f.EncodeWire()
	if err != nil {
		return err
	}
	if err := mw.Write("filterload", pl); err != nil {
		return err
	}
	applog.Line("spv", "sent BIP37 filterload to peer")
	return nil
}

// FilterAddScript inserts a new script and pushes filteradd when the peer already has our filter.
func (c *SPVBloomClient) FilterAddScript(mw *MsgWriter, peerServices uint64, script []byte) error {
	if c == nil || len(script) == 0 {
		return nil
	}
	c.mu.Lock()
	if c.filter == nil {
		c.mu.Unlock()
		return c.RebuildFromWallet()
	}
	c.filter.Insert(script)
	c.active = true
	c.mu.Unlock()
	if mw == nil || peerServices&chain.ServiceBloom == 0 {
		return nil
	}
	pl := bloom.EncodeFilterAdd(script)
	return mw.Write("filteradd", pl)
}

// RequestFilteredBlocks converts block inv to MSG_FILTERED_BLOCK getdata when SPV bloom is active.
func (c *SPVBloomClient) RequestFilteredBlocks(mw *MsgWriter, entries []wire.InvEntry) error {
	if c == nil || mw == nil || !c.Active() {
		return nil
	}
	var want []wire.InvEntry
	for _, e := range entries {
		if e.Type == wire.InvTypeBlock || e.Type == wire.InvTypeWitnessBlock || e.Type == wire.InvTypeFilteredBlock {
			want = append(want, wire.InvEntry{Type: wire.InvTypeFilteredBlock, Hash: e.Hash})
		}
	}
	if len(want) == 0 {
		return nil
	}
	if len(want) > 16 {
		want = want[:16]
	}
	pl, err := wire.EncodeGetData(want)
	if err != nil {
		return err
	}
	return mw.Write("getdata", pl)
}

// HandleMerkleBlock verifies merkleblock, records pending matched txids, and returns match count.
func (c *SPVBloomClient) HandleMerkleBlock(payload []byte) (int, error) {
	header80, matches, err := parseMerkleBlockMatches(payload)
	if err != nil {
		return 0, err
	}
	height := int64(-1)
	if c != nil && c.journal != nil {
		id := pow.BlockHashLE(header80)
		if h, herr := c.journal.HeightByBlockHashLE(id); herr == nil {
			height = h
		}
	}
	c.mu.Lock()
	c.pendingHeight = height
	c.pendingMatch = make(map[[32]byte]struct{}, len(matches))
	for _, m := range matches {
		c.pendingMatch[m] = struct{}{}
	}
	c.mu.Unlock()
	if len(matches) > 0 {
		applog.Line("spv", fmt.Sprintf("merkleblock: %d matched tx(s) at height %d", len(matches), height))
	}
	return len(matches), nil
}

// HandleMatchedTx ingests a tx if it was listed in the last merkleblock (or always classifies when no pending set).
func (c *SPVBloomClient) HandleMatchedTx(raw []byte) error {
	if c == nil || c.wallet == nil || len(raw) == 0 {
		return nil
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil || tx == nil {
		return err
	}
	hash := tx.TxHash()
	c.mu.Lock()
	height := c.pendingHeight
	if len(c.pendingMatch) > 0 {
		if _, ok := c.pendingMatch[hash]; !ok {
			c.mu.Unlock()
			return nil
		}
		delete(c.pendingMatch, hash)
	}
	c.mu.Unlock()
	return c.wallet.IngestSPVMatchedTx(tx, height, c.params.PubkeyHashAddrID, c.params.ScriptHashAddrID)
}

func parseMerkleBlockMatches(payload []byte) (header80 []byte, matches [][32]byte, err error) {
	header80, pmt, err := wire.ParseMerkleBlockProof(payload)
	if err != nil {
		return nil, nil, err
	}
	root, matched, _, ok := pmt.ExtractMatches()
	if !ok {
		return nil, nil, fmt.Errorf("merkleblock: extract failed")
	}
	var hdrRoot [32]byte
	copy(hdrRoot[:], header80[36:68])
	if root != hdrRoot {
		return nil, nil, fmt.Errorf("merkleblock: merkle root mismatch")
	}
	return header80, matched, nil
}

// HandleMerkleBlockStandalone verifies a merkleblock (exported for tests).
func HandleMerkleBlockStandalone(payload []byte) (header80 []byte, matches [][32]byte, err error) {
	return parseMerkleBlockMatches(payload)
}

// MaybePushSPVBloom sends filterload after handshake when SPV bloom is active.
func MaybePushSPVBloom(spv *SPVBloomClient, mw *MsgWriter, dv *wire.DecodedVersion) {
	if spv == nil || mw == nil || dv == nil {
		return
	}
	if err := spv.SendFilterLoad(mw, dv.Services); err != nil {
		applog.Line("spv", "filterload: "+err.Error())
	}
}
