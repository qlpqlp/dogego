// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/chain"
)

// PrunedImport records a receive credited via importprunedfunds (no local block data required).
type PrunedImport struct {
	TxID        string
	BlockHeight int64
	BlockHash   string
	Vout        uint32
	AmountKoinu int64
	Script      []byte
}

type prunedImportJSON struct {
	TxID        string `json:"txid"`
	BlockHeight int64  `json:"block_height"`
	BlockHash   string `json:"block_hash,omitempty"`
	Vout        uint32 `json:"vout"`
	AmountKoinu int64  `json:"amount_koinu"`
	ScriptHex   string `json:"script_hex"`
}

func (w *Disk) loadPrunedImports(rows []prunedImportJSON) {
	w.prunedImports = w.prunedImports[:0]
	for _, r := range rows {
		txid := normalizeWalletTxID(r.TxID)
		if len(txid) != 64 {
			continue
		}
		script, err := hex.DecodeString(strings.TrimSpace(r.ScriptHex))
		if err != nil || len(script) == 0 {
			continue
		}
		w.prunedImports = append(w.prunedImports, PrunedImport{
			TxID: txid, BlockHeight: r.BlockHeight, BlockHash: normalizeWalletTxID(r.BlockHash),
			Vout: r.Vout, AmountKoinu: r.AmountKoinu, Script: script,
		})
	}
}

// ImportPrunedReceive records outputs proven via importprunedfunds (deduped by txid:vout).
func (w *Disk) ImportPrunedReceive(txid string, height int64, blockHash string, vout uint32, amountKoinu int64, script []byte) error {
	txid = normalizeWalletTxID(txid)
	if len(txid) != 64 || len(script) == 0 || amountKoinu <= 0 {
		return fmt.Errorf("invalid pruned import")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, p := range w.prunedImports {
		if p.TxID == txid && p.Vout == vout {
			return nil
		}
	}
	w.prunedImports = append(w.prunedImports, PrunedImport{
		TxID: txid, BlockHeight: height, BlockHash: blockHash,
		Vout: vout, AmountKoinu: amountKoinu, Script: append([]byte(nil), script...),
	})
	return w.saveLocked()
}

// RemovePrunedImportByTxID drops all pruned imports for txid (removeprunedfunds).
func (w *Disk) RemovePrunedImportByTxID(txid string) bool {
	txid = normalizeWalletTxID(txid)
	if len(txid) != 64 {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	found := false
	kept := w.prunedImports[:0]
	for _, p := range w.prunedImports {
		if p.TxID == txid {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	if !found {
		return false
	}
	w.prunedImports = kept
	_ = w.saveLocked()
	return true
}

// ListPrunedImports returns a copy of pruned-fund imports.
func (w *Disk) ListPrunedImports() []PrunedImport {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make([]PrunedImport, len(w.prunedImports))
	copy(out, w.prunedImports)
	return out
}

// OwnsScript reports whether script belongs to this wallet (spend key or watch import).
func (w *Disk) OwnsScript(script []byte) bool {
	if len(script) == 0 {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.hdOwnsScriptLocked(script) {
		return true
	}
	if w.priv != nil {
		if bytesEqual(script, w.p2pkhScriptLocked()) {
			return true
		}
	}
	for _, pk := range w.watchScripts {
		if bytesEqual(script, pk) {
			return true
		}
	}
	return false
}

func (w *Disk) p2pkhScriptLocked() []byte {
	_, h160, err := chain.Base58CheckDecode(w.addr)
	if err != nil {
		return nil
	}
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
