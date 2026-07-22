// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Serialize encodes a PSBT (BIP-174).
func (p *Psbt) Serialize() ([]byte, error) {
	if p == nil || p.UnsignedTx == nil {
		return nil, fmt.Errorf("psbt: nil")
	}
	var b bytes.Buffer
	b.Write(psbtMagic)
	global := appendGlobalUnsignedTx(p.Global, p.UnsignedTx)
	if err := writePsbtMap(&b, global); err != nil {
		return nil, err
	}
	for _, inMap := range p.Inputs {
		if err := writePsbtMap(&b, inMap); err != nil {
			return nil, err
		}
	}
	for _, outMap := range p.Outputs {
		if err := writePsbtMap(&b, outMap); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}

func appendGlobalUnsignedTx(global []PsbtKeyValue, tx *Tx) []PsbtKeyValue {
	unsigned := tx.SerializeForHash()
	out := make([]PsbtKeyValue, 0, len(global)+1)
	replaced := false
	for _, kv := range global {
		if kv.Type == PsbtGlobalUnsignedTx {
			out = append(out, PsbtKeyValue{Type: PsbtGlobalUnsignedTx, Value: unsigned})
			replaced = true
			continue
		}
		out = append(out, kv)
	}
	if !replaced {
		out = append([]PsbtKeyValue{{Type: PsbtGlobalUnsignedTx, Value: unsigned}}, out...)
	}
	return out
}

func writePsbtMap(w io.Writer, m []PsbtKeyValue) error {
	for _, kv := range m {
		key := append([]byte{kv.Type}, kv.Subkey...)
		if err := WriteCompactSize(w, uint64(len(key))); err != nil {
			return err
		}
		if _, err := w.Write(key); err != nil {
			return err
		}
		if err := WriteCompactSize(w, uint64(len(kv.Value))); err != nil {
			return err
		}
		if _, err := w.Write(kv.Value); err != nil {
			return err
		}
	}
	return WriteCompactSize(w, 0)
}

// CombinePSBT merges PSBTs that share the same unsigned transaction (BIP-174 combiner).
func CombinePSBT(psbts []*Psbt) (*Psbt, error) {
	if len(psbts) == 0 {
		return nil, fmt.Errorf("psbt: no inputs to combine")
	}
	base := psbts[0]
	if base == nil || base.UnsignedTx == nil {
		return nil, fmt.Errorf("psbt: invalid base")
	}
	want := base.UnsignedTx.SerializeForHash()
	out := clonePsbt(base)
	for pi := 1; pi < len(psbts); pi++ {
		p := psbts[pi]
		if p == nil || p.UnsignedTx == nil {
			return nil, fmt.Errorf("psbt: invalid psbt at index %d", pi)
		}
		if !bytes.Equal(p.UnsignedTx.SerializeForHash(), want) {
			return nil, fmt.Errorf("psbt: cannot combine PSBTs with different unsigned transactions")
		}
		if len(p.Inputs) != len(out.Inputs) || len(p.Outputs) != len(out.Outputs) {
			return nil, fmt.Errorf("psbt: input/output count mismatch")
		}
		var err error
		out.Global, err = mergePsbtKVMaps(out.Global, p.Global, true)
		if err != nil {
			return nil, err
		}
		for i := range out.Inputs {
			var err error
			out.Inputs[i], err = mergePsbtKVMaps(out.Inputs[i], p.Inputs[i], false)
			if err != nil {
				return nil, fmt.Errorf("psbt input %d: %w", i, err)
			}
		}
		for i := range out.Outputs {
			var err error
			out.Outputs[i], err = mergePsbtKVMaps(out.Outputs[i], p.Outputs[i], false)
			if err != nil {
				return nil, fmt.Errorf("psbt output %d: %w", i, err)
			}
		}
	}
	return out, nil
}

func clonePsbt(p *Psbt) *Psbt {
	out := &Psbt{
		UnsignedTx: cloneTx(p.UnsignedTx),
		Version:    p.Version,
		Global:     cloneKVSlice(p.Global),
		Inputs:     make([][]PsbtKeyValue, len(p.Inputs)),
		Outputs:    make([][]PsbtKeyValue, len(p.Outputs)),
	}
	for i, m := range p.Inputs {
		out.Inputs[i] = cloneKVSlice(m)
	}
	for i, m := range p.Outputs {
		out.Outputs[i] = cloneKVSlice(m)
	}
	return out
}

func cloneTx(t *Tx) *Tx {
	if t == nil {
		return nil
	}
	c := *t
	c.Vin = make([]TxIn, len(t.Vin))
	copy(c.Vin, t.Vin)
	c.Vout = make([]TxOut, len(t.Vout))
	for i, o := range t.Vout {
		c.Vout[i] = TxOut{Value: o.Value, PkScript: append([]byte(nil), o.PkScript...)}
	}
	return &c
}

func cloneKVSlice(m []PsbtKeyValue) []PsbtKeyValue {
	out := make([]PsbtKeyValue, len(m))
	for i, kv := range m {
		out[i] = PsbtKeyValue{
			Type:   kv.Type,
			Subkey: append([]byte(nil), kv.Subkey...),
			Value:  append([]byte(nil), kv.Value...),
		}
	}
	return out
}

func psbtKVKey(kv PsbtKeyValue) string {
	return string(append([]byte{kv.Type}, kv.Subkey...))
}

func mergePsbtKVMaps(dst, src []PsbtKeyValue, skipUnsigned bool) ([]PsbtKeyValue, error) {
	seen := make(map[string]int, len(dst))
	for i, kv := range dst {
		if skipUnsigned && kv.Type == PsbtGlobalUnsignedTx {
			continue
		}
		seen[psbtKVKey(kv)] = i
	}
	for _, kv := range src {
		if skipUnsigned && kv.Type == PsbtGlobalUnsignedTx {
			continue
		}
		k := psbtKVKey(kv)
		if idx, ok := seen[k]; ok {
			if !bytes.Equal(dst[idx].Value, kv.Value) {
				return nil, fmt.Errorf("conflicting value for key type 0x%02x", kv.Type)
			}
			continue
		}
		dst = append(dst, PsbtKeyValue{
			Type:   kv.Type,
			Subkey: append([]byte(nil), kv.Subkey...),
			Value:  append([]byte(nil), kv.Value...),
		})
		seen[k] = len(dst) - 1
	}
	return dst, nil
}

// InputValue returns the prevout value in koinu for input i when UTXO data is present.
func (p *Psbt) InputValue(i int) (int64, bool) {
	if p == nil || i < 0 || i >= len(p.Inputs) {
		return 0, false
	}
	for _, kv := range p.Inputs[i] {
		switch kv.Type {
		case PsbtInWitnessUtxo:
			if len(kv.Value) >= 8 {
				v := int64(uint64(kv.Value[0]) | uint64(kv.Value[1])<<8 | uint64(kv.Value[2])<<16 |
					uint64(kv.Value[3])<<24 | uint64(kv.Value[4])<<32 | uint64(kv.Value[5])<<40 |
					uint64(kv.Value[6])<<48 | uint64(kv.Value[7])<<56)
				return v, true
			}
		case PsbtInNonWitnessUtxo:
			tx, err := DeserializeTx(kv.Value)
			if err != nil {
				continue
			}
			in := &p.UnsignedTx.Vin[i]
			if int(in.PrevIdx) >= len(tx.Vout) {
				continue
			}
			return tx.Vout[in.PrevIdx].Value, true
		}
	}
	return 0, false
}

// InputHasFinalScriptSig reports whether input i has a final scriptSig.
func (p *Psbt) InputHasFinalScriptSig(i int) bool {
	for _, kv := range p.Inputs[i] {
		if kv.Type == PsbtInFinalScriptSig && len(kv.Value) > 0 {
			return true
		}
	}
	return false
}

// InputFinalScriptSig returns the final scriptSig for input i.
func (p *Psbt) InputFinalScriptSig(i int) []byte {
	for _, kv := range p.Inputs[i] {
		if kv.Type == PsbtInFinalScriptSig {
			return append([]byte(nil), kv.Value...)
		}
	}
	return nil
}

// InputHasUTXO reports non_witness_utxo or witness_utxo on input i.
func (p *Psbt) InputHasUTXO(i int) bool {
	for _, kv := range p.Inputs[i] {
		if kv.Type == PsbtInNonWitnessUtxo || kv.Type == PsbtInWitnessUtxo {
			return true
		}
	}
	return false
}

// SetInputNonWitnessUtxo sets or replaces PSBT_IN_NON_WITNESS_UTXO for input i.
func (p *Psbt) SetInputNonWitnessUtxo(i int, txBytes []byte) {
	p.setInputKV(i, PsbtInNonWitnessUtxo, nil, txBytes)
}

// SetInputFinalScriptSig sets PSBT_IN_FINAL_SCRIPTSIG for input i.
func (p *Psbt) SetInputFinalScriptSig(i int, scriptSig []byte) {
	p.setInputKV(i, PsbtInFinalScriptSig, nil, scriptSig)
}

func (p *Psbt) setInputKV(i int, typ byte, subkey, val []byte) {
	if i < 0 || i >= len(p.Inputs) {
		return
	}
	m := p.Inputs[i]
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
	p.Inputs[i] = out
}

// ExtractedTx builds a network transaction from unsigned tx + final scriptSigs.
func (p *Psbt) ExtractedTx() (*Tx, bool) {
	if p == nil || p.UnsignedTx == nil {
		return nil, false
	}
	tx := cloneTx(p.UnsignedTx)
	complete := true
	for i := range tx.Vin {
		sig := p.InputFinalScriptSig(i)
		if len(sig) == 0 {
			complete = false
			continue
		}
		tx.Vin[i].Script = sig
	}
	return tx, complete
}

// GlobalVersionFromMap reads PSBT_GLOBAL_VERSION if set.
func GlobalVersionFromMap(global []PsbtKeyValue) uint32 {
	for _, kv := range global {
		if kv.Type == PsbtGlobalVersion && len(kv.Value) == 4 {
			return binary.LittleEndian.Uint32(kv.Value)
		}
	}
	return 0
}
