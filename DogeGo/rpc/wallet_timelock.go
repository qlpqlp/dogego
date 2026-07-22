// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/consensus"
	"dogego/wire"
)

// applyWalletSpendTimelocks sets nLockTime / input nSequence for wallet CLTV/CSV P2SH spends (Core wallet behaviour).
func applyWalletSpendTimelocks(tx *wire.Tx, paths *DataPaths) {
	if tx == nil || paths == nil || paths.Utxo == nil {
		return
	}
	var maxLT int64
	needsCSV := false
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if consensus.IsNullOutpoint(in) {
			continue
		}
		id := txidToRPC(in.PrevHash)
		e, ok := paths.Utxo.Lookup(id, in.PrevIdx)
		if !ok {
			continue
		}
		redeem := lookupWalletInputRedeem(paths, e.PkScript)
		if op, ok := consensus.CSVOperandFromRedeem(redeem); ok {
			needsCSV = true
			if !consensus.CSVInputSequenceSatisfies(in.Sequence, op) {
				in.Sequence = consensus.CSVOperandToInputSequence(op)
			}
		}
		if lock, ok := consensus.CLTVLockTimeFromRedeem(redeem); ok && lock > maxLT {
			maxLT = lock
			if in.Sequence == wire.SequenceFinal {
				in.Sequence = wire.MaxBIP125RBFSequence
			}
		}
	}
	if needsCSV && tx.Version < 2 {
		tx.Version = 2
	}
	if maxLT <= 0 {
		return
	}
	if int64(tx.LockTime) < maxLT {
		if maxLT > 0xffffffff {
			tx.LockTime = 0xffffffff
		} else {
			tx.LockTime = uint32(maxLT)
		}
	}
}

func lookupWalletInputRedeem(paths *DataPaths, pkScript []byte) []byte {
	if paths == nil || len(pkScript) == 0 {
		return nil
	}
	if paths.WalletWatchRedeemScript != nil {
		if redeem := paths.WalletWatchRedeemScript(pkScript); len(redeem) > 0 {
			return redeem
		}
	}
	return nil
}
