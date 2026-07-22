// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pqcrypto"
	"dogego/store"
	"dogego/wire"
)

// rpcWalletPqCarrierSendEnabled reports whether wallet sends should publish TX_C + TX_R.
func rpcWalletPqCarrierSendEnabled(paths *DataPaths) bool {
	if paths != nil && paths.WalletPqCarrierEnabled != nil && paths.WalletPqCarrierEnabled() {
		return true
	}
	return rpcWalletPqCommitmentsEnabled(paths)
}

func walletShouldUsePQCarrier(paths *DataPaths, outputs map[string]float64, extraFundOpts map[string]interface{}) bool {
	if len(outputs) != 1 {
		return false
	}
	if !rpcWalletPqCarrierSendEnabled(paths) {
		return false
	}
	if paths == nil || paths.WalletPQCarrierKeyMaterial == nil {
		return false
	}
	opts := cloneSendFundOptions(extraFundOpts)
	if v, ok := opts["skip_pq_commitment"].(bool); ok && v {
		return false
	}
	if extraFundOpts != nil {
		if _, ok := extraFundOpts["pqcommit"]; ok {
			return false
		}
	}
	return true
}

type walletPQCarrierSendResult struct {
	TxcTxid string
	TxrTxid string
	TxcHex  string
	TxrHex  string
	Tag     string
}

func walletBroadcastPQCarrierPayment(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	raw *store.RawBlockStore,
	outputs map[string]float64,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
	errPrefix string,
	extraFundOpts map[string]interface{},
) (*walletPQCarrierSendResult, int, string) {
	fundOpts := cloneSendFundOptions(extraFundOpts)
	if fundOpts == nil {
		fundOpts = map[string]interface{}{}
	}
	tag := consensus.PQTagFalcon
	if fundOpts != nil {
		if t, ok := fundOpts["pq_tag"].(string); ok && strings.TrimSpace(t) != "" {
			tag = strings.TrimSpace(strings.ToUpper(t))
		}
	}
	fundOpts["skip_pq_commitment"] = true
	fundedHex, code, msg := walletFundUnsignedPayment(chainName, paths, j, raw, txIndex, outputs, errPrefix, fundOpts)
	if code != 0 {
		return nil, code, msg
	}
	txBase, err := wire.DeserializeTx(mustDecodeHex(fundedHex))
	if err != nil {
		return nil, -8, errPrefix+": decode funded tx"
	}
	scheme, err := pqcrypto.ByOPReturnTag(tag)
	if err != nil {
		return nil, -8, errPrefix+": unknown pq_tag"
	}
	_, pk, sk, err := paths.WalletPQCarrierKeyMaterial(tag)
	if err != nil {
		return nil, -1, errPrefix+": "+err.Error()
	}
	pkScript := rpcWalletPrimarySpendScript(paths)
	if len(pkScript) == 0 {
		return nil, -1, errPrefix+": wallet spend script unavailable"
	}
	plan, err := consensus.BuildPQCarrierTransactions(txBase, scheme, pk, sk, 0, pkScript, wire.SigHashAll, consensus.PQCarrierMinOutputKoinu())
	if err != nil {
		return nil, -8, errPrefix+": "+err.Error()
	}
	txcUnsigned, err := serializeTxHex(plan.TXC)
	if err != nil {
		return nil, -1, errPrefix+": "+err.Error()
	}
	txcHex, code, msg := walletSignRawHex(chainName, paths, txcUnsigned)
	if code != 0 {
		return nil, code, errPrefix+" tx_c: "+msg
	}
	signedTXC, _ := wire.DeserializeTx(mustDecodeHex(txcHex))
	consensus.RefreshPQCarrierTXRPrevouts(signedTXC, plan.TXR)
	txrHex, err := serializeTxHex(plan.TXR)
	if err != nil {
		return nil, -1, errPrefix+" tx_r: "+err.Error()
	}
	txcParam, _ := json.Marshal(txcHex)
	txcRes, code, msg := execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{txcParam}, relayTx, allowUnverified, net)
	if code != 0 {
		return nil, code, errPrefix+" broadcast tx_c: "+msg
	}
	txrParam, _ := json.Marshal(txrHex)
	_, code, msg = execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{txrParam}, relayTx, allowUnverified, net)
	if code != 0 {
		return nil, code, errPrefix+" broadcast tx_r: "+msg
	}
	txcID, _ := txcRes.(string)
	txrTx, _ := wire.DeserializeTx(mustDecodeHex(txrHex))
	txrID := mempool.TxIDDisplayHex(txrTx.TxHash())
	walletRecordTxHex(paths, txcID, txcHex)
	walletRecordTxHex(paths, txrID, txrHex)
	return &walletPQCarrierSendResult{
		TxcTxid: txcID,
		TxrTxid: txrID,
		TxcHex:  txcHex,
		TxrHex:  txrHex,
		Tag:     plan.OPReturnTag,
	}, 0, ""
}
