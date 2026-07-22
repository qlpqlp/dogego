// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/wire"
)

// execCombineRawTransaction implements combinerawtransaction for legacy transactions: all hex strings must
// deserialize to the same transaction except per-input scriptSig; non-empty scripts for an input must agree.
func execCombineRawTransaction(params []json.RawMessage) (string, int, string) {
	if len(params) < 1 {
		return "", -8, "combinerawtransaction: array of hex-encoded raw transactions required"
	}
	var hexStrs []string
	if err := json.Unmarshal(params[0], &hexStrs); err != nil {
		return "", -8, "combinerawtransaction: first argument must be a JSON array of hex strings"
	}
	if len(hexStrs) < 2 {
		return "", -8, "combinerawtransaction: at least two transactions required"
	}
	txs := make([]*wire.Tx, 0, len(hexStrs))
	for _, hs := range hexStrs {
		hs = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hs), "0x"))
		raw, err := hex.DecodeString(hs)
		if err != nil || len(raw) == 0 {
			return "", -8, "combinerawtransaction: invalid transaction hex"
		}
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			return "", -8, "combinerawtransaction: " + err.Error()
		}
		if tx.HasWitness() {
			return "", -8, "combinerawtransaction: witness transactions are not supported in this build"
		}
		txs = append(txs, tx)
	}
	base := txs[0]
	for ti := 1; ti < len(txs); ti++ {
		if !txSkeletonEqual(base, txs[ti]) {
			return "", -8, fmt.Sprintf("combinerawtransaction: transaction %d differs from the first (version, inputs, outputs, or locktime must match)", ti)
		}
	}
	merged := cloneTxForCombine(base)
	for i := range merged.Vin {
		var chosen []byte
		for ti := 0; ti < len(txs); ti++ {
			s := txs[ti].Vin[i].Script
			if len(s) == 0 {
				continue
			}
			if chosen == nil {
				chosen = append([]byte(nil), s...)
				continue
			}
			if !bytes.Equal(chosen, s) {
				return "", -8, fmt.Sprintf("combinerawtransaction: conflicting scriptSig on input %d", i)
			}
		}
		merged.Vin[i].Script = chosen
	}
	out, err := merged.Serialize()
	if err != nil {
		return "", -8, err.Error()
	}
	return hex.EncodeToString(out), 0, ""
}

func txSkeletonEqual(a, b *wire.Tx) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Version != b.Version || a.LockTime != b.LockTime || len(a.Vin) != len(b.Vin) || len(a.Vout) != len(b.Vout) {
		return false
	}
	for i := range a.Vin {
		if a.Vin[i].PrevHash != b.Vin[i].PrevHash || a.Vin[i].PrevIdx != b.Vin[i].PrevIdx || a.Vin[i].Sequence != b.Vin[i].Sequence {
			return false
		}
	}
	for i := range a.Vout {
		if a.Vout[i].Value != b.Vout[i].Value || !bytes.Equal(a.Vout[i].PkScript, b.Vout[i].PkScript) {
			return false
		}
	}
	return true
}

func cloneTxForCombine(t *wire.Tx) *wire.Tx {
	out := &wire.Tx{
		Version:  t.Version,
		LockTime: t.LockTime,
		Vin:      make([]wire.TxIn, len(t.Vin)),
		Vout:     make([]wire.TxOut, len(t.Vout)),
	}
	for i := range t.Vin {
		out.Vin[i].PrevHash = t.Vin[i].PrevHash
		out.Vin[i].PrevIdx = t.Vin[i].PrevIdx
		out.Vin[i].Sequence = t.Vin[i].Sequence
	}
	copy(out.Vout, t.Vout)
	return out
}
