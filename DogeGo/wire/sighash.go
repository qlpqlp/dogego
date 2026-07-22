// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

// Legacy sighash type bits (end of ECDSA signature / preimage).
const (
	SigHashAll          uint32 = 0x01
	SigHashNone         uint32 = 0x02
	SigHashSingle       uint32 = 0x03
	SigHashAnyoneCanPay uint32 = 0x80
	sigHashMask                = 0x1f
)

// CalcSignatureHashLegacy computes the digest signed for legacy (non-segwit) inputs,
// matching Bitcoin / Dogecoin pre-BIP143 behavior for standard P2PKH spends.
func CalcSignatureHashLegacy(subScript []byte, hashType uint32, tx *Tx, idx int) ([32]byte, error) {
	if tx == nil {
		return [32]byte{}, fmt.Errorf("nil transaction")
	}
	if idx < 0 || idx >= len(tx.Vin) {
		return [32]byte{}, fmt.Errorf("input index out of range")
	}
	if tx.HasWitness() {
		return [32]byte{}, fmt.Errorf("witness transaction sighash is not supported (Dogecoin legacy chain)")
	}

	ht := hashType
	if ht&sigHashMask == SigHashSingle && idx >= len(tx.Vout) {
		var h [32]byte
		h[0] = 0x01
		return h, nil
	}

	sub := stripCodeSeparatorsSimple(subScript)
	cpy := shallowCopyTxForSigHash(tx)

	for i := range cpy.Vin {
		if i == idx {
			cpy.Vin[i].Script = append([]byte(nil), sub...)
		} else {
			cpy.Vin[i].Script = nil
		}
	}

	switch ht & sigHashMask {
	case SigHashNone:
		cpy.Vout = cpy.Vout[:0]
		for i := range cpy.Vin {
			if i != idx {
				cpy.Vin[i].Sequence = 0
			}
		}
	case SigHashSingle:
		cpy.Vout = append([]TxOut(nil), cpy.Vout[:idx+1]...)
		for i := 0; i < idx; i++ {
			cpy.Vout[i].Value = -1
			cpy.Vout[i].PkScript = nil
		}
		for i := range cpy.Vin {
			if i != idx {
				cpy.Vin[i].Sequence = 0
			}
		}
	default:
		// SigHashAll, undefined mask bits: same as ALL (Core consensus).
	}

	if ht&SigHashAnyoneCanPay != 0 {
		cpy.Vin = []TxIn{cpy.Vin[idx]}
	}

	var tail bytes.Buffer
	tail.Write(cpy.SerializeForHash())
	if err := binary.Write(&tail, binary.LittleEndian, ht); err != nil {
		return [32]byte{}, err
	}
	s := sha256.Sum256(tail.Bytes())
	s2 := sha256.Sum256(s[:])
	var out [32]byte
	copy(out[:], s2[:])
	return out, nil
}

func shallowCopyTxForSigHash(tx *Tx) *Tx {
	c := &Tx{
		Version:  tx.Version,
		LockTime: tx.LockTime,
		Vin:      make([]TxIn, len(tx.Vin)),
		Vout:     make([]TxOut, len(tx.Vout)),
	}
	for i := range tx.Vin {
		c.Vin[i].PrevHash = tx.Vin[i].PrevHash
		c.Vin[i].PrevIdx = tx.Vin[i].PrevIdx
		c.Vin[i].Sequence = tx.Vin[i].Sequence
		c.Vin[i].Script = append([]byte(nil), tx.Vin[i].Script...)
	}
	for i := range tx.Vout {
		c.Vout[i].Value = tx.Vout[i].Value
		c.Vout[i].PkScript = append([]byte(nil), tx.Vout[i].PkScript...)
	}
	return c
}

// stripCodeSeparatorsSimple removes OP_CODESEPARATOR (0xab) outside pushdata regions.
func stripCodeSeparatorsSimple(script []byte) []byte {
	if len(script) == 0 {
		return script
	}
	var out []byte
	i := 0
	for i < len(script) {
		op := script[i]
		if op == 0xab {
			i++
			continue
		}
		if op >= 0x01 && op <= 0x4b {
			n := int(op)
			if i+1+n > len(script) {
				out = append(out, script[i:]...)
				break
			}
			out = append(out, script[i:i+1+n]...)
			i += 1 + n
			continue
		}
		if op == 0x4c || op == 0x4d || op == 0x4e {
			start := i
			_, next, err := readScriptPushForSigHash(script, i)
			if err != nil {
				out = append(out, script[i:]...)
				break
			}
			out = append(out, script[start:next]...)
			i = next
			continue
		}
		if op == 0x4f || (op >= 0x51 && op <= 0x60) {
			out = append(out, op)
			i++
			continue
		}
		out = append(out, op)
		i++
	}
	return out
}

func readScriptPushForSigHash(script []byte, off int) ([]byte, int, error) {
	if off >= len(script) {
		return nil, 0, fmt.Errorf("truncated")
	}
	op := script[off]
	off++
	switch {
	case op == 0x00:
		return []byte{}, off, nil
	case op >= 0x01 && op <= 0x4b:
		n := int(op)
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4c:
		if off >= len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		n := int(script[off])
		off++
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4d:
		if off+1 >= len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		n := int(script[off]) | int(script[off+1])<<8
		off += 2
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	case op == 0x4e:
		if off+3 >= len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		n := int(script[off]) | int(script[off+1])<<8 | int(script[off+2])<<16 | int(script[off+3])<<24
		off += 4
		if off+n > len(script) {
			return nil, 0, fmt.Errorf("truncated")
		}
		return append([]byte(nil), script[off:off+n]...), off + n, nil
	default:
		return nil, 0, fmt.Errorf("non-push")
	}
}
