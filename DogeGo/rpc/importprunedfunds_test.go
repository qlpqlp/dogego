// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wallet"
	"dogego/wire"
)

func TestExecImportPrunedFundsGenesisWithWallet(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	rawBlock, err := chain.RebootTestnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	h80 := append([]byte(nil), rawBlock[:80]...)
	best := pow.BlockHashHex(h80)
	j := &memJournal{count: 1, tip: 0, best: best, gen: best, hdrs: [][]byte{h80}}

	pb, err := wire.ParseBlock(rawBlock)
	if err != nil {
		t.Fatal(err)
	}
	tx := pb.Txs[0]
	txRaw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	vTxid := make([][32]byte, len(pb.Txs))
	vMatch := make([]bool, len(pb.Txs))
	for i, btx := range pb.Txs {
		vTxid[i] = btx.TxHash()
		vMatch[i] = btx.TxHash() == tx.TxHash()
	}
	pmt, err := wire.NewPartialMerkleTree(vTxid, vMatch)
	if err != nil {
		t.Fatal(err)
	}
	proof, err := wire.SerializeMerkleBlock(h80, pmt)
	if err != nil {
		t.Fatal(err)
	}
	proofHex := hex.EncodeToString(proof)

	dir := t.TempDir()
	w, err := wallet.LoadOrCreate(filepath.Join(dir, "wallet.json"), p.PubkeyHashAddrID)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.AddWatchScript(append([]byte(nil), tx.Vout[0].PkScript...)); err != nil {
		t.Fatal(err)
	}

	paths := &DataPaths{}
	wireWalletPrunedImportsTest(paths, w)

	txJ, _ := json.Marshal(hex.EncodeToString(txRaw))
	proofJ, _ := json.Marshal(proofHex)
	res, code, msg := execImportPrunedFunds("testnet", paths, j, []json.RawMessage{txJ, proofJ})
	if code != 0 {
		t.Fatalf("import: code=%d msg=%q", code, msg)
	}
	if res != nil {
		t.Fatalf("want null result, got %#v", res)
	}
	imports := w.ListPrunedImports()
	if len(imports) != 1 {
		t.Fatalf("imports: %d", len(imports))
	}
}

func wireWalletPrunedImportsTest(paths *DataPaths, disk *wallet.Disk) {
	paths.WalletOwnsScript = func(script []byte) bool { return disk.OwnsScript(script) }
	paths.WalletImportPrunedReceive = func(txid string, height int64, blockHash string, vout uint32, amountKoinu int64, script []byte) error {
		return disk.ImportPrunedReceive(txid, height, blockHash, vout, amountKoinu, script)
	}
	paths.WalletListPrunedImports = func() []WalletPrunedImport {
		rows := disk.ListPrunedImports()
		out := make([]WalletPrunedImport, len(rows))
		for i, r := range rows {
			out[i] = WalletPrunedImport{
				TxID: r.TxID, BlockHeight: r.BlockHeight, BlockHash: r.BlockHash,
				Vout: r.Vout, AmountKoinu: r.AmountKoinu, Script: r.Script,
			}
		}
		return out
	}
}
