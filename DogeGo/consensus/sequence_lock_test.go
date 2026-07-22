// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"errors"
	"testing"

	"dogego/chain"
	"dogego/wire"
)

func TestSequenceLocks_heightRelative(t *testing.T) {
	j := &seqTestJournal{times: map[int64]uint32{0: 100, 1: 200, 2: 300, 3: 400, 4: 500}}
	tx := &wire.Tx{
		Version: 2,
		Vin: []wire.TxIn{{
			Sequence: 2, // 2 blocks after coin height
		}},
		Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	prevHeights := []int{1}
	if SequenceLocks(tx, SequenceEvalBlock{Height: 2}, prevHeights, j, true) {
		t.Fatal("height 2 should be too early (need >2)")
	}
	if !SequenceLocks(tx, SequenceEvalBlock{Height: 3}, prevHeights, j, true) {
		t.Fatal("height 3 should satisfy 2-block relative lock from height 1")
	}
}

func TestSequenceLocks_disableFlag(t *testing.T) {
	tx := &wire.Tx{
		Version: 2,
		Vin:     []wire.TxIn{{Sequence: wire.SequenceLocktimeDisableFlag | 5}},
		Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	if !SequenceLocks(tx, SequenceEvalBlock{Height: 0}, []int{0}, nil, true) {
		t.Fatal("disabled sequence should not enforce")
	}
}

func TestCheckTxSequenceLocks_rejects(t *testing.T) {
	j := &seqTestJournal{times: map[int64]uint32{499_994: 1000}}
	tx := &wire.Tx{
		Version:  2,
		LockTime: 0,
		Vin:      []wire.TxIn{{Sequence: 10}},
		Vout:     []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
	}
	// Post-CSV mainnet height; coin at 499995 needs block > 500004 for 10-block relative lock.
	err := CheckTxSequenceLocks(tx, SequenceEvalBlock{Height: 500_000}, []int{499_995}, j, chain.MainnetDogecoin)
	if !errors.Is(err, ErrSequenceLock) {
		t.Fatalf("got %v", err)
	}
}

type seqTestJournal struct {
	times map[int64]uint32
}

func (s *seqTestJournal) TipHeight() (int64, error) {
	var max int64
	for h := range s.times {
		if h > max {
			max = h
		}
	}
	return max, nil
}

func (s *seqTestJournal) ReadHeaderAt(h int64) ([]byte, error) {
	var b [80]byte
	if t, ok := s.times[h]; ok {
		b[68] = byte(t)
		b[69] = byte(t >> 8)
		b[70] = byte(t >> 16)
		b[71] = byte(t >> 24)
	}
	return b[:], nil
}

func (s *seqTestJournal) HeightByDisplayHash(string) (int64, error) {
	return 0, nil
}
