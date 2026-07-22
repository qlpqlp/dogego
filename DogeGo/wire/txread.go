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
	"io"
)

const maxTxBytes = 4 << 20 // 4 MiB (generous for aux coinbase)

// Tx is a deserialized transaction (witness may be present).
type Tx struct {
	Version  int32
	Vin      []TxIn
	Vout     []TxOut
	LockTime uint32
}

type TxIn struct {
	PrevHash [32]byte
	PrevIdx  uint32
	Script   []byte
	Sequence uint32
	Witness  [][]byte // stack, empty if none
}

type TxOut struct {
	Value    int64
	PkScript []byte
}

// TxHash returns double-SHA256 of legacy serialization (no witness), Core CTransaction::GetHash.
func (t *Tx) TxHash() [32]byte {
	s := t.SerializeForHash()
	h := sha256.Sum256(s)
	h2 := sha256.Sum256(h[:])
	var out [32]byte
	copy(out[:], h2[:])
	return out
}

// SerializeForHash serializes version|vin|vout|locktime without witness markers.
func (t *Tx) SerializeForHash() []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, t.Version)
	_ = WriteCompactSize(&b, uint64(len(t.Vin)))
	for _, in := range t.Vin {
		_, _ = b.Write(in.PrevHash[:])
		_ = binary.Write(&b, binary.LittleEndian, in.PrevIdx)
		_ = WriteCompactSize(&b, uint64(len(in.Script)))
		_, _ = b.Write(in.Script)
		_ = binary.Write(&b, binary.LittleEndian, in.Sequence)
	}
	// BIP141: zero-input txs use extended serialization (flags byte before outputs).
	if len(t.Vin) == 0 {
		_ = binary.Write(&b, binary.LittleEndian, byte(0))
	}
	_ = WriteCompactSize(&b, uint64(len(t.Vout)))
	for _, o := range t.Vout {
		_ = binary.Write(&b, binary.LittleEndian, o.Value)
		_ = WriteCompactSize(&b, uint64(len(o.PkScript)))
		_, _ = b.Write(o.PkScript)
	}
	_ = binary.Write(&b, binary.LittleEndian, t.LockTime)
	return b.Bytes()
}

// ReadTx reads a network-encoded transaction (witness-capable, Core-compatible).
func ReadTx(r *bytes.Reader) (*Tx, error) {
	t := &Tx{}
	if err := binary.Read(r, binary.LittleEndian, &t.Version); err != nil {
		return nil, err
	}
	flags := byte(0)
	nin, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	t.Vin = make([]TxIn, nin)
	for i := range t.Vin {
		if err := readTxIn(r, &t.Vin[i]); err != nil {
			return nil, err
		}
	}

	if len(t.Vin) == 0 {
		if err := binary.Read(r, binary.LittleEndian, &flags); err != nil {
			return nil, err
		}
		if flags != 0 {
			nin2, err := ReadCompactSize(r)
			if err != nil {
				return nil, err
			}
			t.Vin = make([]TxIn, nin2)
			for i := range t.Vin {
				if err := readTxIn(r, &t.Vin[i]); err != nil {
					return nil, err
				}
			}
		}
		if err := readTxOuts(r, t); err != nil {
			return nil, err
		}
	} else {
		if err := readTxOuts(r, t); err != nil {
			return nil, err
		}
	}

	if flags&1 != 0 {
		flags ^= 1
		for i := range t.Vin {
			ns, err := ReadCompactSize(r)
			if err != nil {
				return nil, err
			}
			t.Vin[i].Witness = make([][]byte, ns)
			for j := range t.Vin[i].Witness {
				sz, err := ReadCompactSize(r)
				if err != nil {
					return nil, err
				}
				if sz > maxTxBytes {
					return nil, fmt.Errorf("witness item too large")
				}
				t.Vin[i].Witness[j] = make([]byte, sz)
				if _, err := io.ReadFull(r, t.Vin[i].Witness[j]); err != nil {
					return nil, err
				}
			}
		}
	}
	if flags != 0 {
		return nil, fmt.Errorf("unknown tx optional data 0x%x", flags)
	}
	if err := binary.Read(r, binary.LittleEndian, &t.LockTime); err != nil {
		return nil, err
	}
	return t, nil
}

func readTxOuts(r *bytes.Reader, t *Tx) error {
	nout, err := ReadCompactSize(r)
	if err != nil {
		return err
	}
	t.Vout = make([]TxOut, nout)
	for i := range t.Vout {
		if err := readTxOut(r, &t.Vout[i]); err != nil {
			return err
		}
	}
	return nil
}

func readTxIn(r *bytes.Reader, o *TxIn) error {
	if _, err := io.ReadFull(r, o.PrevHash[:]); err != nil {
		return err
	}
	if err := binary.Read(r, binary.LittleEndian, &o.PrevIdx); err != nil {
		return err
	}
	sl, err := ReadCompactSize(r)
	if err != nil {
		return err
	}
	if sl > maxTxBytes {
		return fmt.Errorf("script too long")
	}
	o.Script = make([]byte, sl)
	if _, err := io.ReadFull(r, o.Script); err != nil {
		return err
	}
	return binary.Read(r, binary.LittleEndian, &o.Sequence)
}

func readTxOut(r *bytes.Reader, o *TxOut) error {
	if err := binary.Read(r, binary.LittleEndian, &o.Value); err != nil {
		return err
	}
	sl, err := ReadCompactSize(r)
	if err != nil {
		return err
	}
	if sl > maxTxBytes {
		return fmt.Errorf("pk script too long")
	}
	o.PkScript = make([]byte, sl)
	_, err = io.ReadFull(r, o.PkScript)
	return err
}
