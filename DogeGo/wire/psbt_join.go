// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"encoding/binary"
	"fmt"
)

// JoinPSBT merges distinct PSBTs into one unsigned transaction (Core joinpsbts).
// No prevout may appear in more than one PSBT.
func JoinPSBT(psbts []*Psbt) (*Psbt, error) {
	if len(psbts) == 0 {
		return nil, fmt.Errorf("psbt: no inputs to join")
	}
	seenPrevout := make(map[string]struct{})
	var joined Tx
	var global []PsbtKeyValue
	var inputs [][]PsbtKeyValue
	var outputs [][]PsbtKeyValue
	var psbtVersion uint32

	for pi, p := range psbts {
		if p == nil || p.UnsignedTx == nil {
			return nil, fmt.Errorf("psbt: invalid psbt at index %d", pi)
		}
		if p.UnsignedTx.HasWitness() {
			return nil, fmt.Errorf("psbt: witness PSBT is not supported")
		}
		tx := p.UnsignedTx
		if tx.Version > joined.Version {
			joined.Version = tx.Version
		}
		if tx.LockTime > joined.LockTime {
			joined.LockTime = tx.LockTime
		}
		if v := GlobalVersionFromMap(p.Global); v > psbtVersion {
			psbtVersion = v
		}
		for _, in := range tx.Vin {
			key := prevoutKey(in.PrevHash, in.PrevIdx)
			if _, ok := seenPrevout[key]; ok {
				return nil, fmt.Errorf("psbt: duplicate input detected")
			}
			seenPrevout[key] = struct{}{}
			cin := in
			cin.Script = append([]byte(nil), in.Script...)
			if len(in.Witness) > 0 {
				cin.Witness = make([][]byte, len(in.Witness))
				for wi, w := range in.Witness {
					cin.Witness[wi] = append([]byte(nil), w...)
				}
			}
			joined.Vin = append(joined.Vin, cin)
		}
		for _, o := range tx.Vout {
			joined.Vout = append(joined.Vout, TxOut{
				Value:    o.Value,
				PkScript: append([]byte(nil), o.PkScript...),
			})
		}
		inputs = append(inputs, p.Inputs...)
		outputs = append(outputs, p.Outputs...)
		var err error
		global, err = mergePsbtKVMaps(global, p.Global, true)
		if err != nil {
			return nil, fmt.Errorf("psbt global: %w", err)
		}
	}
	if joined.Version == 0 {
		joined.Version = 1
	}
	out := &Psbt{
		UnsignedTx: &joined,
		Version:    psbtVersion,
		Global: append([]PsbtKeyValue{{
			Type:  PsbtGlobalUnsignedTx,
			Value: joined.SerializeForHash(),
		}}, global...),
		Inputs:  inputs,
		Outputs: outputs,
	}
	if psbtVersion > 0 {
		var verBuf [4]byte
		binary.LittleEndian.PutUint32(verBuf[:], psbtVersion)
		out.Global = appendGlobalVersion(out.Global, verBuf[:])
	}
	return out, nil
}

func prevoutKey(hash [32]byte, idx uint32) string {
	return fmt.Sprintf("%x:%d", hash, idx)
}

func appendGlobalVersion(global []PsbtKeyValue, ver []byte) []PsbtKeyValue {
	out := make([]PsbtKeyValue, 0, len(global)+1)
	replaced := false
	for _, kv := range global {
		if kv.Type == PsbtGlobalVersion {
			out = append(out, PsbtKeyValue{Type: PsbtGlobalVersion, Value: append([]byte(nil), ver...)})
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append(out, PsbtKeyValue{Type: PsbtGlobalVersion, Value: ver})
	}
	return out
}
