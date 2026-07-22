// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"time"

	"dogego/wire"
)

// P2PKHPkScript returns a standard pay-to-pubkey-hash scriptPubKey.
func P2PKHPkScript(pubKeyHash [20]byte) []byte {
	out := make([]byte, 0, 25)
	out = append(out, 0x76, 0xa9, 0x14)
	out = append(out, pubKeyHash[:]...)
	out = append(out, 0x88, 0xac)
	return out
}

// BIP34HeightPush encodes block height for the coinbase scriptSig (Core CScript << height).
func BIP34HeightPush(height int64) []byte {
	if height <= 0x7f {
		return []byte{1, byte(height)}
	}
	if height <= 0xffff {
		return []byte{2, byte(height), byte(height >> 8)}
	}
	if height <= 0xffffff {
		return []byte{3, byte(height), byte(height >> 8), byte(height >> 16)}
	}
	b := make([]byte, 5)
	b[0] = 4
	binary.LittleEndian.PutUint32(b[1:], uint32(height))
	return b
}

// BuildCoinbaseTx builds an unsigned legacy coinbase transaction for block templates.
func BuildCoinbaseTx(height int64, value int64, pkScript []byte) *wire.Tx {
	extra := make([]byte, 8)
	binary.LittleEndian.PutUint64(extra, uint64(time.Now().UnixNano()))
	script := append(BIP34HeightPush(height), extra...)
	return &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: [32]byte{},
			PrevIdx:  0xffffffff,
			Script:   script,
			Sequence: 0xffffffff,
		}},
		LockTime: 0,
		Vout:     []wire.TxOut{{Value: value, PkScript: pkScript}},
	}
}
