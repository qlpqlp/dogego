// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
)

// EncodeBIP32DerivationValue builds PSBT BIP32 derivation value bytes:
// 4-byte master key fingerprint + little-endian uint32 path indices.
func EncodeBIP32DerivationValue(masterFingerprint uint32, path []uint32) []byte {
	val := make([]byte, 4+4*len(path))
	binary.BigEndian.PutUint32(val[:4], masterFingerprint)
	for i, idx := range path {
		binary.LittleEndian.PutUint32(val[4+4*i:], idx)
	}
	return val
}

// SetInputBIP32Derivation sets PSBT_IN_BIP32_DERIVATION for input i (pubkey subkey).
func (p *Psbt) SetInputBIP32Derivation(i int, pubkey33, derivValue []byte) {
	if len(pubkey33) == 0 {
		return
	}
	p.setInputKV(i, PsbtInBIP32Derivation, append([]byte(nil), pubkey33...), derivValue)
}

// SetOutputBIP32Derivation sets PSBT_OUT_BIP32_DERIVATION for output i (pubkey subkey).
func (p *Psbt) SetOutputBIP32Derivation(i int, pubkey33, derivValue []byte) {
	if len(pubkey33) == 0 {
		return
	}
	p.setOutputKV(i, PsbtOutBIP32Derivation, append([]byte(nil), pubkey33...), derivValue)
}

func (p *Psbt) setOutputKV(i int, typ byte, subkey, val []byte) {
	if p == nil || i < 0 || i >= len(p.Outputs) {
		return
	}
	m := p.Outputs[i]
	out := make([]PsbtKeyValue, 0, len(m)+1)
	replaced := false
	for _, kv := range m {
		if kv.Type == typ && bytes.Equal(kv.Subkey, subkey) {
			out = append(out, PsbtKeyValue{Type: typ, Subkey: subkey, Value: append([]byte(nil), val...)})
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, PsbtKeyValue{Type: typ, Subkey: subkey, Value: append([]byte(nil), val...)})
	}
	p.Outputs[i] = out
}
