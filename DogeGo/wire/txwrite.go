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
)

// HasWitness reports whether any input carries a non-empty witness stack.
func (t *Tx) HasWitness() bool {
	for i := range t.Vin {
		if len(t.Vin[i].Witness) > 0 {
			return true
		}
	}
	return false
}

// Serialize returns full wire encoding (legacy or extended witness layout).
func (t *Tx) Serialize() ([]byte, error) {
	if !t.HasWitness() {
		return append([]byte(nil), t.SerializeForHash()...), nil
	}
	var b bytes.Buffer
	if err := binary.Write(&b, binary.LittleEndian, t.Version); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, 0); err != nil {
		return nil, err
	}
	const witnessFlag byte = 1
	if err := binary.Write(&b, binary.LittleEndian, witnessFlag); err != nil {
		return nil, err
	}
	if err := WriteCompactSize(&b, uint64(len(t.Vin))); err != nil {
		return nil, err
	}
	for i := range t.Vin {
		if err := writeTxInBuf(&b, &t.Vin[i]); err != nil {
			return nil, err
		}
	}
	if err := writeTxOutsBuf(&b, t); err != nil {
		return nil, err
	}
	for i := range t.Vin {
		in := &t.Vin[i]
		if err := WriteCompactSize(&b, uint64(len(in.Witness))); err != nil {
			return nil, err
		}
		for _, w := range in.Witness {
			if err := WriteCompactSize(&b, uint64(len(w))); err != nil {
				return nil, err
			}
			if _, err := b.Write(w); err != nil {
				return nil, err
			}
		}
	}
	if err := binary.Write(&b, binary.LittleEndian, t.LockTime); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// WTxHash is the witness transaction id (BIP141): double-SHA256 of Serialize().
// For legacy transactions (no witness stacks) this equals TxHash().
func (t *Tx) WTxHash() [32]byte {
	b, err := t.Serialize()
	if err != nil || len(b) == 0 {
		return [32]byte{}
	}
	h := sha256.Sum256(b)
	h2 := sha256.Sum256(h[:])
	var out [32]byte
	copy(out[:], h2[:])
	return out
}

func writeTxInBuf(b *bytes.Buffer, in *TxIn) error {
	if _, err := b.Write(in.PrevHash[:]); err != nil {
		return err
	}
	if err := binary.Write(b, binary.LittleEndian, in.PrevIdx); err != nil {
		return err
	}
	if err := WriteCompactSize(b, uint64(len(in.Script))); err != nil {
		return err
	}
	if _, err := b.Write(in.Script); err != nil {
		return err
	}
	return binary.Write(b, binary.LittleEndian, in.Sequence)
}

func writeTxOutsBuf(b *bytes.Buffer, t *Tx) error {
	if err := WriteCompactSize(b, uint64(len(t.Vout))); err != nil {
		return err
	}
	for i := range t.Vout {
		if err := writeTxOutBuf(b, &t.Vout[i]); err != nil {
			return err
		}
	}
	return nil
}

func writeTxOutBuf(b *bytes.Buffer, o *TxOut) error {
	if err := binary.Write(b, binary.LittleEndian, o.Value); err != nil {
		return err
	}
	if err := WriteCompactSize(b, uint64(len(o.PkScript))); err != nil {
		return err
	}
	_, err := b.Write(o.PkScript)
	return err
}

// DeserializeTx is a convenience wrapper around ReadTx.
func DeserializeTx(data []byte) (*Tx, error) {
	return ReadTx(bytes.NewReader(data))
}
