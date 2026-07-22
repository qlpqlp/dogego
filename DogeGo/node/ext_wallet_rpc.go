// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"encoding/json"
	"fmt"

	"dogego/extensions"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
)

type extensionWalletRPC struct {
	chainName              string
	j                      *store.HeaderJournal
	pool                   *mempool.Pool
	paths                  *rpc.DataPaths
	raw                    *store.RawBlockStore
	txIndex                *store.TxIndex
	relayTx                func([]byte) error
	allowUnverifiedMempool bool
}

func (b *extensionWalletRPC) WalletUnlocked() bool {
	return b != nil && b.paths != nil && b.paths.WalletIsUnlocked != nil && b.paths.WalletIsUnlocked()
}

func (b *extensionWalletRPC) Call(method string, params []json.RawMessage) (interface{}, error) {
	if b == nil || b.paths == nil {
		return nil, fmt.Errorf("wallet not configured")
	}
	resp := rpc.Dispatch(
		b.chainName, b.j, b.pool, b.paths, b.raw, b.txIndex,
		b.relayTx, b.allowUnverifiedMempool,
		method, params, json.RawMessage(`1`),
	)
	return rpc.ParseDispatchResponse(resp)
}

func newExtensionWalletRPC(
	chainName string,
	j *store.HeaderJournal,
	pool *mempool.Pool,
	paths *rpc.DataPaths,
	raw *store.RawBlockStore,
	txIndex *store.TxIndex,
	relayTx func([]byte) error,
	allowUnverifiedMempool bool,
) extensions.WalletRPCCaller {
	return &extensionWalletRPC{
		chainName:              chainName,
		j:                      j,
		pool:                   pool,
		paths:                  paths,
		raw:                    raw,
		txIndex:                txIndex,
		relayTx:                relayTx,
		allowUnverifiedMempool: allowUnverifiedMempool,
	}
}

func wireExtensionWalletRPC(
	extMgr *extensions.Manager,
	chainName string,
	j *store.HeaderJournal,
	pool *mempool.Pool,
	paths *rpc.DataPaths,
	raw *store.RawBlockStore,
	txIndex *store.TxIndex,
	relayTx func([]byte) error,
	allowUnverifiedMempool bool,
) {
	if extMgr == nil {
		return
	}
	extMgr.SetWalletRPC(newExtensionWalletRPC(chainName, j, pool, paths, raw, txIndex, relayTx, allowUnverifiedMempool))
}
