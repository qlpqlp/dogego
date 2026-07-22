// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pqcrypto"
	"dogego/store"
	"dogego/wire"
)

// execDogegoCreatePQCarrier builds unsigned TX_C + TX_R carrier material from a funded TX_BASE hex.
// Params: { "tx_base_hex", "input_index"?, "tag"?, "carrier_value"?, "pk_script_hex"? }
func execDogegoCreatePQCarrier(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -8, "dogego_createpqcarrier: expected 1 object argument"
	}
	if !rpcWalletPqCarrierEnabled(paths) {
		return nil, -8, "dogego_createpqcarrier: requires setwalletflag pq_carrier true"
	}
	var req struct {
		TxBaseHex    string  `json:"tx_base_hex"`
		InputIndex   int     `json:"input_index"`
		Tag          string  `json:"tag"`
		CarrierValue float64 `json:"carrier_value"`
		PkScriptHex  string  `json:"pk_script_hex"`
	}
	if err := json.Unmarshal(params[0], &req); err != nil {
		return nil, -8, "dogego_createpqcarrier: bad argument object"
	}
	txBase, code, msg := decodeRPCTxHex(req.TxBaseHex, "dogego_createpqcarrier", "tx_base_hex")
	if code != 0 {
		return nil, code, msg
	}
	tag := strings.TrimSpace(strings.ToUpper(req.Tag))
	if tag == "" {
		tag = consensus.PQTagFalcon
	}
	scheme, err := pqcrypto.ByOPReturnTag(tag)
	if err != nil {
		return nil, -8, "dogego_createpqcarrier: unknown tag (FLC1/DIL2/RCG4)"
	}
	if paths == nil || paths.WalletPQCarrierKeyMaterial == nil {
		return nil, -1, "dogego_createpqcarrier: wallet PQ keys unavailable"
	}
	_, pk, sk, err := paths.WalletPQCarrierKeyMaterial(tag)
	if err != nil {
		return nil, -1, "dogego_createpqcarrier: "+err.Error()
	}
	pkScript, code, msg := decodeOptionalScriptHex(req.PkScriptHex, "dogego_createpqcarrier", "pk_script_hex")
	if code != 0 {
		return nil, code, msg
	}
	if len(pkScript) == 0 {
		return nil, -8, "dogego_createpqcarrier: pk_script_hex required for sighash"
	}
	carrierValue := consensus.PQCarrierMinOutputKoinu()
	if req.CarrierValue > 0 {
		carrierValue = int64(req.CarrierValue * 1e8)
	}
	plan, err := consensus.BuildPQCarrierTransactions(txBase, scheme, pk, sk, req.InputIndex, pkScript, wire.SigHashAll, carrierValue)
	if err != nil {
		return nil, -8, "dogego_createpqcarrier: "+err.Error()
	}
	return pqCarrierPlanRPC(plan)
}

// execDogegoVerifyPQCarrier validates TX_C + TX_R carrier pair off-chain.
func execDogegoVerifyPQCarrier(params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 1 {
		return nil, -8, "dogego_verifypqcarrier: expected 1 object argument"
	}
	var req struct {
		TxCHex      string `json:"txc_hex"`
		TxRHex      string `json:"txr_hex"`
		InputIndex  int    `json:"input_index"`
		PkScriptHex string `json:"pk_script_hex"`
		Tag         string `json:"tag"`
	}
	if err := json.Unmarshal(params[0], &req); err != nil {
		return nil, -8, "dogego_verifypqcarrier: bad argument object"
	}
	txc, code, msg := decodeRPCTxHex(req.TxCHex, "dogego_verifypqcarrier", "txc_hex")
	if code != 0 {
		return nil, code, msg
	}
	txr, code, msg := decodeRPCTxHex(req.TxRHex, "dogego_verifypqcarrier", "txr_hex")
	if code != 0 {
		return nil, code, msg
	}
	pkScript, code, msg := decodeOptionalScriptHex(req.PkScriptHex, "dogego_verifypqcarrier", "pk_script_hex")
	if code != 0 {
		return nil, code, msg
	}
	if len(pkScript) == 0 {
		return nil, -8, "dogego_verifypqcarrier: pk_script_hex required"
	}
	tag := strings.TrimSpace(strings.ToUpper(req.Tag))
	var scheme pqcrypto.Scheme
	var err error
	if tag != "" {
		scheme, err = pqcrypto.ByOPReturnTag(tag)
		if err != nil {
			return nil, -8, "dogego_verifypqcarrier: unknown tag"
		}
	} else if c, _, ok := consensus.DetectPQCommitmentInTx(txc); ok {
		scheme, _ = pqcrypto.ByOPReturnTag(c.Tag)
	}
	out, err := consensus.VerifyPQCarrierPair(txc, txr, req.InputIndex, pkScript, wire.SigHashAll, scheme)
	if err != nil {
		return nil, -8, "dogego_verifypqcarrier: "+err.Error()
	}
	out["mode"] = "carrier_scriptsig"
	return out, 0, ""
}

// execDogegoSendPQCarrier funds, signs, and broadcasts TX_C + TX_R from the built-in wallet.
func execDogegoSendPQCarrier(
	chainName string,
	paths *DataPaths,
	j HeaderJournal,
	raw *store.RawBlockStore,
	pool *mempool.Pool,
	txIndex *store.TxIndex,
	params []json.RawMessage,
	relayTx func([]byte) error,
	allowUnverified bool,
	net chain.Network,
) (interface{}, int, string) {
	if len(params) < 2 {
		return nil, -8, "dogego_sendpqcarrier: expected address and amount"
	}
	if !rpcWalletPqCarrierEnabled(paths) {
		return nil, -8, "dogego_sendpqcarrier: requires setwalletflag pq_carrier true"
	}
	var addr string
	var amount float64
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return nil, -8, "dogego_sendpqcarrier: bad address"
	}
	if err := json.Unmarshal(params[1], &amount); err != nil {
		return nil, -8, "dogego_sendpqcarrier: bad amount"
	}
	var fundOpts map[string]interface{}
	if len(params) > 2 {
		_ = json.Unmarshal(params[2], &fundOpts)
	}
	fundOpts = cloneSendFundOptions(fundOpts)
	tag := consensus.PQTagFalcon
	if fundOpts != nil {
		if t, ok := fundOpts["pq_tag"].(string); ok && strings.TrimSpace(t) != "" {
			tag = strings.TrimSpace(strings.ToUpper(t))
		}
	}
	fundOpts["skip_pq_commitment"] = true
	fundedHex, code, msg := walletFundUnsignedPayment(chainName, paths, j, raw, txIndex, map[string]float64{addr: amount}, "dogego_sendpqcarrier", fundOpts)
	if code != 0 {
		return nil, code, msg
	}
	txBase, err := wire.DeserializeTx(mustDecodeHex(fundedHex))
	if err != nil {
		return nil, -8, "dogego_sendpqcarrier: decode funded tx"
	}
	scheme, err := pqcrypto.ByOPReturnTag(tag)
	if err != nil {
		return nil, -8, "dogego_sendpqcarrier: unknown pq_tag"
	}
	if paths.WalletPQCarrierKeyMaterial == nil {
		return nil, -1, "dogego_sendpqcarrier: wallet PQ keys unavailable"
	}
	_, pk, sk, err := paths.WalletPQCarrierKeyMaterial(tag)
	if err != nil {
		return nil, -1, "dogego_sendpqcarrier: "+err.Error()
	}
	pkScript := rpcWalletPrimarySpendScript(paths)
	if len(pkScript) == 0 {
		return nil, -1, "dogego_sendpqcarrier: wallet spend script unavailable"
	}
	plan, err := consensus.BuildPQCarrierTransactions(txBase, scheme, pk, sk, 0, pkScript, wire.SigHashAll, consensus.PQCarrierMinOutputKoinu())
	if err != nil {
		return nil, -8, "dogego_sendpqcarrier: "+err.Error()
	}
	txcUnsigned, err := serializeTxHex(plan.TXC)
	if err != nil {
		return nil, -1, "dogego_sendpqcarrier: "+err.Error()
	}
	txcHex, code, msg := walletSignRawHex(chainName, paths, txcUnsigned)
	if code != 0 {
		return nil, code, "dogego_sendpqcarrier tx_c: "+msg
	}
	signedTXC, _ := wire.DeserializeTx(mustDecodeHex(txcHex))
	consensus.RefreshPQCarrierTXRPrevouts(signedTXC, plan.TXR)
	txrHex, err := serializeTxHex(plan.TXR)
	if err != nil {
		return nil, -1, "dogego_sendpqcarrier tx_r: "+err.Error()
	}
	txcParam, _ := json.Marshal(txcHex)
	txcRes, code, msg := execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{txcParam}, relayTx, allowUnverified, net)
	if code != 0 {
		return nil, code, "dogego_sendpqcarrier broadcast tx_c: "+msg
	}
	txrParam, _ := json.Marshal(txrHex)
	txrRes, code, msg := execSendRawTransaction(pool, txIndex, raw, j, paths, []json.RawMessage{txrParam}, relayTx, allowUnverified, net)
	if code != 0 {
		return nil, code, "dogego_sendpqcarrier broadcast tx_r: "+msg
	}
	txcID, _ := txcRes.(string)
	txrID, _ := txrRes.(string)
	return map[string]interface{}{
		"txc_txid":    txcID,
		"txr_txid":    txrID,
		"commitment":  hex.EncodeToString(plan.Commitment[:]),
		"scheme":      plan.Scheme.Name(),
		"tag":         plan.OPReturnTag,
		"carrier_tag": plan.CarrierTag8,
		"parts":       plan.PartTotal,
	}, 0, ""
}

func pqCarrierPlanRPC(plan *consensus.PQCarrierBuildPlan) (map[string]interface{}, int, string) {
	if plan == nil {
		return nil, -1, "dogego_createpqcarrier: empty plan"
	}
	txcHex, err := serializeTxHex(plan.TXC)
	if err != nil {
		return nil, -1, err.Error()
	}
	txrHex, err := serializeTxHex(plan.TXR)
	if err != nil {
		return nil, -1, err.Error()
	}
	baseHex, err := serializeTxHex(plan.TXBase)
	if err != nil {
		return nil, -1, err.Error()
	}
	return map[string]interface{}{
		"tx_base_hex": baseHex,
		"txc_hex":     txcHex,
		"txr_hex":     txrHex,
		"commitment":  hex.EncodeToString(plan.Commitment[:]),
		"sighash32":   hex.EncodeToString(plan.Sighash32[:]),
		"scheme":      plan.Scheme.Name(),
		"tag":         plan.OPReturnTag,
		"carrier_tag": plan.CarrierTag8,
		"parts":       plan.PartTotal,
		"pk_len":      len(plan.PK),
		"sig_len":     len(plan.Sig),
	}, 0, ""
}

func decodeRPCTxHex(h, prefix, field string) (*wire.Tx, int, string) {
	h = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(h), "0x"))
	if h == "" {
		return nil, -8, prefix + ": " + field + " required"
	}
	raw, err := hex.DecodeString(h)
	if err != nil {
		return nil, -8, prefix + ": invalid " + field
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return nil, -8, prefix + ": cannot decode " + field
	}
	return tx, 0, ""
}

func decodeOptionalScriptHex(h, prefix, field string) ([]byte, int, string) {
	h = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(h), "0x"))
	if h == "" {
		return nil, 0, ""
	}
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, -8, prefix + ": invalid " + field
	}
	return b, 0, ""
}

func serializeTxHex(tx *wire.Tx) (string, error) {
	b, err := tx.Serialize()
	if err != nil {
		return "", fmt.Errorf("serialize tx: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func rpcWalletPqCarrierEnabled(paths *DataPaths) bool {
	return rpcWalletPqCarrierSendEnabled(paths)
}

func rpcWalletPrimarySpendScript(paths *DataPaths) []byte {
	scripts := rpcWalletSpendScripts(paths)
	if len(scripts) == 0 {
		return nil
	}
	return scripts[0]
}

func mustDecodeHex(s string) []byte {
	b, _ := hex.DecodeString(strings.TrimSpace(strings.TrimPrefix(strings.ToLower(s), "0x")))
	return b
}
