// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"math"
	"strings"

	"dogego/wire"
)

// fundRawTxOptions holds parsed fundrawtransaction option fields (Core subset).
type fundRawTxOptions struct {
	changeAddr           string
	changePos            int
	feePerKB             uint64
	subtractFeeFrom      []int
	inputSequence        uint32
	lockUnspents         bool
	includeWatching      bool
	addInputs            bool
	minimumTotalFeeKoinu int64
}

func defaultFundRawTxOptions() fundRawTxOptions {
	return fundRawTxOptions{
		changePos:     -1,
		inputSequence: wire.MaxBIP125RBFSequence,
		lockUnspents:  true,
		addInputs:     true,
	}
}

func parseFundRawTxOptions(paths *DataPaths, opts map[string]json.RawMessage) (fundRawTxOptions, int, string) {
	o := defaultFundRawTxOptions()
	if opts == nil {
		return o, 0, ""
	}
	if v, ok := opts["changeAddress"]; ok {
		if err := json.Unmarshal(v, &o.changeAddr); err != nil {
			return o, -8, "fundrawtransaction: changeAddress must be a string"
		}
		o.changeAddr = strings.TrimSpace(o.changeAddr)
	}
	if v, ok := opts["changePosition"]; ok {
		var cp float64
		if err := json.Unmarshal(v, &cp); err != nil || cp < -1 || cp != float64(int(cp)) {
			return o, -8, "fundrawtransaction: changePosition must be an integer"
		}
		o.changePos = int(cp)
	}
	if rate, code, msg := fundFeePerKBFromOptions(paths, opts); code != 0 {
		return o, code, msg
	} else if rate > 0 {
		o.feePerKB = rate
	}
	if v, ok := opts["subtractFeeFromOutputs"]; ok {
		var arr []json.RawMessage
		if err := json.Unmarshal(v, &arr); err != nil {
			return o, -8, "fundrawtransaction: subtractFeeFromOutputs must be a JSON array"
		}
		for _, elem := range arr {
			var idx float64
			if err := json.Unmarshal(elem, &idx); err != nil || idx < 0 || idx != float64(int(idx)) {
				return o, -8, "fundrawtransaction: invalid vout index in subtractFeeFromOutputs"
			}
			o.subtractFeeFrom = append(o.subtractFeeFrom, int(idx))
		}
	}
	if v, ok := opts["replaceable"]; ok {
		rep, code, msg := parseRPCBoolOpt(v, true, "fundrawtransaction", "replaceable")
		if code != 0 {
			return o, code, msg
		}
		if !rep {
			o.inputSequence = 0xffffffff
		}
	}
	if v, ok := opts["lockUnspents"]; ok {
		lock, code, msg := parseRPCBoolOpt(v, true, "fundrawtransaction", "lockUnspents")
		if code != 0 {
			return o, code, msg
		}
		o.lockUnspents = lock
	}
	if v, ok := opts["includeWatching"]; ok {
		inc, code, msg := parseRPCBoolOpt(v, false, "fundrawtransaction", "includeWatching")
		if code != 0 {
			return o, code, msg
		}
		o.includeWatching = inc
	}
	if v, ok := opts["add_inputs"]; ok {
		add, code, msg := parseRPCBoolOpt(v, true, "fundrawtransaction", "add_inputs")
		if code != 0 {
			return o, code, msg
		}
		o.addInputs = add
	}
	if v, ok := opts["minimumTotalFee"]; ok {
		amt, code, msg := parseRPCAmountField(v, "fundrawtransaction", "minimumTotalFee")
		if code != 0 {
			return o, code, msg
		}
		if amt < 0 || math.IsNaN(amt) || math.IsInf(amt, 0) {
			return o, -8, "fundrawtransaction: invalid minimumTotalFee"
		}
		o.minimumTotalFeeKoinu = int64(math.Round(amt * 1e8))
	}
	return o, 0, ""
}

// enforceMinimumTotalFee raises fee to the option floor by reducing change when possible.
func enforceMinimumTotalFee(tx *wire.Tx, fee *int64, minFee int64) bool {
	if minFee <= 0 || *fee >= minFee {
		return true
	}
	need := minFee - *fee
	for i := len(tx.Vout) - 1; i >= 0; i-- {
		if tx.Vout[i].Value > need {
			tx.Vout[i].Value -= need
			*fee = minFee
			return true
		}
	}
	*fee = minFee
	return need == 0
}
