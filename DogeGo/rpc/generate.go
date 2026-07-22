// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/clock"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const defaultGenerateMaxTries = 1_000_000
const rebootTestnetGenerateMaxTries = 10_000_000

func effectiveGenerateMaxTries(p chain.Params, maxTries uint64) uint64 {
	if p.IsRebootTestnet() && maxTries < rebootTestnetGenerateMaxTries {
		return rebootTestnetGenerateMaxTries
	}
	return maxTries
}

// execGenerateToAddress mines up to nblocks legacy blocks paying to address (Core generatetoaddress).
// Merge-mined heights require createauxblock instead.
func execGenerateToAddress(
	j HeaderJournal,
	aux *store.HeaderAuxJournal,
	paths *DataPaths,
	raw *store.RawBlockStore,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	chainName string,
	params []json.RawMessage,
) (interface{}, int, string) {
	if len(params) < 2 || len(params) > 4 {
		return nil, -32602, "Wrong number of arguments"
	}
	var nblocks float64
	if err := json.Unmarshal(params[0], &nblocks); err != nil {
		return nil, -8, "generatetoaddress: invalid nblocks"
	}
	if nblocks < 1 || nblocks != float64(int64(nblocks)) {
		return nil, -8, "generatetoaddress: nblocks must be a positive integer"
	}
	var addr string
	if err := json.Unmarshal(params[1], &addr); err != nil {
		return nil, -8, "generatetoaddress: address must be a string"
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, -8, "generatetoaddress: address required"
	}
	maxTries := uint64(defaultGenerateMaxTries)
	if len(params) > 2 && strings.TrimSpace(string(params[2])) != "null" {
		var mt float64
		if err := json.Unmarshal(params[2], &mt); err != nil || mt < 1 || mt != float64(uint64(mt)) {
			return nil, -8, "generatetoaddress: invalid maxtries"
		}
		maxTries = uint64(mt)
	}
	if code, msg := parseOptionalMaxTriesAuxPow(params, 2, 3, "generatetoaddress"); code != 0 {
		return nil, code, msg
	}
	h160, err := p2pkhScriptFromAddress(chainName, addr)
	if err != nil {
		if strings.Contains(err.Error(), "invalid") {
			return nil, -5, "Error: Invalid address"
		}
		return nil, -8, "generatetoaddress: " + err.Error()
	}
	hj, ok := j.(*store.HeaderJournal)
	if !ok || hj == nil {
		return nil, -1, "generatetoaddress: header journal not available"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -1, err.Error()
	}
	out := make([]interface{}, 0, int(nblocks))
	for i := 0; i < int(nblocks); i++ {
		display, payload, err := mineLegacyBlockToAddress(hj, raw, paths, pool, txIndex, p, net, h160, maxTries)
		if err != nil {
			return nil, -1, "generatetoaddress: " + err.Error()
		}
		if paths != nil && raw != nil {
			hexPayload := hex.EncodeToString(payload)
			res, code, msg := execSubmitBlock(j, aux, raw, paths, chainName, []json.RawMessage{
				json.RawMessage(`"` + hexPayload + `"`),
			})
			if code != 0 {
				return nil, code, msg
			}
			if s, ok := res.(string); ok && s != "" {
				return nil, -1, "generatetoaddress: " + s
			}
		} else {
			tip, err := hj.TipHeight()
			if err != nil {
				return nil, -1, "generatetoaddress: " + err.Error()
			}
			if _, err := consensus.ExtendHeadersFromPayload(hj, aux, p, payload, tip, clock.UnixNow()); err != nil {
				return nil, -1, "generatetoaddress: " + err.Error()
			}
		}
		out = append(out, display)
	}
	return out, 0, ""
}

func mineLegacyBlockToAddress(j *store.HeaderJournal, raw *store.RawBlockStore, paths *DataPaths, pool *mempool.Pool, ix *store.TxIndex, p chain.Params, net chain.Network, h160 [20]byte, maxTries uint64) (display string, payload []byte, err error) {
	tip, _, _ := activeChainFromJournal(j, raw, paths)
	next := tip + 1
	dc := consensus.LookupConsensus(net, next)
	if !dc.AllowLegacyBlocks {
		return "", nil, fmt.Errorf("height %d requires merge-mined auxpow blocks; use createauxblock", next)
	}
	h80tip, err := j.ReadHeaderAt(tip)
	if err != nil {
		return "", nil, err
	}
	cur := clock.UnixNow()
	mtp, err := medianTimePastAfterPrev(j, tip)
	if err != nil {
		return "", nil, err
	}
	if cur <= mtp {
		cur = mtp + 1
	}
	blockTime := uint32(cur)
	bits, err := consensus.NextBlockBits(j, net, next, blockTime)
	if err != nil {
		return "", nil, err
	}
	pkScript := consensus.P2PKHPkScript(h160)
	var tipHdr [80]byte
	copy(tipHdr[:], h80tip)
	prevLE := pow.BlockHashLE(tipHdr[:])
	subsidy := consensus.BlockSubsidy(next, prevLE, net)
	extraTxs, mempoolFees := legacyBlockMempoolTxs(pool, ix, raw, paths)
	coin := consensus.BuildCoinbaseTx(next, subsidy+mempoolFees, pkScript)
	txs := make([]*wire.Tx, 0, 1+len(extraTxs))
	txs = append(txs, coin)
	txs = append(txs, extraTxs...)
	merkle := wire.BlockMerkleRoot(txs)
	var h80 [80]byte
	binary.LittleEndian.PutUint32(h80[0:4], 1)
	copy(h80[4:36], prevLE[:])
	copy(h80[36:68], merkle[:])
	binary.LittleEndian.PutUint32(h80[68:72], blockTime)
	binary.LittleEndian.PutUint32(h80[72:76], bits)
	// Reboot testnet uses real scrypt PoW (Core-aligned); allow a larger search budget than mainnet RPC default.
	skipPoW := p.RelaxedPoW
	maxTries = effectiveGenerateMaxTries(p, maxTries)
	found := skipPoW
	for nonce := uint32(0); !found && uint64(nonce) < maxTries; nonce++ {
		binary.LittleEndian.PutUint32(h80[76:80], nonce)
		ph := pow.ScryptHashLE(h80[:])
		if pow.CheckProofOfWorkLE(ph, bits, consensus.PowLimitHex) == nil {
			found = true
			break
		}
	}
	if !found {
		return "", nil, fmt.Errorf("exceeded maxtries (%d) mining block at height %d", maxTries, next)
	}
	var buf []byte
	buf = append(buf, h80[:]...)
	buf = append(buf, byte(len(txs)))
	for _, tx := range txs {
		txRaw, err := tx.Serialize()
		if err != nil {
			return "", nil, err
		}
		buf = append(buf, txRaw...)
	}
	display = pow.BlockHashHex(h80[:])
	return display, buf, nil
}

// legacyBlockMempoolTxs selects mempool transactions for a legacy block (same greedy policy as getblocktemplate).
func legacyBlockMempoolTxs(pool *mempool.Pool, ix *store.TxIndex, raw *store.RawBlockStore, paths *DataPaths) ([]*wire.Tx, int64) {
	if pool == nil {
		return nil, 0
	}
	var view consensus.PrevOutView
	if paths != nil && paths.Utxo != nil {
		view = consensus.AdmissionPrevOutViewWithUtxo(pool, paths.Utxo, ix, raw)
	} else {
		view = consensus.AdmissionPrevOutView(pool, ix, raw)
	}
	sel, err := consensus.SelectBlockTemplateTxs(pool, view, consensus.MaxBlockWeight)
	if err != nil || len(sel.Txs) == 0 {
		return nil, sel.TotalFees
	}
	out := make([]*wire.Tx, 0, len(sel.Txs))
	for _, ent := range sel.Txs {
		rawTx, err := hex.DecodeString(ent.Data)
		if err != nil {
			continue
		}
		tx, err := wire.DeserializeTx(rawTx)
		if err != nil {
			continue
		}
		out = append(out, tx)
	}
	return out, sel.TotalFees
}
