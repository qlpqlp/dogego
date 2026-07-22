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

// PSBT magic bytes (BIP-174): psbt\xff
var psbtMagic = []byte{0x70, 0x73, 0x62, 0x74, 0xff}

// PsbtMagic returns the BIP-174 PSBT file magic.
func PsbtMagic() []byte {
	return append([]byte(nil), psbtMagic...)
}

// PSBT key types (BIP-174).
const (
	PsbtGlobalUnsignedTx = 0x00
	PsbtGlobalXPub       = 0x01
	PsbtGlobalVersion    = 0x02
	PsbtInNonWitnessUtxo = 0x00
	PsbtInWitnessUtxo    = 0x01
	PsbtInPartialSig      = 0x02
	PsbtInSighash         = 0x03
	PsbtInRedeemScript     = 0x04
	PsbtInWitnessScript    = 0x05
	PsbtInBIP32Derivation  = 0x06
	PsbtInFinalScriptSig   = 0x07
	PsbtInFinalScriptWit   = 0x08
	PsbtOutRedeemScript    = 0x00
	PsbtOutWitnessScript   = 0x01
	PsbtOutBIP32Derivation = 0x02
)

// PsbtKeyValue is one key-value entry in a PSBT map (key type + optional subkey bytes).
type PsbtKeyValue struct {
	Type    byte
	Subkey  []byte
	Value   []byte
}

// Psbt holds a parsed Partially Signed Bitcoin Transaction (BIP-174).
type Psbt struct {
	UnsignedTx *Tx
	Version    uint32
	Global     []PsbtKeyValue
	Inputs     [][]PsbtKeyValue
	Outputs    [][]PsbtKeyValue
}

// ParsePSBT decodes a PSBT blob (magic + global + per-input + per-output maps).
func ParsePSBT(b []byte) (*Psbt, error) {
	if len(b) < len(psbtMagic) || !bytes.Equal(b[:len(psbtMagic)], psbtMagic) {
		return nil, fmt.Errorf("psbt: invalid magic")
	}
	r := bytes.NewReader(b[len(psbtMagic):])
	global, err := readPsbtMap(r)
	if err != nil {
		return nil, fmt.Errorf("psbt global: %w", err)
	}
	var unsigned []byte
	var version uint32
	for _, kv := range global {
		switch kv.Type {
		case PsbtGlobalUnsignedTx:
			unsigned = append([]byte(nil), kv.Value...)
		case PsbtGlobalVersion:
			if len(kv.Value) == 4 {
				version = binary.LittleEndian.Uint32(kv.Value)
			}
		}
	}
	if len(unsigned) == 0 {
		return nil, fmt.Errorf("psbt: missing global unsigned tx")
	}
	tx, err := DeserializeTx(unsigned)
	if err != nil {
		return nil, fmt.Errorf("psbt unsigned tx: %w", err)
	}
	if tx.HasWitness() {
		return nil, fmt.Errorf("psbt: witness transactions are not supported")
	}
	p := &Psbt{
		UnsignedTx: tx,
		Version:    version,
		Global:     global,
	}
	for range tx.Vin {
		m, err := readPsbtMap(r)
		if err != nil {
			return nil, fmt.Errorf("psbt input: %w", err)
		}
		p.Inputs = append(p.Inputs, m)
	}
	for range tx.Vout {
		m, err := readPsbtMap(r)
		if err != nil {
			return nil, fmt.Errorf("psbt output: %w", err)
		}
		p.Outputs = append(p.Outputs, m)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("psbt: trailing %d bytes", r.Len())
	}
	return p, nil
}

func readPsbtMap(r *bytes.Reader) ([]PsbtKeyValue, error) {
	var out []PsbtKeyValue
	for {
		keyLen, err := ReadCompactSize(r)
		if err != nil {
			return nil, err
		}
		if keyLen == 0 {
			break
		}
		key := make([]byte, keyLen)
		if _, err := io.ReadFull(r, key); err != nil {
			return nil, err
		}
		valLen, err := ReadCompactSize(r)
		if err != nil {
			return nil, err
		}
		val := make([]byte, valLen)
		if _, err := io.ReadFull(r, val); err != nil {
			return nil, err
		}
		typ := key[0]
		sub := []byte(nil)
		if len(key) > 1 {
			sub = append([]byte(nil), key[1:]...)
		}
		out = append(out, PsbtKeyValue{Type: typ, Subkey: sub, Value: val})
	}
	return out, nil
}
