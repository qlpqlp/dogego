// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	"dogego/consensus"
	"dogego/wire"
)

func pqSendTxHex(t *testing.T) string {
	t.Helper()
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(consensus.PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevIdx: 0xffffffff, Sequence: 0xffffffff}},
		Vout: []wire.TxOut{
			{Value: 1, PkScript: []byte{0x51}},
			{Value: 0, PkScript: script},
		},
	}
	raw, err := tx.Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(raw)
}

func TestProbeCoreWalletPqSendOK(t *testing.T) {
	pqHex := pqSendTxHex(t)
	out := ProbeCoreWallet(func(method string, params []json.RawMessage) map[string]interface{} {
		switch method {
		case "getwalletinfo":
			return map[string]interface{}{"result": map[string]interface{}{
				"walletname": "main", "format": "hd", "keypoolsize": float64(99),
				"pq_commitments_enabled": true,
			}}
		case "getbalance":
			return map[string]interface{}{"result": float64(1)}
		case "getnewaddress":
			return map[string]interface{}{"result": "DAddr"}
		case "validateaddress":
			return map[string]interface{}{"result": map[string]interface{}{"isvalid": true}}
		case "dogego_listwalletaddresses":
			return map[string]interface{}{"result": []interface{}{}}
		case "enumeratesigners":
			return map[string]interface{}{"result": []interface{}{}}
		case "listtransactions":
			return map[string]interface{}{"result": []interface{}{
				map[string]interface{}{
					"txid": "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
					"category": "send", "confirmations": float64(10),
				},
			}}
		case "gettransaction":
			return map[string]interface{}{"result": map[string]interface{}{
				"hex": pqHex, "fee": float64(-0.001),
			}}
		default:
			if r, ok := probeInvokePsbtSkip(method); ok {
				return r
			}
			return map[string]interface{}{"result": nil}
		}
	})
	if !out.PqCommitmentsOK || !out.WalletPqSendOK || out.WalletPqTag != consensus.PQTagFalcon {
		t.Fatalf("pq=%+v", out)
	}
	if !out.WalletTxHexOK || !out.WalletTxFeeOK {
		t.Fatalf("metadata=%+v", out)
	}
}
