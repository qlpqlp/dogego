// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// ScannedTx is a wallet-affecting transaction found by scanning raw blocks.
type ScannedTx struct {
	TxID        string
	Category    string // receive | send
	Address     string
	AmountKoinu int64
	FeeKoinu    int64 // send rows: inputs minus all outputs (persisted for compact tx index)
	Vout        uint32
	BlockHeight int64
}

type walletCoin struct {
	value  int64
	script []byte
	addr   string
}

// ScanBlocksRange walks contiguous raw blocks [startHeight, tip] for tracked scriptPubKeys.
// priorReceives seeds the in-memory UTXO map with wallet receive rows below startHeight so
// incremental single-block indexing still records sends.
func ScanBlocksRange(
	j *store.HeaderJournal,
	raw *store.RawBlockStore,
	tracked [][]byte,
	pkhVer, shVer byte,
	startHeight int64,
	txHexByID map[string]string,
	priorReceives []ScannedTx,
) ([]ScannedTx, error) {
	if j == nil || raw == nil {
		return nil, fmt.Errorf("scan: missing chain data")
	}
	if len(tracked) == 0 {
		return nil, nil
	}
	cont, err := store.ContiguousRawBodyHeight(j, raw)
	if err != nil {
		return nil, err
	}
	if cont < 0 {
		return nil, fmt.Errorf("scan: no contiguous raw blocks")
	}
	if startHeight < 0 {
		startHeight = 0
	}
	if startHeight > cont {
		return nil, fmt.Errorf("scan: start height %d above contiguous raw %d", startHeight, cont)
	}
	scriptSet := make(map[string][]byte, len(tracked))
	addrByScript := make(map[string]string, len(tracked))
	for _, pk := range tracked {
		if len(pk) == 0 {
			continue
		}
		k := string(pk)
		scriptSet[k] = pk
		addrByScript[k] = chain.ScriptPubKeyAddress(pk, pkhVer, shVer)
	}
	owned := make(map[[36]byte]walletCoin)
	seedOwnedFromPriorReceives(owned, priorReceives, scriptSet, addrByScript)
	var out []ScannedTx
	for h := startHeight; h <= cont; h++ {
		h80, err := j.ReadHeaderAt(h)
		if err != nil {
			return nil, fmt.Errorf("scan header %d: %w", h, err)
		}
		id := pow.BlockHashLE(h80)
		rawBlk, err := raw.Get(id)
		if err != nil {
			return nil, fmt.Errorf("scan block %d: %w", h, err)
		}
		if err := wire.ForEachBlockTx(rawBlk, func(_ uint32, tx *wire.Tx) error {
			if tx == nil {
				return nil
			}
			txid := mempool.TxIDDisplayHex(tx.TxHash())
			var recvByAddr map[string]int64
			var spentTotal int64
			sendAddr := ""
			for _, in := range tx.Vin {
				if consensus.IsNullOutpoint(&in) {
					continue
				}
				key := walletOutpointKey(in.PrevHash, in.PrevIdx)
				coin, ok := owned[key]
				if !ok {
					continue
				}
				spentTotal += coin.value
				if sendAddr == "" {
					sendAddr = coin.addr
				}
				delete(owned, key)
			}
			for i, o := range tx.Vout {
				pk := o.PkScript
				if len(pk) == 0 {
					continue
				}
				if _, ok := scriptSet[string(pk)]; !ok {
					continue
				}
				addr := addrByScript[string(pk)]
				if addr == "" {
					continue
				}
				if recvByAddr == nil {
					recvByAddr = make(map[string]int64)
				}
				recvByAddr[addr] += o.Value
				key := walletOutpointKey(tx.TxHash(), uint32(i))
				owned[key] = walletCoin{value: o.Value, script: pk, addr: addr}
				out = append(out, ScannedTx{
					TxID: txid, Category: "receive", Address: addr,
					AmountKoinu: o.Value, Vout: uint32(i), BlockHeight: h,
				})
			}
			if spentTotal > 0 {
				var totalOut int64
				for _, o := range tx.Vout {
					totalOut += o.Value
				}
				feeKoinu := spentTotal - totalOut
				if feeKoinu < 0 {
					feeKoinu = 0
				}
				if sendAddr == "" {
					sendAddr = firstAddr(addrByScript)
				}
				sendAmt := SendDisplayKoinu(spentTotal, recvByAddr, sendAddr, tx.Vout, scriptSet)
				out = append(out, ScannedTx{
					TxID: txid, Category: "send", Address: sendAddr,
					AmountKoinu: -sendAmt, FeeKoinu: feeKoinu, Vout: 0, BlockHeight: h,
				})
			}
			if txHexByID != nil && (spentTotal > 0 || len(recvByAddr) > 0) {
				if ser, err := tx.Serialize(); err == nil && len(ser) > 0 {
					txHexByID[txid] = hex.EncodeToString(ser)
				}
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("scan parse %d: %w", h, err)
		}
	}
	return out, nil
}

func walletOutpointKey(hash [32]byte, idx uint32) [36]byte {
	var k [36]byte
	copy(k[:32], hash[:])
	binary.LittleEndian.PutUint32(k[32:], idx)
	return k
}

func firstAddr(m map[string]string) string {
	for _, a := range m {
		if a != "" {
			return a
		}
	}
	return ""
}

// TrackedScripts returns spend + watch scriptPubKeys for rescan (works when encrypted if watch-only).
func (w *Disk) TrackedScripts() [][]byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	seen := make(map[string]struct{})
	var out [][]byte
	add := func(pk []byte) {
		if len(pk) == 0 {
			return
		}
		k := string(pk)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, bytes.Clone(pk))
	}
	for _, s := range w.spendScriptsLocked() {
		add(s)
	}
	if len(out) == 0 {
		add(w.p2pkhScriptLocked())
	}
	for _, pk := range w.watchScripts {
		add(pk)
	}
	return out
}
