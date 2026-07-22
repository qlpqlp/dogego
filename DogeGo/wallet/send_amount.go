// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "dogego/wire"

// SendDisplayKoinu returns the koinu amount to show for a wallet send history row.
// For transfers to another owned address it is the gross output to that address, not only the fee.
func SendDisplayKoinu(spentTotal int64, recvByAddr map[string]int64, sendAddr string, vouts []wire.TxOut, trackedScripts map[string][]byte) int64 {
	if spentTotal <= 0 {
		return 0
	}
	var change int64
	for _, v := range recvByAddr {
		change += v
	}
	// Payment to third parties is always in non-wallet outputs.
	var grossExternal int64
	for _, o := range vouts {
		if len(o.PkScript) == 0 {
			continue
		}
		if trackedScripts != nil {
			if _, ok := trackedScripts[string(o.PkScript)]; ok {
				continue
			}
		}
		grossExternal += o.Value
	}
	if grossExternal > 0 {
		return grossExternal
	}
	// Internal transfer to another owned address (no external outputs).
	var grossOther int64
	for addr, v := range recvByAddr {
		if sendAddr != "" && addr != sendAddr {
			grossOther += v
		}
	}
	if grossOther > 0 {
		return grossOther
	}
	net := spentTotal - change
	if net > 0 {
		return net
	}
	var totalOut int64
	for _, o := range vouts {
		totalOut += o.Value
	}
	fee := spentTotal - totalOut
	if fee > 0 {
		return fee
	}
	return spentTotal
}
