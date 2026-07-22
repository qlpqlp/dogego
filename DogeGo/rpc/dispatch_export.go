// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"

	"dogego/mempool"
	"dogego/store"
)

// Dispatch runs a single JSON-RPC method and returns a Core-shaped response object.
func Dispatch(
	chainName string,
	j HeaderJournal,
	pool *mempool.Pool,
	paths *DataPaths,
	raw *store.RawBlockStore,
	txIndex *store.TxIndex,
	relayTx func([]byte) error,
	allowUnverifiedMempool bool,
	method string,
	params []json.RawMessage,
	id json.RawMessage,
) map[string]interface{} {
	if id == nil {
		id = json.RawMessage(`1`)
	}
	return dispatchRequest(chainName, j, pool, paths, raw, txIndex, relayTx, allowUnverifiedMempool, method, params, id)
}
