// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"testing"

	"dogego/wire"
)

type memSpendJournal struct {
	tip int64
}

func (m *memSpendJournal) TipHeight() (int64, error)  { return m.tip, nil }
func (m *memSpendJournal) ReadHeaderAt(int64) ([]byte, error) { return make([]byte, 80), nil }
func (m *memSpendJournal) HeightByDisplayHash(string) (int64, error) { return 0, nil }

type memUtxo struct {
	tip   int64
	coins map[[36]byte]struct{}
}

func (m *memUtxo) TipHeight() int64 { return m.tip }

func (m *memUtxo) UnspentOutpoint(prevHash [32]byte, vout uint32) (int64, []byte, bool) {
	var k [36]byte
	copy(k[:32], prevHash[:])
	binaryPutU32(k[32:], vout)
	_, ok := m.coins[k]
	return 1, []byte{0x51}, ok
}

func binaryPutU32(b []byte, v uint32) {
	b[0] = byte(v)
	b[1] = byte(v >> 8)
	b[2] = byte(v >> 16)
	b[3] = byte(v >> 24)
}

type memSpendIndex struct {
	tx *wire.Tx
}

func (m *memSpendIndex) Lookup(txidHex string) ([32]byte, uint32, error) {
	_ = txidHex
	return [32]byte{1}, 0, nil
}

type memSpendRaw struct {
	raw []byte
}

func (m *memSpendRaw) Get([32]byte) ([]byte, error) {
	return m.raw, nil
}

func testBlockRaw(txs []*wire.Tx) []byte {
	h := make([]byte, 80)
	mr := wire.BlockMerkleRoot(txs)
	copy(h[36:68], mr[:])
	var buf bytes.Buffer
	buf.Write(h)
	buf.WriteByte(byte(len(txs)))
	for _, tx := range txs {
		b, _ := tx.Serialize()
		buf.Write(b)
	}
	return buf.Bytes()
}

func TestUtxoChainSpendViewSpent(t *testing.T) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	prev := funding.TxHash()
	utxo := &memUtxo{tip: 1, coins: map[[36]byte]struct{}{}} // empty set = spent
	view := NewUtxoChainSpendView(utxo, &memSpendJournal{tip: 1}, &memSpendRaw{raw: testBlockRaw([]*wire.Tx{funding})}, &memSpendIndex{tx: funding})
	spent, err := view.OutpointSpent(prev, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !spent {
		t.Fatal("expected spent")
	}
}

func TestUtxoChainSpendViewUnspent(t *testing.T) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	h := funding.TxHash()
	var k [36]byte
	copy(k[:32], h[:])
	utxo := &memUtxo{tip: 1, coins: map[[36]byte]struct{}{k: {}}}
	view := NewUtxoChainSpendView(utxo, &memSpendJournal{tip: 1}, nil, nil)
	spent, err := view.OutpointSpent(funding.TxHash(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if spent {
		t.Fatal("expected unspent")
	}
}
