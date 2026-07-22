// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
	"dogego/wire"
)

const maxRawTxBytes = 4 << 20 // align with wire per-tx cap

// execSendRawTransaction accepts hex-encoded raw transaction bytes, decodes them,
// runs full-node mempool admission when allowUnverified is false, adds to the mempool,
// and optionally invokes relayTx with the same raw bytes (P2P `inv` MSG_TX per peer feefilter).
// Optional second parameter allowhighfees (boolean) matches Dogecoin Core RPC arity;
// DogeGo has no separate max-tx-fee relay cap, so the flag is accepted but does not change fee policy.
func execSendRawTransaction(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, params []json.RawMessage, relayTx func([]byte) error, allowUnverified bool, net chain.Network) (result interface{}, errCode int, errMsg string) {
	if pool == nil {
		return nil, -18, "sendrawtransaction: mempool not available"
	}
	if len(params) > 3 {
		return nil, -32602, "Wrong number of arguments"
	}
	if len(params) < 1 {
		return nil, -8, "sendrawtransaction: hex string required"
	}
	if len(params) > 1 {
		var allowHighFees bool
		if err := json.Unmarshal(params[1], &allowHighFees); err != nil {
			return nil, -8, "sendrawtransaction: allowhighfees must be boolean"
		}
		_ = allowHighFees
	}
	maxFeeRate, feeErr := parseMaxFeeRateDOGEPerKB(params, 2, "sendrawtransaction")
	if feeErr != "" {
		return nil, -8, feeErr
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "sendrawtransaction: bad hex param"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	raw, err := hex.DecodeString(hexStr)
	if err != nil || len(raw) == 0 {
		return nil, -22, "TX decode failed"
	}
	if len(raw) > maxRawTxBytes {
		return nil, -8, fmt.Sprintf("sendrawtransaction: transaction too large (%d bytes)", len(raw))
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, -22, "TX decode failed"
	}
	rpcTxid := txidToRPC(tx.TxHash())
	if txConfirmed(txIndex, rpcTxid) {
		return nil, -27, "transaction already in block chain"
	}
	inMempool := pool.ContainsTxID(rpcTxid)
	if !inMempool && !allowUnverified {
		adm := newMempoolAdmission(pool, txIndex, blocks, j, paths, net)
		if err := acceptMempoolTxRPC(raw, tx, pool, paths, adm); err != nil {
			code, msg := consensus.SendRawTransactionRPCError(err)
			return nil, code, msg
		}
		if maxFeeRate > 0 {
			view := mempoolAdmissionView(pool, txIndex, blocks)
			if pkg, err := consensus.PackageFeeReportForTx(tx, pool, view); err == nil {
				eff := float64(pkg.EffectiveRatePerKB) / 1e8
				if eff > maxFeeRate {
					return nil, -25, "max-fee-exceeded"
				}
			}
		}
	}
	if !inMempool {
		view := mempoolAdmissionView(pool, txIndex, blocks)
		fees, sizes := consensus.MempoolEvictionMaps(pool, view)
		consensus.AddCandidateEvictionEntry(tx, raw, view, fees, sizes)
		if err := pool.AddWithEviction(raw, fees, sizes); err != nil {
			return nil, -4, "sendrawtransaction: " + err.Error()
		}
		TrackMempoolTxFee(paths, tx, raw, pool, txIndex, blocks, j, net)
	}
	if relayTx != nil {
		_ = relayTx(raw)
	}
	if walletTxSpendsFromWallet(paths, tx) {
		walletRecordTxHex(paths, rpcTxid, hexStr)
	}
	return rpcTxid, 0, ""
}

func txConfirmed(txIndex *store.TxIndex, rpcTxid string) bool {
	if txIndex == nil {
		return false
	}
	_, _, err := txIndex.Lookup(rpcTxid)
	return err == nil
}

// acceptMempoolTxRPC runs admission; when OrphanPool is wired, stores missing-parent txs like P2P.
func acceptMempoolTxRPC(raw []byte, tx *wire.Tx, pool *mempool.Pool, paths *DataPaths, adm consensus.MempoolAdmission) error {
	if paths != nil && paths.OrphanPool != nil {
		err := consensus.AcceptMempoolTxWithOrphans(raw, tx, pool, paths.OrphanPool, adm, "rpc")
		if errors.Is(err, consensus.ErrOrphanTx) {
			return consensus.ErrMissingPrevout
		}
		return err
	}
	return consensus.AcceptMempoolTxAdmission(tx, adm)
}

func minRelayFeeFromPaths(paths *DataPaths) uint64 {
	var peer, rolling uint64
	if paths != nil {
		if paths.FeeFilter != nil {
			peer = paths.FeeFilter()
		}
		if paths.MempoolMinRelayFee != nil {
			rolling = paths.MempoolMinRelayFee()
		}
	}
	return consensus.EffectiveMinRelayFeePerKB(peer, rolling)
}

func fullRBFFromPaths(paths *DataPaths) bool {
	if paths != nil && paths.FullRBF != nil {
		return paths.FullRBF()
	}
	return false
}
