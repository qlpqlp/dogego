// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

type pendingAuxBlock struct {
	height      int64
	header80    [80]byte
	txRaws      [][]byte
	coinValue   int64
	displayHash string
}

func p2pkhScriptFromAddress(chainName, addr string) ([20]byte, error) {
	var z [20]byte
	m, code, msg := ValidateAddressString(chainName, addr)
	if code != 0 {
		return z, fmt.Errorf("%s", msg)
	}
	if valid, _ := m["isvalid"].(bool); !valid {
		return z, fmt.Errorf("invalid coinbase payout address")
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return z, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return z, err
	}
	v, h160, err := chain.Base58CheckDecode(addr)
	if err != nil || v != p.PubkeyHashAddrID {
		return z, fmt.Errorf("invalid coinbase payout address")
	}
	return h160, nil
}

func execCreateAuxBlock(
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	rawBlocks *store.RawBlockStore,
	chainName string,
	paths *DataPaths,
	params []json.RawMessage,
) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -32602, "Wrong number of arguments"
	}
	var addr string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "createauxblock: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, -8, "createauxblock: address required"
	}
	return buildAuxBlockTemplate(j, pool, txIndex, rawBlocks, chainName, paths, addr)
}

func execGetAuxBlock(
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	rawBlocks *store.RawBlockStore,
	chainName string,
	paths *DataPaths,
	params []json.RawMessage,
) (interface{}, int, string) {
	if len(params) != 0 && len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) == 0 {
		return nil, -8, "getauxblock: without arguments requires a wallet in Dogecoin Core; use createauxblock <address> in DogeGo"
	}
	if _, code, msg := parseOneBlockHashParam([]json.RawMessage{params[0]}, "getauxblock"); code != 0 {
		return nil, code, msg
	}
	return submitAuxBlockSolution(j, paths, rawBlocks, chainName, params)
}

func execSubmitAuxBlock(
	j HeaderJournal,
	paths *DataPaths,
	rawBlocks *store.RawBlockStore,
	chainName string,
	params []json.RawMessage,
) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	return submitAuxBlockSolution(j, paths, rawBlocks, chainName, params)
}

func buildAuxBlockTemplate(
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	rawBlocks *store.RawBlockStore,
	chainName string,
	paths *DataPaths,
	payoutAddr string,
) (interface{}, int, string) {
	if j == nil {
		return nil, -1, "createauxblock: header journal not available"
	}
	h160, err := p2pkhScriptFromAddress(chainName, payoutAddr)
	if err != nil {
		if strings.Contains(err.Error(), "invalid") {
			return nil, -5, "Invalid coinbase payout address"
		}
		return nil, -8, "createauxblock: " + err.Error()
	}
	tip, _, _ := activeChainFromJournal(j, rawBlocks, paths)
	h80, err := j.ReadHeaderAt(tip)
	if err != nil || len(h80) != 80 {
		return nil, -1, "createauxblock: header read failed"
	}
	tipHashHex := pow.BlockHashHex(h80)
	globalAuxCache.onTipChange(tipHashHex)
	next := tip + 1
	net, _ := chain.ParseNetwork(chainName)
	dc := consensus.LookupConsensus(net, next)
	if dc.AllowLegacyBlocks {
		return nil, -1, "createauxblock: merge-mining is not yet active at the next block height"
	}
	scriptKey := scriptKeyFromH160(h160)
	var mempoolSeq uint64
	if pool != nil {
		mempoolSeq = pool.ChangeSequence()
	}
	if cached, ok := globalAuxCache.getByScript(scriptKey, mempoolSeq); ok {
		return auxBlockTemplateResponse(cached, h80, dc), 0, ""
	}
	pkScript := consensus.P2PKHPkScript(h160)
	var prevLE [32]byte
	copy(prevLE[:], h80[4:36])
	subsidy := consensus.BlockSubsidy(next, prevLE, net)
	weightLimit := consensus.MaxBlockWeight
	if paths != nil && paths.BlockMaxWeight > 0 {
		weightLimit = paths.BlockMaxWeight
	}
	view := consensus.AdmissionPrevOutView(pool, txIndex, rawBlocks)
	sel, _ := consensus.SelectBlockTemplateTxs(pool, view, weightLimit)
	var feeTotal int64
	for _, ent := range sel.Txs {
		feeTotal += ent.Fee
	}
	coin := consensus.BuildCoinbaseTx(next, subsidy+feeTotal, pkScript)
	var txs []*wire.Tx
	txs = append(txs, coin)
	for _, ent := range sel.Txs {
		raw, err := hex.DecodeString(ent.Data)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		txs = append(txs, tx)
	}
	merkle := wire.BlockMerkleRoot(txs)
	mtp, err := medianTimePastAfterPrev(j, tip)
	if err != nil {
		return nil, -1, "createauxblock: " + err.Error()
	}
	cur := time.Now().Unix()
	if cur < mtp {
		cur = mtp
	}
	bitsU, err := consensus.NextBlockBits(j, net, next, uint32(cur))
	if err != nil {
		return nil, -1, "createauxblock: next bits: " + err.Error()
	}
	ver := consensus.ComputeBlockVersion(j, net, tip)
	var hdr primitivesBlockHeader
	hdr.Version = int32(ver)
	tipHashLE := pow.BlockHashLE(h80)
	copy(hdr.PrevBlock[:], tipHashLE[:])
	hdr.MerkleRoot = merkle
	hdr.Timestamp = uint32(cur)
	hdr.Bits = bitsU
	hdr.Nonce = 0
	h80out := hdr.encode()
	display := pow.BlockHashHex(h80out[:])
	txRaws := make([][]byte, len(txs))
	for i, tx := range txs {
		raw, err := tx.Serialize()
		if err != nil {
			return nil, -1, "createauxblock: tx serialize: " + err.Error()
		}
		txRaws[i] = raw
	}
	pending := &pendingAuxBlock{
		height:      next,
		header80:    h80out,
		txRaws:      txRaws,
		coinValue:   coin.Vout[0].Value,
		displayHash: display,
	}
	globalAuxCache.put(scriptKey, display, mempoolSeq, pending)
	return auxBlockTemplateResponse(pending, h80, dc), 0, ""
}

func auxBlockTemplateResponse(p *pendingAuxBlock, prevHeader80 []byte, dc consensus.DogeConsensus) map[string]interface{} {
	bitsU := binary.LittleEndian.Uint32(p.header80[72:76])
	return map[string]interface{}{
		"hash":              p.displayHash,
		"chainid":           dc.AuxpowChainID,
		"previousblockhash": pow.BlockHashHex(prevHeader80),
		"coinbasevalue":     p.coinValue,
		"bits":              pow.BitsHex(bitsU),
		"height":            p.height,
		"target":            pow.TargetHexFromCompact(bitsU),
		"dogego_note":       "template includes mempool txs selected like getblocktemplate; submit solved auxpow via submitauxblock or getauxblock <hash> <auxpow>",
	}
}

// primitivesBlockHeader avoids importing primitives in rpc for a single encode.
type primitivesBlockHeader struct {
	Version    int32
	PrevBlock  [32]byte
	MerkleRoot [32]byte
	Timestamp  uint32
	Bits       uint32
	Nonce      uint32
}

func (h *primitivesBlockHeader) encode() [80]byte {
	var b [80]byte
	binary.LittleEndian.PutUint32(b[0:4], uint32(h.Version))
	copy(b[4:36], h.PrevBlock[:])
	copy(b[36:68], h.MerkleRoot[:])
	binary.LittleEndian.PutUint32(b[68:72], h.Timestamp)
	binary.LittleEndian.PutUint32(b[72:76], h.Bits)
	binary.LittleEndian.PutUint32(b[76:80], h.Nonce)
	return b
}

func submitAuxBlockSolution(
	j HeaderJournal,
	paths *DataPaths,
	rawBlocks *store.RawBlockStore,
	chainName string,
	params []json.RawMessage,
) (interface{}, int, string) {
	hash, code, msg := parseOneBlockHashParam([]json.RawMessage{params[0]}, "submitauxblock")
	if code != 0 {
		return nil, code, msg
	}
	var auxhex string
	if err := json.Unmarshal(params[1], &auxhex); err != nil {
		return nil, -8, "submitauxblock: auxpow must be a hex string"
	}
	auxhex = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(auxhex), "0x"))
	if len(auxhex)%2 != 0 {
		return nil, -8, "submitauxblock: AuxPow decode failed"
	}
	auxRaw, err := hex.DecodeString(auxhex)
	if err != nil {
		return nil, -8, "submitauxblock: AuxPow decode failed"
	}
	pending, ok := globalAuxCache.getByHash(hash)
	if !ok {
		return nil, -8, "submitauxblock: block hash unknown (create a template with createauxblock first)"
	}
	aux, err := wire.ReadAuxPow(bytes.NewReader(auxRaw))
	if err != nil {
		return nil, -8, "submitauxblock: AuxPow decode failed"
	}
	payload, err := assembleAuxBlockPayload(pending, aux)
	if err != nil {
		return nil, -8, "submitauxblock: " + err.Error()
	}
	hexPayload := hex.EncodeToString(payload)
	res, code, msg := execSubmitBlock(j, paths.HeaderAux, rawBlocks, paths, chainName, []json.RawMessage{
		json.RawMessage(`"` + hexPayload + `"`),
	})
	if code != 0 {
		return nil, code, msg
	}
	if s, ok := res.(string); ok && s != "" {
		return false, 0, ""
	}
	return true, 0, ""
}

func assembleAuxBlockPayload(p *pendingAuxBlock, aux *wire.AuxPow) ([]byte, error) {
	if p == nil || aux == nil {
		return nil, fmt.Errorf("nil template or auxpow")
	}
	auxBytes, err := wire.SerializeAuxPow(aux)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	_, _ = buf.Write(p.header80[:])
	_, _ = buf.Write(auxBytes)
	if err := wire.WriteCompactSize(&buf, uint64(len(p.txRaws))); err != nil {
		return nil, err
	}
	for _, raw := range p.txRaws {
		_, _ = buf.Write(raw)
	}
	return buf.Bytes(), nil
}
