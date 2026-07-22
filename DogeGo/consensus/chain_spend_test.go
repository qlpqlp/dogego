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

type spendScanJournal struct {
	tip int64
	h80 []byte
}

func (s *spendScanJournal) TipHeight() (int64, error)  { return s.tip, nil }
func (s *spendScanJournal) ReadHeaderAt(int64) ([]byte, error) { return s.h80, nil }
func (s *spendScanJournal) HeightByDisplayHash(string) (int64, error) { return 0, nil }

type spendScanRaw struct {
	payload []byte
}

func (s *spendScanRaw) Get([32]byte) ([]byte, error) { return s.payload, nil }

func TestOutpointSpentInBlocks_ForEachScan(t *testing.T) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	prev := funding.TxHash()
	spender := &wire.Tx{
		Version: 1,
		Vin: []wire.TxIn{{
			PrevHash: prev,
			PrevIdx:  0,
			Script:   []byte{0},
		}},
		Vout: []wire.TxOut{{Value: 9e7, PkScript: []byte{0x51}}},
	}
	blockRaw := spendScanBlockRaw([]*wire.Tx{funding, spender})
	h80 := blockRaw[:80]
	j := &spendScanJournal{tip: 0, h80: h80}
	raw := &spendScanRaw{payload: blockRaw}
	spent, err := OutpointSpentInBlocks(j, raw, 0, 0, txidDisplayFromLE(prev), 0)
	if err != nil {
		t.Fatal(err)
	}
	if !spent {
		t.Fatal("expected spent")
	}
}

func spendScanBlockRaw(txs []*wire.Tx) []byte {
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

func TestOutpointSpentInBlocks_Unspent(t *testing.T) {
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{}},
		Vout:    []wire.TxOut{{Value: 1e8, PkScript: []byte{0x51}}},
	}
	blockRaw := spendScanBlockRaw([]*wire.Tx{funding})
	j := &spendScanJournal{tip: 0, h80: blockRaw[:80]}
	raw := &spendScanRaw{payload: blockRaw}
	spent, err := OutpointSpentInBlocks(j, raw, 0, 0, txidDisplayFromLE(funding.TxHash()), 0)
	if err != nil {
		t.Fatal(err)
	}
	if spent {
		t.Fatal("expected unspent")
	}
}
