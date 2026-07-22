// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

// rawTxChainMeta returns Core-shaped confirmation fields relative to chainActive (not header journal during IBD).
func rawTxChainMeta(j HeaderJournal, raw *store.RawBlockStore, blockHeight int64, paths ...*DataPaths) (confirmations int64, bestBlock string, inActive bool) {
	chainTip, _, _ := activeChainFromJournal(j, raw, paths...)
	bestBlock, _ = blockHashHexAt(j, chainTip)
	if blockHeight < 0 {
		return 0, bestBlock, false
	}
	inActive = blockHeight <= chainTip
	if inActive {
		confirmations = chainTip - blockHeight + 1
	}
	return confirmations, bestBlock, inActive
}

// rawTxFromIndexHit serves getrawtransaction from indexes/tx (v2 embedded raw or legacy + block slice).
func rawTxFromIndexHit(_ *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, hit store.TxIndexHit, verbose bool, paths *DataPaths) (interface{}, int, string) {
	blockDisp := pow.LEUint256DisplayHex(hit.BlockHashLE[:])
	height, err := j.HeightByDisplayHash(blockDisp)
	if err != nil {
		height = -1
	}
	var tx *wire.Tx
	var blockTime int64
	var ser []byte
	if len(hit.TxRaw) > 0 {
		ser = hit.TxRaw
		tx, err = wire.DeserializeTx(ser)
		if err != nil {
			return nil, -8, "getrawtransaction: corrupt index tx: " + err.Error()
		}
		if height >= 0 {
			if h80, err := j.ReadHeaderAt(height); err == nil && len(h80) == 80 {
				blockTime = int64(primitivesBlockTime(h80))
			}
		}
	} else {
		payload, err := raw.Get(hit.BlockHashLE)
		if err != nil {
			return nil, -5, "getrawtransaction: block payload missing"
		}
		var meta wire.TxAtBlockMeta
		tx, meta, err = wire.ReadTxAtIndex(payload, hit.TxIndex)
		if err != nil {
			return nil, -8, "getrawtransaction: " + err.Error()
		}
		blockTime = int64(meta.Header.Timestamp)
		ser, err = tx.Serialize()
		if err != nil {
			return nil, -8, err.Error()
		}
	}
	if !verbose {
		return hex.EncodeToString(ser), 0, ""
	}
	jm, err := txToRPCJSON(tx)
	if err != nil {
		return nil, -8, err.Error()
	}
	conf, best, inActive := rawTxChainMeta(j, raw, height, paths)
	jm["hex"] = hex.EncodeToString(ser)
	jm["blockhash"] = blockDisp
	jm["confirmations"] = conf
	if blockTime > 0 {
		jm["blocktime"] = blockTime
		jm["time"] = blockTime
	}
	if height >= 0 {
		jm["height"] = height
	}
	jm["in_active_chain"] = inActive
	if best != "" {
		jm["bestblockhash"] = best
	}
	jm["dogego_note"] = "confirmed from local indexes/tx + rawblocks"
	return jm, 0, ""
}

func primitivesBlockTime(h80 []byte) uint32 {
	if len(h80) < 72 {
		return 0
	}
	return uint32(h80[68]) | uint32(h80[69])<<8 | uint32(h80[70])<<16 | uint32(h80[71])<<24
}

// execGetRawTransaction returns confirmed tx (local index + raw block) or unconfirmed tx from the mempool,
// matching Core's usual ordering (chain first, then mempool).
func execGetRawTransaction(ix *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, pool *mempool.Pool, paths *DataPaths, params []json.RawMessage) (result interface{}, errCode int, errMsg string) {
	if len(params) < 1 {
		return nil, -8, "getrawtransaction: txid required"
	}
	var txidStr string
	if err := json.Unmarshal(params[0], &txidStr); err != nil {
		return nil, -8, "getrawtransaction: bad txid"
	}
	verbose := false
	var blockHashFilter string
	if len(params) > 1 {
		var p1 string
		if err := json.Unmarshal(params[1], &p1); err == nil && len(strings.TrimSpace(p1)) >= 64 {
			blockHashFilter = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(p1), "0x"))
			if len(params) > 2 {
				if err := json.Unmarshal(params[2], &verbose); err != nil {
					return nil, -8, "getrawtransaction: bad verbose flag"
				}
			}
		} else if err := json.Unmarshal(params[1], &verbose); err != nil {
			return nil, -8, "getrawtransaction: bad verbose flag"
		}
	}
	if len(params) > 2 && blockHashFilter == "" {
		var bh string
		if err := json.Unmarshal(params[2], &bh); err != nil {
			return nil, -8, "getrawtransaction: bad blockhash"
		}
		blockHashFilter = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(bh), "0x"))
	}
	if blockHashFilter != "" && len(blockHashFilter) != 64 {
		return nil, -8, "getrawtransaction: blockhash must be 64 hex characters"
	}
	txidStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txidStr), "0x"))

	if ix != nil && raw != nil {
		if blockHashFilter != "" {
			res, code, msg := getRawTxInBlock(ix, raw, j, pool, txidStr, blockHashFilter, verbose, paths)
			return res, code, msg
		}
		if hit, err := ix.LookupHit(txidStr); err == nil {
			return rawTxFromIndexHit(ix, raw, j, hit, verbose, paths)
		}
	}

	if pool != nil {
		blob, err := pool.GetRawByTxID(txidStr)
		if err == nil {
			tx, err := wire.DeserializeTx(blob)
			if err != nil {
				return nil, -8, "getrawtransaction: corrupt mempool entry: " + err.Error()
			}
			ser, err := tx.Serialize()
			if err != nil {
				return nil, -8, err.Error()
			}
			if !verbose {
				return hex.EncodeToString(ser), 0, ""
			}
			jm, err := txToRPCJSON(tx)
			if err != nil {
				return nil, -8, err.Error()
			}
			jm["hex"] = hex.EncodeToString(ser)
			jm["blockhash"] = nil
			jm["confirmations"] = int64(0)
			_, best, _ := rawTxChainMeta(j, raw, -1, paths)
			if best != "" {
				jm["bestblockhash"] = best
			}
			jm["dogego_note"] = "unconfirmed (local mempool only)"
			return jm, 0, ""
		}
	}

	if ix == nil && raw == nil && pool == nil {
		return nil, -18, "getrawtransaction: no chain index, raw blocks, or mempool available"
	}
	return nil, -5, "No such mempool or blockchain transaction"
}

func getRawTxInBlock(_ *store.TxIndex, raw *store.RawBlockStore, j HeaderJournal, pool *mempool.Pool, txidStr, blockHashFilter string, verbose bool, paths *DataPaths) (interface{}, int, string) {
	height, err := j.HeightByDisplayHash(blockHashFilter)
	if err != nil {
		return nil, -5, "No such mempool or blockchain transaction"
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil || len(h80) != 80 {
		return nil, -5, "No such mempool or blockchain transaction"
	}
	if !strings.EqualFold(pow.BlockHashHex(h80), blockHashFilter) {
		return nil, -5, "No such mempool or blockchain transaction"
	}
	blockHashLE := pow.BlockHashLE(h80)
	payload, err := raw.Get(blockHashLE)
	if err != nil {
		return nil, -5, "getrawtransaction: block not found"
	}
	tx, _, err := wire.FindTxByRPCID(payload, txidStr)
	if err != nil {
		if pool != nil {
			if _, perr := pool.GetRawByTxID(txidStr); perr == nil {
				return nil, -5, "No such mempool or blockchain transaction"
			}
		}
		return nil, -5, "No such mempool or blockchain transaction"
	}
	ser, err := tx.Serialize()
	if err != nil {
		return nil, -8, err.Error()
	}
	if !verbose {
		return hex.EncodeToString(ser), 0, ""
	}
	jm, err := txToRPCJSON(tx)
	if err != nil {
		return nil, -8, err.Error()
	}
	conf, best, inActive := rawTxChainMeta(j, raw, height, paths)
	jm["hex"] = hex.EncodeToString(ser)
	jm["blockhash"] = blockHashFilter
	jm["confirmations"] = conf
	jm["height"] = height
	if hdr, err := wire.BlockHeaderFromPayload(payload); err == nil {
		jm["blocktime"] = int64(hdr.Timestamp)
		jm["time"] = int64(hdr.Timestamp)
	}
	jm["in_active_chain"] = inActive
	if best != "" {
		jm["bestblockhash"] = best
	}
	jm["dogego_note"] = "confirmed from blockhash filter"
	return jm, 0, ""
}

// execDecodeRawTransaction decodes hex-encoded serialized tx (no blockchain lookup).
// Optional second parameter iswitness is accepted for Core arity (DogeGo legacy txs ignore it).
func execDecodeRawTransaction(chainName string, params []json.RawMessage) (result interface{}, errCode int, errMsg string) {
	if len(params) < 1 || len(params) > 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) > 1 && strings.TrimSpace(string(params[1])) != "null" {
		var isWitness bool
		if err := json.Unmarshal(params[1], &isWitness); err != nil {
			return nil, -8, "decoderawtransaction: iswitness must be boolean"
		}
		_ = isWitness
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "decoderawtransaction: bad hex param"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		return nil, -8, "decoderawtransaction: invalid hex"
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, -8, "decoderawtransaction: " + err.Error()
	}
	jm, err := txToRPCJSONChain(tx, chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	return jm, 0, ""
}

// DecodeTxHex is exported for the web UI: same rules as decoderawtransaction hex input.
func DecodeTxHex(hexStr string, chainName string) (map[string]interface{}, error) {
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		return nil, fmt.Errorf("invalid hex")
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, err
	}
	return txToRPCJSONChain(tx, chainName)
}

// TxToRPCJSONChain builds Core-shaped JSON with network-aware scriptPubKey addresses.
func TxToRPCJSONChain(tx *wire.Tx, chainName string) (map[string]interface{}, error) {
	return txToRPCJSONChain(tx, chainName)
}

// LookupTxExplorer resolves txid from chain index + rawblocks first, then the local mempool.
func LookupTxExplorer(ix *store.TxIndex, raw *store.RawBlockStore, pool *mempool.Pool, txidHex string) (jm map[string]interface{}, rawTx []byte, source string, err error) {
	if ix != nil && raw != nil {
		if m, ser, err := LookupTxFromIndex(ix, raw, txidHex); err == nil {
			return m, ser, "chain", nil
		}
	}
	if pool != nil {
		id := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(txidHex), "0x"))
		blob, err := pool.GetRawByTxID(id)
		if err == nil {
			tx, err := wire.DeserializeTx(blob)
			if err != nil {
				return nil, nil, "", err
			}
			m, err := txToRPCJSON(tx)
			if err != nil {
				return nil, nil, "", err
			}
			m["confirmations"] = int64(0)
			m["dogego_note"] = "unconfirmed (local mempool)"
			return m, blob, "mempool", nil
		}
	}
	return nil, nil, "", fmt.Errorf("transaction not found")
}

// LookupTxFromIndex loads a confirmed tx by RPC-style txid using tx index + raw block store.
func LookupTxFromIndex(ix *store.TxIndex, raw *store.RawBlockStore, txidHex string) (map[string]interface{}, []byte, error) {
	if ix == nil || raw == nil {
		return nil, nil, fmt.Errorf("tx lookup unavailable")
	}
	tx, err := store.LoadIndexedTx(ix, raw, strings.TrimSpace(txidHex))
	if err != nil {
		return nil, nil, err
	}
	ser, err := tx.Serialize()
	if err != nil {
		return nil, nil, err
	}
	hit, err := ix.LookupHit(strings.TrimSpace(txidHex))
	if err != nil {
		return nil, nil, err
	}
	jm, err := txToRPCJSON(tx)
	if err != nil {
		return nil, nil, err
	}
	jm["blockhash"] = pow.LEUint256DisplayHex(hit.BlockHashLE[:])
	jm["tx_index"] = int(hit.TxIndex)
	return jm, ser, nil
}

// decodeRPCPrevHashHex converts a 64-char RPC display txid to the 32-byte prevout hash used on the wire.
func decodeRPCPrevHashHex(s string) ([32]byte, error) {
	var z [32]byte
	s = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x"))
	if len(s) != 64 {
		return z, fmt.Errorf("txid must be 64 hex characters")
	}
	for _, c := range s {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return z, fmt.Errorf("txid must be hex")
	}
	raw, err := hex.DecodeString(s)
	if err != nil {
		return z, err
	}
	for i := 0; i < 32; i++ {
		z[i] = raw[31-i]
	}
	return z, nil
}
