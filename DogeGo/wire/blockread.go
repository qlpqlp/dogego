// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"crypto/sha256"
	"fmt"

	"dogego/pow"
	"dogego/primitives"
)

// ParsedBlock is a decoded P2P block message body: 80-byte child header,
// optional AuxPoW blob (merge-mined), then transactions.
type ParsedBlock struct {
	Header primitives.BlockHeader
	Aux    *AuxPow // set when header has auxpow version bit
	Txs    []*Tx
}

// ParseBlock decodes a full block payload: 80-byte header, optional CAuxPow, compact tx count, txs.
func ParseBlock(raw []byte) (*ParsedBlock, error) {
	if len(raw) < 81 {
		return nil, fmt.Errorf("block too short %d", len(raw))
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return nil, err
	}
	r := bytes.NewReader(raw[80:])
	var aux *AuxPow
	if isAuxPowVersion(hdr.Version) {
		a, err := ReadAuxPow(r)
		if err != nil {
			return nil, fmt.Errorf("auxpow: %w", err)
		}
		aux = a
	}
	nTx, err := ReadCompactSize(r)
	if err != nil {
		return nil, err
	}
	if nTx == 0 {
		return nil, fmt.Errorf("zero transactions")
	}
	if nTx > 200000 {
		return nil, fmt.Errorf("excessive tx count %d", nTx)
	}
	txs := make([]*Tx, 0, nTx)
	for i := uint64(0); i < nTx; i++ {
		tx, err := ReadTx(r)
		if err != nil {
			return nil, fmt.Errorf("tx %d: %w", i, err)
		}
		txs = append(txs, tx)
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("trailing %d bytes after txs", r.Len())
	}
	return &ParsedBlock{Header: hdr, Aux: aux, Txs: txs}, nil
}

// BlockTxCount returns the transaction count in a block payload without decoding txs.
func BlockTxCount(raw []byte) (uint64, error) {
	_, n, err := openBlockTxReader(raw)
	return n, err
}

func openBlockTxReader(raw []byte) (*bytes.Reader, uint64, error) {
	if len(raw) < 81 {
		return nil, 0, fmt.Errorf("block too short %d", len(raw))
	}
	var hdr primitives.BlockHeader
	if err := hdr.DecodeWire80(raw[:80]); err != nil {
		return nil, 0, err
	}
	r := bytes.NewReader(raw[80:])
	if isAuxPowVersion(hdr.Version) {
		if _, err := ReadAuxPow(r); err != nil {
			return nil, 0, fmt.Errorf("auxpow: %w", err)
		}
	}
	nTx, err := ReadCompactSize(r)
	if err != nil {
		return nil, 0, err
	}
	if nTx == 0 {
		return nil, 0, fmt.Errorf("zero transactions")
	}
	if nTx > 200000 {
		return nil, 0, fmt.Errorf("excessive tx count %d", nTx)
	}
	return r, nTx, nil
}

// ForEachBlockTx decodes each transaction without retaining earlier txs (lower alloc during tx indexing).
func ForEachBlockTx(raw []byte, fn func(txIndex uint32, tx *Tx) error) error {
	r, nTx, err := openBlockTxReader(raw)
	if err != nil {
		return err
	}
	for i := uint64(0); i < nTx; i++ {
		tx, err := ReadTx(r)
		if err != nil {
			return fmt.Errorf("tx %d: %w", i, err)
		}
		if err := fn(uint32(i), tx); err != nil {
			return err
		}
	}
	if r.Len() != 0 {
		return fmt.Errorf("trailing %d bytes after txs", r.Len())
	}
	return nil
}

func isAuxPowVersion(v int32) bool {
	// Dogecoin: VERSION_AUXPOW = (1 << 8) - see src/versionbits.h / auxpow consensus.
	const auxPowBit = 1 << 8
	return v&auxPowBit != 0
}

// HashPair returns double-SHA256(a||b) for merkle tree levels (Bitcoin / Dogecoin merkle node rule).
func HashPair(a, b [32]byte) [32]byte {
	buf := append(append([]byte{}, a[:]...), b[:]...)
	h := sha256.Sum256(buf)
	h2 := sha256.Sum256(h[:])
	var out [32]byte
	copy(out[:], h2[:])
	return out
}

// BlockMerkleRoot returns the merkle root of the tx list (legacy witness hashing).
func BlockMerkleRoot(txs []*Tx) [32]byte {
	if len(txs) == 0 {
		return [32]byte{}
	}
	hashes := make([][32]byte, len(txs))
	for i, tx := range txs {
		hashes[i] = tx.TxHash()
	}
	return MerkleRootFromTxHashes(hashes)
}

// VerifyBlockMerkle checks header merkle root against txs (Core-style tree).
func VerifyBlockMerkle(pb *ParsedBlock) error {
	if pb == nil {
		return fmt.Errorf("nil block")
	}
	got := BlockMerkleRoot(pb.Txs)
	if got != pb.Header.MerkleRoot {
		return fmt.Errorf("merkle root mismatch")
	}
	return nil
}

// VerifyBlockHeaderHash checks double-SHA256 of 80-byte header against expected block id (LE).
func VerifyBlockHeaderHash(pb *ParsedBlock, wantBlockID [32]byte) error {
	if pb == nil {
		return fmt.Errorf("nil block")
	}
	h80 := pb.Header.EncodeWire80()
	got := pow.BlockHashLE(h80[:])
	if got != wantBlockID {
		return fmt.Errorf("block header hash mismatch")
	}
	return nil
}

// ValidateParsedBlock runs merkle + header hash checks (post-parse).
func ValidateParsedBlock(pb *ParsedBlock, wantBlockID [32]byte) error {
	if err := VerifyBlockMerkle(pb); err != nil {
		return err
	}
	return VerifyBlockHeaderHash(pb, wantBlockID)
}
