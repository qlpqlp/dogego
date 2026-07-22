// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"sync"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wallet"
	"dogego/wire"
)

// walletHeaderMetaCache avoids repeated journal reads when listing many solo-miner UTXOs.
var walletHeaderMetaCache struct {
	mu   sync.RWMutex
	tip  int64
	hash map[int64]string
	time map[int64]int64
}

func walletHeaderCacheTip(j HeaderJournal) int64 {
	if j == nil {
		return -1
	}
	tip, err := j.TipHeight()
	if err != nil {
		return -1
	}
	return tip
}

func walletHeaderTimeCached(j HeaderJournal, height int64) int64 {
	if j == nil || height < 0 {
		return 0
	}
	tip := walletHeaderCacheTip(j)
	walletHeaderMetaCache.mu.RLock()
	if walletHeaderMetaCache.time != nil && tip == walletHeaderMetaCache.tip {
		if t, ok := walletHeaderMetaCache.time[height]; ok {
			walletHeaderMetaCache.mu.RUnlock()
			return t
		}
	}
	walletHeaderMetaCache.mu.RUnlock()
	t := walletHeaderTimeAt(j, height)
	if t == 0 {
		return 0
	}
	walletHeaderMetaCache.mu.Lock()
	if walletHeaderMetaCache.time == nil || tip != walletHeaderMetaCache.tip {
		walletHeaderMetaCache.tip = tip
		walletHeaderMetaCache.hash = make(map[int64]string)
		walletHeaderMetaCache.time = make(map[int64]int64)
	}
	walletHeaderMetaCache.time[height] = t
	walletHeaderMetaCache.mu.Unlock()
	return t
}

func walletBlockHashCached(j HeaderJournal, height int64) string {
	if j == nil || height < 0 {
		return ""
	}
	tip := walletHeaderCacheTip(j)
	walletHeaderMetaCache.mu.RLock()
	if walletHeaderMetaCache.hash != nil && tip == walletHeaderMetaCache.tip {
		if h, ok := walletHeaderMetaCache.hash[height]; ok {
			walletHeaderMetaCache.mu.RUnlock()
			return h
		}
	}
	walletHeaderMetaCache.mu.RUnlock()
	h := walletBlockHashAt(j, height)
	if h == "" {
		return ""
	}
	walletHeaderMetaCache.mu.Lock()
	if walletHeaderMetaCache.hash == nil || tip != walletHeaderMetaCache.tip {
		walletHeaderMetaCache.tip = tip
		walletHeaderMetaCache.hash = make(map[int64]string)
		walletHeaderMetaCache.time = make(map[int64]int64)
	}
	walletHeaderMetaCache.hash[height] = h
	walletHeaderMetaCache.mu.Unlock()
	return h
}

// walletHeaderTimeAt returns the header nTime for height (0 if unavailable).
func walletHeaderTimeAt(j HeaderJournal, height int64) int64 {
	if j == nil || height < 0 {
		return 0
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return 0
	}
	return int64(binary.LittleEndian.Uint32(h80[68:72]))
}

// walletBIP125Replaceable returns Core listtransactions/gettransaction bip125-replaceable ("yes"|"no"|"unknown").
func walletBIP125Replaceable(pool *mempool.Pool, txid string, blockHeight int64) string {
	if blockHeight >= 0 {
		return "no"
	}
	if pool == nil || txid == "" {
		return "unknown"
	}
	raw, err := pool.GetRawByTxID(txid)
	if err != nil {
		return "unknown"
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "unknown"
	}
	if wire.IsBIP125Replaceable(tx) {
		return "yes"
	}
	return "no"
}

// walletLookupTxHex returns serialized tx hex (mempool, wallet cache, tx index, or scan at block height).
func walletLookupTxHex(pool *mempool.Pool, paths *DataPaths, ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, txid string, blockHeight int64) string {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return ""
	}
	if pool != nil {
		if b, err := pool.GetRawByTxID(txid); err == nil && len(b) > 0 {
			return hex.EncodeToString(b)
		}
	}
	if paths != nil && paths.WalletTxHexLookup != nil {
		if h, ok := paths.WalletTxHexLookup(txid); ok && h != "" {
			return h
		}
	}
	if ix != nil && raw != nil {
		if tx, err := store.LoadIndexedTx(ix, raw, txid); err == nil {
			if ser, err := tx.Serialize(); err == nil {
				return hex.EncodeToString(ser)
			}
		}
	}
	if j != nil && raw != nil && blockHeight >= 0 {
		return walletTxHexFromBlockHeight(j, raw, txid, blockHeight)
	}
	return ""
}

// walletLookupTxHexFast returns hex from mempool or wallet cache only (no block / tx-index walk).
func walletLookupTxHexFast(pool *mempool.Pool, paths *DataPaths, txid string) string {
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid == "" {
		return ""
	}
	if pool != nil {
		if b, err := pool.GetRawByTxID(txid); err == nil && len(b) > 0 {
			return hex.EncodeToString(b)
		}
	}
	if paths != nil && paths.WalletTxHexLookup != nil {
		if h, ok := paths.WalletTxHexLookup(txid); ok && h != "" {
			return h
		}
	}
	return ""
}

// walletEnrichTxKindList classifies rows for web history lists without loading blocks per row.
func walletEnrichTxKindList(paths *DataPaths, pool *mempool.Pool, r walletTxRow) (kind, pqTag string) {
	if r.abandoned {
		return "abandoned", ""
	}
	switch r.category {
	case "send":
		if hex := walletLookupTxHexFast(pool, paths, r.txid); hex != "" {
			if tag, source := walletPQMetaFromTxHex(hex); tag != "" {
				_ = source
				return "sent_pq", tag
			}
		}
		return "sent", ""
	case "receive":
		if hex := walletLookupTxHexFast(pool, paths, r.txid); hex != "" {
			if tag, _ := walletPQMetaFromTxHex(hex); tag != "" {
				return "received_pq", tag
			}
		}
		return "received", ""
	default:
		if r.category != "" {
			return r.category, ""
		}
		return "unknown", ""
	}
}

// walletPQTagFromTxHex scans vouts for canonical Phase-1 PQ OP_RETURN commitment tag.
func walletPQTagFromTxHex(hexStr string) string {
	tag, _ := walletPQMetaFromTxHex(hexStr)
	return tag
}

// walletPQMetaFromTxHex returns PQ tag and source (commitment_only or carrier_scriptsig).
func walletPQMetaFromTxHex(hexStr string) (tag, source string) {
	raw, err := hex.DecodeString(strings.TrimSpace(hexStr))
	if err != nil {
		return "", ""
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "", ""
	}
	for _, o := range tx.Vout {
		if c, ok := consensus.DetectPQCommitment(o.PkScript); ok {
			return c.Tag, "commitment_only"
		}
	}
	for _, in := range tx.Vin {
		if part, err := consensus.ParsePQCarrierPartScriptSig(in.Script); err == nil {
			if algo, ok := consensus.PQCarrierAlgoForCarrierTag(part.CarrierTag8); ok {
				return algo.OPReturnTag, "carrier_scriptsig"
			}
		}
	}
	return "", ""
}

// walletEnrichTxKind classifies a wallet row for the web UI (mining, quantum send, etc.).
func walletEnrichTxKind(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, pool *mempool.Pool, ix *store.TxIndex, r walletTxRow) (kind, pqTag string) {
	if r.abandoned {
		return "abandoned", ""
	}
	switch r.category {
	case "send":
		kind = "sent"
		if hex := walletLookupTxHex(pool, paths, ix, raw, j, r.txid, r.blockHeight); hex != "" {
			if tag, source := walletPQMetaFromTxHex(hex); tag != "" {
				if source == "carrier_scriptsig" {
					return "sent_pq", tag
				}
				return "sent_pq", tag
			}
		}
		return kind, ""
	case "receive":
		kind := walletReceiveTxKind(chainName, paths, j, raw, ix, r)
		if hex := walletLookupTxHex(pool, paths, ix, raw, j, r.txid, r.blockHeight); hex != "" {
			if tag, _ := walletPQMetaFromTxHex(hex); tag != "" {
				return "received_pq", tag
			}
		}
		return kind, ""
	default:
		if r.category != "" {
			return r.category, ""
		}
		return "unknown", ""
	}
}

// walletReceiveTxKind classifies receive rows (mining vs received). With compact tx index
// (embed_tx off), vout 0 uses the same coinbase heuristic as walletUtxoImmatureCoinbase
// so listtransactions does not load a full block per solo-mined coinbase.
func walletReceiveTxKind(chainName string, paths *DataPaths, j HeaderJournal, raw *store.RawBlockStore, ix *store.TxIndex, r walletTxRow) string {
	mat := walletCoinbaseMaturity(chainName, j, raw, paths)
	if r.vout == 0 && ix != nil && !ix.EmbedTx {
		if r.confirmations > 0 && r.confirmations < mat {
			return "mining_immature"
		}
		return "mining"
	}
	if walletFundingTxIsCoinbase(ix, raw, r.txid) {
		if r.confirmations > 0 && r.confirmations < mat {
			return "mining_immature"
		}
		return "mining"
	}
	return "received"
}

func walletTxHexFromBlockHeight(j HeaderJournal, raw *store.RawBlockStore, txid string, height int64) string {
	h80, err := j.ReadHeaderAt(height)
	if err != nil {
		return ""
	}
	id := pow.BlockHashLE(h80)
	payload, err := raw.Get(id)
	if err != nil {
		return ""
	}
	want := strings.ToLower(txid)
	var hexOut string
	_ = wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
		if hexOut != "" {
			return nil
		}
		if strings.EqualFold(mempool.TxIDDisplayHex(tx.TxHash()), want) {
			if ser, err := tx.Serialize(); err == nil {
				hexOut = hex.EncodeToString(ser)
			}
		}
		return nil
	})
	return hexOut
}

func walletSpendScriptSet(paths *DataPaths) map[string]struct{} {
	return walletScriptSet(rpcWalletSpendScripts(paths))
}

// walletCoinbaseMaturity returns chain coinbase maturity at chainActive height (Core nCoinbaseMaturity).
func walletCoinbaseMaturity(chainName string, j HeaderJournal, raw *store.RawBlockStore, paths ...*DataPaths) int64 {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return 30
	}
	height := int64(0)
	if j != nil {
		height, _, _ = activeChainFromJournal(j, raw, paths...)
		if height < 0 {
			height = 0
		}
	}
	return int64(consensus.LookupConsensus(net, height).CoinbaseMaturity)
}

// walletUtxoImmatureCoinbase reports whether a sub-maturity UTXO is coinbase-funded.
// With compact tx index (embed_tx off), loading each tx pulls a full block - use vout 0
// heuristic for the common solo-miner coinbase case.
func walletUtxoImmatureCoinbase(row store.UtxoDumpRow, ix *store.TxIndex, raw *store.RawBlockStore) bool {
	if row.Vout == 0 && ix != nil && !ix.EmbedTx {
		return true
	}
	return walletFundingTxIsCoinbase(ix, raw, row.TxID)
}

// walletFundingTxIsCoinbase reports whether txid is a confirmed coinbase transaction.
func walletFundingTxIsCoinbase(ix *store.TxIndex, raw *store.RawBlockStore, txid string) bool {
	if ix == nil || raw == nil {
		return false
	}
	tx, err := store.LoadIndexedTx(ix, raw, strings.ToLower(strings.TrimSpace(txid)))
	if err != nil {
		return false
	}
	return consensus.IsCoinbaseTx(tx)
}

// walletUtxoIsSafe matches Core IS_SAFE for listunspent (confirmed, mature coinbase when applicable).
func walletUtxoIsSafe(m walletUtxoMatch, maturity int64, ix *store.TxIndex, raw *store.RawBlockStore) bool {
	if m.confirmations < 1 {
		return false
	}
	if m.confirmations < maturity && walletUtxoImmatureCoinbase(m.row, ix, raw) {
		return false
	}
	return true
}

// walletPrevoutEntry resolves a prevout from the UTXO cache or confirmed tx index (for fee on spent wallet inputs).
func walletPrevoutEntry(paths *DataPaths, ix *store.TxIndex, raw *store.RawBlockStore, prevHash [32]byte, vout uint32) (store.UtxoEntry, bool) {
	if paths != nil && paths.Utxo != nil {
		if e, ok := paths.Utxo.LookupOutpoint(prevHash, vout); ok {
			return e, true
		}
	}
	if ix == nil || raw == nil {
		return store.UtxoEntry{}, false
	}
	txid := mempool.TxIDDisplayHex(prevHash)
	val, spk, ok := store.LoadIndexedTxVout(ix, raw, txid, vout)
	if !ok {
		return store.UtxoEntry{}, false
	}
	return store.UtxoEntry{Value: val, PkScript: spk}, true
}

// walletSendFeeKoinu estimates fee for a wallet send (mempool or confirmed) using all HD spend scripts.
func walletSendFeeKoinu(chainName string, paths *DataPaths, pool *mempool.Pool, ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, txid string, blockHeight int64) int64 {
	if paths == nil {
		return 0
	}
	txid = strings.ToLower(strings.TrimSpace(txid))
	if txid != "" && paths.WalletSendFeeLookup != nil {
		if fee, ok := paths.WalletSendFeeLookup(txid); ok && fee > 0 {
			return fee
		}
	}
	spendSet := walletSpendScriptSet(paths)
	if len(spendSet) == 0 {
		return 0
	}
	var rawTx []byte
	if pool != nil {
		if b, err := pool.GetRawByTxID(txid); err == nil {
			rawTx = b
		}
	}
	if len(rawTx) == 0 {
		if h := walletLookupTxHex(pool, paths, ix, raw, j, txid, blockHeight); h != "" {
			rawTx, _ = hex.DecodeString(h)
		}
	}
	if len(rawTx) == 0 {
		return 0
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		return 0
	}
	var spent, totalOut int64
	for _, in := range tx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		e, ok := walletPrevoutEntry(paths, ix, raw, in.PrevHash, in.PrevIdx)
		if !ok {
			continue
		}
		if _, mine := spendSet[hex.EncodeToString(e.PkScript)]; mine {
			spent += e.Value
		}
	}
	for _, o := range tx.Vout {
		totalOut += o.Value
	}
	if spent <= 0 {
		return 0
	}
	fee := spent - totalOut
	if fee < 0 {
		return 0
	}
	return fee
}

// walletSendDisplayFromHex recomputes send display amount and spend address from tx hex.
// amountKoinu is negative (wallet send row convention).
func walletSendDisplayFromHex(chainName string, paths *DataPaths, pool *mempool.Pool, ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, txid string, blockHeight int64) (sendAddr string, amountKoinu int64, ok bool) {
	if paths == nil {
		return "", 0, false
	}
	txid = strings.ToLower(strings.TrimSpace(txid))
	spendSet := walletSpendScriptSet(paths)
	if len(spendSet) == 0 {
		return "", 0, false
	}
	tracked := rpcWalletTrackedScripts(paths)
	trackedSet := make(map[string][]byte, len(tracked))
	for _, pk := range tracked {
		if len(pk) > 0 {
			trackedSet[string(pk)] = pk
		}
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", 0, false
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", 0, false
	}
	scriptAddr := make(map[string]string)
	for _, pk := range tracked {
		scriptAddr[hex.EncodeToString(pk)] = chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	}
	var rawTx []byte
	if pool != nil {
		if b, err := pool.GetRawByTxID(txid); err == nil {
			rawTx = b
		}
	}
	if len(rawTx) == 0 {
		if h := walletLookupTxHex(pool, paths, ix, raw, j, txid, blockHeight); h != "" {
			rawTx, _ = hex.DecodeString(h)
		}
	}
	if len(rawTx) == 0 {
		return "", 0, false
	}
	tx, err := wire.DeserializeTx(rawTx)
	if err != nil {
		return "", 0, false
	}
	recvByAddr := make(map[string]int64)
	var spentTotal int64
	for _, in := range tx.Vin {
		if isCoinbaseWireIn(&in) {
			continue
		}
		e, found := walletPrevoutEntry(paths, ix, raw, in.PrevHash, in.PrevIdx)
		if !found {
			continue
		}
		if _, mine := spendSet[hex.EncodeToString(e.PkScript)]; mine {
			spentTotal += e.Value
			if sendAddr == "" {
				sendAddr = scriptAddr[hex.EncodeToString(e.PkScript)]
			}
		}
	}
	if spentTotal <= 0 {
		return "", 0, false
	}
	for _, o := range tx.Vout {
		a, trackedOut := scriptAddr[hex.EncodeToString(o.PkScript)]
		if !trackedOut || a == "" {
			continue
		}
		recvByAddr[a] += o.Value
	}
	if sendAddr == "" {
		sendAddr = rpcWalletDefaultAddress(paths)
	}
	display := wallet.SendDisplayKoinu(spentTotal, recvByAddr, sendAddr, tx.Vout, trackedSet)
	if display <= 0 {
		return "", 0, false
	}
	return sendAddr, -display, true
}

// walletMempoolSendFeeKoinu estimates fee for a mempool wallet send (alias for walletSendFeeKoinu).
func walletMempoolSendFeeKoinu(chainName string, paths *DataPaths, pool *mempool.Pool, txid string) int64 {
	return walletSendFeeKoinu(chainName, paths, pool, nil, nil, nil, txid, -1)
}

// walletBlockHashAt returns the display block hash at height.
func walletBlockHashAt(j HeaderJournal, height int64) string {
	if j == nil || height < 0 {
		return ""
	}
	h, err := blockHashHexAt(j, height)
	if err != nil {
		return ""
	}
	return h
}
