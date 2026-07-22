// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	ProtocolVersion  = 1
	ProofModeGroth16 = 1
	// ProofModeCommitment is a SHA256 payload commitment (overlay; not a Groth16 SNARK).
	ProofModeCommitment = 2
)

// L2BlockHeader is the canonical L2 block header hashed for anchors and sync.
type L2BlockHeader struct {
	Version       uint32 `json:"version"`
	L2Height      uint64 `json:"l2_height"`
	ParentHash    string `json:"parent_hash"`    // 64 hex
	StateRoot     string `json:"state_root"`     // 64 hex
	ProofDigest   string `json:"proof_digest"`   // 64 hex SHA256(proof_blob)
	DogeAnchorTx  string `json:"doge_anchor_tx,omitempty"`
	DogeHeight    int64  `json:"doge_height,omitempty"`
	ProofMode     uint32 `json:"proof_mode"`
	SignerAddress string `json:"signer_address,omitempty"`
	Signature     string `json:"signature,omitempty"` // base64 or hex dogecoin message signature
}

// L2Block is header + opaque proof payload stored off L1.
type L2Block struct {
	Header    L2BlockHeader `json:"header"`
	ProofBlob string        `json:"proof_blob,omitempty"` // hex
	PublicInputs []string   `json:"public_inputs,omitempty"`
}

// AnchorRecord indexes an L1 OP_RETURN ZKDG anchor.
type AnchorRecord struct {
	AnchorHash string `json:"anchor_hash"`
	TxID       string `json:"txid"`
	Height     int64  `json:"height"`
	Vout       uint32 `json:"vout"`
}

// AnchorMessage is signed by the operator wallet to bind an L2 transition (via signmessage RPC).
type AnchorMessage struct {
	Network    string `json:"network"`
	L2Height   uint64 `json:"l2_height"`
	ParentHash string `json:"parent_hash"`
	StateRoot  string `json:"state_root"`
	ProofDigest string `json:"proof_digest"`
}

// HashHeader returns SHA256 of canonical header fields (excluding signature fields).
func HashHeader(h L2BlockHeader) ([32]byte, error) {
	var zero [32]byte
	parent, err := decodeHash32(h.ParentHash)
	if err != nil {
		return zero, err
	}
	state, err := decodeHash32(h.StateRoot)
	if err != nil {
		return zero, err
	}
	proof, err := decodeHash32(h.ProofDigest)
	if err != nil {
		return zero, err
	}
	buf := make([]byte, 4+8+32*3+8+4)
	binary.LittleEndian.PutUint32(buf[0:4], h.Version)
	binary.LittleEndian.PutUint64(buf[4:12], h.L2Height)
	copy(buf[12:44], parent[:])
	copy(buf[44:76], state[:])
	copy(buf[76:108], proof[:])
	binary.LittleEndian.PutUint64(buf[108:116], uint64(h.DogeHeight))
	binary.LittleEndian.PutUint32(buf[116:120], h.ProofMode)
	return sha256.Sum256(buf), nil
}

// AnchorHashFromHeader is the ZKDG OP_RETURN commitment (header hash).
func AnchorHashFromHeader(h L2BlockHeader) (string, error) {
	sum, err := HashHeader(h)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum[:]), nil
}

// PrepareAnchorMessageJSON builds the message users sign with signmessage (wallet RPC only).
func PrepareAnchorMessageJSON(network string, h L2BlockHeader) (string, error) {
	msg := AnchorMessage{
		Network:     strings.ToLower(strings.TrimSpace(network)),
		L2Height:    h.L2Height,
		ParentHash:  strings.ToLower(h.ParentHash),
		StateRoot:   strings.ToLower(h.StateRoot),
		ProofDigest: strings.ToLower(h.ProofDigest),
	}
	raw, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ProofDigest computes SHA256(proof_blob bytes).
func ProofDigest(proofHex string) (string, error) {
	b, err := hex.DecodeString(strings.TrimSpace(proofHex))
	if err != nil {
		return "", fmt.Errorf("proof_blob hex: %w", err)
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func decodeHash32(h string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(strings.TrimSpace(h))
	if err != nil || len(b) != 32 {
		return out, fmt.Errorf("want 32-byte hash hex")
	}
	copy(out[:], b)
	return out, nil
}

// ValidateL2Header checks header fields without requiring proof_blob.
func ValidateL2Header(h L2BlockHeader) error {
	if h.Version == 0 {
		return fmt.Errorf("l2 header version required")
	}
	for _, f := range []string{h.ParentHash, h.StateRoot, h.ProofDigest} {
		if _, err := decodeHash32(f); err != nil {
			return err
		}
	}
	if h.L2Height == 0 && !strings.EqualFold(h.ParentHash, strings.Repeat("0", 64)) {
		return fmt.Errorf("genesis l2 block requires zero parent")
	}
	if h.ProofMode != 0 && h.ProofMode != ProofModeGroth16 {
		return fmt.Errorf("unsupported proof_mode %d", h.ProofMode)
	}
	return nil
}

// ValidateL2Block performs structural checks (not full ZK crypto yet).
func ValidateL2Block(b L2Block) error {
	if err := ValidateL2Header(b.Header); err != nil {
		return err
	}
	if b.Header.ProofMode == 0 {
		b.Header.ProofMode = ProofModeGroth16
	}
	if b.ProofBlob != "" {
		d, err := ProofDigest(b.ProofBlob)
		if err != nil {
			return err
		}
		if !strings.EqualFold(d, b.Header.ProofDigest) {
			return fmt.Errorf("proof_digest mismatch")
		}
	}
	anchor, err := AnchorHashFromHeader(b.Header)
	if err != nil {
		return err
	}
	_ = anchor
	return nil
}
