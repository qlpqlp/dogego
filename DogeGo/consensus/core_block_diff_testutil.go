// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/pow"
	"dogego/primitives"
	"dogego/store"
	"dogego/wire"
)

type coreBlockVector struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Network         string `json:"network"`
	Height          int64  `json:"height"`
	ChainTipHeight  int64  `json:"chain_tip_height"`
	Source          string `json:"source"`
	Hex             string `json:"hex"`
	Mutation        string `json:"mutation"`
	WantAccept      bool   `json:"want_accept"`
	WantErrorSubstr string `json:"want_error_substr"`
}

func (v coreBlockVector) resolvedChainTipHeight() int64 {
	if v.ChainTipHeight > 0 {
		return v.ChainTipHeight
	}
	return v.Height
}

func minimalBlockRaw() ([]byte, [32]byte) {
	return store.TestMinimalBlock()
}

func serializeParsedBlock(pb *wire.ParsedBlock) ([]byte, error) {
	if pb == nil || len(pb.Txs) == 0 {
		return nil, fmt.Errorf("empty block")
	}
	var buf bytes.Buffer
	h80 := pb.Header.EncodeWire80()
	if _, err := buf.Write(h80[:]); err != nil {
		return nil, err
	}
	if err := wire.WriteCompactSize(&buf, uint64(len(pb.Txs))); err != nil {
		return nil, err
	}
	for _, tx := range pb.Txs {
		raw, err := tx.Serialize()
		if err != nil {
			return nil, err
		}
		if _, err := buf.Write(raw); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func applyBlockMutation(raw []byte, mutation string) ([]byte, error) {
	if mutation == "" {
		return append([]byte(nil), raw...), nil
	}
	switch mutation {
	case "bad_merkle":
		out := append([]byte(nil), raw...)
		if len(out) < 37 {
			return nil, fmt.Errorf("short block")
		}
		out[36] ^= 1
		return out, nil
	case "duplicate_txid":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		dup := make([]*wire.Tx, 0, len(pb.Txs)*2)
		dup = append(dup, pb.Txs...)
		dup = append(dup, pb.Txs[0])
		pb.Txs = dup
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "oversize_coinbase":
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: make([]byte, MaxBlockBaseSize)}},
		}
		hdr := primitives.BlockHeader{
			Version:    1,
			PrevBlock:  [32]byte{},
			MerkleRoot: tx.TxHash(),
			Timestamp:  1747000000,
			Bits:       0x1e0ffff0,
			Nonce:      1,
		}
		pb := &wire.ParsedBlock{
			Header: hdr,
			Txs: []*wire.Tx{
				{Version: 1, Vin: []wire.TxIn{{Sequence: 0xffffffff}}, Vout: []wire.TxOut{{Value: 50, PkScript: []byte{0x51}}}},
				tx,
			},
		}
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_cb_multiple":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		dup := &wire.Tx{
			Version: pb.Txs[0].Version,
			Vin: []wire.TxIn{{
				PrevHash: pb.Txs[0].Vin[0].PrevHash,
				PrevIdx:  pb.Txs[0].Vin[0].PrevIdx,
				Script:   append(append([]byte(nil), pb.Txs[0].Vin[0].Script...), 0x02),
				Sequence: pb.Txs[0].Vin[0].Sequence,
			}},
			Vout: append([]wire.TxOut(nil), pb.Txs[0].Vout...),
		}
		pb.Txs = append(pb.Txs, dup)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_cb_missing":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		pb.Txs[0] = &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{1}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "header_hash_mismatch":
		out := append([]byte(nil), raw...)
		if len(out) < 80 {
			return nil, fmt.Errorf("short block")
		}
		out[76] ^= 1
		return out, nil
	case "duplicate_spend":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		prev := [32]byte{2}
		tx1 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		tx2 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prev, PrevIdx: 0, Sequence: 0xffffffff, Script: []byte{0x01}}},
			Vout:    []wire.TxOut{{Value: 2, PkScript: []byte{0x52}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx1, tx2)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vout_negative":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{4}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: -1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vout_empty":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{5}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    nil,
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_prevout_null":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{
				{PrevHash: [32]byte{7}, PrevIdx: 0, Sequence: 0xffffffff},
				{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Sequence: 0xffffffff},
			},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_cb_length":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 || len(pb.Txs[0].Vin) == 0 {
			return nil, fmt.Errorf("empty coinbase")
		}
		pb.Txs[0].Vin[0].Script = []byte{0x01}
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vout_empty_scriptpubkey":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{8}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: nil}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vin_empty":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     nil,
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vout_toolarge":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: MaxMoney + 1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_txouttotal_toolarge":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		half := int64(MaxMoney / 2)
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{10}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout: []wire.TxOut{
				{Value: half + 1, PkScript: []byte{0x51}},
				{Value: half + 1, PkScript: []byte{0x52}},
			},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_witness":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{11},
				PrevIdx:  0,
				Sequence: 0xffffffff,
				Witness:  [][]byte{{0x01}},
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "unspendable_output_with_value":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{12}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: []byte{0x6a, 0x00}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_vout_script_toolarge":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		bigScript := make([]byte, MaxBlockBaseSize+1)
		bigScript[0] = 0x51
		tx := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{14}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 1, PkScript: bigScript}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	case "bad_tx_oversize":
		pb, err := wire.ParseBlock(raw)
		if err != nil {
			return nil, err
		}
		if len(pb.Txs) == 0 {
			return nil, fmt.Errorf("empty block")
		}
		bigScript := make([]byte, MaxBlockBaseSize+10)
		tx := &wire.Tx{
			Version: 1,
			Vin: []wire.TxIn{{
				PrevHash: [32]byte{15},
				PrevIdx:  0,
				Sequence: 0xffffffff,
				Script:   bigScript,
			}},
			Vout: []wire.TxOut{{Value: 1, PkScript: []byte{0x51}}},
		}
		pb.Txs = append([]*wire.Tx{pb.Txs[0]}, tx)
		pb.Header.MerkleRoot = wire.BlockMerkleRoot(pb.Txs)
		return serializeParsedBlock(pb)
	default:
		return nil, fmt.Errorf("unknown block mutation %q", mutation)
	}
}

func blockRawForVector(v coreBlockVector) ([]byte, [32]byte, error) {
	var raw []byte
	switch v.Source {
	case "", "minimal":
		r, _ := minimalBlockRaw()
		raw = r
	case "hex":
		h := strings.TrimSpace(v.Hex)
		if strings.HasPrefix(strings.ToLower(h), "0x") {
			h = h[2:]
		}
		decoded, err := hex.DecodeString(h)
		if err != nil {
			return nil, [32]byte{}, fmt.Errorf("hex block: %w", err)
		}
		raw = decoded
	case "chain_genesis":
		net, err := networkFromFixture(v.Network)
		if err != nil {
			return nil, [32]byte{}, err
		}
		gen, err := chain.GenesisBlockRaw(net)
		if err != nil {
			return nil, [32]byte{}, fmt.Errorf("chain genesis: %w", err)
		}
		raw = gen
	default:
		return nil, [32]byte{}, fmt.Errorf("unsupported block source %q", v.Source)
	}
	mutated, err := applyBlockMutation(raw, v.Mutation)
	if err != nil {
		return nil, [32]byte{}, err
	}
	wantID := pow.BlockHashLE(mutated[:80])
	if v.Mutation == "header_hash_mismatch" {
		wantID = pow.BlockHashLE(raw[:80])
	}
	return mutated, wantID, nil
}

// minimalChainedBlockRaw builds a single-coinbase block whose header80 is wired to prevHash.
func minimalChainedBlockRaw(prevHash [32]byte, timestamp uint32, nonce uint32) ([]byte, [32]byte, error) {
	tx := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{}, PrevIdx: 0xffffffff, Script: []byte{2, 0}}},
		Vout:    []wire.TxOut{{Value: 50e8, PkScript: []byte{0x51}}},
	}
	txRaw, err := tx.Serialize()
	if err != nil {
		return nil, [32]byte{}, err
	}
	hdr := primitives.BlockHeader{
		Version:    1,
		PrevBlock:  prevHash,
		MerkleRoot: tx.TxHash(),
		Timestamp:  timestamp,
		Bits:       0x1e0ffff0,
		Nonce:      nonce,
	}
	h80 := hdr.EncodeWire80()
	root := wire.BlockMerkleRoot([]*wire.Tx{tx})
	copy(h80[36:68], root[:])
	var buf []byte
	buf = append(buf, h80[:]...)
	buf = append(buf, 1)
	buf = append(buf, txRaw...)
	id := pow.BlockHashLE(h80[:])
	return buf, id, nil
}
