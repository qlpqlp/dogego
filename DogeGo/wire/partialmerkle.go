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

// PartialMerkleTree is a Core-compatible CPartialMerkleTree (BIP37 / merkleblock.cpp).
type PartialMerkleTree struct {
	nTransactions uint32
	vBits           []bool
	vHash           [][32]byte
	fBad            bool
}

func calcTreeWidth(nTransactions int, height int) int {
	return (nTransactions + (1 << height) - 1) >> height
}

func merkleTreeDepth(nTransactions int) int {
	h := 0
	for calcTreeWidth(nTransactions, h) > 1 {
		h++
	}
	return h
}

func (t *PartialMerkleTree) calcHash(height int, pos int, vTxid [][32]byte) [32]byte {
	if height == 0 {
		return vTxid[pos]
	}
	left := t.calcHash(height-1, pos*2, vTxid)
	var right [32]byte
	if pos*2+1 < calcTreeWidth(len(vTxid), height-1) {
		right = t.calcHash(height-1, pos*2+1, vTxid)
	} else {
		right = left
	}
	return HashPair(left, right)
}

func (t *PartialMerkleTree) traverseAndBuild(height, pos int, vTxid [][32]byte, vMatch []bool) {
	fParentOfMatch := false
	for p := pos << height; p < (pos+1)<<height && p < len(vTxid); p++ {
		fParentOfMatch = fParentOfMatch || vMatch[p]
	}
	t.vBits = append(t.vBits, fParentOfMatch)
	if height == 0 || !fParentOfMatch {
		t.vHash = append(t.vHash, t.calcHash(height, pos, vTxid))
	} else {
		t.traverseAndBuild(height-1, pos*2, vTxid, vMatch)
		if pos*2+1 < calcTreeWidth(len(vTxid), height-1) {
			t.traverseAndBuild(height-1, pos*2+1, vTxid, vMatch)
		}
	}
}

// NewPartialMerkleTree builds a partial merkle tree from full txid list (leaf order) and match flags.
func NewPartialMerkleTree(vTxid [][32]byte, vMatch []bool) (*PartialMerkleTree, error) {
	if len(vTxid) != len(vMatch) {
		return nil, fmt.Errorf("partial merkle: vTxid/vMatch length mismatch")
	}
	if len(vTxid) == 0 {
		return nil, fmt.Errorf("partial merkle: no transactions")
	}
	t := &PartialMerkleTree{nTransactions: uint32(len(vTxid)), fBad: false}
	depth := merkleTreeDepth(len(vTxid))
	t.traverseAndBuild(depth, 0, vTxid, vMatch)
	return t, nil
}

func (t *PartialMerkleTree) traverseAndExtract(height, pos int, nBitsUsed, nHashUsed *uint32, vMatch *[][32]byte, vnIndex *[]uint32) [32]byte {
	if int(*nBitsUsed) >= len(t.vBits) {
		t.fBad = true
		return [32]byte{}
	}
	fParentOfMatch := t.vBits[*nBitsUsed]
	*nBitsUsed++
	if height == 0 || !fParentOfMatch {
		if int(*nHashUsed) >= len(t.vHash) {
			t.fBad = true
			return [32]byte{}
		}
		h := t.vHash[*nHashUsed]
		*nHashUsed++
		if height == 0 && fParentOfMatch {
			*vMatch = append(*vMatch, h)
			*vnIndex = append(*vnIndex, uint32(pos))
		}
		return h
	}
	left := t.traverseAndExtract(height-1, pos*2, nBitsUsed, nHashUsed, vMatch, vnIndex)
	var right [32]byte
	if pos*2+1 < calcTreeWidth(int(t.nTransactions), height-1) {
		right = t.traverseAndExtract(height-1, pos*2+1, nBitsUsed, nHashUsed, vMatch, vnIndex)
		if right == left {
			t.fBad = true
		}
	} else {
		right = left
	}
	return HashPair(left, right)
}

// ExtractMatches returns the merkle root implied by the tree and matched tx internal hashes (TxHash order).
func (t *PartialMerkleTree) ExtractMatches() (merkleRoot [32]byte, matches [][32]byte, indices []uint32, ok bool) {
	matches = nil
	indices = nil
	if t.nTransactions == 0 {
		return [32]byte{}, nil, nil, false
	}
	if len(t.vHash) > int(t.nTransactions) {
		return [32]byte{}, nil, nil, false
	}
	if len(t.vBits) < len(t.vHash) {
		return [32]byte{}, nil, nil, false
	}
	depth := merkleTreeDepth(int(t.nTransactions))
	var nBitsUsed, nHashUsed uint32
	root := t.traverseAndExtract(depth, 0, &nBitsUsed, &nHashUsed, &matches, &indices)
	if t.fBad {
		return [32]byte{}, nil, nil, false
	}
	if (nBitsUsed+7)/8 != uint32((len(t.vBits)+7)/8) {
		return [32]byte{}, nil, nil, false
	}
	if int(nHashUsed) != len(t.vHash) {
		return [32]byte{}, nil, nil, false
	}
	return root, matches, indices, true
}

// WritePartialMerkle encodes nTransactions, hashes, and flag bits (Core-compatible).
func (t *PartialMerkleTree) WritePartialMerkle(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, t.nTransactions); err != nil {
		return err
	}
	if err := WriteCompactSize(w, uint64(len(t.vHash))); err != nil {
		return err
	}
	for _, h := range t.vHash {
		if _, err := w.Write(h[:]); err != nil {
			return err
		}
	}
	nFlagBytes := (len(t.vBits) + 7) / 8
	if err := WriteCompactSize(w, uint64(nFlagBytes)); err != nil {
		return err
	}
	var flags = make([]byte, nFlagBytes)
	for p := 0; p < len(t.vBits); p++ {
		if t.vBits[p] {
			flags[p/8] |= 1 << uint(p%8)
		}
	}
	_, err := w.Write(flags)
	return err
}

// ReadPartialMerklePayload reads varint hash count, hashes, varint flag bytes, flags (after nTransactions LE uint32).
func ReadPartialMerklePayload(r *bytes.Reader, nTx uint32) (*PartialMerkleTree, error) {
	t := &PartialMerkleTree{nTransactions: nTx, fBad: false}
	nHash, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nHash > 1_000_000 {
		return nil, fmt.Errorf("partial merkle: excessive hash count")
	}
	t.vHash = make([][32]byte, nHash)
	for i := uint64(0); i < nHash; i++ {
		if _, err := io.ReadFull(r, t.vHash[i][:]); err != nil {
			return nil, err
		}
	}
	nFlagBytes, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nFlagBytes > 1_000_000 {
		return nil, fmt.Errorf("partial merkle: excessive flag bytes")
	}
	flagBytes := make([]byte, nFlagBytes)
	if _, err := io.ReadFull(r, flagBytes); err != nil {
		return nil, err
	}
	t.vBits = make([]bool, nFlagBytes*8)
	for p := 0; p < len(t.vBits); p++ {
		t.vBits[p] = (flagBytes[p/8] & (1 << uint(p%8))) != 0
	}
	return t, nil
}

// SerializeMerkleBlock writes 80-byte header + partial merkle tree (CMerkleBlock wire encoding).
func SerializeMerkleBlock(header80 []byte, pmt *PartialMerkleTree) ([]byte, error) {
	if len(header80) != 80 {
		return nil, fmt.Errorf("merkle block: want 80-byte header, got %d", len(header80))
	}
	var buf bytes.Buffer
	if _, err := buf.Write(header80); err != nil {
		return nil, err
	}
	if err := pmt.WritePartialMerkle(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseMerkleBlockProof parses CMerkleBlock bytes into header80 and partial tree.
func ParseMerkleBlockProof(data []byte) (header80 []byte, pmt *PartialMerkleTree, err error) {
	if len(data) < 80+4 {
		return nil, nil, fmt.Errorf("merkle proof too short")
	}
	header80 = append([]byte(nil), data[:80]...)
	r := bytes.NewReader(data[80:])
	var nTx uint32
	if err := binary.Read(r, binary.LittleEndian, &nTx); err != nil {
		return nil, nil, err
	}
	pmt, err = ReadPartialMerklePayload(r, nTx)
	if err != nil {
		return nil, nil, err
	}
	if r.Len() != 0 {
		return nil, nil, fmt.Errorf("merkle proof trailing %d bytes", r.Len())
	}
	return header80, pmt, nil
}
