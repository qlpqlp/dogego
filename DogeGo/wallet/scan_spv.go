// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/wallet/txdb"
	"dogego/wire"
)

// ClassifyTxAgainstTracked returns receive/send scan rows for one transaction.
func ClassifyTxAgainstTracked(
	tx *wire.Tx,
	height int64,
	tracked [][]byte,
	pkhVer, shVer byte,
	priorReceives []ScannedTx,
) []ScannedTx {
	if tx == nil || len(tracked) == 0 {
		return nil
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
	if len(scriptSet) == 0 {
		return nil
	}
	owned := make(map[[36]byte]walletCoin)
	seedOwnedFromPriorReceives(owned, priorReceives, scriptSet, addrByScript)

	txid := mempool.TxIDDisplayHex(tx.TxHash())
	var recvByAddr map[string]int64
	var spentTotal int64
	sendAddr := ""
	var out []ScannedTx
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
		out = append(out, ScannedTx{
			TxID: txid, Category: "receive", Address: addr,
			AmountKoinu: o.Value, Vout: uint32(i), BlockHeight: height,
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
			AmountKoinu: -sendAmt, FeeKoinu: feeKoinu, Vout: 0, BlockHeight: height,
		})
	}
	return out
}

// IngestSPVMatchedTx classifies a BIP37 matched tx and persists wallet history rows.
func (w *Disk) IngestSPVMatchedTx(tx *wire.Tx, height int64, pkhVer, shVer byte) error {
	if w == nil || tx == nil {
		return nil
	}
	prior := w.priorReceiveRows(height)
	rows := ClassifyTxAgainstTracked(tx, height, w.TrackedScripts(), pkhVer, shVer, prior)
	if len(rows) == 0 {
		return nil
	}
	txid := rows[0].TxID
	hexByID := make(map[string]string)
	if ser, err := tx.Serialize(); err == nil && len(ser) > 0 {
		hexByID[txid] = hex.EncodeToString(ser)
	}
	w.mu.Lock()
	kept := w.scannedTx[:0]
	for _, r := range w.scannedTx {
		if r.TxID == txid {
			continue
		}
		kept = append(kept, r)
	}
	kept = append(kept, rows...)
	w.scannedTx = kept
	_ = w.consumeKeypoolFromScannedLocked(rows)
	w.rebuildUsedRecvScriptsLocked()
	saveErr := w.saveLocked()
	w.mu.Unlock()
	if saveErr != nil {
		return saveErr
	}
	if err := w.withTxDB(func(db *txdb.DB) error {
		return db.AppendBlock(height, scannedTxToTxRows(rows))
	}); err != nil {
		return err
	}
	w.rememberTxHexBatch(hexByID)
	return nil
}
