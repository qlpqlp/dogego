// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"

	"dogego/wire"
)

// execAnalyzePsbt reports PSBT completion status (Core analyzepsbt subset).
func execAnalyzePsbt(params []json.RawMessage) (interface{}, int, string) {
	p, code, msg := loadPSBTParam(params)
	if code != 0 {
		if !strings.HasPrefix(msg, "analyzepsbt:") {
			msg = "analyzepsbt: " + msg
		}
		return nil, code, msg
	}
	return analyzePsbtJSON(p), 0, ""
}

func analyzePsbtJSON(p *wire.Psbt) map[string]interface{} {
	inputs := make([]interface{}, len(p.Inputs))
	allUTXO := true
	allFinal := true
	var inSum, outSum int64
	for i := range p.Inputs {
		hasUTXO := p.InputHasUTXO(i)
		isFinal := p.InputHasFinalScriptSig(i)
		if !hasUTXO {
			allUTXO = false
		}
		if !isFinal {
			allFinal = false
		}
		if v, ok := p.InputValue(i); ok {
			inSum += v
		}
		row := map[string]interface{}{
			"has_utxo": hasUTXO,
			"is_final": isFinal,
		}
		if !isFinal {
			missing := psbtInputMissing(p, i)
			if len(missing) > 0 {
				row["missing"] = missing
			}
			row["next"] = "signer"
		}
		inputs[i] = row
	}
	for _, o := range p.UnsignedTx.Vout {
		outSum += o.Value
	}
	out := map[string]interface{}{
		"inputs": inputs,
		"next":   psbtGlobalNext(allFinal, allUTXO),
	}
	if allUTXO {
		vsize, _ := wire.TransactionVirtualSize(p.UnsignedTx)
		out["estimated_vsize"] = vsize
		feeKoinu := inSum - outSum
		if feeKoinu >= 0 && vsize > 0 {
			out["fee"] = float64(feeKoinu) / 1e8
			out["estimated_feerate"] = float64(feeKoinu) * 1000 / float64(vsize) / 1e8
		}
	}
	if !allUTXO {
		out["error"] = "PSBT is not fully populated yet"
	}
	return out
}

func psbtGlobalNext(allFinal, allUTXO bool) string {
	if allFinal {
		return "extractor"
	}
	if allUTXO {
		return "signer"
	}
	return "updater"
}

func psbtInputMissing(p *wire.Psbt, i int) map[string]interface{} {
	missing := make(map[string]interface{})
	if !p.InputHasUTXO(i) {
		return missing
	}
	hasSig := false
	for _, kv := range p.Inputs[i] {
		if kv.Type == wire.PsbtInPartialSig && len(kv.Value) > 0 {
			hasSig = true
		}
		if kv.Type == wire.PsbtInFinalScriptSig && len(kv.Value) > 0 {
			hasSig = true
		}
	}
	if !hasSig {
		missing["signatures"] = []interface{}{}
	}
	for _, kv := range p.Inputs[i] {
		if kv.Type == wire.PsbtInRedeemScript {
			return missing
		}
	}
	// P2SH without redeem_script in PSBT - Core reports hash; we omit deep script analysis.
	return missing
}
