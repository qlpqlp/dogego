// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package mempool

import (
	"strings"

	"dogego/wire"
)

func isCoinbaseWire(in *wire.TxIn) bool {
	var z [32]byte
	return in.PrevIdx == 0xffffffff && in.PrevHash == z
}

// SpendsOutpoint reports whether any pooled transaction spends rpcPrevTxid:vout.
// rpcPrevTxid is normalized 64 lowercase hex (no 0x). Coinbase inputs are ignored.
func (p *Pool) SpendsOutpoint(rpcPrevTxid string, vout uint32) bool {
	rpcPrevTxid = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(rpcPrevTxid), "0x"))
	if len(rpcPrevTxid) != 64 {
		return false
	}
	for _, c := range rpcPrevTxid {
		if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') {
			continue
		}
		return false
	}
	p.mu.Lock()
	blobs := make([][]byte, 0, len(p.raw))
	for _, v := range p.raw {
		blobs = append(blobs, v)
	}
	p.mu.Unlock()
	for _, raw := range blobs {
		tx, err := wire.DeserializeTx(raw)
		if err != nil {
			continue
		}
		for i := range tx.Vin {
			in := &tx.Vin[i]
			if isCoinbaseWire(in) {
				continue
			}
			if in.PrevIdx != vout {
				continue
			}
			if txidDisplayHex(in.PrevHash) == rpcPrevTxid {
				return true
			}
		}
	}
	return false
}
