// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"

	"dogego/chain"
	"dogego/mempool"
	"dogego/wire"
)

// walletMempoolTxSnapshot captures wallet send/receive rows for a mempool txid (for abandontransaction).
func walletMempoolTxSnapshot(chainName string, paths *DataPaths, pool *mempool.Pool, txid string) (category, addr string, amountKoinu int64, ok bool) {
	if pool == nil || paths == nil {
		return "", "", 0, false
	}
	raw, err := pool.GetRawByTxID(txid)
	if err != nil || len(raw) == 0 {
		return "", "", 0, false
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return "", "", 0, false
	}
	tracked := rpcWalletTrackedScripts(paths)
	if len(tracked) == 0 {
		return "", "", 0, false
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", "", 0, false
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", "", 0, false
	}
	scriptAddr := make(map[string]string)
	for _, pk := range tracked {
		scriptAddr[hex.EncodeToString(pk)] = chain.ScriptPubKeyAddress(pk, p.PubkeyHashAddrID, p.ScriptHashAddrID)
	}
	spendSet := walletSpendScriptSet(paths)
	recvByAddr := make(map[string]int64)
	var spent int64
	sendAddr := ""
	for _, o := range tx.Vout {
		a, ok := scriptAddr[hex.EncodeToString(o.PkScript)]
		if !ok || a == "" {
			continue
		}
		recvByAddr[a] += o.Value
	}
	if len(spendSet) > 0 && paths.Utxo != nil {
		for _, in := range tx.Vin {
			e, ok := paths.Utxo.LookupOutpoint(in.PrevHash, in.PrevIdx)
			if !ok {
				continue
			}
			if _, mine := spendSet[hex.EncodeToString(e.PkScript)]; mine {
				spent += e.Value
				if sendAddr == "" {
					sendAddr = scriptAddr[hex.EncodeToString(e.PkScript)]
				}
			}
		}
	}
	if spent > 0 {
		var recv int64
		for _, v := range recvByAddr {
			recv += v
		}
		netAmt := spent - recv
		if netAmt <= 0 {
			netAmt = spent
		}
		if sendAddr == "" {
			sendAddr = rpcWalletDefaultAddress(paths)
		}
		return "send", sendAddr, -netAmt, true
	}
	for a, recv := range recvByAddr {
		if recv > 0 {
			return "receive", a, recv, true
		}
	}
	return "", "", 0, false
}
