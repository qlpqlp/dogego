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

	"dogego/chain"
	"dogego/consensus"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

// txidToRPC returns the reversed-byte-order hex string used in Bitcoin-style RPC tx lists.
func txidToRPC(h [32]byte) string {
	b := make([]byte, 32)
	for i := 0; i < 32; i++ {
		b[i] = h[31-i]
	}
	return hex.EncodeToString(b)
}

// execGetBlock implements getblock (verbosity 0 = hex, 1 = txid list, 2 = full tx objects).
func execGetBlock(j HeaderJournal, raw *store.RawBlockStore, aux *store.HeaderAuxJournal, chainName string, paths *DataPaths, params []json.RawMessage) (result interface{}, errCode int, errMsg string) {
	if raw == nil {
		return nil, -18, "getblock: block store not available"
	}
	hashLE, height, err := resolveBlockLocation(j, params)
	if err != nil {
		return nil, -8, err.Error()
	}
	verbosity := 1
	if len(params) > 1 {
		var v float64
		if err := json.Unmarshal(params[1], &v); err != nil {
			return nil, -8, "getblock: bad verbosity"
		}
		if v != float64(int(v)) || v < 0 || v > 2 {
			return nil, -8, "getblock: verbosity must be 0, 1, or 2"
		}
		verbosity = int(v)
	}
	payload, err := loadBlockPayload(raw, hashLE)
	if err != nil {
		return nil, -5, "getblock: block not stored on disk (headers-only or outside fetched range)"
	}
	if err := wire.ValidateBlockPayload(payload, hashLE); err != nil {
		return nil, -8, "getblock: invalid stored block: " + err.Error()
	}
	if verbosity == 0 {
		return hex.EncodeToString(payload), 0, ""
	}
	hdr, err := wire.BlockHeaderFromPayload(payload)
	if err != nil {
		return nil, -8, "getblock: corrupt stored block: " + err.Error()
	}
	chainTip, _, _ := activeChainFromJournal(j, raw, paths)
	if verbosity == 1 {
		txids, err := wire.RPCTxidsFromPayload(payload)
		if err != nil {
			return nil, -8, "getblock: " + err.Error()
		}
		return buildGetBlockJSON(j, hdr, payload, txids, height, aux, chainTip), 0, ""
	}
	txids, err := wire.RPCTxidsFromPayload(payload)
	if err != nil {
		return nil, -8, "getblock: " + err.Error()
	}
	m := buildGetBlockJSON(j, hdr, payload, txids, height, aux, chainTip)
	if verbosity == 2 {
		arr := make([]interface{}, 0, len(txids))
		if err := wire.ForEachBlockTx(payload, func(_ uint32, tx *wire.Tx) error {
			jm, err := txToRPCJSONChain(tx, chainName)
			if err != nil {
				return err
			}
			arr = append(arr, jm)
			return nil
		}); err != nil {
			return nil, -8, "getblock: " + err.Error()
		}
		m["tx"] = arr
	}
	return m, 0, ""
}

func buildGetBlockJSON(j HeaderJournal, hdr primitives.BlockHeader, raw []byte, txids []string, height int64, aux *store.HeaderAuxJournal, confirmTip int64) map[string]interface{} {
	h80 := hdr.EncodeWire80()
	bitsU := hdr.Bits
	diff := float64(0)
	if d, err := pow.DifficultyFromCompact(bitsU); err == nil {
		diff = d
	}
	conf := int64(1)
	if height >= 0 && confirmTip >= height {
		conf = confirmTip - height + 1
	}
	if txids == nil {
		txids = []string{}
	}
	nextHash := interface{}(nil)
	if height >= 0 {
		if nh80, err := j.ReadHeaderAt(height + 1); err == nil {
			nextHash = pow.BlockHashHex(nh80)
		}
	}
	cw := "0"
	if s, err := cumulativeChainworkHex(j, height); err == nil {
		cw = s
	}
	mediantime := int64(hdr.Timestamp)
	if m, err := headerMedianTimePast(j, height); err == nil {
		mediantime = m
	}
	sz := len(raw)
	// Legacy Dogecoin blocks: RPC weight matches Core non-witness rule (4 × serialized block bytes).
	blkWeight := 4 * sz
	out := map[string]interface{}{
		"hash":              pow.BlockHashHex(h80[:]),
		"confirmations":     conf,
		"strippedsize":      sz,
		"size":              sz,
		"weight":            blkWeight,
		"height":            height,
		"version":           hdr.Version,
		"versionHex":        fmt.Sprintf("%08x", uint32(hdr.Version)),
		"merkleroot":        pow.LEUint256DisplayHex(hdr.MerkleRoot[:]),
		"tx":                txids,
		"time":              int64(hdr.Timestamp),
		"mediantime":        mediantime,
		"nonce":             hdr.Nonce,
		"bits":              fmt.Sprintf("%08x", bitsU),
		"difficulty":        diff,
		"chainwork":         cw,
		"nTx":               len(txids),
		"previousblockhash": pow.LEUint256DisplayHex(hdr.PrevBlock[:]),
		"nextblockhash":     nextHash,
		"dogego_note":       "Block from native rawblocks store (validated on connect when chain caught up); weight = 4× serialized size (legacy Dogecoin RPC rule)",
	}
	attachAuxPowField(out, nil, aux, height)
	return out
}

func isCoinbaseInput(in *wire.TxIn) bool {
	var z [32]byte
	return in.PrevIdx == 0xffffffff && in.PrevHash == z
}

// txToRPCJSON builds a Core-shaped transaction object (decoderawtransaction / getblock verbosity 2).
func txToRPCJSON(tx *wire.Tx) (map[string]interface{}, error) {
	return txToRPCJSONChain(tx, "")
}

// txToRPCJSONChain adds network-aware scriptPubKey addresses when chainName is set.
func txToRPCJSONChain(tx *wire.Tx, chainName string) (map[string]interface{}, error) {
	ser, err := tx.Serialize()
	if err != nil {
		return nil, err
	}
	txh := tx.TxHash()
	wth := tx.WTxHash()
	sz, err := wire.TransactionTotalSize(tx)
	if err != nil {
		return nil, err
	}
	vs, err := wire.TransactionVirtualSize(tx)
	if err != nil {
		return nil, err
	}
	weight, err := wire.TransactionWeight(tx)
	if err != nil {
		return nil, err
	}
	var spkParams chain.Params
	if chainName != "" {
		if net, err := networkFromRPCChainName(chainName); err == nil {
			if p, err := chain.ParamsFor(net); err == nil {
				spkParams = p
			}
		}
	}
	vin := make([]interface{}, len(tx.Vin))
	for i := range tx.Vin {
		in := &tx.Vin[i]
		if isCoinbaseInput(in) {
			vin[i] = map[string]interface{}{
				"coinbase": hex.EncodeToString(in.Script),
				"sequence": in.Sequence,
			}
			continue
		}
		m := map[string]interface{}{
			"txid":      txidToRPC(in.PrevHash),
			"vout":      in.PrevIdx,
			"scriptSig": scriptSigRPC(in.Script),
			"sequence":  in.Sequence,
		}
		if len(in.Witness) > 0 {
			wh := make([]string, len(in.Witness))
			for k, w := range in.Witness {
				wh[k] = hex.EncodeToString(w)
			}
			m["txinwitness"] = wh
		}
		vin[i] = m
	}
	vout := make([]interface{}, len(tx.Vout))
	for i := range tx.Vout {
		o := &tx.Vout[i]
		spk := scriptPubKeyRPC(o.PkScript, spkParams)
		vout[i] = map[string]interface{}{
			"value":        float64(o.Value) / 1e8,
			"n":            i,
			"scriptPubKey": spk,
		}
	}
	note := "Dogecoin does not enable segwit - witness txs are rejected at mempool/P2P admission."
	if !tx.HasWitness() {
		note = "Legacy transaction (no witness stacks)."
	}
	return map[string]interface{}{
		"txid":        txidToRPC(txh),
		"hash":        txidToRPC(wth),
		"version":     tx.Version,
		"size":        sz,
		"vsize":       vs,
		"weight":      weight,
		"locktime":    tx.LockTime,
		"vin":         vin,
		"vout":        vout,
		"hex":         hex.EncodeToString(ser),
		"dogego_note": note,
	}, nil
}

func scriptSigRPC(script []byte) map[string]interface{} {
	out := map[string]interface{}{
		"hex": hex.EncodeToString(script),
		"asm": scriptToASM(script),
	}
	metas := consensus.ScriptSigRedeemMetas(script)
	if len(metas) > 0 {
		enriched := make([]interface{}, len(metas))
		for i, m := range metas {
			entry := make(map[string]interface{}, len(m)+1)
			for k, v := range m {
				entry[k] = v
			}
			if idx, ok := m["dogego_push_index"].(int); ok {
				if pushes, err := consensus.ScriptSigPushes(script); err == nil && idx < len(pushes) {
					entry["hex"] = hex.EncodeToString(pushes[idx])
				}
			}
			enriched[i] = entry
		}
		out["dogego_redeem_pushes"] = enriched
		out["dogego_redeem"] = enriched[len(enriched)-1]
	} else if redeem, err := consensus.LastScriptPush(script); err == nil && len(redeem) > 0 {
		if meta := consensus.RedeemScriptMeta(redeem); meta != nil {
			out["dogego_redeem"] = meta
		}
	}
	return out
}

func loadBlockPayload(raw *store.RawBlockStore, hashLE [32]byte) ([]byte, error) {
	if raw == nil {
		return nil, fmt.Errorf("block not in store")
	}
	return raw.Get(hashLE)
}

func scriptPubKeyRPC(script []byte, p chain.Params) map[string]interface{} {
	if p.PubkeyHashAddrID == 0 && p.ScriptHashAddrID == 0 {
		return map[string]interface{}{
			"hex":       hex.EncodeToString(script),
			"asm":       scriptToASM(script),
			"type":      "nonstandard",
			"addresses": []interface{}{},
		}
	}
	return scriptPubKeyDecode(script, p)
}
