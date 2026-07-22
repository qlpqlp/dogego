// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/store"
)

func newMempoolAdmission(pool *mempool.Pool, txIndex *store.TxIndex, blocks *store.RawBlockStore, j HeaderJournal, paths *DataPaths, net chain.Network) consensus.MempoolAdmission {
	var utxo consensus.UtxoOutpointSource
	if paths != nil {
		utxo = paths.Utxo
	}
	var idx consensus.TxIndexer
	if txIndex != nil {
		idx = txIndex
	}
	var journal consensus.HeaderChain
	if j != nil {
		journal = j
	}
	adm := consensus.NewMempoolAdmissionWithUtxo(pool, pool, utxo, idx, blocks, journal, net)
	adm.MinRelayFeePerKB = minRelayFeeFromPaths(paths)
	adm.FullRBF = fullRBFFromPaths(paths)
	adm.Standard = standardPolicyFromPaths(paths)
	if paths != nil && paths.MempoolLimits != nil {
		paths.MempoolLimits().Apply(&adm)
	}
	if paths != nil && paths.MempoolAdmissionView != nil {
		adm.View = paths.MempoolAdmissionView
	}
	if paths != nil && paths.MempoolAdmissionIndex != nil {
		adm.Index = paths.MempoolAdmissionIndex
	}
	if paths != nil && paths.MempoolAdmissionJournal != nil {
		adm.Journal = paths.MempoolAdmissionJournal
	}
	return adm
}

func standardPolicyFromPaths(paths *DataPaths) consensus.StandardPolicy {
	if paths != nil && paths.Standard != nil {
		return paths.Standard()
	}
	return consensus.StandardPolicy{}
}
