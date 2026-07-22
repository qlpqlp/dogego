// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/rpc"
	"dogego/wallet"
)

func wireWalletPrunedImports(paths *rpc.DataPaths, disk *wallet.Disk) {
	if paths == nil || disk == nil {
		return
	}
	paths.WalletOwnsScript = func(script []byte) bool { return disk.OwnsScript(script) }
	paths.WalletImportPrunedReceive = func(txid string, height int64, blockHash string, vout uint32, amountKoinu int64, script []byte) error {
		return disk.ImportPrunedReceive(txid, height, blockHash, vout, amountKoinu, script)
	}
	paths.WalletListPrunedImports = func() []rpc.WalletPrunedImport {
		rows := disk.ListPrunedImports()
		out := make([]rpc.WalletPrunedImport, len(rows))
		for i, r := range rows {
			out[i] = rpc.WalletPrunedImport{
				TxID: r.TxID, BlockHeight: r.BlockHeight, BlockHash: r.BlockHash,
				Vout: r.Vout, AmountKoinu: r.AmountKoinu, Script: r.Script,
			}
		}
		return out
	}
	paths.WalletRemovePrunedImport = func(txid string) bool { return disk.RemovePrunedImportByTxID(txid) }
}
