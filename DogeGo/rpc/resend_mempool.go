// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"

	"dogego/mempool"
)

// execResendWalletTransactions re-broadcasts all mempool transactions (no wallet in DogeGo).
func execResendWalletTransactions(pool *mempool.Pool, params []json.RawMessage, relayTx func([]byte) error) (interface{}, int, string) {
	if len(params) != 0 {
		return nil, -32602, "Wrong number of arguments"
	}
	if pool == nil || relayTx == nil {
		return []interface{}{}, 0, ""
	}
	ids, err := pool.RawMemPoolTxIDs()
	if err != nil {
		return nil, -1, "resendwallettransactions: " + err.Error()
	}
	sent := make([]interface{}, 0, len(ids))
	for _, id := range ids {
		raw, err := pool.GetRawByTxID(id)
		if err != nil {
			continue
		}
		if err := relayTx(raw); err != nil {
			continue
		}
		sent = append(sent, id)
	}
	return sent, 0, ""
}
