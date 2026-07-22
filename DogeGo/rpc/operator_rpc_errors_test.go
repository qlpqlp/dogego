// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/primitives"
	"dogego/secp256k1"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

// Golden operator RPC error codes (Core-shaped classes for common failure paths).
func TestOperatorRPCErrorCodesGolden(t *testing.T) {
	t.Run("pruneblockchain_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execPruneBlockchain(nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_bad_height", func(t *testing.T) {
		paths := &DataPaths{TruncateToHeight: func(int64) error { return nil }}
		_, code, msg := execTruncateToHeight(paths, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "invalid height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindextx_no_datadir", func(t *testing.T) {
		_, code, msg := execReindexTx(nil, nil)
		if code != -1 || !strings.Contains(msg, "chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("recoverheaders_unwired", func(t *testing.T) {
		_, code, msg := execDogegoRecoverHeaders(nil)
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_bad_hash", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "test", nil, []json.RawMessage{json.RawMessage(`"not-a-hash"`)})
		if code != -8 {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_store_unavailable", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, nil, nil, "test", nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "block store") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_missing_inputs", func(t *testing.T) {
		code, msg := consensus.SendRawTransactionRPCError(consensus.ErrMissingPrevout)
		if code != -25 || msg != "Missing inputs" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_policy_reject", func(t *testing.T) {
		code, msg := consensus.SendRawTransactionRPCError(consensus.ErrMempoolCoinbase)
		if code != -26 {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_param_count", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_bad_checklevel", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`9`)})
		if code != -8 || !strings.Contains(msg, "checklevel") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_no_pool", func(t *testing.T) {
		_, code, msg := execTestMempoolAccept(nil, nil, nil, nil, nil, nil, false, chain.RebootTestnet)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_not_found", func(t *testing.T) {
		paths := &DataPaths{
			InvalidateBlock: func(string) error { return fmt.Errorf("block not found") },
		}
		_, code, msg := execInvalidateBlock(nil, paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)})
		if code != -5 || msg != "Block not found" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_not_found", func(t *testing.T) {
		paths := &DataPaths{
			ReconsiderBlock: func(string) error { return fmt.Errorf("block not found") },
		}
		_, code, msg := execReconsiderBlock(paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('b') + `"`)})
		if code != -5 || msg != "Block not found" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("savemempool_no_datadir", func(t *testing.T) {
		_, code, msg := execSaveMempool(mempool.New(10), nil)
		if code != -1 || !strings.Contains(msg, "chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadmempool_no_datadir", func(t *testing.T) {
		_, code, msg := execLoadMempool(mempool.New(10), nil, nil, nil, nil, chain.RebootTestnet)
		if code != -1 || !strings.Contains(msg, "chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_unwired", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_unwired", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('c') + `"`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_not_found", func(t *testing.T) {
		paths := &DataPaths{
			MarkPreciousBlock: func(string) error { return fmt.Errorf("block not found") },
		}
		_, code, msg := execPreciousBlock(nil, paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('d') + `"`)})
		if code != -5 || msg != "Block not found" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_unwired", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('e') + `"`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_bad_height", func(t *testing.T) {
		j := &memJournal{tip: 2, best: "b", gen: "g", count: 3, hdrs: [][]byte{make([]byte, 80), make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{json.RawMessage(`"not-a-number"`)})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_unwired", func(t *testing.T) {
		_, code, msg := execPruneBlockchain(nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`2`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindexblockfilters_no_datadir", func(t *testing.T) {
		_, code, msg := execReindexBlockFilters(nil, nil, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_genesis", func(t *testing.T) {
		paths := &DataPaths{
			InvalidateBlock: func(string) error { return fmt.Errorf("cannot invalidate genesis") },
		}
		_, code, msg := execInvalidateBlock(nil, paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('f') + `"`)})
		if code != -8 || !strings.Contains(msg, "genesis") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_no_param", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "test", nil, nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("savemempool_no_pool", func(t *testing.T) {
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execSaveMempool(nil, paths)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_bad_nblocks", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("upgradetxindex_bad_maxfiles", func(t *testing.T) {
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execUpgradeTxIndex(paths, []json.RawMessage{json.RawMessage(`"nope"`)})
		if code != -8 || !strings.Contains(msg, "max_files") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_bad_verbosity", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "test", nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`9`),
		})
		if code != -8 || !strings.Contains(msg, "verbosity") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("disconnectnode_no_address", func(t *testing.T) {
		_, code, msg := execDisconnectNode(&DataPaths{DisconnectNode: func(string) error { return nil }}, nil)
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("disconnectnode_p2p_disabled", func(t *testing.T) {
		_, code, msg := execDisconnectNode(nil, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`)})
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_missing_args", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{json.RawMessage(`"1.2.3.4"`)})
		if code != -8 || !strings.Contains(msg, "command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_p2p_disabled", func(t *testing.T) {
		_, code, msg := execSetBan(nil, []json.RawMessage{json.RawMessage(`"1.2.3.4"`), json.RawMessage(`"add"`)})
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_p2p_disabled", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`), json.RawMessage(`"add"`)})
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_no_txid", func(t *testing.T) {
		_, code, msg := execGetRawTransaction(nil, nil, nil, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_not_found", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		txid := repeatHex('a')
		_, code, msg := execGetRawTransaction(ix, raw, j, mempool.New(10), nil, []json.RawMessage{json.RawMessage(`"` + txid + `"`)})
		if code != -5 || msg != "No such mempool or blockchain transaction" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_no_backend", func(t *testing.T) {
		_, code, msg := execGetRawTransaction(nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('b') + `"`)})
		if code != -18 || !strings.Contains(msg, "no chain index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_bad_blockhash", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetRawTransaction(ix, raw, j, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`false`),
			json.RawMessage(`"deadbeef"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "test", []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_bad_nblocks", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "test", []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"D7YQWv2X8K9mN3pL6rT1sU4vW5xY7zA8bC9dE0fG1hJ"`),
		})
		if code != -8 || !strings.Contains(msg, "positive integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_no_pool", func(t *testing.T) {
		_, code, msg := execSendRawTransaction(nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"010203"`)}, nil, true, chain.RebootTestnet)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_bad_hex", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSendRawTransaction(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"nothex"`)}, nil, true, chain.RebootTestnet)
		if code != -22 || msg != "TX decode failed" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_no_hex", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSendRawTransaction(p, nil, nil, nil, nil, nil, nil, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_no_pool", func(t *testing.T) {
		_, code, msg := execGetMempoolEntry(nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('d') + `"`)})
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_not_found", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('e') + `"`)})
		if code != -5 || msg != "Transaction not in mempool" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_no_pool", func(t *testing.T) {
		_, code, msg := execGetMempoolAncestors(nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('f') + `"`)})
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_not_found", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('1') + `"`)})
		if code != -5 || msg != "Transaction not in mempool" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindextx_bad_clear", func(t *testing.T) {
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execReindexTx(paths, []json.RawMessage{json.RawMessage(`"yes"`)})
		if code != -8 || !strings.Contains(msg, "clear must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_no_pool", func(t *testing.T) {
		_, code, msg := execImportMempool(nil, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{json.RawMessage(`"/tmp/x.json"`)})
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_wrong_arg_count", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execImportMempool(p, nil, nil, nil, nil, chain.RebootTestnet, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_missing_file", func(t *testing.T) {
		p := mempool.New(10)
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execImportMempool(p, paths, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{json.RawMessage(`"missing_mempool.json"`)})
		if code != -8 || !strings.Contains(msg, "importmempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execScanBlocks("test", nil, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("syncutxo_unavailable", func(t *testing.T) {
		_, code, msg := execSyncUtxo(nil, nil)
		if code != -18 || !strings.Contains(msg, "syncutxo") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getwalletinfo_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetWalletInfo(paths, nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_bad_minconf", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execListUnspent("testnet", paths, nil, nil, nil, []json.RawMessage{json.RawMessage(`"x"`)})
		if code != -8 || !strings.Contains(msg, "minconf") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_path_required", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		j := &memJournal{tip: 0}
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execLoadTxOutSet(j, nil, paths, nil)
		if code != -8 || !strings.Contains(msg, "path required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_unwired", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_no_pool", func(t *testing.T) {
		_, code, msg := execSubmitPackage(nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`[]`)}, nil, false, chain.RebootTestnet)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_empty_package", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`[]`)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "empty package") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_no_pool", func(t *testing.T) {
		_, code, msg := execBumpFee("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)}, nil, chain.RebootTestnet)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execScanTxOutSet("testnet", nil, nil, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_unknown_action", func(t *testing.T) {
		_, code, msg := execScanTxOutSet("testnet", nil, nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"nope"`)})
		if code != -8 || !strings.Contains(msg, "unknown action") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetBalance(paths, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`), json.RawMessage(`1`), json.RawMessage(`false`), json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalances_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetBalances("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_no_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execTestMempoolAccept(p, nil, nil, nil, nil, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_empty_array", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execTestMempoolAccept(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`[]`)}, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "empty array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execAbandonTransactionWallet("testnet", nil, nil, nil, mempool.New(10), nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_no_pool", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('b') + `"`)})
		if code != -1 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmempoolpaused_no_pool", func(t *testing.T) {
		_, code, msg := execSetMempoolPaused(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmempoolpaused_no_param", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSetMempoolPaused(p, nil)
		if code != -8 || !strings.Contains(msg, "paused") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_unwired", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_bad_height", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_invalid_txid", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetTransactionWallet("testnet", paths, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"deadbeef"`)})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_no_wallet", func(t *testing.T) {
		_, code, msg := execGetNewAddress("testnet", nil, nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_wrong_arg_count", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(nil, raw, ix, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"x"`)}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_no_wallet", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_no_utxo", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"0100000000"`)})
		if code != -1 || !strings.Contains(msg, "UTXO") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_bad_hex", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{json.RawMessage(`"nothex"`)})
		if code != -8 || !strings.Contains(msg, "decode") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumptxoutset_no_journal", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execDumpTxOutSet(nil, nil, paths, nil)
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_wrong_arg_count", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_no_params", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("validateaddress_no_param", func(t *testing.T) {
		_, code, msg := execValidateAddress("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritise_no_pool", func(t *testing.T) {
		_, code, msg := execPrioritiseTransaction(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`1000`),
		})
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatesmartfee_bad_mode", func(t *testing.T) {
		_, code, msg := execEstimateSmartFee(nil, []json.RawMessage{
			json.RawMessage(`6`),
			json.RawMessage(`"fast"`),
		})
		if code != -8 || !strings.Contains(msg, "estimate_mode") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_no_wallet", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + addr + `"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_p2sh_with_address", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -5 || !strings.Contains(msg, "p2sh") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_no_wallet", func(t *testing.T) {
		pub := "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + pub + `"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_bad_hex", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"nothex"`)})
		if code != -5 || !strings.Contains(msg, "Pubkey") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_invalid_address", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`600`),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_not_implemented", func(t *testing.T) {
		_, code, msg := execEncryptWallet([]json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_missing_params", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_no_hex", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithWallet("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_no_wallet", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithWallet("testnet", nil, []json.RawMessage{json.RawMessage(`"0100000000"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_invalid_address", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"notanaddress"`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_invalid_hex", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{json.RawMessage(`"nothex"`)})
		if code != -8 || !strings.Contains(msg, "invalid hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_no_params", func(t *testing.T) {
		_, code, msg := execCombineRawTransaction(nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_one_tx", func(t *testing.T) {
		_, code, msg := execCombineRawTransaction([]json.RawMessage{json.RawMessage(`["0100000000"]`)})
		if code != -8 || !strings.Contains(msg, "at least two") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{json.RawMessage(`"msg"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_invalid_address", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{
			json.RawMessage(`"notanaddress"`),
			json.RawMessage(`"hello"`),
		})
		if code != -3 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_missing_params", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_empty_amounts", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_no_wallet", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(fmt.Sprintf(`{"%s": 1.0}`, addr)),
		}, nil, false, chain.RebootTestnet)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletLock(nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_not_implemented", func(t *testing.T) {
		_, code, msg := execRescan(nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_no_wallet", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{json.RawMessage(`"backup.dat"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_invalid_amount", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_not_implemented", func(t *testing.T) {
		k1, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		k2, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
		p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(fmt.Sprintf(`["%s","%s"]`, p1, p2)),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_segwit_disabled", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{json.RawMessage(`"` + addr + `"`)})
		if code != -4 || !strings.Contains(msg, "Segregated witness") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getunconfirmedbalance_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetUnconfirmedBalance("testnet", nil, nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_no_params", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", nil)
		if code != -8 || !strings.Contains(msg, "required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_no_wallet", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_no_wallet", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{json.RawMessage(`"wallet.txt"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_no_hex", func(t *testing.T) {
		_, code, msg := execSignRawTransaction("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_bad_txid", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[{"txid":"bad","vout":0}]`),
		})
		if code != -8 || !strings.Contains(msg, "txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_negative_count", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "Negative count") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_non_wallet", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{json.RawMessage(`"` + repeatHex('a') + `"`)})
		if code != -5 || !strings.Contains(msg, "non-wallet") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getpeerinfo_p2p_disabled", func(t *testing.T) {
		_, code, msg := execGetPeerInfoRPC(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("ping_p2p_disabled", func(t *testing.T) {
		_, code, msg := execPing(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatefee_no_params", func(t *testing.T) {
		_, code, msg := execEstimateFee(nil, nil)
		if code != -8 || !strings.Contains(msg, "nblocks required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatepriority_no_params", func(t *testing.T) {
		_, code, msg := execEstimatePriority(nil, nil)
		if code != -8 || !strings.Contains(msg, "nblocks required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_extra_args", func(t *testing.T) {
		_, code, msg := execListLockUnspent([]json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_witness_not_supported", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`"cHNidP8BAHECAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAA"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "witness PSBT") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_no_params", func(t *testing.T) {
		_, code, msg := execFinalizePsbt(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("analyzepsbt_no_params", func(t *testing.T) {
		_, code, msg := execAnalyzePsbt(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbts_empty_array", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{json.RawMessage(`[]`)})
		if code != -8 || !strings.Contains(msg, "at least one PSBT") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_no_wallet", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`{}`),
		})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_no_utxo", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`{}`),
		})
		if code != -1 || !strings.Contains(msg, "UTXO cache not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_no_pool", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execPsbtBumpFee("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
		})
		if code != -18 || !strings.Contains(msg, "mempool not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_no_params", func(t *testing.T) {
		_, code, msg := execConvertToPsbt(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_empty_outputs", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, []json.RawMessage{json.RawMessage(`[]`)})
		if code != -8 || !strings.Contains(msg, "outputs array required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_bad_txid", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, []json.RawMessage{
			json.RawMessage(`[{"txid":"not64hex","vout":0}]`),
		})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_no_wallet", func(t *testing.T) {
		_, code, msg := execSimulateRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["0100000000"]`),
		})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_empty_array", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "array of hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_no_wallet", func(t *testing.T) {
		_, code, msg := execWalletProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"cHNidP8BAHECAAAAAQAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAQAAAAAA"`),
		})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_no_params", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execWalletProcessPsbt("testnet", paths, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("utxoupdatepsbt_no_params", func(t *testing.T) {
		_, code, msg := execUtxoUpdatePsbt(nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_no_params", func(t *testing.T) {
		_, code, msg := execCombinePsbt(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importmnemonic_unwired", func(t *testing.T) {
		_, code, msg := execDogegoImportMnemonic("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"`),
		})
		if code != -1 || !strings.Contains(msg, "dogego_importmnemonic") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_unwired", func(t *testing.T) {
		_, code, msg := execDogegoImportBIP38("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"6PTestKey"`),
			json.RawMessage(`"pass"`),
		})
		if code != -1 || !strings.Contains(msg, "dogego_importbip38") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_missing_passphrase", func(t *testing.T) {
		paths := &DataPaths{WalletImportBIP38: func(enc, pass string) (string, error) { return "", nil }}
		_, code, msg := execDogegoImportBIP38("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`"6PTestKey"`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_no_index", func(t *testing.T) {
		_, code, msg := execGetTxOutProof(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["` + repeatHex('a') + `"]`),
		})
		if code != -18 || !strings.Contains(msg, "transaction index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_empty_txids", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetTxOutProof(ix, raw, nil, []json.RawMessage{
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_bad_txid", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetTxOutProof(ix, raw, nil, []json.RawMessage{
			json.RawMessage(`["nothex"]`),
		})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifytxoutproof_no_params", func(t *testing.T) {
		_, code, msg := execVerifyTxOutProof(nil, nil)
		if code != -8 || !strings.Contains(msg, "proof hex required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_no_params", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid and vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_bad_vout", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutsetinfo_no_journal", func(t *testing.T) {
		_, code, msg := execGetTxOutSetInfo(nil, nil, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "no chain state") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_no_params", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_bad_hex", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"zz"`),
		})
		if code != -8 || !strings.Contains(msg, "decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_no_journal", func(t *testing.T) {
		_, code, msg := execGetBlockTemplate(nil, nil, nil, nil, nil, "testnet", 0, nil)
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_proposal_missing_data", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{"mode":"proposal"}`),
		})
		if code != -8 || !strings.Contains(msg, "Missing data") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_segwit_rules_rejected", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{"rules":["segwit"]}`),
		})
		if code != -8 || !strings.Contains(msg, "segwit") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, []json.RawMessage{json.RawMessage(`"avoid_reuse"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_unwired", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, []json.RawMessage{
			json.RawMessage(`"avoid_reuse"`),
			json.RawMessage(`true`),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_unknown_flag", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetAvoidReuse:  func(bool) error { return nil },
		}
		_, code, msg := execSetWalletFlag(paths, []json.RawMessage{
			json.RawMessage(`"not_a_flag"`),
			json.RawMessage(`true`),
		})
		if code != -4 || !strings.Contains(msg, "unknown flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_no_params", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_bad_descriptor", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"not_a_descriptor"`),
		})
		if code != -5 || !strings.Contains(msg, "Unsupported descriptor") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listdescriptors_extra_args", func(t *testing.T) {
		_, code, msg := execListDescriptors("testnet", nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_empty_array", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "array of descriptors") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_wallet_not_enabled", func(t *testing.T) {
		_, code, msg := execImportDescriptors("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(n)","timestamp":0}]`),
		})
		if code != -1 || !strings.Contains(msg, "not enabled") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_no_params", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_bad_descriptor", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"not_a_descriptor"`),
		})
		if code != -5 || !strings.Contains(msg, "unsupported descriptor") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_no_params", func(t *testing.T) {
		_, code, msg := execExtractDescriptor(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_bad_descriptor", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"not_a_descriptor"`),
		})
		if code != -5 || !strings.Contains(msg, "unsupported descriptor") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_no_journal", func(t *testing.T) {
		_, code, msg := execGetChainTxStats(nil, nil, nil, nil, "testnet", nil)
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execCreateAuxBlock(nil, nil, nil, nil, "testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_no_journal", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execCreateAuxBlock(nil, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
		})
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_empty_address", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getauxblock_no_params", func(t *testing.T) {
		_, code, msg := execGetAuxBlock(nil, nil, nil, nil, "testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "createauxblock") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSubmitAuxBlock(nil, nil, nil, "testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_unwired", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
		})
		if code != -1 || !strings.Contains(msg, "tip notifications") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_empty_requests", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wallet_unwired", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"address":"n"}}]`),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_no_hex", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_too_many_args", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"0100000000"`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodescript_no_params", func(t *testing.T) {
		_, code, msg := execDecodeScript("testnet", nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodescript_invalid_hex", func(t *testing.T) {
		_, code, msg := execDecodeScript("testnet", []json.RawMessage{json.RawMessage(`"zz"`)})
		if code != -8 || !strings.Contains(msg, "invalid hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_no_store", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockStats(j, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "block store") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_block_not_stored", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -5 || !strings.Contains(msg, "not stored") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_no_journal", func(t *testing.T) {
		_, code, msg := execGetDeploymentInfo(nil, nil, nil, "testnet", nil)
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_block_not_found", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
		})
		if code != -5 || msg != "Block not found" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_empty_filename", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid filename") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_not_implemented", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"wallet.dump"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importwalletdat_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execImportWalletDat("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importwalletdat_unwired", func(t *testing.T) {
		dir := t.TempDir()
		dump := filepath.Join(dir, "dump.txt")
		if err := os.WriteFile(dump, []byte("# wallet dump\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		paths := PathsWithDataDir(dir)
		_, code, msg := execImportWalletDat("testnet", paths, nil, nil, mustWalletJSONParam(t, dump))
		if code != -1 || !strings.Contains(msg, "dogego_importwalletdat") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importwalletdat_empty_filename", func(t *testing.T) {
		paths := &DataPaths{WalletImportSpendKey: func(string) error { return nil }}
		_, code, msg := execImportWalletDat("testnet", paths, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid filename") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("enumeratesigners_extra_args", func(t *testing.T) {
		_, code, msg := execEnumerateSigners(nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signerdisplayaddress_no_signer", func(t *testing.T) {
		_, code, msg := execSignerDisplayAddress(nil, []json.RawMessage{json.RawMessage(`"pkh(02abc)"`)})
		if code != -1 || !strings.Contains(msg, "signerdisplayaddress") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signerdisplayaddress_empty_desc", func(t *testing.T) {
		paths := &DataPaths{SignerCommand: []string{"echo"}}
		_, code, msg := execSignerDisplayAddress(paths, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "descriptor required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_probewalletdat_wrong_arg_count", func(t *testing.T) {
		_, code, _ := execProbeWalletDat("test", nil, nil)
		if code != -32602 {
			t.Fatalf("code=%d", code)
		}
	})
	t.Run("dogego_probewalletdat_empty_filename", func(t *testing.T) {
		_, code, msg := execProbeWalletDat("test", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid filename") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_invalid_wif", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"not-a-wif"`)})
		if code != -5 || !strings.Contains(msg, "Invalid private key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"c"`),
			json.RawMessage(`"lbl"`),
			json.RawMessage(`true`),
			json.RawMessage(`0`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getzmqnotifications_extra_args", func(t *testing.T) {
		_, code, msg := execGetZMQNotifications(nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_too_many_args", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{json.RawMessage(`"receive"`), json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{json.RawMessage(`"nAddr"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_invalid_address", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetLabel:       func(string, string) error { return nil },
		}
		_, code, msg := execSetLabelWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"not-an-address"`),
			json.RawMessage(`"mine"`),
		})
		if code != -5 || msg != "Invalid address" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_no_journal", func(t *testing.T) {
		_, code, msg := execLoadTxOutSet(nil, nil, nil, []json.RawMessage{json.RawMessage(`"snapshot.json"`)})
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_no_utxo", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execLoadTxOutSet(j, nil, nil, []json.RawMessage{json.RawMessage(`"snapshot.json"`)})
		if code != -1 || !strings.Contains(msg, "UTXO cache") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_extra_args", func(t *testing.T) {
		_, code, msg := execResendWalletTransactions(nil, []json.RawMessage{json.RawMessage(`1`)}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_too_many_args", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`10`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_bad_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitAuxBlock(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"not-a-hash"`),
			json.RawMessage(`"00"`),
		})
		if code != -8 || !strings.Contains(msg, "submitauxblock") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_unknown_template", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitAuxBlock(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`"00"`),
		})
		if code != -8 || !strings.Contains(msg, "unknown") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"n"`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_invalid_address", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"not-an-address"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_not_implemented", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_height_out_of_range", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRescanWallet(paths, j, nil, []json.RawMessage{json.RawMessage(`99`)})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_no_wallet", func(t *testing.T) {
		_, code, msg := execKeypoolRefillWallet(nil, nil)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_bad_newsize", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execKeypoolRefillWallet(paths, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "newsize") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("upgradetxindex_no_datadir", func(t *testing.T) {
		_, code, msg := execUpgradeTxIndex(nil, nil)
		if code != -1 || !strings.Contains(msg, "chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_bad_blockhash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"not-a-hash"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnetworkhashps_bad_nblocks", func(t *testing.T) {
		j := &memJournal{tip: 1, best: "b", gen: "g", count: 2, hdrs: [][]byte{make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`"x"`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnetworkhashps_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 1, best: "b", gen: "g", count: 2, hdrs: [][]byte{make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`120`),
			json.RawMessage(`1`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setnetworkactive_no_params", func(t *testing.T) {
		_, code, msg := execSetNetworkActive(nil, nil)
		if code != -8 || !strings.Contains(msg, "state required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setnetworkactive_p2p_disabled", func(t *testing.T) {
		_, code, msg := execSetNetworkActive(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmaxconnections_too_low", func(t *testing.T) {
		_, code, msg := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -8 || !strings.Contains(msg, "between") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmaxconnections_p2p_disabled", func(t *testing.T) {
		_, code, msg := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`16`)})
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_bad_txid", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRemovePrunedFunds(paths, []json.RawMessage{json.RawMessage(`"not-a-txid"`)})
		if code != -8 || !strings.Contains(msg, "txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_not_in_wallet", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRemovePrunedFunds(paths, []json.RawMessage{json.RawMessage(`"` + repeatHex('e') + `"`)})
		if code != -8 || !strings.Contains(msg, "does not exist") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_no_mining_address", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerate(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`1`)})
		if code != -1 || !strings.Contains(msg, "mining address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_invalid_address", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`"not-an-address"`),
			json.RawMessage(`"sig"`),
			json.RawMessage(`"hello"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabelWallet("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_bad_target_confirmations", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid parameter") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_no_journal", func(t *testing.T) {
		_, code, msg := execScanBlocks("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"status"`)})
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_no_filters", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execScanBlocks("testnet", j, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"status"`)})
		if code != -1 || !strings.Contains(msg, "filter index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_unknown_action", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, nil, nil, filters, nil, []json.RawMessage{json.RawMessage(`"nope"`)})
		if code != -8 || !strings.Contains(msg, "unknown action") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_start_no_scanobjects", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{json.RawMessage(`"start"`)})
		if code != -32602 || !strings.Contains(msg, "scanobjects") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_no_store", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockFilterHeader(j, nil, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "block store") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_unsupported_filtertype", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, txIx, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"mutated"`),
		})
		if code != -8 || !strings.Contains(msg, "basic filtertype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_too_many_args", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_too_many_args", func(t *testing.T) {
		_, code, msg := execListStuckTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execMoveWallet([]json.RawMessage{json.RawMessage(`""`), json.RawMessage(`""`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_invalid_amount", func(t *testing.T) {
		_, code, msg := execMoveWallet([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`0`),
		})
		if code != -3 || !strings.Contains(msg, "Invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_no_wallet", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{json.RawMessage(`"backup.dat"`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_empty_dest", func(t *testing.T) {
		paths := &DataPaths{WalletPath: func() string { return "wallet.json" }}
		_, code, msg := execBackupWallet(paths, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid destination") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_empty_filename", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid filename") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_bad_nrequired", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "nrequired") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_invalid_pubkey", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`["not-a-pubkey"]`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid public key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_too_many_args", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{json.RawMessage(`"getblock"`), json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_wallet_not_implemented", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execAbandonTransactionWallet("testnet", nil, nil, nil, pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
		})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_no_wallet", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnodeaddresses_p2p_disabled", func(t *testing.T) {
		_, code, msg := execGetNodeAddresses(nil, nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnodeaddresses_too_many_args", func(t *testing.T) {
		paths := &DataPaths{NodeAddresses: func(int, string) []map[string]interface{} { return nil }}
		_, code, msg := execGetNodeAddresses(paths, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"ipv4"`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnodeaddresses_bad_network", func(t *testing.T) {
		paths := &DataPaths{NodeAddresses: func(int, string) []map[string]interface{} { return nil }}
		_, code, msg := execGetNodeAddresses(paths, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"tor"`),
		})
		if code != -8 || !strings.Contains(msg, "unknown network") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listbanned_p2p_disabled", func(t *testing.T) {
		_, code, msg := execListBanned(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("clearbanned_p2p_disabled", func(t *testing.T) {
		_, code, msg := execClearBanned(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddednodeinfo_too_many_args", func(t *testing.T) {
		_, code, msg := execGetAddedNodeInfo(nil, []json.RawMessage{json.RawMessage(`"1"`), json.RawMessage(`"2"`)})
		if code != -8 || !strings.Contains(msg, "too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_too_many_args", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_invalid_account", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"*"`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid account") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_no_wallet", func(t *testing.T) {
		_, code, msg := execGetAccountAddressWallet(nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -1 || !strings.Contains(msg, "not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_invalid_address", func(t *testing.T) {
		_, code, msg := execSetAccountWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"not-an-address"`),
			json.RawMessage(`""`),
		})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_untracked_address", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nOther" }}
		_, code, msg := execSetAccountWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`""`),
		})
		if code != -1 || !strings.Contains(msg, "own address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_invalid_address", func(t *testing.T) {
		_, code, msg := execGetAccountWallet(nil, "testnet", []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_invalid_address", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"not-an-address"`),
		})
		if code != -5 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_empty_address", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindexblockfilters_no_header_journal", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execReindexBlockFilters(paths, j, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutsetinfo_no_journal", func(t *testing.T) {
		_, code, msg := execGetTxOutSetInfo(nil, nil, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "chain state") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmininginfo_header_read_failed", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: nil}
		_, code, msg := execGetMiningInfo(j, nil, nil, nil, "testnet", nil, 0)
		if code != -1 || msg == "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importmnemonic_missing_mnemonic", func(t *testing.T) {
		paths := &DataPaths{WalletImportMnemonic: func(string, string) error { return nil }}
		_, code, msg := execDogegoImportMnemonic("testnet", paths, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "mnemonic required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_not_array", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`"not-an-array"`),
		})
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_unsupported_addr_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletNewAddress:     func() (string, error) { return "nAddr", nil },
		}
		_, code, msg := execGetNewAddress("testnet", paths, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"bech32"`),
		})
		if code != -8 || !strings.Contains(msg, "not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_invalid_address", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_wallet_extra_args", func(t *testing.T) {
		_, code, msg := execResendWalletTransactionsWallet("testnet", nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`1`)}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`)})
		if code != -8 || !strings.Contains(msg, "command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_unknown_command", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{
			json.RawMessage(`"127.0.0.1:22556"`),
			json.RawMessage(`"nope"`),
		})
		if code != -8 || !strings.Contains(msg, "unknown command") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_invalid_subnet", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"not-a-subnet"`),
			json.RawMessage(`"add"`),
		})
		if code != CodeRPCInvalidIPOrSubnet || !strings.Contains(msg, "Invalid IP") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_unknown_command", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"1.2.3.4"`),
			json.RawMessage(`"nope"`),
		})
		if code != -8 || !strings.Contains(msg, "unknown setban command") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddednodeinfo_node_not_added", func(t *testing.T) {
		_, code, msg := execGetAddedNodeInfo(&DataPaths{AddedNodes: func() []string { return []string{"127.0.0.1:22556"} }}, []json.RawMessage{
			json.RawMessage(`"9.9.9.9:22556"`),
		})
		if code != CodeRPCNodeNotAdded || msg != ErrNodeNotAdded {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_invalid_script", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"not-valid"`),
		})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_outputs", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`"not-an-object"`),
		})
		if code != -8 || !strings.Contains(msg, "outputs must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_too_many_args", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "createrawtransaction") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_wrong_arg_count", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
		}, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_not_in_mempool", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
		}, nil, chain.RebootTestnet)
		if code != -5 || !strings.Contains(msg, "Invalid or non-wallet") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_invalid_address", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_invalid_amount", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`0`),
		}, nil, false, chain.RebootTestnet)
		if code != -3 || !strings.Contains(msg, "Invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_wrong_arg_count", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"0100000000"`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumptxoutset_no_utxo", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execDumpTxOutSet(j, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "UTXO cache") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_legacy_no_journal", func(t *testing.T) {
		_, code, msg := execGetBlockTemplateLegacy([]json.RawMessage{json.RawMessage(`{}`)})
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_bad_nblocks", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnetworkhashps_bad_height", func(t *testing.T) {
		j := &memJournal{tip: 1, best: "b", gen: "g", count: 2, hdrs: [][]byte{make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execGetNetworkHashPS(j, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`120`),
			json.RawMessage(`"x"`),
		})
		if code != -8 || !strings.Contains(msg, "height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaddressgroupings_extra_args", func(t *testing.T) {
		_, code, msg := execListAddressGroupingsWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccountWallet("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_invalid_txid", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad-txid"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_no_wallet_key", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"hello"`),
		})
		if code != -4 || !strings.Contains(msg, "Private key not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_malformed_signature", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"!!!"`),
			json.RawMessage(`"hello"`),
		})
		if code != -8 || !strings.Contains(msg, "malformed base64") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_bad_minconf", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"deadbeef"`),
			json.RawMessage(`0`),
			json.RawMessage(`1000`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_not_found", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('2') + `"`)})
		if code != -5 || msg != "Transaction not in mempool" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_too_many_args", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('3') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -8 || !strings.Contains(msg, "verbose") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_null_transactions", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`null`),
		})
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("validateaddress_bad_redeem_script", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execValidateAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"zzzz"`),
		})
		if code != -8 || !strings.Contains(msg, "redeemScript") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_start_no_journal", func(t *testing.T) {
		_, code, msg := execScanTxOutSet("testnet", nil, nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"start"`)})
		if code != -1 || !strings.Contains(msg, "header journal") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_start_no_scanobjects", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"start"`)})
		if code != -32602 || !strings.Contains(msg, "scanobjects") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_not_array", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"not-an-array"`)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_too_many_args", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_empty_passphrase", func(t *testing.T) {
		_, code, msg := execEncryptWalletPaths(nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_empty_passphrase", func(t *testing.T) {
		_, code, msg := execWalletPassphrasePaths(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_address_not_in_wallet", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetLabel:       func(string, string) error { return nil },
		}
		_, code, msg := execSetLabelWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"mine"`),
		})
		if code != -4 || !strings.Contains(msg, "not found in wallet") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_maxconf_out_of_range", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execListUnspent("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "maxconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_minconf_out_of_range", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetBalance(paths, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_unsupported_filtertype", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, txIx, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"mutated"`),
		})
		if code != -8 || !strings.Contains(msg, "basic filtertype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_bad_stats_array", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "array of strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindexblockfilters_no_tx_index", func(t *testing.T) {
		dir := t.TempDir()
		j, err := store.OpenHeaderJournal(dir+"/headers.bin", make([]byte, 80))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{ChainDataDir: dir}
		_, code, msg := execReindexBlockFilters(paths, j, raw, nil, nil)
		if code != -1 || !strings.Contains(msg, "tx index required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_bad_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, []json.RawMessage{json.RawMessage(`"bad-txid"`)})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("disconnectnode_not_connected", func(t *testing.T) {
		paths := &DataPaths{
			DisconnectNode: func(string) error { return fmt.Errorf("Node not found in connected nodes") },
		}
		_, code, msg := execDisconnectNode(paths, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`)})
		if code != CodeRPCNodeNotConnected || msg != ErrNodeNotConnected {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmaxconnections_too_high", func(t *testing.T) {
		_, code, msg := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`33`)})
		if code != -8 || !strings.Contains(msg, "between") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_bad_psbt", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{json.RawMessage(`"!!!"`)})
		if code != -8 || !strings.Contains(msg, "invalid base64") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_bad_outputs", func(t *testing.T) {
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`"not-an-object"`),
		})
		if code != -8 || !strings.Contains(msg, "outputs must be") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_too_many_args", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`30`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_bad_maxtries", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "maxtries") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_too_many_args", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('4') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -8 || !strings.Contains(msg, "verbose") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_bad_verbose", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('5') + `"`),
			json.RawMessage(`"yes"`),
		})
		if code != -8 || !strings.Contains(msg, "verbose") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_wrong_arg_count", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('6') + `"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_fee_delta", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('7') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`"not-an-int"`),
		})
		if code != -8 || !strings.Contains(msg, "fee_delta") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawmempool_bad_verbose", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetRawMempool(p, nil, nil, []json.RawMessage{json.RawMessage(`"yes"`)})
		if code != -8 || !strings.Contains(msg, "verbose") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_empty_passphrase", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletLockPaths(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_height_out_of_range", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRescanWallet(paths, j, nil, []json.RawMessage{json.RawMessage(`9`)})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_negative_height", func(t *testing.T) {
		_, code, msg := execRescan([]json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatefee_invalid_nblocks", func(t *testing.T) {
		_, code, msg := execEstimateFee(nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -8 || !strings.Contains(msg, "invalid nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_bad_minconf", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_bad_minconf", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbts_not_array", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{json.RawMessage(`"not-an-array"`)})
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_bad_hex", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{json.RawMessage(`"zz"`)})
		if code != -8 || !strings.Contains(msg, "invalid hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("utxoupdatepsbt_bad_psbt", func(t *testing.T) {
		_, code, msg := execUtxoUpdatePsbt(nil, nil, nil, []json.RawMessage{json.RawMessage(`"!!!"`)})
		if code != -8 || !strings.Contains(msg, "invalid base64") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_too_many_args", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`30`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_bad_txid", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad-txid"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_not_array", func(t *testing.T) {
		_, code, msg := execImportDescriptors("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"not-an-array"`)})
		if code != -8 || !strings.Contains(msg, "array of descriptors") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifytxoutproof_bad_hex_param", func(t *testing.T) {
		_, code, msg := execVerifyTxOutProof(nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockheader_invalid_height", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, _, err := resolveGetBlockHeader(j, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if err == nil || !strings.Contains(err.Error(), "invalid height") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("getblockheader_bad_param", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, _, err := resolveGetBlockHeader(j, nil, []json.RawMessage{json.RawMessage(`"not-a-hash"`)})
		if err == nil || !strings.Contains(err.Error(), "block hash hex") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("getblockheader_height_out_of_range", func(t *testing.T) {
		j := &memJournal{tip: 1, best: "b", gen: "g", count: 2, hdrs: [][]byte{make([]byte, 80), make([]byte, 80)}}
		_, _, err := resolveGetBlockHeader(j, nil, []json.RawMessage{json.RawMessage(`9`)})
		if err == nil || !strings.Contains(err.Error(), "height out of range") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("getblockheader_unsupported_param", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, _, err := resolveGetBlockHeader(j, nil, []json.RawMessage{json.RawMessage(`true`)})
		if err == nil || !strings.Contains(err.Error(), "unsupported param type") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("importaddress_height_out_of_range", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execImportAddress("testnet", paths, j, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`999`),
		})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_rescan_flag", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_label_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_options_not_object", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[{}]`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_options_bad_rescan", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[{}]`),
			json.RawMessage(`{"rescan":"nope"}`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolinfo_no_pool", func(t *testing.T) {
		_, code, msg := execGetMempoolInfo(nil, nil, nil, 0, 0, 0, 0, false, 0, nil)
		if code != -18 || !strings.Contains(msg, "mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_bad_psbt", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{json.RawMessage(`"!!!"`)})
		if code != -8 || !strings.Contains(msg, "invalid base64") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("analyzepsbt_bad_psbt", func(t *testing.T) {
		_, code, msg := execAnalyzePsbt([]json.RawMessage{json.RawMessage(`"!!!"`)})
		if code != -8 || !strings.Contains(msg, "invalid base64") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execWalletProcessPsbt("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"!!!"`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_bad_sign_flag", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execWalletProcessPsbt("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"!!!"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "sign must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimaterawfee_bad_estimate_mode", func(t *testing.T) {
		_, code, msg := execEstimateRawFee(nil, []json.RawMessage{
			json.RawMessage(`6`),
			json.RawMessage(`"fast"`),
		})
		if code != -8 || !strings.Contains(msg, "estimate_mode") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_bad_path_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		_, code, msg := execLoadTxOutSet(j, nil, paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "path must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_bad_blockhash_param", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"deadbeef"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_include_mempool_bad_flag", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('8') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`"yes"`),
		})
		if code != -8 || !strings.Contains(msg, "include_mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		p := mempool.New(10)
		_, code, msg := execPsbtBumpFee("testnet", paths, p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('9') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintips_no_headers", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1}
		_, err := buildGetChainTips(j, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "no headers") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("sendmany_invalid_amount", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{"` + addr + `": -1}`),
		}, nil, false, chain.RebootTestnet)
		if code != -3 || !strings.Contains(msg, "Invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_invalid_address", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{"not-an-address": 1.0}`),
		}, nil, false, chain.RebootTestnet)
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_options_not_object", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_invalid_change_address", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"changeAddress":"not-an-address"}`),
		})
		if code != -8 || !strings.Contains(msg, "changeAddress") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_change_position_invalid", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"changePosition":-2}`),
		})
		if code != -8 || !strings.Contains(msg, "changePosition") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_subtract_fee_bad_index", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"subtractFeeFromOutputs":["bad"]}`),
		})
		if code != -8 || !strings.Contains(msg, "subtractFeeFromOutputs") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_inputs_not_array", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"not-an-array"`),
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "inputs must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_locktime", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "locktime") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_not_array", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execTestMempoolAccept(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)}, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_verbosity_not_number", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "test", nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"two"`),
		})
		if code != -8 || !strings.Contains(msg, "verbosity") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_options_not_object", func(t *testing.T) {
		pool := mempool.New(100)
		parentHash := [32]byte{7}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidToRPC(old.TxHash())
		_ = pool.Add(oldRaw)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + oldID + `"`),
			json.RawMessage(`"not-object"`),
		}, nil, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_bad_txid", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"deadbeef"`),
		}, nil, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_bad_hash", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`"deadbeef"`)})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_bad_hash", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`"deadbeef"`)})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnettotals_p2p_disabled", func(t *testing.T) {
		_, code, msg := execGetNetTotals(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_bad_rescan_flag", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x01
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_timeout_out_of_range", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "timeout out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_not_array", func(t *testing.T) {
		_, code, msg := execCombineRawTransaction([]json.RawMessage{json.RawMessage(`"not-an-array"`)})
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_txids_not_array", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{json.RawMessage(`"not-array"`)})
		if code != -8 || !strings.Contains(msg, "JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_invalid_hex", func(t *testing.T) {
		p := mempool.New(10)
		pkg, _ := json.Marshal([]string{"zz"})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(pkg)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid hex at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_decode_failed", func(t *testing.T) {
		p := mempool.New(10)
		pkg, _ := json.Marshal([]string{"0100000000"})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(pkg)}, nil, false, chain.RebootTestnet)
		if code != -22 || !strings.Contains(msg, "TX decode failed at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_not_topologically_sorted", func(t *testing.T) {
		p := mempool.New(10)
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		child := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		childRaw, _ := child.Serialize()
		parentRaw, _ := parent.Serialize()
		pkg, _ := json.Marshal([]string{hex.EncodeToString(childRaw), hex.EncodeToString(parentRaw)})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(pkg)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "not topologically sorted") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_bad_maxfeerate", func(t *testing.T) {
		p := mempool.New(10)
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		pkg, _ := json.Marshal([]string{hex.EncodeToString(parentRaw)})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(pkg),
			json.RawMessage(`"bad"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid maxfeerate") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_bad_maxburnamount", func(t *testing.T) {
		p := mempool.New(10)
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		pkg, _ := json.Marshal([]string{hex.EncodeToString(parentRaw)})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(pkg),
			json.RawMessage(`null`),
			json.RawMessage(`-1`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid maxburnamount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_bad_hash_length", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"deadbeef"`)})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetTransaction(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_bad_label_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetAddressesByLabelWallet("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_height_out_of_range", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x01
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`"-1"`),
		})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_block_height_out_of_range", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x02
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execImportPrivKey("testnet", nil, j, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`999`),
		})
		if code != -8 || !strings.Contains(msg, "Block height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importmnemonic_bad_rescan", func(t *testing.T) {
		paths := &DataPaths{WalletImportMnemonic: func(string, string) error { return nil }}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execDogegoImportMnemonic("testnet", paths, j, nil, []json.RawMessage{
			json.RawMessage(`"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_bad_rescan", func(t *testing.T) {
		paths := &DataPaths{WalletImportBIP38: func(string, string) (string, error) { return "DAddr", nil }}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execDogegoImportBIP38("testnet", paths, j, nil, []json.RawMessage{
			json.RawMessage(`"6P..."`),
			json.RawMessage(`"pass"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_bad_blockhash", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		txid := repeatHex('a')
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{
			json.RawMessage(`["` + txid + `"]`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad blockhash") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_blockhash_not_64", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		txid := repeatHex('b')
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{
			json.RawMessage(`["` + txid + `"]`),
			json.RawMessage(`"deadbeef"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_duplicated_txid", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		txid := repeatHex('c')
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{
			json.RawMessage(`["` + txid + `","` + txid + `"]`),
		})
		if code != -8 || !strings.Contains(msg, "duplicated txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_missing_privkeys", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x33
		payTo, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		prevTxid := repeatHex('d')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		outObj := map[string]interface{}{payTo: 0.1}
		outJSON, _ := json.Marshal(outObj)
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, outJSON})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		pubC := mustPubCompressed(t, sec)
		h160 := pubkeyHash160(pubC)
		pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
		pkScript = append(pkScript, 0x88, 0xac)
		prevEntry := map[string]interface{}{
			"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
		_, code, msg = execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
		})
		if code != -8 || !strings.Contains(msg, "privkeys array required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_privkeys_not_array", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x44
		payTo, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		prevTxid := repeatHex('e')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		outObj := map[string]interface{}{payTo: 0.1}
		outJSON, _ := json.Marshal(outObj)
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, outJSON})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		pubC := mustPubCompressed(t, sec)
		h160 := pubkeyHash160(pubC)
		pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
		pkScript = append(pkScript, 0x88, 0xac)
		prevEntry := map[string]interface{}{
			"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
		_, code, msg = execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "privkeys must be an array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_bad_filtertype", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"segwit"`),
		})
		if code != -8 || !strings.Contains(msg, "only basic filtertype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_filtertype_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "filtertype must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_witness_not_supported", func(t *testing.T) {
		p := mempool.New(10)
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{},
				PrevIdx:  0xffffffff,
				Sequence: 0xffffffff,
				Witness:  [][]byte{{0x01}},
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		pkg, _ := json.Marshal([]string{hex.EncodeToString(raw)})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(pkg)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "witness transactions are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_dependent_parents", func(t *testing.T) {
		p := mempool.New(10)
		p1 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		p2 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: p1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		child := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: p2.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		p1raw, _ := p1.Serialize()
		p2raw, _ := p2.Serialize()
		childRaw, _ := child.Serialize()
		pkg, _ := json.Marshal([]string{
			hex.EncodeToString(p1raw),
			hex.EncodeToString(p2raw),
			hex.EncodeToString(childRaw),
		})
		_, code, msg := execSubmitPackage(p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(pkg)}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "dependent parents") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_bad_verbose_flag", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetRawTransaction(ix, raw, j, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "bad verbose flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_bad_txid_type", func(t *testing.T) {
		_, code, msg := execGetRawTransaction(nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_bad_txid_hex", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		bad := repeatHex('g')
		_, code, msg := execGetRawTransaction(ix, raw, j, mempool.New(10), nil, []json.RawMessage{json.RawMessage(`"` + bad + `"`)})
		if code != -5 || !strings.Contains(msg, "No such mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifytxoutproof_block_not_in_chain", func(t *testing.T) {
		txb := minimalCoinbaseTxBytes(t)
		cbTx, err := wire.DeserializeTx(txb)
		if err != nil {
			t.Fatal(err)
		}
		mr0 := wire.BlockMerkleRoot([]*wire.Tx{cbTx})
		hdr0 := primitives.BlockHeader{
			Version: 1, MerkleRoot: mr0, Timestamp: 1700000000, Bits: 0x1e0ffff0, Nonce: 42,
		}
		h0 := hdr0.EncodeWire80()
		id0 := pow.BlockHashLE(h0[:])
		var block0 bytes.Buffer
		_, _ = block0.Write(h0[:])
		_ = wire.WriteCompactSize(&block0, 1)
		_, _ = block0.Write(txb)
		dir := t.TempDir()
		rs, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(id0, block0.Bytes()); err != nil {
			t.Fatal(err)
		}
		if err := ix.IndexBlock(id0, block0.Bytes()); err != nil {
			t.Fatal(err)
		}
		best := pow.BlockHashHex(h0[:])
		j := &memJournal{tip: 0, best: best, gen: best, count: 1, hdrs: [][]byte{append([]byte(nil), h0[:]...)}}
		spendID := txidToRPC(cbTx.TxHash())
		txidsJSON, _ := json.Marshal([]string{spendID})
		proof, code, msg := execGetTxOutProof(ix, rs, j, []json.RawMessage{json.RawMessage(txidsJSON)})
		if code != 0 || proof == "" {
			t.Fatalf("gettxoutproof: code=%d msg=%q proof=%q", code, msg, proof)
		}
		jWrong := &memJournal{tip: 0, best: repeatHex('a'), gen: repeatHex('b'), count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg = execVerifyTxOutProof(jWrong, []json.RawMessage{json.RawMessage(`"` + proof.(string) + `"`)})
		if code != -5 || !strings.Contains(msg, "block not found in chain") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_stub_request_not_object", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{json.RawMessage(`[123]`)})
		if code != -8 || !strings.Contains(msg, "each request must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_missing_scriptpubkey", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "missing scriptPubKey or desc") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_not_object", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[123]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "each request must be a JSON object") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_bad_redeemscript", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"hex":"76a9140102030405060708090a0b0c0d0e0f10111213141516171888ac"},"redeemscript":"zz"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "redeemscript must be a hex string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_pubkeys_empty", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"pubkeys":[]}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "pubkeys must be a non-empty array") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("walletpassphrase_change_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{json.RawMessage(`"old"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_change_empty_passphrase", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_bad_rescan_flag", func(t *testing.T) {
		sec := make([]byte, 32)
		sec[0] = 0x66
		pubC := mustPubCompressed(t, sec)
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(pubC) + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_prevtxs_not_array", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "prevtxs must be an array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getconnectioncount_p2p_disabled", func(t *testing.T) {
		_, code, msg := execGetConnectionCount(nil)
		if code != CodeRPCP2PDisabled || msg != ErrP2PDisabled {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_invalid_hex", func(t *testing.T) {
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{json.RawMessage(`"zz"`)})
		if code != -8 || !strings.Contains(msg, "invalid transaction hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_witness_not_supported", func(t *testing.T) {
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{},
				PrevIdx:  0xffffffff,
				Sequence: 0xffffffff,
				Witness:  [][]byte{{0x01}},
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(raw) + `"`),
		})
		if code != -8 || !strings.Contains(msg, "witness transactions are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_bad_sighashtype", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x77
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		payTo, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		prevTxid := repeatHex('f')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		outObj := map[string]interface{}{payTo: 0.1}
		outJSON, _ := json.Marshal(outObj)
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, outJSON})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		pubC := mustPubCompressed(t, sec)
		h160 := pubkeyHash160(pubC)
		pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
		pkScript = append(pkScript, 0x88, 0xac)
		prevEntry := map[string]interface{}{
			"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg = execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
			privArr,
			json.RawMessage(`"BOGUS"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid sighashtype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_bad_descriptor", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"not-a-desc()"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "unsupported descriptor") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_scriptpubkey_bad_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":123}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "scriptPubKey must be a string or object") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("getrawchangeaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAccountAddress(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetAccountAddress([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_height_out_of_range", func(t *testing.T) {
		sec := make([]byte, 32)
		sec[0] = 0x88
		pubC := mustPubCompressed(t, sec)
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execImportPubKey("testnet", nil, j, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(pubC) + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`999`),
		})
		if code != -8 || !strings.Contains(msg, "Block height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_bad_label_type", func(t *testing.T) {
		sec := make([]byte, 32)
		sec[0] = 0x99
		pubC := mustPubCompressed(t, sec)
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(pubC) + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_bad_minconf", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{
			json.RawMessage(`"acct"`),
			json.RawMessage(`"-1"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_addresses_not_array", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-an-array"`),
		})
		if code != -8 || !strings.Contains(msg, "addresses must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_duplicate_address", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		addrs, _ := json.Marshal([]string{addr, addr})
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			addrs,
		})
		if code != -8 || !strings.Contains(msg, "duplicated address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccount(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_stub_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccount([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_invalid_star_account", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccount([]json.RawMessage{json.RawMessage(`"*"`)})
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_unlock_not_bool", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{json.RawMessage(`"notbool"`)})
		if code != -8 || !strings.Contains(msg, "unlock") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_bad_include_mempool", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_mempool") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_bad_privkey", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{
			json.RawMessage(`"hello"`),
			json.RawMessage(`"not-a-wif"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid private key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_bad_message_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x11
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"` + wif + `"`),
		})
		if code != -8 || !strings.Contains(msg, "bad message") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_hexstring_not_string", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "hexstring must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_bad_account_type", func(t *testing.T) {
		k1, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		k2, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
		p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(fmt.Sprintf(`["%s","%s"]`, p1, p2)),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_keys_bad_element", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
			WalletImportPrivKey:  func(string) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"keys":[123]}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "keys must be an array of strings") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("getreceivedbylabel_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_stub_bad_label_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_stub_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_stub_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByLabel([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_stub_bad_minconf", func(t *testing.T) {
		_, code, msg := execListReceivedByLabel([]json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByAccount([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_bad_height_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRescanWallet(paths, j, nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{json.RawMessage(`2`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_wrong_arg_count", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_txid_not_hex", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "txid must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_bad_txid_type", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_bad_timeout_type", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_include_unsafe_not_bool", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_unsafe") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_not_object", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "query_options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_unknown_key", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"bogus":1}`),
		})
		if code != -8 || !strings.Contains(msg, "unknown query_options key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_invalid_maximum_count", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"maximumCount":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid maximumCount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_bad_address_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_bad_include_watchonly", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_bad_address_type", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_bad_account_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSetAccount("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_bad_address_type", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_bad_fromaccount_type", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"to"`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "fromaccount must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"to"`),
			json.RawMessage(`1`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_bad_comment_type", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"to"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "comment must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_bad_minconf", func(t *testing.T) {
		_, code, msg := execListAccounts([]json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execRemovePrunedFunds(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_pubkeys_required_out_of_range", func(t *testing.T) {
		k1, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		k2, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
		p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`[{"pubkeys":["%s","%s"],"required":99}]`, p1, p2)),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "required out of range") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("listreceivedbyaddress_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByAddress([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_bad_toaccount_type", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`123`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "toaccount must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_invalid_star_fromaccount", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`"*"`),
			json.RawMessage(`"to"`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListAccounts([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_stub_bad_minconf", func(t *testing.T) {
		_, code, msg := execListReceivedByAddress([]json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_stub_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByAddress([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_stub_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByAddress([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_bad_minconf_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_too_many_args", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_invalid_star_account", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSetAccount("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"*"`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_invalid_minimum_amount", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"minimumAmount":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid minimumAmount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_fromaccount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "fromaccount must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_minconf_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`"bad"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_comment_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "comment must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_bad_height_type", func(t *testing.T) {
		_, code, msg := execRescan([]json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execRescan([]json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListLockUnspent([]json.RawMessage{json.RawMessage(`null`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_bad_vout", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[{"txid":"` + repeatHex('a') + `","vout":-1}]`),
		})
		if code != -8 || !strings.Contains(msg, "vout must be positive") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_transaction_not_object", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "expected object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_keys_not_array", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "bad keys array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_stub_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabel(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_invalid_pubkey", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"pubkeys":["not-a-pubkey"],"required":1}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "Invalid public key") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("listreceivedbyaccount_stub_bad_minconf", func(t *testing.T) {
		_, code, msg := execListReceivedByAccount([]json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_comment_to", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "comment_to must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_fund_options", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "fund options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_bad_fromaccount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			amounts,
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "fromaccount must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_amounts_not_object", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`"not-object"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "amounts must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_bad_minconf_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`"bad"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_bad_comment_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "comment must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_subtractfeefrom_not_array", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-array"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "subtractfeefrom must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_bad_fund_options", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "fund options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_invalid_address", func(t *testing.T) {
		addrs, _ := json.Marshal([]string{"not-an-address"})
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			addrs,
		})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_stub_too_many_args", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_invalid_address", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{json.RawMessage(`"not-an-address"`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_bad_address_type", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execKeypoolRefill([]json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_bad_enc_key_type", func(t *testing.T) {
		paths := &DataPaths{WalletImportBIP38: func(enc, pass string) (string, error) { return "", nil }}
		_, code, msg := execDogegoImportBIP38("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"pass"`),
		})
		if code != -8 || !strings.Contains(msg, "encrypted key must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execImportDescriptors("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_options_not_object", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		_, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(n)","timestamp":0}]`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_bad_descriptor_row", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"not-valid()"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "unsupported descriptor") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("encryptwallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execEncryptWallet(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_bad_passphrase_type", func(t *testing.T) {
		_, code, msg := execEncryptWallet([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_comment_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`123`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "comment must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_comment_to_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "comment_to must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_subtractfeefrom_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "subtractfeefromamount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_fund_options", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_too_many_args", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_bad_label_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletNewAddress:     func() (string, error) { return "nAddr", nil },
		}
		_, code, msg := execGetNewAddress("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletNewAddress:     func() (string, error) { return "nAddr", nil },
		}
		_, code, msg := execGetNewAddress("testnet", paths, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"legacy"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_bad_addr_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletNewAddress:     func() (string, error) { return "nAddr", nil },
		}
		_, code, msg := execGetNewAddress("testnet", paths, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "address_type must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_bad_filename_type", func(t *testing.T) {
		paths := &DataPaths{WalletPath: func() string { return "wallet.dat" }}
		_, code, msg := execDumpWallet("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "filename must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"a.dat"`),
			json.RawMessage(`"b.dat"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_stub_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByLabel([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_stub_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByLabel([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_stub_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByAccount([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_null_amounts", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			json.RawMessage(`null`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "amounts must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_duplicate_address", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts := json.RawMessage(`{"` + addr + `": 1.0, " ` + addr + ` ": 2.0}`)
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "duplicated address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_subtractfeefrom_not_in_outputs", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr1, _ := chain.RandomP2PKHAddress(p)
		addr2, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr1: 1.0})
		sub, _ := json.Marshal([]string{addr2})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			sub,
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "address not in outputs") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_bad_rescan", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		_, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(n)","timestamp":0}]`),
			json.RawMessage(`{"rescan":"notbool"}`),
		})
		if code != -8 || !strings.Contains(msg, "rescan") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_bad_passphrase_type", func(t *testing.T) {
		paths := &DataPaths{WalletImportBIP38: func(enc, pass string) (string, error) { return "", nil }}
		_, code, msg := execDogegoImportBIP38("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`"6PTestKey"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importmnemonic_bad_mnemonic_type", func(t *testing.T) {
		paths := &DataPaths{WalletImportMnemonic: func(string, string) error { return nil }}
		_, code, msg := execDogegoImportMnemonic("testnet", paths, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "mnemonic must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importmnemonic_bad_passphrase_type", func(t *testing.T) {
		paths := &DataPaths{WalletImportMnemonic: func(string, string) error { return nil }}
		_, code, msg := execDogegoImportMnemonic("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`"abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_bad_passphrase_type", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_bad_old_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "oldpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_bad_new_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "newpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_bad_dest_type", func(t *testing.T) {
		paths := &DataPaths{WalletPath: func() string { return "wallet.dat" }}
		_, code, msg := execBackupWallet(paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "destination must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_bad_filename_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletImportSpendKey: func(string) error { return nil },
			WalletImportWatch:    func([]byte) error { return nil },
			WalletAddress:        func() string { return "nAddr" },
		}
		_, code, msg := execImportWallet("testnet", paths, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "filename must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_subtractfeefrom_invalid_address", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		sub, _ := json.Marshal([]string{"not-an-address"})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"from"`),
			amounts,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			sub,
		}, nil, false, chain.RebootTestnet)
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_address_type", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`1`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_bad_address_type", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_bad_redeem_script_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "redeemScript must be hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_stub_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByAccount([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_bad_label_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_wallet_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_wallet_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_bad_maxconf_type", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "maxconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_bad_address_element_type", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "expected string address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_bad_address_type", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"sig"`),
			json.RawMessage(`"msg"`),
		})
		if code != -8 || !strings.Contains(msg, "bad address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_bad_signature_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
			json.RawMessage(`"msg"`),
		})
		if code != -8 || !strings.Contains(msg, "bad signature") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_bad_message_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"sig"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad message") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{json.RawMessage(`"hello"`)})
		if code != -8 || !strings.Contains(msg, "message and private key required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_bad_privkey_type", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{
			json.RawMessage(`"hello"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad private key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_replaceable_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"replaceable":"notbool"}`),
		})
		if code != -8 || !strings.Contains(msg, "replaceable") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_conf_target_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"conf_target":"bad"}`),
		})
		if code != -8 || !strings.Contains(msg, "conf_target must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_conf_target_out_of_range", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"conf_target":0}`),
		})
		if code != -8 || !strings.Contains(msg, "conf_target out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_estimate_mode_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"conf_target":6,"estimate_mode":"nope"}`),
		})
		if code != -8 || !strings.Contains(msg, "estimate_mode") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_fee_rate_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"fee_rate":true}`),
		})
		if code != -8 || !strings.Contains(msg, "feeRate must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_change_address_not_string", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"changeAddress":123}`),
		})
		if code != -8 || !strings.Contains(msg, "changeAddress must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_subtract_fee_outputs_not_array", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"subtractFeeFromOutputs":"bad"}`),
		})
		if code != -8 || !strings.Contains(msg, "subtractFeeFromOutputs must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_bad_include_unsafe", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_unsafe") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_bad_address_type", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_bad_address_type", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"hello"`),
		})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_bad_message_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "message must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_bad_label_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x02
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodescript_bad_hex_param_type", func(t *testing.T) {
		_, code, msg := execDecodeScript("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex param") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_minimum_total_fee_invalid", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"minimumTotalFee":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid minimumTotalFee") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_lock_unspents_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"lockUnspents":"notbool"}`),
		})
		if code != -8 || !strings.Contains(msg, "lockUnspents") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_include_watching_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"includeWatching":"notbool"}`),
		})
		if code != -8 || !strings.Contains(msg, "includeWatching") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_add_inputs_bad", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"add_inputs":"notbool"}`),
		})
		if code != -8 || !strings.Contains(msg, "add_inputs") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_fee_rate_negative", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"fee_rate":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid feeRate") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_bad_privkey_type", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "privkey must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_bad_height_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x03
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_too_many_args", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		sec := make([]byte, 32)
		sec[0] = 0x04
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_bad_include_watchonly", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_bad_target_confirmations_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "target_confirmations must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_bad_account_type", func(t *testing.T) {
		code, msg := execListTransactionsValidate([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_bad_count_type", func(t *testing.T) {
		code, msg := execListTransactionsValidate([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "count must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_bad_skip_type", func(t *testing.T) {
		code, msg := execListTransactionsValidate([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "skip must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_bad_include_watchonly", func(t *testing.T) {
		code, msg := execListTransactionsValidate([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_bad_txid_type", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "txid must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_minconf_out_of_range", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execListUnspent("testnet", paths, nil, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_bad_txid_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execRemovePrunedFunds(paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "txid must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_bad_verbose", func(t *testing.T) {
		_, _, code, msg := execListStuckTransactionsValidate([]json.RawMessage{json.RawMessage(`"notbool"`)})
		if code != -8 || !strings.Contains(msg, "verbose") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_bad_include_watchonly", func(t *testing.T) {
		_, _, code, msg := execListStuckTransactionsValidate([]json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_wallet_bad_newsize_type", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execKeypoolRefillWallet(paths, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "newsize must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_stub_bad_txid_type", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "txid must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_stub_too_many_args", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_minimum_total_fee_bad_type", func(t *testing.T) {
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		coinbaseHex := "01000000010000000000000000000000000000000000000000000000000000000000000000ffffffff0100000000000000000051ffffffff00000000"
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + coinbaseHex + `"`),
			json.RawMessage(`{"minimumTotalFee":true}`),
		})
		if code != -8 || !strings.Contains(msg, "minimumTotalFee") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_script_type", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "script or address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_p2sh_type", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"addr"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "p2sh") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_height_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_too_many_args", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_bad_pubkey_type", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "pubkey must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_bad_height_type", func(t *testing.T) {
		sec := make([]byte, 32)
		sec[0] = 0x05
		pubC := mustPubCompressed(t, sec)
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(pubC) + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_too_many_args", func(t *testing.T) {
		sec := make([]byte, 32)
		sec[0] = 0x06
		pubC := mustPubCompressed(t, sec)
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(pubC) + `"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_too_many_args", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many arguments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_bad_hex_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"label"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_transactions_not_array", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "transactions must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_stub_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{
			json.RawMessage(`"acct"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_bad_label_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{WalletAddress: func() string { return addr }}
		_, code, msg := execSetLabelWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_bad_address_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execSetLabelWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"label"`),
		})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_stub_too_many_args", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":"00"}]`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wallet_too_many_args", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		_, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":"00"}]`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_null_key", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"minimumAmount":null}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid minimumAmount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_stub_too_many_args", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{
			json.RawMessage(`"acct"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execMoveWallet([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_wallet_bad_amount_type", func(t *testing.T) {
		_, code, msg := execMoveWallet([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_transactions_not_array", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "transactions must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_negative_height", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execRescanWallet(paths, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_bad_passphrase_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_bad_timeout_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_builtin_bad_passphrase_type", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_keys_not_array", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
			WalletImportPrivKey:  func(string) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"keys":[{}]}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "keys must be an array of strings") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_required_bad_type", func(t *testing.T) {
		k1, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		k2, err := secp256k1.NewPrivateKey()
		if err != nil {
			t.Fatal(err)
		}
		p1 := hex.EncodeToString(k1.PubKey().SerializeCompressed())
		p2 := hex.EncodeToString(k2.PubKey().SerializeCompressed())
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(fmt.Sprintf(`[{"pubkeys":["%s","%s"],"required":"bad"}]`, p1, p2)),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "required must be a number") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("listreceivedbylabel_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_bad_hex_type", func(t *testing.T) {
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccountWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_wallet_bad_account_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execGetAddressesByAccountWallet("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaddressgroupings_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListAddressGroupingsWallet("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_bad_include_empty", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_bad_include_watchonly", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wallet_bad_account_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wallet_too_many_args", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_wallet_bad_address_type", func(t *testing.T) {
		_, code, msg := execGetAccountWallet(nil, "testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_wallet_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execGetAccountWallet(nil, "testnet", []json.RawMessage{
			json.RawMessage(`"addr"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_wallet_bad_address_type", func(t *testing.T) {
		_, code, msg := execSetAccountWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_wallet_bad_account_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{WalletAddress: func() string { return addr }}
		_, code, msg := execSetAccountWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_wallet_too_many_args", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execGetAccountAddressWallet(paths, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_builtin_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_unencrypted_bad_old_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "oldpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_unencrypted_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletLockUnencrypted([]json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_pubkeys_not_array", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"pubkeys":"not-array"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "pubkeys must be a non-empty array") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("walletpassphrasechange_unencrypted_bad_new_type", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "newpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_unencrypted_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{json.RawMessage(`"old"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_empty_passphrase", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_builtin_empty_passphrase", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin([]json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "passphrase must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_wallet_wrong_arg_count", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSetAccountWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`""`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_wallet_bad_account_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execGetAccountAddressWallet(paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "account must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_scriptpubkey_object_bad_address_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"address":123}}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "scriptPubKey address must be a string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_scriptpubkey_object_bad_hex_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"hex":123}}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "scriptPubKey hex must be a string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_scriptpubkey_object_missing_fields", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{}}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "must contain address or hex") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("signrawtransaction_prevtxs_bad_entry", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x88
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`[123]`),
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "prevtxs entry must be an object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_privkeys_not_array", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevTxid := repeatHex('c')
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
		})
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "privkeys must be an array of strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_privkeys_bad_element_type", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevTxid := repeatHex('d')
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
		})
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "privkeys must be an array of strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_prevtxs_bad_txid_length", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x89
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": "abc", "vout": 0, "scriptPubKey": "00"},
		})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "prevtxs txid must be 64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_prevtxs_duplicate_entry", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x8a
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevTxid := repeatHex('b')
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
		})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "duplicate prevtxs entry for") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_invalid_maximum_amount", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"maximumAmount":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid maximumAmount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_unlock_not_bool", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{json.RawMessage(`"notbool"`)})
		if code != -8 || !strings.Contains(msg, "unlock") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_bad_hex_param_type", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex param") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_bad_iswitness_type", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "iswitness must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_too_many_args", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_input_object", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[123]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "bad input object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_missing_txid", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[{"vout":0}]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "each input needs txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_vout", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp, _ := json.Marshal([]map[string]interface{}{
			{"txid": repeatHex('a'), "vout": "x"},
		})
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, out})
		if code != -8 || !strings.Contains(msg, "each input needs integer vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_empty_outputs", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "at least one output required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_data_output_not_hex", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"data":123}`),
		})
		if code != -8 || !strings.Contains(msg, "data output must be hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_amount_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: true})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "amount must be number or string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_invalid_output_address", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"notvalidaddr":0.1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid output address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_sequence", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp, _ := json.Marshal([]map[string]interface{}{
			{"txid": repeatHex('b'), "vout": 0, "sequence": -1},
		})
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, out})
		if code != -8 || !strings.Contains(msg, "bad sequence") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_locktime_uint32_overflow", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
			json.RawMessage(`4294967296`),
		})
		if code != -8 || !strings.Contains(msg, "locktime must be uint32") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_txids_bad_element_type", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetTxOutProof(ix, raw, nil, []json.RawMessage{
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "expected JSON array of txid strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_blockhash_not_string", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetTxOutProof(ix, raw, nil, []json.RawMessage{
			json.RawMessage(`["` + repeatHex('c') + `"]`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad blockhash") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_allowhighfees_not_bool", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"010203"`),
			json.RawMessage(`"notbool"`),
		}, nil, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "allowhighfees must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_bad_hex_param_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		}, nil, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "bad hex param") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_too_many_args", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"010203"`),
			json.RawMessage(`false`),
			json.RawMessage(`0.1`),
			json.RawMessage(`1`),
		}, nil, true, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_invalid_maxfeerate", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"010203"`),
			json.RawMessage(`false`),
			json.RawMessage(`-1`),
		}, nil, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid maxfeerate") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_unencrypted_empty_new_passphrase", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_unencrypted_called", func(t *testing.T) {
		_, code, msg := execWalletLockUnencrypted(nil)
		if code != -15 || !strings.Contains(msg, "unencrypted wallet") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_bad_blockhash_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("validateaddress_redeemscript_bad_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execValidateAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "redeemScript must be hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_invalid_maxfeerate", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execTestMempoolAccept(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["010203"]`),
			json.RawMessage(`-1`),
		}, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid maxfeerate") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_redeemscript_bad_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"hex":"76a9140102030405060708090a0b0c0d0e0f10111213141516171888ac"},"redeemscript":123}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "redeemscript must be a hex string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("createrawtransaction_op_return_too_large", func(t *testing.T) {
		dataHex := strings.Repeat("ff", 81)
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"data":"` + dataHex + `"}`),
		})
		if code != -8 || !strings.Contains(msg, "OP_RETURN data exceeds 80 bytes") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_invalid_data_hex", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"data":"zz"}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid data hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_txid_length", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": "abc", "vout": 0}})
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, out})
		if code != -8 || !strings.Contains(msg, "txid must be 64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_txid_hex", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		badTxid := repeatHex('a')[:63] + "g"
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": badTxid, "vout": 0}})
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, out})
		if code != -8 || !strings.Contains(msg, "txid must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_zero_amount", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "bad amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_negative_amount", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: -1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "bad amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_invalid_hex", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		validHex := hex.EncodeToString(parentRaw)
		_, code, msg := execCombineRawTransaction([]json.RawMessage{
			json.RawMessage(`["` + validHex + `","zz"]`),
		})
		if code != -8 || !strings.Contains(msg, "invalid transaction hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_bad_purpose_type", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "purpose must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_wallet_bad_label_type", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execGetAddressesByLabelWallet("testnet", paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "label must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_bad_target_confirmations_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "target_confirmations must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_bad_include_watchonly", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "include_watchonly") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_redeemscript_empty", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nWatchAddr" },
			WalletAddress:        func() string { return "nWatchAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"scriptPubKey":{"hex":"76a9140102030405060708090a0b0c0d0e0f10111213141516171888ac"},"redeemscript":""}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "redeemscript must be a hex string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("listreceivedbyaddress_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_wallet_bad_minconf_type", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "minconf must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_not_implemented", func(t *testing.T) {
		_, code, msg := execEncryptWalletPaths(nil, []json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -1 || !strings.Contains(msg, "wallet is not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletPassphrasePaths(nil, []json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`60`),
		})
		if code != -1 || !strings.Contains(msg, "wallet is not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangePaths(nil, []json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
		})
		if code != -1 || !strings.Contains(msg, "wallet is not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_pqcommit_bad_object", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"pqcommit":"not-object"}`),
		})
		if code != -8 || !strings.Contains(msg, "pqcommit must be object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_pqcommit_unknown_tag", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"pqcommit":{"tag":"BAD1","commitment":"` + repeatHex('a') + `"}}`),
		})
		if code != -8 || !strings.Contains(msg, "unknown tag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_pqcommit_bad_commitment_hex", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{"pqcommit":{"tag":"FLC1","commitment":"abc"}}`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex chars") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_prevtxs_bad_entry", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x91
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`[123]`),
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "prevtxs entry must be an object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_prevtxs_bad_txid_length", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x92
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": "abc", "vout": 0, "scriptPubKey": "00"},
		})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "prevtxs txid must be 64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_prevtxs_not_array", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x93
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`"not-array"`),
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "prevtxs must be an array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_bad_hex_type", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_privkeys_bad_element_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x94
		payTo, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		prevTxid := repeatHex('9')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		outObj := map[string]interface{}{payTo: 0.1}
		outJSON, _ := json.Marshal(outObj)
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, outJSON})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		pubC := mustPubCompressed(t, sec)
		h160 := pubkeyHash160(pubC)
		pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
		pkScript = append(pkScript, 0x88, 0xac)
		prevEntry := map[string]interface{}{
			"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
		_, code, msg = execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "privkeys must be an array of strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_bad_blockhash_type", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetRawTransaction(ix, raw, j, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`false`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad blockhash") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_paths_not_implemented", func(t *testing.T) {
		_, code, msg := execWalletLockPaths(nil, nil)
		if code != -1 || !strings.Contains(msg, "wallet is not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_bad_nrequired_type", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`"bad"`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "bad nrequired") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_bad_key_element_type", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "bad key at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_not_enough_keys", func(t *testing.T) {
		pub := "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
		keys, _ := json.Marshal([]string{pub})
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			keys,
		})
		if code != -8 || !strings.Contains(msg, "not enough keys supplied") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_duplicate_key", func(t *testing.T) {
		pub := "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
		keys, _ := json.Marshal([]string{pub, pub})
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`1`),
			keys,
		})
		if code != -8 || !strings.Contains(msg, "duplicate key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_txid_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(pool, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_priority_delta_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`"bad"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "bad priority_delta") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_bad_fee_delta_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "bad fee_delta") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wallet_minconf_out_of_range", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "minconf out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("testmempoolaccept_bad_tx_array_element_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execTestMempoolAccept(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[123]`),
		}, true, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "expected JSON array of hex strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_timeout_out_of_range", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "timeout out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_witness_not_supported", func(t *testing.T) {
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{},
				PrevIdx:  0xffffffff,
				Sequence: 0xffffffff,
				Witness:  [][]byte{{0x01}},
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		hexStr := hex.EncodeToString(raw)
		_, code, msg := execCombineRawTransaction([]json.RawMessage{
			json.RawMessage(`["` + hexStr + `","` + hexStr + `"]`),
		})
		if code != -8 || !strings.Contains(msg, "witness transactions are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_transactions_differ", func(t *testing.T) {
		tx1 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		tx2 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 2, PkScript: []byte{0x51}}},
		}
		raw1, _ := tx1.Serialize()
		raw2, _ := tx2.Serialize()
		_, code, msg := execCombineRawTransaction([]json.RawMessage{
			json.RawMessage(`["` + hex.EncodeToString(raw1) + `","` + hex.EncodeToString(raw2) + `"]`),
		})
		if code != -8 || !strings.Contains(msg, "differs from the first") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_bad_txid_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		}, nil, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "txid must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_empty_passphrase", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return false },
			WalletEncrypt:     func(string) (string, error) { return "encrypted", nil },
		}
		_, code, msg := execEncryptWalletPaths(paths, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "passphrase must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_empty_passphrase", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletUnlock:      func(string, int64) error { return nil },
		}
		_, code, msg := execWalletPassphrasePaths(paths, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_empty_passphrase", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted:      func() bool { return true },
			WalletChangePassphrase: func(string, string) error { return nil },
		}
		_, code, msg := execWalletPassphraseChangePaths(paths, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "must not be empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_already_encrypted", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletEncrypt:     func(string) (string, error) { return "encrypted", nil },
		}
		_, code, msg := execEncryptWalletPaths(paths, []json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -1 || !strings.Contains(msg, "already encrypted") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_bad_target_confirmations_zero", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid parameter") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_amount_too_small", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 1e-9})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "amount too small") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_amount_string", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: "not-a-number"})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "bad amount string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_prevtxs_duplicate_entry", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x95
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		prevTxid := repeatHex('c')
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "00"},
		})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			prevArr,
			privArr,
		})
		if code != -8 || !strings.Contains(msg, "duplicate prevtxs entry for") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_bad_sighashtype", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x96
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		payTo, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		prevTxid := repeatHex('8')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		outObj := map[string]interface{}{payTo: 0.1}
		outJSON, _ := json.Marshal(outObj)
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, outJSON})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		pubC := mustPubCompressed(t, sec)
		h160 := pubkeyHash160(pubC)
		pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
		pkScript = append(pkScript, 0x88, 0xac)
		prevEntry := map[string]interface{}{
			"txid": prevTxid, "vout": 0, "scriptPubKey": hex.EncodeToString(pkScript),
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{prevEntry})
		privArr, _ := json.Marshal([]string{wif})
		_, code, msg = execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
			privArr,
			json.RawMessage(`"BOGUS"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid sighashtype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_empty_key_at_index", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`[""]`),
		})
		if code != -8 || !strings.Contains(msg, "empty key at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_too_many_pubkeys", func(t *testing.T) {
		pubs := make([]string, 17)
		for i := range pubs {
			sec := make([]byte, 32)
			sec[0] = byte(i + 1)
			pubC := mustPubCompressed(t, sec)
			pubs[i] = hex.EncodeToString(pubC)
		}
		keys, _ := json.Marshal(pubs)
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`1`),
			keys,
		})
		if code != -8 || !strings.Contains(msg, "> 16") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_bad_txid_type", func(t *testing.T) {
		pool := mempool.New(10)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
		}
		_, code, msg := execPsbtBumpFee("testnet", paths, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "txid must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_not_array", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "expected array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_options_not_object", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["010203"]`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["010203"]`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_wallet_bad_txid_type", func(t *testing.T) {
		pool := mempool.New(10)
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, pool, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_query_options_invalid_minimum_sum_amount", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`{"minimumSumAmount":-1}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid minimumSumAmount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_no_params", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinerawtransaction_conflicting_scriptsig", func(t *testing.T) {
		base := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff,
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		tx1 := cloneTxForCombine(base)
		tx1.Vin[0].Script = []byte{0x01}
		tx2 := cloneTxForCombine(base)
		tx2.Vin[0].Script = []byte{0x02}
		raw1, _ := tx1.Serialize()
		raw2, _ := tx2.Serialize()
		_, code, msg := execCombineRawTransaction([]json.RawMessage{
			json.RawMessage(`["` + hex.EncodeToString(raw1) + `","` + hex.EncodeToString(raw2) + `"]`),
		})
		if code != -8 || !strings.Contains(msg, "conflicting scriptSig") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_nrequired_above_16", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{
			json.RawMessage(`17`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "bad nrequired") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_blockhash_filter_bad_verbose", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetRawTransaction(ix, raw, j, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "bad verbose flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletUnlock:      func(string, int64) error { return nil },
		}
		_, code, msg := execWalletPassphrasePaths(paths, []json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`60`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_bad_timeout_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletUnlock:      func(string, int64) error { return nil },
		}
		_, code, msg := execWalletPassphrasePaths(paths, []json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted:      func() bool { return true },
			WalletChangePassphrase: func(string, string) error { return nil },
		}
		_, code, msg := execWalletPassphraseChangePaths(paths, []json.RawMessage{
			json.RawMessage(`"old"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return false },
			WalletEncrypt:     func(string) (string, error) { return "encrypted", nil },
		}
		_, code, msg := execEncryptWalletPaths(paths, []json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_locked_wallet", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"00"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_wallet_locked", func(t *testing.T) {
		pool := mempool.New(10)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execPsbtBumpFee("testnet", paths, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_subtract_fee_outputs_bad_element", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`{"subtractFeeFromOutputs":["bad"]}`),
		})
		if code != -8 || !strings.Contains(msg, "invalid vout index in subtractFeeFromOutputs") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_transactions_null", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`null`),
		})
		if code != -8 || !strings.Contains(msg, "transactions must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_transactions_bad_entry", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[123]`),
		})
		if code != -8 || !strings.Contains(msg, "expected object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_bad_vout_negative", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		entry, _ := json.Marshal(map[string]interface{}{"txid": repeatHex('a'), "vout": -1})
		_, code, msg := execLockUnspentWallet(paths, []json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[` + string(entry) + `]`),
		})
		if code != -8 || !strings.Contains(msg, "vout must be positive") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_timestamp_bad_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","timestamp":true}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "timestamp must be a number") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_timestamp_negative", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","timestamp":-1}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "non-negative number") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importmulti_row_internal_bad_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","internal":"notbool"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "internal") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("walletpassphrasechange_paths_bad_old_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted:      func() bool { return true },
			WalletChangePassphrase: func(string, string) error { return nil },
		}
		_, code, msg := execWalletPassphraseChangePaths(paths, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"new"`),
		})
		if code != -8 || !strings.Contains(msg, "oldpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_unsupported_address_version", func(t *testing.T) {
		mainP, err := chain.ParamsFor(chain.MainnetDogecoin)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(mainP)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
		})
		if code != -8 || !strings.Contains(msg, "unsupported address version") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_wallet_wrong_arg_count", func(t *testing.T) {
		pool := mempool.New(10)
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_options_not_object", func(t *testing.T) {
		pool := mempool.New(100)
		parentHash := [32]byte{7}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidToRPC(old.TxHash())
		_ = pool.Add(oldRaw)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
		}
		_, code, msg := execPsbtBumpFee("testnet", paths, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + oldID + `"`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_timeout_out_of_range", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletUnlock:      func(string, int64) error { return nil },
		}
		_, code, msg := execWalletPassphrasePaths(paths, []json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "timeout out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_bad_new_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted:      func() bool { return true },
			WalletChangePassphrase: func(string, string) error { return nil },
		}
		_, code, msg := execWalletPassphraseChangePaths(paths, []json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "newpassphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_bad_passphrase_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return false },
			WalletEncrypt:     func(string) (string, error) { return "encrypted", nil },
		}
		_, code, msg := execEncryptWalletPaths(paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_locked_wallet", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletChangePassphrase: func(string, string) error {
				return wallet.ErrWalletLocked
			},
		}
		_, code, msg := execWalletPassphraseChangePaths(paths, []json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_paths_wrong_arg_count", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletLock:        func() error { return nil },
		}
		_, code, msg := execWalletLockPaths(paths, []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_amount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "amount must be a number or string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_amount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "amount must be a number or string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_bad_amount_type", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "amount must be a number or string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_prevtxs_not_array", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "prevtxs must be an array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_bad_privkeys_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
		}
		prevTxid := repeatHex('a')
		inp, _ := json.Marshal([]map[string]interface{}{{"txid": prevTxid, "vout": 0}})
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		rawHex, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{inp, out})
		if code != 0 {
			t.Fatalf("createraw: code=%d msg=%q", code, msg)
		}
		prevArr, _ := json.Marshal([]map[string]interface{}{
			{"txid": prevTxid, "vout": 0, "scriptPubKey": "51"},
		})
		_, code, msg = execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + rawHex.(string) + `"`),
			prevArr,
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "privkeys must be an array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_row_timestamp_bad_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","timestamp":true}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "timestamp must be a number") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importdescriptors_row_internal_bad_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","internal":"notbool"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "internal") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importdescriptors_row_missing_desc", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"label":"watch"}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "missing desc") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importdescriptors_row_timestamp_negative", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":"pkh(` + addr + `)","timestamp":-1}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "non-negative number") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importdescriptors_row_desc_bad_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":123}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "desc must be a string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("decoderawtransaction_empty_hex", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_invalid_tx_at_index", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["nothex"]`),
		})
		if code != -22 || !strings.Contains(msg, "invalid tx at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_locktime_bad_type", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := json.Marshal(map[string]interface{}{addr: 0.1})
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			out,
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "bad locktime") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_bad_descriptor_type", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "descriptor must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_rawtx_bad_type", func(t *testing.T) {
		pool := mempool.New(100)
		parentHash := [32]byte{8}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidToRPC(old.TxHash())
		_ = pool.Add(oldRaw)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + oldID + `"`),
			json.RawMessage(`{"rawtx":123}`),
		}, nil, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "rawtx must be a hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_rawtx_invalid_hex", func(t *testing.T) {
		pool := mempool.New(100)
		parentHash := [32]byte{9}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidToRPC(old.TxHash())
		_ = pool.Add(oldRaw)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + oldID + `"`),
			json.RawMessage(`{"rawtx":"zz"}`),
		}, nil, chain.RebootTestnet)
		if code != -22 || !strings.Contains(msg, "TX decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_wallet_locked", func(t *testing.T) {
		pool := mempool.New(100)
		parentHash := [32]byte{10}
		old := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: wire.MaxBIP125RBFSequence}},
			Vout:    []wire.TxOut{{Value: 100_000_000, PkScript: []byte{0x51}}},
		}
		oldRaw, _ := old.Serialize()
		oldID := txidToRPC(old.TxHash())
		_ = pool.Add(oldRaw)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, paths, []json.RawMessage{
			json.RawMessage(`"` + oldID + `"`),
		}, nil, chain.RebootTestnet)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_bad_descriptor_type", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "descriptor must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_too_many_args", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"pkh(addr)"`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_too_many_args", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"pkh(addr)"`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_row_desc_bad_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[{"desc":123}]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		if rows[0]["success"].(bool) {
			t.Fatalf("want success=false row=%#v", rows[0])
		}
		errMap := rows[0]["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "desc must be a string") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("importdescriptors_row_not_object", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportWatch:    func([]byte) error { return nil },
		}
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`[123]`),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		row := rows[0].(map[string]interface{})
		if row["success"].(bool) {
			t.Fatalf("want success=false row=%#v", row)
		}
		errMap := row["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "must be a JSON object") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("simulaterawtransaction_invalid_tx_at_index_1", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		okTx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		okRaw, _ := okTx.Serialize()
		arr, _ := json.Marshal([]string{hex.EncodeToString(okRaw), "zz"})
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{arr})
		if code != -22 || !strings.Contains(msg, "invalid tx at index 1") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_tx_decode_failed", func(t *testing.T) {
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg := execSimulateRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`["deadbeef"]`),
		})
		if code != -22 || !strings.Contains(msg, "TX decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_bad_amount_string", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"not-a-number"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_bad_amount_string", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"not-a-number"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_bad_amount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]interface{}{addr: true})
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			amounts,
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "amount must be a number or string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_wallet_locked", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execSendToAddress("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_wallet_locked", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execSendFrom("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_wallet_locked", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return addr },
			WalletAddress:        func() string { return addr },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
			WalletWIF:            func() string { return "6dummy" },
		}
		_, code, msg := execDumpPrivKey("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_wallet_locked", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x42
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportPrivKey:  func(string) error { return nil },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execImportPrivKey("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + wif + `"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_wallet_locked", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return addr },
			WalletAddress:        func() string { return addr },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execSignMessage("testnet", paths, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"hello"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_wallet_locked", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{
			Utxo:                 utxo,
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(parentRaw) + `"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_bad_passphrase_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletIsEncrypted: func() bool { return true },
			WalletUnlock:      func(string, int64) error { return nil },
		}
		_, code, msg := execWalletPassphrasePaths(paths, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`60`),
		})
		if code != -8 || !strings.Contains(msg, "passphrase must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_no_params", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment(nil)
		if code != -8 || !strings.Contains(msg, "expected 1 or 2 arguments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_too_many_args", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{
			json.RawMessage(`"51"`),
			json.RawMessage(`"FLC1"`),
			json.RawMessage(`"00"`),
		})
		if code != -8 || !strings.Contains(msg, "expected 1 or 2 arguments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_bad_script_type", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad script hex argument") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_empty_script", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "empty script") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_invalid_hex", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{json.RawMessage(`"zz"`)})
		if code != -8 || !strings.Contains(msg, "invalid hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_not_canonical", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{json.RawMessage(`"51"`)})
		if code != -8 || !strings.Contains(msg, "not a canonical Phase-1") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDogegoCreatePQCarrier(nil, nil)
		if code != -8 || !strings.Contains(msg, "expected 1 object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_flag_off", func(t *testing.T) {
		_, code, msg := execDogegoCreatePQCarrier(&DataPaths{}, []json.RawMessage{
			json.RawMessage(`{"tx_base_hex":"00"}`),
		})
		if code != -8 || !strings.Contains(msg, "pq_carrier") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCarrier(nil)
		if code != -8 || !strings.Contains(msg, "expected 1 object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_bad_object", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{json.RawMessage(`"not-object"`)})
		if code != -8 || !strings.Contains(msg, "bad argument object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_sendpqcarrier_missing_args", func(t *testing.T) {
		_, code, msg := execDogegoSendPQCarrier("testnet", nil, nil, nil, nil, nil, nil, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "expected address and amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_sendpqcarrier_flag_off", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execDogegoSendPQCarrier("testnet", &DataPaths{}, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "pq_carrier") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_pq_carrier_unwired", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetAvoidReuse:  func(bool) error { return nil },
		}
		_, code, msg := execSetWalletFlag(paths, []json.RawMessage{
			json.RawMessage(`"pq_carrier"`),
			json.RawMessage(`true`),
		})
		if code != -4 || !strings.Contains(msg, "unknown flag pq_carrier") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_pq_commitments_unwired", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetAvoidReuse:  func(bool) error { return nil },
		}
		_, code, msg := execSetWalletFlag(paths, []json.RawMessage{
			json.RawMessage(`"pq_commitments"`),
			json.RawMessage(`true`),
		})
		if code != -4 || !strings.Contains(msg, "unknown flag pq_commitments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_bad_flag_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetAvoidReuse:  func(bool) error { return nil },
		}
		_, code, msg := execSetWalletFlag(paths, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "flag must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_wallet_locked", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		amounts, _ := json.Marshal(map[string]float64{addr: 1.0})
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletAddress:        func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execSendMany("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			amounts,
		}, nil, false, chain.RebootTestnet)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_wallet_locked", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`{}`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_wallet_locked", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp := `[{"txid":"` + repeatHex('a') + `","vout":0}]`
		outObj := `{"` + addr + `":1.0}`
		b64Res, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(outObj),
		})
		if code != 0 {
			t.Fatalf("createpsbt code=%d msg=%q", code, msg)
		}
		b64, ok := b64Res.(string)
		if !ok || b64 == "" {
			t.Fatalf("createpsbt result %#v", b64Res)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		_, code, msg = execWalletProcessPsbt("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + b64 + `"`),
		})
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_spend_keys_wallet_locked", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x42
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		importAddr, err := addressFromWIF("testnet", wif)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportPrivKey:  func(string) error { return nil },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		row, _ := json.Marshal(map[string]interface{}{"desc": "pkh(" + importAddr + ")", "keys": []string{wif}})
		res, code, msg := execImportDescriptors("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage("[" + string(row) + "]"),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]interface{})
		r := rows[0].(map[string]interface{})
		if r["success"].(bool) {
			t.Fatalf("want success=false row=%#v", r)
		}
		errMap := r["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "walletpassphrase") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("combinerawtransaction_empty_array", func(t *testing.T) {
		_, code, msg := execCombineRawTransaction([]json.RawMessage{json.RawMessage(`[]`)})
		if code != -8 || !strings.Contains(msg, "at least two transactions") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_bad_object", func(t *testing.T) {
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{json.RawMessage(`"not-object"`)})
		if code != -8 || !strings.Contains(msg, "bad argument object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_unknown_tag", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		req, _ := json.Marshal(map[string]interface{}{
			"tx_base_hex": hex.EncodeToString(parentRaw),
			"tag":         "BOGUS",
		})
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "unknown tag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_missing_pk_script", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		paths := &DataPaths{
			WalletPqCarrierEnabled: func() bool { return true },
			WalletPQCarrierKeyMaterial: func(string) (string, []byte, []byte, error) {
				return "FLC1", make([]byte, 32), make([]byte, 64), nil
			},
		}
		req, _ := json.Marshal(map[string]interface{}{"tx_base_hex": hex.EncodeToString(parentRaw)})
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "pk_script_hex required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_invalid_tx_base_hex", func(t *testing.T) {
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{
			json.RawMessage(`{"tx_base_hex":"deadbeef"}`),
		})
		if code != -8 || !strings.Contains(msg, "cannot decode tx_base_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_missing_txr", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		h := hex.EncodeToString(parentRaw)
		req, _ := json.Marshal(map[string]interface{}{"txc_hex": h, "txr_hex": ""})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "txr_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_missing_pk_script", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		h := hex.EncodeToString(parentRaw)
		req, _ := json.Marshal(map[string]interface{}{"txc_hex": h, "txr_hex": h})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "pk_script_hex required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_bad_tag_type", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"00"`),
		})
		if code != -8 || !strings.Contains(msg, "bad tag argument") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_bad_commitment_hex_type", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{
			json.RawMessage(`"FLC1"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad commitment hex argument") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_sendpqcarrier_bad_address_type", func(t *testing.T) {
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		_, code, msg := execDogegoSendPQCarrier("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`1.0`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "bad address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_sendpqcarrier_bad_amount_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		_, code, msg := execDogegoSendPQCarrier("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`"not-a-number"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "bad amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_bad_value_type", func(t *testing.T) {
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletSetAvoidReuse:  func(bool) error { return nil },
		}
		_, code, msg := execSetWalletFlag(paths, []json.RawMessage{
			json.RawMessage(`"avoid_reuse"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "value") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_keys_wallet_locked", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x55
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportPrivKey:  func(string) error { return nil },
			WalletImportWatch:    func([]byte) error { return nil },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		row, _ := json.Marshal(map[string]interface{}{"keys": []string{wif}})
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage("[" + string(row) + "]"),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		r := rows[0]
		if r["success"].(bool) {
			t.Fatalf("want success=false row=%#v", r)
		}
		errMap := r["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "walletpassphrase") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("combinepsbt_not_array", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{json.RawMessage(`"not-array"`)})
		if code != -8 || !strings.Contains(msg, "JSON array of PSBT strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_empty_array", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{json.RawMessage(`[]`)})
		if code != -8 || !strings.Contains(msg, "at least one PSBT") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_bad_inputs_not_array", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"not-array"`),
			json.RawMessage(`{"` + addr + `":1.0}`),
		})
		if code != -8 || !strings.Contains(msg, "inputs must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_bad_locktime", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		inp := `[{"txid":"` + repeatHex('b') + `","vout":0}]`
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "locktime must be uint32") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_bad_sighashtype", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp := `[{"txid":"` + repeatHex('c') + `","vout":0}]`
		b64Res, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
		})
		if code != 0 {
			t.Fatalf("createpsbt code=%d msg=%q", code, msg)
		}
		b64 := b64Res.(string)
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg = execWalletProcessPsbt("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + b64 + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`"BOGUS"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid sighashtype") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_bad_finalize_flag", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp := `[{"txid":"` + repeatHex('d') + `","vout":0}]`
		b64Res, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
		})
		if code != 0 {
			t.Fatalf("createpsbt code=%d msg=%q", code, msg)
		}
		b64 := b64Res.(string)
		paths := &DataPaths{WalletDefaultAddress: func() string { return "nAddr" }}
		_, code, msg = execWalletProcessPsbt("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + b64 + `"`),
			json.RawMessage(`false`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "finalize must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_descriptor_must_be_string", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "descriptor must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_bad_version", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		inp := `[{"txid":"` + repeatHex('e') + `","vout":0}]`
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`1.5`),
		})
		if code != -8 || !strings.Contains(msg, "version must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_bad_replaceable", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		inp := `[{"txid":"` + repeatHex('f') + `","vout":0}]`
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
			json.RawMessage(`null`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "replaceable must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`"!!!"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_bad_extract_flag", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp := `[{"txid":"` + repeatHex('1') + `","vout":0}]`
		b64Res, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr + `":1.0}`),
		})
		if code != 0 {
			t.Fatalf("createpsbt code=%d msg=%q", code, msg)
		}
		b64 := b64Res.(string)
		_, code, msg = execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`"` + b64 + `"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "extract must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_no_params", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_bad_iswitness_type", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`"abc"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "iswitness must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_psbt_must_be_string", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "psbt must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_bad_permitsigdata_type", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "permitsigdata must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_witness_not_supported", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "witness transactions are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_hex_must_be_string", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "hex must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_input_has_scriptsig", func(t *testing.T) {
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff, Script: []byte{0x01}}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		raw, err := tx.Serialize()
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"` + hex.EncodeToString(raw) + `"`),
		})
		if code != -8 || !strings.Contains(msg, "has scriptSig") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_keys_unavailable", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		req, _ := json.Marshal(map[string]interface{}{
			"tx_base_hex":   hex.EncodeToString(parentRaw),
			"pk_script_hex": "76a914000000000000000000000000000000000000000088ac",
		})
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{req})
		if code != -1 || !strings.Contains(msg, "PQ keys unavailable") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_createpqcarrier_invalid_pk_script_hex", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		paths := &DataPaths{
			WalletPqCarrierEnabled: func() bool { return true },
			WalletPQCarrierKeyMaterial: func(string) (string, []byte, []byte, error) {
				return "FLC1", make([]byte, 32), make([]byte, 64), nil
			},
		}
		req, _ := json.Marshal(map[string]interface{}{
			"tx_base_hex":   hex.EncodeToString(parentRaw),
			"pk_script_hex": "zz",
		})
		_, code, msg := execDogegoCreatePQCarrier(paths, []json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "invalid pk_script_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_unknown_tag", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		h := hex.EncodeToString(parentRaw)
		req, _ := json.Marshal(map[string]interface{}{
			"txc_hex":       h,
			"txr_hex":       h,
			"pk_script_hex": "76a914000000000000000000000000000000000000000088ac",
			"tag":           "BOGUS",
		})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "unknown tag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_wallet_locked", func(t *testing.T) {
		paths := &DataPaths{
			WalletNewChangeAddress: func() (string, error) { return "", wallet.ErrWalletLocked },
		}
		_, code, msg := execGetRawChangeAddress(paths, nil)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_wallet_locked", func(t *testing.T) {
		paths := &DataPaths{
			WalletNewAddress: func() (string, error) { return "", wallet.ErrWalletLocked },
		}
		_, code, msg := execGetNewAddress("testnet", paths, nil)
		if code != -13 || !strings.Contains(msg, "walletpassphrase") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatepriority_invalid_nblocks", func(t *testing.T) {
		_, code, msg := execEstimatePriority(nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -8 || !strings.Contains(msg, "invalid nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbts_invalid_psbt_at_index", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{json.RawMessage(`["!!!"]`)})
		if code != -8 || !strings.Contains(msg, "PSBT at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_bad_outputs", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			Utxo:                 utxo,
		}
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "outputs must be") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_bad_locktime", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		utxo := store.NewUtxoCache()
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			Utxo:                 utxo,
		}
		outs, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			outs,
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "locktime must be uint32") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_bad_filepath_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execImportMempool(pool, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "filepath must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_relative_without_datadir", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execImportMempool(pool, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`"mempool.json"`),
		})
		if code != -8 || !strings.Contains(msg, "relative filepath requires chain data directory") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmempoolpaused_too_many_args", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSetMempoolPaused(pool, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -8 || !strings.Contains(msg, "paused (boolean) required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmempoolpaused_bad_paused_type", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSetMempoolPaused(pool, []json.RawMessage{
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "paused must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_bad_range_type", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(` + addr + `)"`),
			json.RawMessage(`{"bad":1}`),
		})
		if code != -8 || !strings.Contains(msg, "range must be [begin,end]") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_range_not_needed", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(` + addr + `)"`),
			json.RawMessage(`[1,2]`),
		})
		if code != -5 || !strings.Contains(msg, "Range is not needed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_bad_version", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		utxo := store.NewUtxoCache()
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			Utxo:                 utxo,
		}
		outs, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			outs,
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
			json.RawMessage(`1.5`),
		})
		if code != -8 || !strings.Contains(msg, "version must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_bad_options_not_object", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		utxo := store.NewUtxoCache()
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			Utxo:                 utxo,
		}
		outs, _ := json.Marshal(map[string]float64{addr: 1.0})
		_, code, msg := execWalletCreateFundedPsbt("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			outs,
			json.RawMessage(`null`),
			json.RawMessage(`"not-object"`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_wallet_file_not_found", func(t *testing.T) {
		missing := fmt.Sprintf("%s/missing-wallet.json", t.TempDir())
		paths := &DataPaths{WalletPath: func() string { return missing }}
		dest := fmt.Sprintf("%s/backup.dat", t.TempDir())
		destParam, _ := json.Marshal(dest)
		_, code, msg := execBackupWallet(paths, []json.RawMessage{destParam})
		if code != -1 || !strings.Contains(msg, "wallet file not found") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_empty_outputs_object", func(t *testing.T) {
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "outputs must be") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_empty_outputs_array", func(t *testing.T) {
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "at least one output required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`"abc"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_invalid_tx_decode", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"deadbeef"`),
		})
		if code != -8 || !strings.Contains(msg, "converttopsbt:") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_hex_must_be_string", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "hexdata must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_height_must_be_number", func(t *testing.T) {
		paths := &DataPaths{TruncateToHeight: func(int64) error { return nil }}
		_, code, msg := execTruncateToHeight(paths, []json.RawMessage{
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_bad_pk_script_hex", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		h := hex.EncodeToString(parentRaw)
		req, _ := json.Marshal(map[string]interface{}{
			"txc_hex": h, "txr_hex": h, "pk_script_hex": "zz",
		})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "invalid pk_script_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_invalid_txc_hex", func(t *testing.T) {
		req, _ := json.Marshal(map[string]interface{}{
			"txc_hex": "deadbeef", "txr_hex": "00", "pk_script_hex": "76a914",
		})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "cannot decode txc_hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_desc_wallet_locked", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		sec := make([]byte, 32)
		sec[0] = 0x77
		wif, err := chain.EncodeWIF(sec, p.PrivKeyWIFVersion, true)
		if err != nil {
			t.Fatal(err)
		}
		importAddr, err := addressFromWIF("testnet", wif)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{
			WalletDefaultAddress: func() string { return "nAddr" },
			WalletImportPrivKey:  func(string) error { return nil },
			WalletImportWatch:    func([]byte) error { return nil },
			WalletIsEncrypted:    func() bool { return true },
			WalletIsUnlocked:     func() bool { return false },
		}
		row, _ := json.Marshal(map[string]interface{}{"desc": "pkh(" + importAddr + ")", "keys": []string{wif}})
		res, code, msg := execImportMultiWallet("testnet", paths, nil, nil, []json.RawMessage{
			json.RawMessage("[" + string(row) + "]"),
		})
		if code != 0 || msg != "" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
		rows := res.([]map[string]interface{})
		r := rows[0]
		if r["success"].(bool) {
			t.Fatalf("want success=false row=%#v", r)
		}
		errMap := r["error"].(map[string]interface{})
		if !strings.Contains(errMap["message"].(string), "walletpassphrase") {
			t.Fatalf("row error=%#v", errMap)
		}
	})
	t.Run("combinepsbt_different_unsigned_transactions", func(t *testing.T) {
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr1, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		addr2, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		inp := `[{"txid":"` + repeatHex('2') + `","vout":0}]`
		b64a, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr1 + `":1.0}`),
		})
		if code != 0 {
			t.Fatalf("createpsbt a code=%d msg=%q", code, msg)
		}
		b64b, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(inp),
			json.RawMessage(`{"` + addr2 + `":2.0}`),
		})
		if code != 0 {
			t.Fatalf("createpsbt b code=%d msg=%q", code, msg)
		}
		arr, _ := json.Marshal([]string{b64a.(string), b64b.(string)})
		_, code, msg = execCombinePsbt([]json.RawMessage{arr})
		if code != -8 || !strings.Contains(msg, "different unsigned transactions") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_command_must_be_string", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{json.RawMessage(`123`)})
		if code != -32602 || !strings.Contains(msg, "command must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_blockhash_must_be_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_too_many_arguments", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{
			json.RawMessage(`"getblock"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many arguments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbts_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadmempool_no_pool", func(t *testing.T) {
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execLoadMempool(nil, paths, nil, nil, nil, chain.RebootTestnet)
		if code != -18 || !strings.Contains(msg, "mempool not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_empty_filepath", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execImportMempool(pool, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "filepath must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_invalid_optional_params", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{not-json`),
		})
		if code != -8 || !strings.Contains(msg, "invalid optional parameters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_missing_vout_param", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "txid and vout index required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_wrong_arg_count", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_bad_hash_type", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_bad_hash_type", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_bad_nblocks_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_bad_verbose_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3`),
			json.RawMessage(`6`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "verbose must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_blockhash_must_be_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_bad_commitment_length", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{
			json.RawMessage(`"FLC1"`),
			json.RawMessage(`"00"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex chars") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_bad_action_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, nil, nil, filters, nil, []json.RawMessage{
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "action must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_filtertype_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, txIx, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "filtertype must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_bad_param_type", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_bad_address_type", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_filtertype_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, txIx, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "filtertype must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_wrong_arg_count", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, txIx, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_no_tx_index", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "tx index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_nblocks_bad_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks must be a non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_bad_checklevel_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "checklevel must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_blockhash_required", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_blockhash_required", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_blockhash_required", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, nil)
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_blockhash_not_hex", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_blockhash_wrong_length", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"abcd"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_vout_bad_type", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "bad vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcarrier_missing_txc_hex", func(t *testing.T) {
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		parentRaw, _ := parent.Serialize()
		h := hex.EncodeToString(parentRaw)
		req, _ := json.Marshal(map[string]interface{}{"txc_hex": "", "txr_hex": h})
		_, code, msg := execDogegoVerifyPQCarrier([]json.RawMessage{req})
		if code != -8 || !strings.Contains(msg, "txc_hex required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_sendpqcarrier_address_only", func(t *testing.T) {
		paths := &DataPaths{WalletPqCarrierEnabled: func() bool { return true }}
		p, err := chain.ParamsFor(chain.RebootTestnet)
		if err != nil {
			t.Fatal(err)
		}
		addr, err := chain.RandomP2PKHAddress(p)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execDogegoSendPQCarrier("testnet", paths, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "expected address and amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_verifypqcommitment_unknown_tag", func(t *testing.T) {
		_, code, msg := execDogegoVerifyPQCommitment([]json.RawMessage{
			json.RawMessage(`"NOPE"`),
			json.RawMessage(`"` + repeatHex('1') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "unknown tag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_start_bad_start_height_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "start_height must be a non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_start_bad_stop_height_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
			json.RawMessage(`null`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "stop_height must be a non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockhash_height_required", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockHashGolden(j, nil)
		if code != -8 || !strings.Contains(msg, "height required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockhash_invalid_height_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockHashGolden(j, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "invalid height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockheader_bad_verbose_flag", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockHeaderGolden(j, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"notbool"`),
		})
		if code != -8 || !strings.Contains(msg, "bad verbose flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_bad_hash_type", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockhash_height_out_of_range", func(t *testing.T) {
		j := &memJournal{tip: 1, best: "b", gen: "g", count: 2, hdrs: [][]byte{make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execGetBlockHashGolden(j, []json.RawMessage{json.RawMessage(`9`)})
		if code != -8 || !strings.Contains(msg, "height out of range") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockhash_negative_height", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockHashGolden(j, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "invalid height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_blockhash_wrong_length", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_blockhash_wrong_length", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_blockhash_not_hex", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_blockhash_not_in_chain", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
		})
		if code != -5 || msg != "Block not found" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_blockhash_not_hex", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindexblockfilters_no_raw_store", func(t *testing.T) {
		dir := t.TempDir()
		j, err := store.OpenHeaderJournal(dir+"/headers.bin", make([]byte, 80))
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{ChainDataDir: dir}
		_, code, msg := execReindexBlockFilters(paths, j, nil, nil, nil)
		if code != -1 || !strings.Contains(msg, "raw block store") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reindexblockfilters_no_filters", func(t *testing.T) {
		dir := t.TempDir()
		j, err := store.OpenHeaderJournal(dir+"/headers.bin", make([]byte, 80))
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		paths := &DataPaths{ChainDataDir: dir}
		_, code, msg := execReindexBlockFilters(paths, j, raw, txIx, nil)
		if code != -1 || !strings.Contains(msg, "filter index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_empty_path", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		_, code, msg := execLoadTxOutSet(j, nil, paths, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "path required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_recoverheaders_unwired", func(t *testing.T) {
		_, code, msg := execDogegoRecoverHeaders(nil)
		if code != -1 || !strings.Contains(msg, "header journal not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_bad_address_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "address must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_bad_txid_type", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_nblocks_bad_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_bad_auxpow_type", func(t *testing.T) {
		_, code, msg := execSubmitAuxBlock(nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "auxpow must be a hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_bad_auxpow_odd_hex", func(t *testing.T) {
		_, code, msg := execSubmitAuxBlock(nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`"abc"`),
		})
		if code != -8 || !strings.Contains(msg, "AuxPow decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_no_store", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockFilter(j, nil, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "block store not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_bad_bantime_type", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"127.0.0.1"`),
			json.RawMessage(`"add"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "bad bantime") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_bad_node_type", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"add"`),
		})
		if code != -8 || !strings.Contains(msg, "bad node") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_bad_stats_not_array", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"feerate"`),
		})
		if code != -8 || !strings.Contains(msg, "array of strings") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_no_tx_index", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, nil, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -18 || !strings.Contains(msg, "tx index") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_bad_timeout_type", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be an integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_bad_hash_type", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_bad_height_type", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "height must be a number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_bad_timeout_type", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "timeout must be an integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setnetworkactive_bad_state_type", func(t *testing.T) {
		_, code, msg := execSetNetworkActive(nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "bad state") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmaxconnections_bad_count_type", func(t *testing.T) {
		_, code, msg := execSetMaxConnections(nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "bad newconnectioncount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("disconnectnode_bad_address_type", func(t *testing.T) {
		_, code, msg := execDisconnectNode(nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("disconnectnode_empty_address", func(t *testing.T) {
		_, code, msg := execDisconnectNode(nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_bad_action_type", func(t *testing.T) {
		_, code, msg := execScanTxOutSet("testnet", nil, nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "action must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_start_empty_scanobjects", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "scanobjects array is empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_start_bad_scanobjects_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			json.RawMessage(`"not-array"`),
		})
		if code != -8 || !strings.Contains(msg, "scanobjects must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("loadtxoutset_file_not_found", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		paths := &DataPaths{Utxo: store.NewUtxoCache()}
		missing := filepath.Join(t.TempDir(), "missing-utxo.jsonl")
		dest, _ := json.Marshal(missing)
		_, code, msg := execLoadTxOutSet(j, nil, paths, []json.RawMessage{dest})
		if code != -1 || !strings.Contains(msg, "loadtxoutset:") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumptxoutset_bad_path_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		utxo := store.NewUtxoCache()
		utxo.ApplyBlock(&wire.ParsedBlock{
			Txs: []*wire.Tx{{
				Version: 1,
				Vin:     []wire.TxIn{{PrevIdx: 0xffffffff}},
				Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
			}},
		}, 0)
		paths := &DataPaths{ChainDataDir: t.TempDir(), Utxo: utxo}
		_, code, msg := execDumpTxOutSet(j, nil, paths, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "path must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_invalid_payout_address", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`"not-a-valid-address"`),
		})
		if code != -5 || !strings.Contains(msg, "Invalid coinbase payout address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifytxoutproof_bad_proof_type", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyTxOutProof(j, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad hex param") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_start_empty_scanobjects", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "scanobjects array is empty") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("upgradetxindex_negative_maxfiles", func(t *testing.T) {
		paths := &DataPaths{ChainDataDir: t.TempDir()}
		_, code, msg := execUpgradeTxIndex(paths, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "max_files must be a non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_bad_subnet_type", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`123`),
			json.RawMessage(`"add"`),
		})
		if code != -8 || !strings.Contains(msg, "bad subnet") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_blockhash_not_hex", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "blockhash must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_bad_txid_type", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_bad_txid_type", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_bad_txid_type", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "bad txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_no_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_bad_verbose_type", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('4') + `"`),
			json.RawMessage(`"yes"`),
		})
		if code != -8 || !strings.Contains(msg, "bad verbose flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_outputs_not_array", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, []json.RawMessage{json.RawMessage(`"not-array"`)})
		if code != -8 || !strings.Contains(msg, "outputs must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_invalid_txid_in_outputs", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, []json.RawMessage{
			json.RawMessage(`[{"txid":"bad","vout":0}]`),
		})
		if code != -8 || !strings.Contains(msg, "invalid txid in outputs[0]") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_bad_command_type", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{
			json.RawMessage(`"127.0.0.1:22556"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad command") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_node_too_long", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{
			json.RawMessage(`"` + strings.Repeat("a", 257) + `"`),
			json.RawMessage(`"add"`),
		})
		if code != -8 || !strings.Contains(msg, "node address is invalid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_bad_command_type", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"127.0.0.1"`),
			json.RawMessage(`123`),
		})
		if code != -8 || !strings.Contains(msg, "bad command") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_blockhash_wrong_length", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_bad_timeout_type", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be an integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_block_not_stored", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, txIx, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -5 || !strings.Contains(msg, "Block not found") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_block_not_stored", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, txIx, nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -5 || !strings.Contains(msg, "Block not found") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_range_wrong_length", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(00)"`),
			json.RawMessage(`[0]`),
		})
		if code != -8 || !strings.Contains(msg, "range must be [begin,end]") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_start_invalid_scan_object", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		scanObjs, _ := json.Marshal([]string{""})
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -8 || !strings.Contains(msg, "invalid scan object at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_start_invalid_scan_object", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]string{""})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -8 || !strings.Contains(msg, "invalid scan object at index 0") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("validateaddress_address_required", func(t *testing.T) {
		_, code, msg := execValidateAddress("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "address required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_block_not_in_chain", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		txid := repeatHex('a')
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{
			json.RawMessage(`["` + txid + `"]`),
			json.RawMessage(`"` + repeatHex('f') + `"`),
		})
		if code != -5 || !strings.Contains(msg, "block not found") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_bad_address_type", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_no_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_no_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_two_args_only", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('8') + `"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_not_available", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, []json.RawMessage{json.RawMessage(`0`)})
		if code != -1 || !strings.Contains(msg, "not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_negative_height", func(t *testing.T) {
		j := &memJournal{tip: 2, best: "b", gen: "g", count: 3, hdrs: [][]byte{make([]byte, 80), make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "negative or non-integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_bad_unlock_type", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{json.RawMessage(`"yes"`)})
		if code != -8 || !strings.Contains(msg, "unlock") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_invalid_transaction_object", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`["not-an-object"]`),
		})
		if code != -8 || !strings.Contains(msg, "expected object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setnetworkactive_missing_state", func(t *testing.T) {
		_, code, msg := execSetNetworkActive(nil, nil)
		if code != -8 || !strings.Contains(msg, "state required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnodeaddresses_bad_count_type", func(t *testing.T) {
		paths := &DataPaths{NodeAddresses: func(int, string) []map[string]interface{} { return nil }}
		_, code, msg := execGetNodeAddresses(paths, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "bad argument") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnodeaddresses_negative_count", func(t *testing.T) {
		paths := &DataPaths{NodeAddresses: func(int, string) []map[string]interface{} { return nil }}
		_, code, msg := execGetNodeAddresses(paths, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "non-negative") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_no_pubkey", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_invalid_pubkey_hex", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`"zz"`)})
		if code != -5 || !strings.Contains(msg, "Pubkey must be a hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_missing_command", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`)})
		if code != -8 || !strings.Contains(msg, "node and command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_checklevel_non_integer", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3.5`),
		})
		if code != -8 || !strings.Contains(msg, "checklevel must be integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_negative_timeout", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, []json.RawMessage{json.RawMessage(`-1`)})
		if code != -8 || !strings.Contains(msg, "timeout must be non-negative") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_negative_timeout", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be non-negative") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"abcd"`),
			json.RawMessage(`0`),
			json.RawMessage(`1000`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_empty_hex_decode", func(t *testing.T) {
		_, code, msg := execSendRawTransaction(mempool.New(10), nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)}, nil, false, chain.RebootTestnet)
		if code != -22 || msg != "TX decode failed" {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_empty_tx_decode", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "TX decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_bad_absolute_flag", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"127.0.0.1"`),
			json.RawMessage(`"add"`),
			json.RawMessage(`3600`),
			json.RawMessage(`"yes"`),
		})
		if code != -8 || !strings.Contains(msg, "bad absolute flag") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_bantime_not_integer", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, []json.RawMessage{
			json.RawMessage(`"127.0.0.1"`),
			json.RawMessage(`"add"`),
			json.RawMessage(`3.5`),
		})
		if code != -8 || !strings.Contains(msg, "bantime must be an integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setban_no_args", func(t *testing.T) {
		_, code, msg := execSetBan(&DataPaths{BanManager: NewMemoryBanManager()}, nil)
		if code != -8 || !strings.Contains(msg, "subnet and command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_no_utxo", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -1 || !strings.Contains(msg, "UTXO cache not available") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_utxo_not_synced", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		utxo := store.NewUtxoCache()
		utxo.SetTipHeightForTest(99)
		scanObjs, _ := json.Marshal([]string{`raw(51)`})
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, utxo, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -1 || !strings.Contains(msg, "not synced to chainActive tip") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmaxconnections_missing_count", func(t *testing.T) {
		_, code, msg := execSetMaxConnections(nil, nil)
		if code != -8 || !strings.Contains(msg, "newconnectioncount required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_negative_timeout", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "timeout must be non-negative") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifychain_verbose_not_boolean", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execVerifyChain("testnet", j, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`3`),
			json.RawMessage(`6`),
			json.RawMessage(`"yes"`),
		})
		if code != -8 || !strings.Contains(msg, "verbose must be boolean") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_vout_negative", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_short_txid", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, mempool.New(10), []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_no_params", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_empty_wif", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid private key encoding") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_short_txid", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[{"txid":"abcd","vout":0}]`),
		})
		if code != -8 || !strings.Contains(msg, "hex txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_fee_delta_not_integer", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('9') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`3.5`),
		})
		if code != -8 || !strings.Contains(msg, "fee_delta") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setnetworkactive_too_many_args", func(t *testing.T) {
		_, code, msg := execSetNetworkActive(nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -8 || !strings.Contains(msg, "state required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_non_integer_height", func(t *testing.T) {
		j := &memJournal{tip: 2, best: "b", gen: "g", count: 3, hdrs: [][]byte{make([]byte, 80), make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{json.RawMessage(`3.5`)})
		if code != -8 || !strings.Contains(msg, "non-integer block height") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_bad_request_not_object", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, mempool.New(10), nil, nil, nil, "testnet", 0, []json.RawMessage{json.RawMessage(`123`)})
		if code != -8 || !strings.Contains(msg, "JSON object or null expected") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_nblocks_non_integer", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`3.5`),
		})
		if code != -8 || !strings.Contains(msg, "nblocks must be a non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_non_hex_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "txid must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_non_hex_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "txid must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_non_hex_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('g') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`1000`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_vout_non_integer", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`3.5`),
		})
		if code != -8 || !strings.Contains(msg, "non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_no_params", func(t *testing.T) {
		_, code, msg := execGetRawTransaction(nil, nil, nil, nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_no_params", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_no_params", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_no_params", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_no_params", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_no_params", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_no_params", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, nil)
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_no_params", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{})
		if code != -8 || !strings.Contains(msg, "blockhash required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_short_txid", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetTxOutProof(ix, raw, j, []json.RawMessage{json.RawMessage(`["abcd"]`)})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_no_params", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, mempool.New(10), nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dogego_importbip38_one_param", func(t *testing.T) {
		paths := &DataPaths{WalletImportBIP38: func(string, string) (string, error) { return "", nil }}
		_, code, msg := execDogegoImportBIP38("testnet", paths, nil, nil, []json.RawMessage{json.RawMessage(`"enc"`)})
		if code != -8 || !strings.Contains(msg, "encrypted key and passphrase required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatefee_bad_nblocks_type", func(t *testing.T) {
		_, code, msg := execEstimateFee(nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "invalid nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("estimatepriority_bad_nblocks_type", func(t *testing.T) {
		_, code, msg := execEstimatePriority(nil, []json.RawMessage{json.RawMessage(`"bad"`)})
		if code != -8 || !strings.Contains(msg, "invalid nblocks") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_zero_nblocks", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"` + addr + `"`),
		})
		if code != -8 || !strings.Contains(msg, "positive integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_short_txid", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetTransactionWallet("testnet", paths, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_hd_range_not_supported", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		scanObjs, _ := json.Marshal([]map[string]interface{}{{"desc": "raw(51)", "range": []int{0, 1}}})
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -8 || !strings.Contains(msg, "HD range descriptors are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_no_params", func(t *testing.T) {
		_, code, msg := execEncryptWallet(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_one_param", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{json.RawMessage(`"secret"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_two_params", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`"addr"`),
			json.RawMessage(`"sig"`),
		})
		if code != -8 || !strings.Contains(msg, "message required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_no_params", func(t *testing.T) {
		j := &memJournal{tip: 2, best: "b", gen: "g", count: 3, hdrs: [][]byte{make([]byte, 80), make([]byte, 80), make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_no_params", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_no_params", func(t *testing.T) {
		_, code, msg := execGetTxSpendingPrevout(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_no_params", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"` + addr + `"`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_one_param", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_no_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_no_params", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_no_params", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_one_param", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, []json.RawMessage{json.RawMessage(`"avoid_reuse"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_short_blockhash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_non_hex_txid", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, mempool.New(10), []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execBumpFee("testnet", p, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)}, nil, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_non_hex_txid", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetTransactionWallet("testnet", paths, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "invalid txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_empty_descriptor", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Unsupported descriptor") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxoutproof_no_params", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetTxOutProof(ix, raw, j, nil)
		if code != -8 || !strings.Contains(msg, "txids array required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_too_many_args", func(t *testing.T) {
		_, code, msg := execGetNewAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"legacy"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_too_many_args", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`9999999`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_too_many_args", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getwalletinfo_too_many_args", func(t *testing.T) {
		_, code, msg := execGetWalletInfo(nil, nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_one_param", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{json.RawMessage(`"nAddr"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_no_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccount(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_no_params", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_no_params", func(t *testing.T) {
		_, code, msg := execGetAccountAddress(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_no_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_no_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabel(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_one_param", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{json.RawMessage(`"addr"`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_no_params", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_hd_range_not_supported", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		scanObjs, _ := json.Marshal([]map[string]interface{}{{"desc": "raw(51)", "range": []int{0, 1}}})
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, []json.RawMessage{
			json.RawMessage(`"start"`),
			scanObjs,
		})
		if code != -8 || !strings.Contains(msg, "HD range descriptors are not supported") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_one_param", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + addr + `"`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_no_params", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_no_params", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execGetTransactionWallet("testnet", paths, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_empty_descriptor", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "unsupported descriptor") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_empty_address", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_empty_pubkey", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Pubkey must be a hex string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_non_hex_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "txid must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_non_hex_txid", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`false`),
			json.RawMessage(`[{"txid":"` + repeatHex('g') + `","vout":0}]`),
		})
		if code != -8 || !strings.Contains(msg, "hex txid") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_no_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_short_txid", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"abcd"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_no_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execScanTxOutSet("testnet", nil, j, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_no_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		txIx, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, raw, txIx, filters, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_too_many_args", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"01000000000000000000000000000000000000000000000000000000000000000000000000ffffffff"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_no_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, nil)
		if code != -8 || !strings.Contains(msg, "fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_short_hash", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_short_hash", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_short_hash", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_no_params", func(t *testing.T) {
		_, code, msg := execMove(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_one_param", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{json.RawMessage(`[]`)})
		if code != -8 || !strings.Contains(msg, "inputs and outputs required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_no_params", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", nil)
		if code != -8 || !strings.Contains(msg, "message required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_bad_vout_type", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`"bad"`),
		})
		if code != -8 || !strings.Contains(msg, "bad vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxout_one_param", func(t *testing.T) {
		_, code, msg := execGetTxOut(nil, nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
		})
		if code != -8 || !strings.Contains(msg, "txid and vout") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_two_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_no_params", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_no_params", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_no_params", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_no_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_no_params", func(t *testing.T) {
		_, code, msg := execSignRawTransaction("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockheader_short_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockHeaderGolden(j, nil, nil, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64-char block hash hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("invalidateblock_non_hex_hash", func(t *testing.T) {
		_, code, msg := execInvalidateBlock(nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("reconsiderblock_non_hex_hash", func(t *testing.T) {
		_, code, msg := execReconsiderBlock(nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("preciousblock_non_hex_hash", func(t *testing.T) {
		_, code, msg := execPreciousBlock(nil, nil, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_one_param", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{json.RawMessage(`""`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_two_params", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_outputs_not_object", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "outputs must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createrawtransaction_bad_inputs_type", func(t *testing.T) {
		_, code, msg := execCreateRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`{}`),
		})
		if code != -8 || !strings.Contains(msg, "inputs must be a JSON array") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_no_params", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithWallet("testnet", nil, nil)
		if code != -8 || !strings.Contains(msg, "hex string required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_short_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_non_hex_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "must be hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_one_param", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_too_many_args", func(t *testing.T) {
		p, _ := chain.ParamsFor(chain.RebootTestnet)
		addr, _ := chain.RandomP2PKHAddress(p)
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`"` + addr + `"`),
			json.RawMessage(`1000`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalances_too_many_args", func(t *testing.T) {
		_, code, msg := execGetBalances("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_no_params", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_no_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execBumpFee("testnet", p, nil, nil, nil, nil, nil, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_no_params", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "DAddr" }}
		_, code, msg := execAbandonTransactionWallet("testnet", paths, nil, nil, mempool.New(10), nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getzmqnotifications_too_many_args", func(t *testing.T) {
		_, code, msg := execGetZMQNotifications(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_one_param", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{json.RawMessage(`"` + repeatHex('c') + `"`)})
		if code != -8 || !strings.Contains(msg, "fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_too_many_args", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetMempoolEntry(p, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('4') + `"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawtransaction_short_blockhash", func(t *testing.T) {
		dir := t.TempDir()
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetRawTransaction(ix, raw, j, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`false`),
			json.RawMessage(`"abcd"`),
		})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_short_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilter(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_short_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`"abcd"`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_no_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilter(j, raw, ix, filters, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_no_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, filters, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_no_params", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_one_param", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`[]`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_no_params", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_too_many_args", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_too_many_args", func(t *testing.T) {
		_, code, msg := execEncryptWallet([]json.RawMessage{
			json.RawMessage(`"secret"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_bad_options_type", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"01000000000000000000000000000000000000000000000000000000000000000000000000ffffffff"`),
			json.RawMessage(`[]`),
		})
		if code != -8 || !strings.Contains(msg, "options must be a JSON object") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransaction_empty_hex", func(t *testing.T) {
		_, code, msg := execSignRawTransaction("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "invalid transaction hex") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_empty_string", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_empty_address", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_empty_address", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_one_param", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{json.RawMessage(`"addr"`)})
		if code != -8 || !strings.Contains(msg, "message required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessage_empty_address", func(t *testing.T) {
		_, code, msg := execSignMessage("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"hello"`),
		})
		if code != -3 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_too_many_args", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_too_many_args", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_too_many_args", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execGetTxSpendingPrevout(p, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_empty_address", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_empty_address", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_empty_address", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"label"`),
		})
		if code != -5 || !strings.Contains(msg, "Invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_no_params", func(t *testing.T) {
		_, code, msg := execRemovePrunedFunds(nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scantxoutset_empty_action", func(t *testing.T) {
		_, code, msg := execScanTxOutSet("testnet", nil, nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "unknown action") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("scanblocks_empty_action", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execScanBlocks("testnet", j, nil, nil, filters, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "unknown action") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("verifymessage_empty_address", func(t *testing.T) {
		_, code, msg := execVerifyMessage("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"sig"`),
			json.RawMessage(`"hello"`),
		})
		if code != -8 || !strings.Contains(msg, "invalid address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_too_many_args", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"addr(A)"`),
			json.RawMessage(`null`),
			json.RawMessage(`null`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createpsbt_one_param", func(t *testing.T) {
		_, code, msg := execCreatePsbt("testnet", nil, nil, nil, []json.RawMessage{json.RawMessage(`[]`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_no_params", func(t *testing.T) {
		_, code, msg := execImportMulti(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_too_many_args", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_three_args", func(t *testing.T) {
		_, _, code, msg := execListStuckTransactionsValidate([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_four_params", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(p, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
		})
		if code != -8 || !strings.Contains(msg, "fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_empty_txid", func(t *testing.T) {
		p := mempool.New(10)
		_, code, msg := execMempoolExists(p, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "64 hex characters") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_empty_hex", func(t *testing.T) {
		utxo := store.NewUtxoCache()
		paths := &DataPaths{Utxo: utxo}
		_, code, msg := execFundRawTransaction("testnet", paths, nil, nil, nil, []json.RawMessage{json.RawMessage(`""`)})
		if code != -8 || !strings.Contains(msg, "TX decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_too_many_args", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"n"`),
			json.RawMessage(`1.0`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_too_many_args", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_too_many_args", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`2`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_non_hex_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilter(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "invalid block hash") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_non_hex_hash", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`"` + repeatHex('g') + `"`)})
		if code != -8 || !strings.Contains(msg, "invalid block hash") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_negative_amount", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`"-1"`),
		})
		if code != -3 || !strings.Contains(msg, "Invalid amount") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_empty_wif", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{
			json.RawMessage(`"hello"`),
			json.RawMessage(`""`),
		})
		if code != -8 || !strings.Contains(msg, "invalid private key") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_no_params", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_too_many_args", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_no_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_too_many_args", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_too_many_args", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`"cHNidP8BAHEC"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_too_many_args", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_too_many_args", func(t *testing.T) {
		_, code, msg := execKeypoolRefill([]json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_too_many_args", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{
			json.RawMessage(`"backup.dat"`),
			json.RawMessage(`"extra.dat"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_too_many_args", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"wallet.txt"`),
			json.RawMessage(`"extra.txt"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_one_param", func(t *testing.T) {
		_, code, msg := execWalletLock([]json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_no_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_too_many_args", func(t *testing.T) {
		_, code, msg := execGetAccountAddress([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createmultisig_one_param", func(t *testing.T) {
		_, code, msg := execCreateMultisig("testnet", []json.RawMessage{json.RawMessage(`2`)})
		if code != -8 || !strings.Contains(msg, "keys array required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_empty_address", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{json.RawMessage(`""`)})
		if code != -5 || !strings.Contains(msg, "Invalid Dogecoin address") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilter(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, filters, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_nblocks_negative", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`-1`),
		})
		if code != -8 || !strings.Contains(msg, "non-negative integer") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_two_params", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"n"`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_invalid_star", func(t *testing.T) {
		_, code, msg := execGetAccountAddress([]json.RawMessage{json.RawMessage(`"*"`)})
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblock_blockhash_not_string", func(t *testing.T) {
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlock(j, raw, nil, "testnet", nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockheader_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockHeaderGolden(j, raw, nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "unsupported param type") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdeploymentinfo_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetDeploymentInfo(j, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getchaintxstats_blockhash_not_string", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetChainTxStats(j, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`true`)})
		if code != -8 || !strings.Contains(msg, "blockhash must be a string") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getunconfirmedbalance_one_param", func(t *testing.T) {
		_, code, msg := execGetUnconfirmedBalance("testnet", nil, nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_one_param", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_eight_params", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolentry_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetMempoolEntry(pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "txid required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempoolancestors_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetMempoolAncestors(pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "txid (and optional verbose)") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmempooldescendants_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetMempoolDescendants(pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "txid (and optional verbose)") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_one_param", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{json.RawMessage(`"127.0.0.1:22556"`)})
		if code != -8 || !strings.Contains(msg, "node and command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_three_params", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"010203"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_two_params", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_two_params", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_three_params", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_one_param", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{json.RawMessage(`2`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_three_params", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_three_params", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_three_params", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_three_params", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"010203"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_three_params", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_odd_hex", func(t *testing.T) {
		_, code, msg := execSubmitBlock(nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`"0"`)})
		if code != -8 || !strings.Contains(msg, "Block decode failed") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_six_params", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_six_params", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_three_params", func(t *testing.T) {
		_, code, msg := execListAccounts([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_too_many_args", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`"l"`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_three_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{
			json.RawMessage(`"a"`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_zero_params", func(t *testing.T) {
		_, code, msg := execWalletPassphrase(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_one_param", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithWallet("testnet", nil, []json.RawMessage{json.RawMessage(`"010203"`)})
		if code != -1 || !strings.Contains(msg, "wallet is not implemented") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_invalid_star_account", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`"*"`)},
		)
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_seven_params", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_two_params", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_three_params", func(t *testing.T) {
		_, code, msg := execGetNewAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"legacy"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_two_params", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_two_params", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_invalid_star_account", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"*"`),
			json.RawMessage(`{}`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_invalid_star_account", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"*"`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
		}, nil, false, chain.RebootTestnet)
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_five_params", func(t *testing.T) {
		_, code, msg := execGenerateToAddress(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_three_params", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_two_params", func(t *testing.T) {
		_, code, msg := execAbandonTransaction(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_six_params", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`9999999`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_three_params", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaddressgroupings_one_param", func(t *testing.T) {
		_, code, msg := execListAddressGroupings([]json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_one_param", func(t *testing.T) {
		_, code, msg := execListLockUnspent([]json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalances_one_param", func(t *testing.T) {
		_, code, msg := execGetBalances("testnet", nil, nil, nil, nil, nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getwalletinfo_one_param", func(t *testing.T) {
		_, code, msg := execGetWalletInfo(nil, nil, nil, nil, nil, "testnet", []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_zero_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange(nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChange([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execImportMempool(pool, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`"/tmp/mempool.json"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddednodeinfo_two_params", func(t *testing.T) {
		_, code, msg := execGetAddedNodeInfo(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "too many arguments") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addnode_three_params", func(t *testing.T) {
		_, code, msg := execAddNode(nil, []json.RawMessage{
			json.RawMessage(`"127.0.0.1:22556"`),
			json.RawMessage(`"add"`),
			json.RawMessage(`true`),
		})
		if code != -8 || !strings.Contains(msg, "node and command required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_three_params", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_two_params", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByLabel([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("prioritisetransaction_five_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPrioritiseTransaction(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
			json.RawMessage(`0`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -8 || !strings.Contains(msg, "txid, priority_delta, and fee_delta required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_three_params", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_too_many_args", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{
			json.RawMessage(`0.001`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_two_params", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"wallet.txt"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_two_params", func(t *testing.T) {
		_, code, msg := execRemovePrunedFunds(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_three_params", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`[]`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_four_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_four_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`"l"`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByAccount([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByAddress([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_five_params", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_six_params", func(t *testing.T) {
		_, code, msg := execWalletProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_seven_params", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`2`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_three_params", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`10`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_invalid_star_toaccount", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"*"`),
			json.RawMessage(`1`),
		})
		if code != -8 || !strings.Contains(msg, "Invalid account name") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signmessagewithprivkey_one_param", func(t *testing.T) {
		_, code, msg := execSignMessageWithPrivkey("testnet", []json.RawMessage{json.RawMessage(`"hello"`)})
		if code != -8 || !strings.Contains(msg, "message and private key required") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_five_params", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"02"` + repeatHex('a')),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`null`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_four_params", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_three_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_two_params", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_four_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccount([]json.RawMessage{
			json.RawMessage(`"a"`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		filters := &store.BlockFilterIndex{}
		_, code, msg := execGetBlockFilter(j, raw, ix, filters, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_four_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_two_params", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbylabel_wallet_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaddress_wallet_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByAddressWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listreceivedbyaccount_wallet_four_params", func(t *testing.T) {
		_, code, msg := execListReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_four_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"l"`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactionswallet_five_params", func(t *testing.T) {
		_, code, msg := execListTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_three_params", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_two_params", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`10`),
			json.RawMessage(`0`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_three_params", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_three_params", func(t *testing.T) {
		_, code, msg := execImportDescriptors("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_wallet_two_params", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{
			json.RawMessage(`"receive"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_wallet_three_params", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"l"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_wallet_two_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"l"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_wallet_two_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefillWallet(nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_wallet_one_param", func(t *testing.T) {
		_, code, msg := execListLockUnspentWallet(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_three_params", func(t *testing.T) {
		_, code, msg := execLockUnspentWallet(nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`[]`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblockheight_four_params", func(t *testing.T) {
		_, code, msg := execWaitForBlockHeight(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`10`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getauxblock_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_two_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitAuxBlock(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execMempoolExists(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_two_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		}, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPsbtBumpFee("testnet", nil, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprunedfunds_three_params", func(t *testing.T) {
		_, code, msg := execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmocktime_two_params", func(t *testing.T) {
		_, code, msg := execSetMockTime([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`2`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getmocktime_one_param", func(t *testing.T) {
		_, code, msg := execGetMockTime([]json.RawMessage{json.RawMessage(`1`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_one_param", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execResendWalletTransactions(pool, []json.RawMessage{json.RawMessage(`1`)}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_wallet_four_params", func(t *testing.T) {
		_, code, msg := execAddMultisigAddressWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_three_params", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaccount_wallet_three_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAccountWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_four_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_three_params", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_four_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSubmitPackage(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_wallet_three_params", func(t *testing.T) {
		_, code, msg := execListStuckTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_seven_params", func(t *testing.T) {
		_, code, msg := execWalletProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_six_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_seven_params", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_three_params", func(t *testing.T) {
		_, code, msg := execSimulateRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalances_two_params", func(t *testing.T) {
		_, code, msg := execGetBalances("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getwalletinfo_two_params", func(t *testing.T) {
		_, code, msg := execGetWalletInfo(nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getunconfirmedbalance_two_params", func(t *testing.T) {
		_, code, msg := execGetUnconfirmedBalance("testnet", nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listdescriptors_two_params", func(t *testing.T) {
		_, code, msg := execListDescriptors("testnet", nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaddressgroupings_wallet_two_params", func(t *testing.T) {
		_, code, msg := execListAddressGroupingsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getzmqnotifications_two_params", func(t *testing.T) {
		_, code, msg := execGetZMQNotifications(nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_two_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_two_params", func(t *testing.T) {
		_, code, msg := execRescan([]json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wallet_three_params", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_wallet_two_params", func(t *testing.T) {
		_, code, msg := execResendWalletTransactionsWallet("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("descriptorprocesspsbt_six_params", func(t *testing.T) {
		_, code, msg := execDescriptorProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_four_params", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_two_params", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_one_param", func(t *testing.T) {
		_, code, msg := execWalletLock([]json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_two_params", func(t *testing.T) {
		_, code, msg := execGetAccountAddress([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_two_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccount([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_two_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabel([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_two_params", func(t *testing.T) {
		_, code, msg := execEncryptWallet([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_five_params", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"5"` + repeatHex('b')),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_four_params", func(t *testing.T) {
		_, code, msg := execGetNewAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"legacy"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_two_params", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`2`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_two_params", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_two_params", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"wallet.dump"`),
			json.RawMessage(`"extra.dump"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_two_params", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithkey_five_params", func(t *testing.T) {
		_, code, msg := execSignRawTransactionWithKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`"0100000000"`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_four_params", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"0100000000"`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_wallet_two_params", func(t *testing.T) {
		_, code, msg := execGetAccountAddressWallet(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbyaccount_wallet_two_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByAccountWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`60`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_two_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefill([]json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_wallet_two_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefillWallet(nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_three_params", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, []json.RawMessage{
			json.RawMessage(`"avoid_reuse"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_four_params", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_four_params", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletlock_paths_one_param", func(t *testing.T) {
		_, code, msg := execWalletLockPaths(nil, []json.RawMessage{json.RawMessage(`true`)})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_two_params", func(t *testing.T) {
		_, code, msg := execEncryptWalletPaths(nil, []json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_three_params", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_four_params", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_four_params", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_three_params", func(t *testing.T) {
		_, code, msg := execListStuckTransactions([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_two_params", func(t *testing.T) {
		_, code, msg := execListLockUnspent([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_four_params", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wallet_four_params", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphrasePaths(nil, []json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`60`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_paths_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangePaths(nil, []json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_builtin_two_params", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_unencrypted_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseUnencrypted([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`60`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrasechange_unencrypted_three_params", func(t *testing.T) {
		_, code, msg := execWalletPassphraseChangeUnencrypted([]json.RawMessage{
			json.RawMessage(`"old"`),
			json.RawMessage(`"new"`),
			json.RawMessage(`"extra"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_three_params", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_three_params", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_three_params", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_four_params", func(t *testing.T) {
		_, code, msg := execListAccounts([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_wallet_four_params", func(t *testing.T) {
		_, code, msg := execListAccountsWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetTxSpendingPrevout(pool, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitblock_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitBlock(j, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_four_params", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_four_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPsbtBumpFee("testnet", nil, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_four_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_eight_params", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_seven_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_eight_params", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_three_params", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execResendWalletTransactions(pool, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprunedfunds_four_params", func(t *testing.T) {
		_, code, msg := execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_three_params", func(t *testing.T) {
		_, code, msg := execRemovePrunedFunds(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_two_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaddressgroupings_two_params", func(t *testing.T) {
		_, code, msg := execListAddressGroupings([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_wallet_three_params", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{
			json.RawMessage(`"receive"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_wallet_three_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_wallet_two_params", func(t *testing.T) {
		_, code, msg := execGetAccountWallet(nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_five_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_wallet_five_params", func(t *testing.T) {
		_, code, msg := execAddMultisigAddressWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_wallet_two_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execAbandonTransactionWallet("testnet", nil, nil, nil, pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('f') + `"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_wallet_four_params", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"label"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_three_params", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_seven_params", func(t *testing.T) {
		_, code, msg := execMoveWallet([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_wallet_four_params", func(t *testing.T) {
		_, code, msg := execLockUnspentWallet(nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_four_params", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_five_params", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_five_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSubmitPackage(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importdescriptors_four_params", func(t *testing.T) {
		_, code, msg := execImportDescriptors("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_four_params", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_five_params", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_four_params", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_four_params", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_seven_params", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_nine_params", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_two_params", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{
			json.RawMessage(`"getblock"`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, ix, &store.BlockFilterIndex{}, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_five_params", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_eight_params", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_four_params", func(t *testing.T) {
		_, code, msg := execWalletPassphrase([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`60`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_four_params", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_five_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importpubkey_six_params", func(t *testing.T) {
		_, code, msg := execImportPubKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"02"` + repeatHex('a')),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_three_params", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_four_params", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getnewaddress_five_params", func(t *testing.T) {
		_, code, msg := execGetNewAddress("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`"legacy"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setaccount_four_params", func(t *testing.T) {
		_, code, msg := execSetAccount("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_six_params", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"5"`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_five_params", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("simulaterawtransaction_four_params", func(t *testing.T) {
		_, code, msg := execSimulateRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_eight_params", func(t *testing.T) {
		_, code, msg := execWalletProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("descriptorprocesspsbt_eight_params", func(t *testing.T) {
		_, code, msg := execDescriptorProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getauxblock_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitAuxBlock(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_five_params", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_five_params", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_wallet_five_params", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execAddMultisigAddressWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_eight_params", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_six_params", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_four_params", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_three_params", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetTxSpendingPrevout(pool, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_five_params", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlabels_three_params", func(t *testing.T) {
		_, code, msg := execListLabelsWallet(nil, []json.RawMessage{
			json.RawMessage(`"receive"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getdescriptorinfo_four_params", func(t *testing.T) {
		_, code, msg := execGetDescriptorInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_three_params", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_four_params", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("removeprunedfunds_four_params", func(t *testing.T) {
		_, code, msg := execRemovePrunedFunds(nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescan_four_params", func(t *testing.T) {
		_, code, msg := execRescan([]json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_four_params", func(t *testing.T) {
		_, code, msg := execListStuckTransactions([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("liststucktransactions_wallet_four_params", func(t *testing.T) {
		_, code, msg := execListStuckTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactionswallet_six_params", func(t *testing.T) {
		_, code, msg := execListTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_six_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_five_params", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_nine_params", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_nine_params", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_eight_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_four_params", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_three_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefill([]json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
			json.RawMessage(`300`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_three_params", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{
			json.RawMessage(`0.01`),
			json.RawMessage(`0.02`),
			json.RawMessage(`0.03`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listdescriptors_three_params", func(t *testing.T) {
		_, code, msg := execListDescriptors("testnet", nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_three_params", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
			json.RawMessage(`"third.dat"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_three_params", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"wallet.dump"`),
			json.RawMessage(`"extra.dump"`),
			json.RawMessage(`"third.dump"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_three_params", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"pass2"`),
			json.RawMessage(`"pass3"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_five_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_five_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPsbtBumpFee("testnet", nil, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprunedfunds_five_params", func(t *testing.T) {
		_, code, msg := execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_four_params", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_three_params", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{
			json.RawMessage(`"getblock"`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_five_params", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_six_params", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_six_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSubmitPackage(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_six_params", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_eight_params", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_five_params", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_five_params", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendfrom_ten_params", func(t *testing.T) {
		_, code, msg := execSendFrom("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_six_params", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_five_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabel([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_four_params", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_six_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendmany_nine_params", func(t *testing.T) {
		_, code, msg := execSendMany("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`{}`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_six_params", func(t *testing.T) {
		_, code, msg := execImportMulti([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setwalletflag_four_params", func(t *testing.T) {
		_, code, msg := execSetWalletFlag(nil, []json.RawMessage{
			json.RawMessage(`"avoid_reuse"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpprivkey_four_params", func(t *testing.T) {
		_, code, msg := execDumpPrivKey("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_wallet_three_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefillWallet(nil, []json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
			json.RawMessage(`300`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_three_params", func(t *testing.T) {
		_, code, msg := execListLockUnspent([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getrawchangeaddress_four_params", func(t *testing.T) {
		_, code, msg := execGetRawChangeAddress(nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmulti_wallet_five_params", func(t *testing.T) {
		_, code, msg := execImportMultiWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generatetoaddress_nine_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGenerateToAddress(j, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletcreatefundedpsbt_ten_params", func(t *testing.T) {
		_, code, msg := execWalletCreateFundedPsbt("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`{}`),
			json.RawMessage(`0`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("combinepsbt_five_params", func(t *testing.T) {
		_, code, msg := execCombinePsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockstats_six_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockStats(j, raw, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("signrawtransactionwithwallet_five_params", func(t *testing.T) {
		paths := &DataPaths{WalletAddress: func() string { return "nAddr" }}
		_, code, msg := execSignRawTransactionWithWallet("testnet", paths, []json.RawMessage{
			json.RawMessage(`"0100000000"`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_six_params", func(t *testing.T) {
		_, code, msg := execGetTransaction([]json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_seven_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlock(j, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listunspent_seven_params", func(t *testing.T) {
		_, code, msg := execListUnspent("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`9999999`),
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getbalance_six_params", func(t *testing.T) {
		_, code, msg := execGetBalance(nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_paths_three_params", func(t *testing.T) {
		_, code, msg := execEncryptWalletPaths(nil, []json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"extra"`),
			json.RawMessage(`"third"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbylabel_wallet_five_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByLabelWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("backupwallet_four_params", func(t *testing.T) {
		_, code, msg := execBackupWallet(nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
			json.RawMessage(`"third.dat"`),
			json.RawMessage(`"fourth.dat"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendtoaddress_ten_params", func(t *testing.T) {
		_, code, msg := execSendToAddress("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decodepsbt_six_params", func(t *testing.T) {
		_, code, msg := execDecodePsbt("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importwallet_five_params", func(t *testing.T) {
		_, code, msg := execImportWallet("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"wallet.dat"`),
			json.RawMessage(`"extra.dat"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addwitnessaddress_four_params", func(t *testing.T) {
		_, code, msg := execAddWitnessAddress("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listdescriptors_four_params", func(t *testing.T) {
		_, code, msg := execListDescriptors("testnet", nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("rescanwallet_five_params", func(t *testing.T) {
		_, code, msg := execRescanWallet(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("joinpsbt_seven_params", func(t *testing.T) {
		_, code, msg := execJoinPsbt([]json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
			json.RawMessage(`[]`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("extractdescriptor_five_params", func(t *testing.T) {
		_, code, msg := execExtractDescriptor([]json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("converttopsbt_seven_params", func(t *testing.T) {
		_, code, msg := execConvertToPsbt([]json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("finalizepsbt_six_params", func(t *testing.T) {
		_, code, msg := execFinalizePsbt([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitpackage_seven_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSubmitPackage(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`0`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("generate_seven_params", func(t *testing.T) {
		_, code, msg := execGenerate(nil, nil, nil, nil, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("help_four_params", func(t *testing.T) {
		_, code, msg := execHelp([]json.RawMessage{
			json.RawMessage(`"getblock"`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Too many") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitforblock_five_params", func(t *testing.T) {
		_, code, msg := execWaitForBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('b') + `"`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("bumpfee_six_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execBumpFee("testnet", pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("psbtbumpfee_six_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execPsbtBumpFee("testnet", nil, pool, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettransaction_wallet_six_params", func(t *testing.T) {
		_, code, msg := execGetTransactionWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("fundrawtransaction_seven_params", func(t *testing.T) {
		_, code, msg := execFundRawTransaction("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletprocesspsbt_nine_params", func(t *testing.T) {
		_, code, msg := execWalletProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("lockunspent_five_params", func(t *testing.T) {
		_, code, msg := execLockUnspent([]json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`[]`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("dumpwallet_four_params", func(t *testing.T) {
		_, code, msg := execDumpWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`"wallet.dump"`),
			json.RawMessage(`"extra.dump"`),
			json.RawMessage(`"third.dump"`),
			json.RawMessage(`"fourth.dump"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("encryptwallet_four_params", func(t *testing.T) {
		_, code, msg := execEncryptWalletBuiltin([]json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`"pass2"`),
			json.RawMessage(`"pass3"`),
			json.RawMessage(`"pass4"`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importaddress_nine_params", func(t *testing.T) {
		_, code, msg := execImportAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressinfo_six_params", func(t *testing.T) {
		_, code, msg := execGetAddressInfo("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprunedfunds_six_params", func(t *testing.T) {
		_, code, msg := execImportPrunedFunds("testnet", nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccount_four_params", func(t *testing.T) {
		_, code, msg := execGetAccount("testnet", []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listaccounts_five_params", func(t *testing.T) {
		_, code, msg := execListAccounts([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("truncatetoheight_four_params", func(t *testing.T) {
		_, code, msg := execTruncateToHeight(nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("pruneblockchain_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execPruneBlockchain(j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblocktemplate_four_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetBlockTemplate(j, nil, nil, nil, nil, "testnet", 0, []json.RawMessage{
			json.RawMessage(`{}`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("gettxspendingprevout_four_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execGetTxSpendingPrevout(pool, []json.RawMessage{
			json.RawMessage(`[]`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("descriptorprocesspsbt_nine_params", func(t *testing.T) {
		_, code, msg := execDescriptorProcessPsbt("testnet", nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`"ALL"`),
			json.RawMessage(`true`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listsinceblock_wallet_seven_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execListSinceBlockWallet("testnet", nil, j, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`null`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactionswallet_seven_params", func(t *testing.T) {
		_, code, msg := execListTransactionsWallet("testnet", nil, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listtransactions_seven_params", func(t *testing.T) {
		_, code, msg := execListTransactions([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("sendrawtransaction_five_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execSendRawTransaction(pool, nil, nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		}, nil, false, chain.RebootTestnet)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("settxfee_four_params", func(t *testing.T) {
		_, code, msg := execSetTxFee(nil, []json.RawMessage{
			json.RawMessage(`0.01`),
			json.RawMessage(`0.02`),
			json.RawMessage(`0.03`),
			json.RawMessage(`0.04`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("keypoolrefill_four_params", func(t *testing.T) {
		_, code, msg := execKeypoolRefill([]json.RawMessage{
			json.RawMessage(`100`),
			json.RawMessage(`200`),
			json.RawMessage(`300`),
			json.RawMessage(`400`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getreceivedbyaddress_six_params", func(t *testing.T) {
		_, code, msg := execGetReceivedByAddress("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilter_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilter(j, raw, ix, &store.BlockFilterIndex{}, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getblockfilterheader_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		dir := t.TempDir()
		raw, err := store.OpenRawBlockStore(dir)
		if err != nil {
			t.Fatal(err)
		}
		ix, err := store.OpenTxIndex(dir)
		if err != nil {
			t.Fatal(err)
		}
		_, code, msg := execGetBlockFilterHeader(j, raw, ix, nil, []json.RawMessage{
			json.RawMessage(`0`),
			json.RawMessage(`"basic"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("submitauxblock_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execSubmitAuxBlock(j, nil, nil, "testnet", []json.RawMessage{
			json.RawMessage(`"` + repeatHex('d') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getauxblock_five_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execGetAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('c') + `"`),
			json.RawMessage(`"00"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("createauxblock_three_params", func(t *testing.T) {
		j := &memJournal{tip: 0, best: "b", gen: "g", count: 1, hdrs: [][]byte{make([]byte, 80)}}
		_, code, msg := execCreateAuxBlock(j, nil, nil, nil, "testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("mempoolexists_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execMempoolExists(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('e') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setmocktime_three_params", func(t *testing.T) {
		_, code, msg := execSetMockTime([]json.RawMessage{
			json.RawMessage(`1`),
			json.RawMessage(`2`),
			json.RawMessage(`3`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("addmultisigaddress_six_params", func(t *testing.T) {
		_, code, msg := execAddMultisigAddress("testnet", []json.RawMessage{
			json.RawMessage(`2`),
			json.RawMessage(`[]`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("move_nine_params", func(t *testing.T) {
		_, code, msg := execMove([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`1`),
			json.RawMessage(`1`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("waitfornewblock_three_params", func(t *testing.T) {
		_, code, msg := execWaitForNewBlock(nil, nil, nil, []json.RawMessage{
			json.RawMessage(`10`),
			json.RawMessage(`0`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("abandontransaction_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execAbandonTransaction(pool, []json.RawMessage{
			json.RawMessage(`"` + repeatHex('a') + `"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("resendwallettransactions_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execResendWalletTransactions(pool, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		}, nil)
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importmempool_three_params", func(t *testing.T) {
		pool := mempool.New(10)
		_, code, msg := execImportMempool(pool, nil, nil, nil, nil, chain.RebootTestnet, []json.RawMessage{
			json.RawMessage(`"/tmp/mempool.json"`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("listlockunspent_wallet_two_params", func(t *testing.T) {
		_, code, msg := execListLockUnspentWallet(nil, []json.RawMessage{
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("walletpassphrase_paths_four_params", func(t *testing.T) {
		_, code, msg := execWalletPassphrasePaths(nil, []json.RawMessage{
			json.RawMessage(`"pass"`),
			json.RawMessage(`60`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaddressesbylabel_four_params", func(t *testing.T) {
		_, code, msg := execGetAddressesByLabel([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("setlabel_four_params", func(t *testing.T) {
		_, code, msg := execSetLabelWallet("testnet", nil, []json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
			json.RawMessage(`""`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("getaccountaddress_three_params", func(t *testing.T) {
		_, code, msg := execGetAccountAddress([]json.RawMessage{
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("importprivkey_seven_params", func(t *testing.T) {
		_, code, msg := execImportPrivKey("testnet", nil, nil, nil, []json.RawMessage{
			json.RawMessage(`"5"`),
			json.RawMessage(`""`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`1`),
			json.RawMessage(`false`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("decoderawtransaction_five_params", func(t *testing.T) {
		_, code, msg := execDecodeRawTransaction("testnet", []json.RawMessage{
			json.RawMessage(`"00"`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
	t.Run("deriveaddresses_five_params", func(t *testing.T) {
		_, code, msg := execDeriveAddresses("testnet", []json.RawMessage{
			json.RawMessage(`"pkh(...)"`),
			json.RawMessage(`null`),
			json.RawMessage(`true`),
			json.RawMessage(`false`),
			json.RawMessage(`true`),
		})
		if code != -32602 || !strings.Contains(msg, "Wrong number") {
			t.Fatalf("code=%d msg=%q", code, msg)
		}
	})
}

// execGetBlockHashGolden mirrors dispatch getblockhash validation for golden error tests.
func execGetBlockHashGolden(j HeaderJournal, params []json.RawMessage) (interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "getblockhash: height required"
	}
	var h float64
	if err := json.Unmarshal(params[0], &h); err != nil || h < 0 || h != float64(int64(h)) {
		return nil, -8, "getblockhash: invalid height"
	}
	buf, err := j.ReadHeaderAt(int64(h))
	if err != nil {
		return nil, -8, err.Error()
	}
	return pow.BlockHashHex(buf), 0, ""
}

// execGetBlockHeaderGolden mirrors dispatch getblockheader verbose parsing for golden error tests.
func execGetBlockHeaderGolden(j HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	verbose := true
	if len(params) > 1 {
		if err := json.Unmarshal(params[1], &verbose); err != nil {
			return nil, -8, "getblockheader: bad verbose flag"
		}
	}
	_, _, err := resolveGetBlockHeader(j, raw, params, paths)
	if err != nil {
		return nil, -8, err.Error()
	}
	if !verbose {
		return "", 0, ""
	}
	return map[string]interface{}{"ok": true}, 0, ""
}
