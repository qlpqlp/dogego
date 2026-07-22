// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"

	"dogego/chain"
	"dogego/mempool"
	"dogego/store"
)

// MineLegacyBlockPayload mines one legacy block at tip+1 (test + tooling helper).
func MineLegacyBlockPayload(
	j *store.HeaderJournal,
	raw *store.RawBlockStore,
	paths *DataPaths,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	net chain.Network,
	h160 [20]byte,
	maxTries uint64,
) (display string, payload []byte, err error) {
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", nil, err
	}
	return mineLegacyBlockToAddress(j, raw, paths, pool, txIndex, p, net, h160, maxTries)
}

// DefaultGenerateMaxTries is the RPC generatetoaddress default scrypt search budget.
func DefaultGenerateMaxTries() uint64 { return defaultGenerateMaxTries }

func MineAndSubmitLegacyBlock(
	j HeaderJournal,
	aux *store.HeaderAuxJournal,
	raw *store.RawBlockStore,
	paths *DataPaths,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	chainName string,
	h160 [20]byte,
	maxTries uint64,
) (display string, err error) {
	hj, ok := j.(*store.HeaderJournal)
	if !ok || hj == nil || raw == nil {
		return "", errMiningJournalUnavailable{}
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", err
	}
	display, payload, err := mineLegacyBlockToAddress(hj, raw, paths, pool, txIndex, p, net, h160, maxTries)
	if err != nil {
		return "", err
	}
	res, code, msg := execSubmitBlock(j, aux, raw, paths, chainName, []json.RawMessage{
		json.RawMessage(`"` + hex.EncodeToString(payload) + `"`),
	})
	if code != 0 {
		return "", &rpcMiningError{msg: msg}
	}
	if s, ok := res.(string); ok && s != "" {
		return "", errSubmitRejected{s}
	}
	return display, nil
}

type rpcMiningError struct{ msg string }

func (e *rpcMiningError) Error() string { return e.msg }

type errMiningJournalUnavailable struct{}

func (e errMiningJournalUnavailable) Error() string {
	return "mining requires header journal and raw block store"
}

type errSubmitRejected struct{ s string }

func (e errSubmitRejected) Error() string { return e.s }

// P2PKHScriptFromAddress decodes a P2PKH address to pubkey hash for mining.
func P2PKHScriptFromAddress(chainName, addr string) ([20]byte, error) {
	return p2pkhScriptFromAddress(chainName, addr)
}
