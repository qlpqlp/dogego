// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package bbpow is an experimental research library for Bitcoin-Backed Proof-of-Work
// (BBPoW / CAuxPoW): verifying SHA-256 Bitcoin work as a Dogecoin security signal.
//
// This does NOT change Dogecoin L1 consensus. Extensions using this package only
// evaluate proofs off-chain. Making BBPoW a valid Dogecoin block proof would be a
// Dogecoin hard fork (it expands the set of valid blocks).
package bbpow

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"dogego/pow"
	"dogego/wire"
)

// CommitmentMagic marks a Dogecoin block hash commitment inside a Bitcoin coinbase
// (OP_RETURN or scriptSig). Distinct from Scrypt AuxPoW / Litecoin merge-mining.
var CommitmentMagic = []byte{'B', 'B', 'P', 'W'}

// BitcoinMainnetPowLimitMSB is Bitcoin's proof-of-work limit (MSB hex / display order).
const BitcoinMainnetPowLimitMSB = "00000000ffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

// Proof is a research-structure Bitcoin-backed proof for a Dogecoin block hash.
// JSON field names match RPC verifyproof payloads.
type Proof struct {
	Version           uint32   `json:"version"`
	DogeBlockHash     string   `json:"doge_block_hash"`      // display hex (MSB / explorer order)
	BitcoinHeaderHex  string   `json:"bitcoin_header_hex"`   // 80-byte header hex
	CoinbaseTxHex     string   `json:"coinbase_tx_hex"`      // full coinbase tx hex
	MerkleBranchHex   []string `json:"merkle_branch_hex"`    // sibling hashes (wire LE hex or display; we accept 64 hex)
	MerkleIndex       int32    `json:"merkle_index"`         // coinbase index in BTC block (usually 0)
	BitcoinContextHex []string `json:"bitcoin_context_hex"`  // optional extra 80-byte headers (newer→older or tip-ward)
	Notes             string   `json:"notes,omitempty"`
}

// VerifyResult is the outcome of ValidateProof (research only; not L1 consensus).
type VerifyResult struct {
	OK                bool   `json:"ok"`
	Error             string `json:"error,omitempty"`
	DogeBlockHash     string `json:"doge_block_hash,omitempty"`
	BitcoinBlockHash  string `json:"bitcoin_block_hash,omitempty"`
	BitcoinBits       uint32 `json:"bitcoin_bits,omitempty"`
	BitcoinTime       uint32 `json:"bitcoin_time,omitempty"`
	WorkApproxHex     string `json:"work_approx_hex,omitempty"`
	CommitmentWhere   string `json:"commitment_where,omitempty"` // scriptsig | op_return
	MerkleOK          bool   `json:"merkle_ok"`
	SHA256PoWOK       bool   `json:"sha256_pow_ok"`
	ContextHeaders    int    `json:"context_headers"`
	NotConsensus      string `json:"not_consensus"`
	HardForkNote      string `json:"hard_fork_note"`
}

// BuildCommitmentPayload returns magic || dogeBlockHashLE (32 bytes LE wire order).
func BuildCommitmentPayload(dogeBlockHashDisplayHex string) ([]byte, error) {
	le, err := displayHexToLE32(dogeBlockHashDisplayHex)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 0, 4+32)
	out = append(out, CommitmentMagic...)
	out = append(out, le[:]...)
	return out, nil
}

// BuildCommitmentHex is BuildCommitmentPayload as hex (for miners / templates).
func BuildCommitmentHex(dogeBlockHashDisplayHex string) (string, error) {
	b, err := BuildCommitmentPayload(dogeBlockHashDisplayHex)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidateProof verifies SHA-256 PoW, commitment presence, and merkle inclusion.
func ValidateProof(p Proof) VerifyResult {
	res := VerifyResult{
		NotConsensus: "BBPoW proofs are evaluated by this extension only; Dogecoin L1 still requires Scrypt/AuxPoW.",
		HardForkNote: "Accepting BBPoW as a Dogecoin block proof would expand validity rules (hard fork), unlike SegWit which tightened them.",
	}
	if p.Version != 0 && p.Version != 1 {
		res.Error = "unsupported proof version (use 1)"
		return res
	}
	dogeDisp := strings.ToLower(strings.TrimSpace(p.DogeBlockHash))
	if dogeDisp == "" {
		res.Error = "doge_block_hash required"
		return res
	}
	dogeLE, err := displayHexToLE32(dogeDisp)
	if err != nil {
		res.Error = "doge_block_hash: " + err.Error()
		return res
	}
	res.DogeBlockHash = dogeDisp

	h80, err := decodeExactHex(p.BitcoinHeaderHex, 80)
	if err != nil {
		res.Error = "bitcoin_header_hex: " + err.Error()
		return res
	}
	bits := binary.LittleEndian.Uint32(h80[72:76])
	timeU := binary.LittleEndian.Uint32(h80[68:72])
	res.BitcoinBits = bits
	res.BitcoinTime = timeU
	res.BitcoinBlockHash = pow.BlockHashHex(h80)

	if err := CheckSHA256PoW(h80, bits); err != nil {
		res.Error = "sha256 pow: " + err.Error()
		return res
	}
	res.SHA256PoWOK = true
	if w, err := pow.BlockProofFromBits(bits); err == nil && w != nil {
		res.WorkApproxHex = w.Text(16)
	}

	cbRaw, err := hex.DecodeString(strings.TrimSpace(p.CoinbaseTxHex))
	if err != nil {
		res.Error = "coinbase_tx_hex: " + err.Error()
		return res
	}
	cb, err := wire.ReadTx(bytes.NewReader(cbRaw))
	if err != nil {
		res.Error = "coinbase decode: " + err.Error()
		return res
	}
	where, ok := FindCommitment(cb, dogeLE)
	if !ok {
		res.Error = "commitment BBPoW||doge_hash not found in coinbase scriptSig or OP_RETURN"
		return res
	}
	res.CommitmentWhere = where

	branch, err := decodeBranch(p.MerkleBranchHex)
	if err != nil {
		res.Error = "merkle_branch: " + err.Error()
		return res
	}
	txHash := cb.TxHash()
	calcRoot := pow.CheckMerkleBranch(txHash, branch, p.MerkleIndex)
	var hdrRoot [32]byte
	copy(hdrRoot[:], h80[36:68])
	if calcRoot != hdrRoot {
		res.Error = fmt.Sprintf("merkle root mismatch (calc %s want %s)", pow.LEUint256DisplayHex(calcRoot[:]), pow.LEUint256DisplayHex(hdrRoot[:]))
		return res
	}
	res.MerkleOK = true

	ctxN, err := verifyContextHeaders(h80, p.BitcoinContextHex)
	if err != nil {
		res.Error = "bitcoin_context: " + err.Error()
		return res
	}
	res.ContextHeaders = ctxN
	res.OK = true
	return res
}

// CheckSHA256PoW verifies double-SHA256(header) meets nBits (hash ≤ target).
// Does not enforce Bitcoin mainnet pow-limit (research proofs may use easy targets).
// Use CheckSHA256PoWMainnet when validating against Bitcoin mainnet rules.
func CheckSHA256PoW(header80 []byte, bits uint32) error {
	if len(header80) != 80 {
		return fmt.Errorf("header must be 80 bytes")
	}
	hashLE := pow.BlockHashLE(header80)
	// All-F limit: only enforce hash ≤ compact target (research / regtest-style bits).
	const researchPowLimitMSB = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	return pow.CheckProofOfWorkLE(hashLE[:], bits, researchPowLimitMSB)
}

// CheckSHA256PoWMainnet is CheckSHA256PoW plus Bitcoin mainnet pow-limit.
func CheckSHA256PoWMainnet(header80 []byte, bits uint32) error {
	if len(header80) != 80 {
		return fmt.Errorf("header must be 80 bytes")
	}
	hashLE := pow.BlockHashLE(header80)
	return pow.CheckProofOfWorkLE(hashLE[:], bits, BitcoinMainnetPowLimitMSB)
}

// FindCommitment locates magic||dogeHashLE in coinbase scriptSig or OP_RETURN outputs.
func FindCommitment(tx *wire.Tx, dogeHashLE [32]byte) (where string, ok bool) {
	if tx == nil {
		return "", false
	}
	needle := append(append([]byte{}, CommitmentMagic...), dogeHashLE[:]...)
	if len(tx.Vin) > 0 && bytes.Contains(tx.Vin[0].Script, needle) {
		return "scriptsig", true
	}
	for _, o := range tx.Vout {
		if isOpReturn(o.PkScript) && bytes.Contains(o.PkScript, needle) {
			return "op_return", true
		}
	}
	return "", false
}

func isOpReturn(pk []byte) bool {
	return len(pk) >= 1 && pk[0] == 0x6a
}

func verifyContextHeaders(tip80 []byte, ctxHex []string) (int, error) {
	if len(ctxHex) == 0 {
		return 0, nil
	}
	cur := tip80
	for i, hx := range ctxHex {
		h, err := decodeExactHex(hx, 80)
		if err != nil {
			return i, fmt.Errorf("header %d: %w", i, err)
		}
		bits := binary.LittleEndian.Uint32(h[72:76])
		if err := CheckSHA256PoW(h, bits); err != nil {
			return i, fmt.Errorf("header %d pow: %w", i, err)
		}
		// Context headers are parents: each entry's hash must equal previous header's prev-hash field.
		parentID := pow.BlockHashLE(h)
		var prev [32]byte
		copy(prev[:], cur[4:36])
		if parentID != prev {
			return i, fmt.Errorf("header %d does not link as parent of previous", i)
		}
		cur = h
	}
	return len(ctxHex), nil
}

func decodeBranch(hexes []string) ([][32]byte, error) {
	out := make([][32]byte, 0, len(hexes))
	for i, hx := range hexes {
		b, err := hex.DecodeString(strings.TrimSpace(hx))
		if err != nil || len(b) != 32 {
			return nil, fmt.Errorf("entry %d: need 32 bytes", i)
		}
		var h [32]byte
		copy(h[:], b)
		out = append(out, h)
	}
	return out, nil
}

func decodeExactHex(s string, n int) ([]byte, error) {
	b, err := hex.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, err
	}
	if len(b) != n {
		return nil, fmt.Errorf("want %d bytes got %d", n, len(b))
	}
	return b, nil
}

func displayHexToLE32(disp string) ([32]byte, error) {
	var out [32]byte
	b, err := hex.DecodeString(strings.TrimSpace(disp))
	if err != nil || len(b) != 32 {
		return out, fmt.Errorf("need 32-byte hex")
	}
	// Display / explorer hex is byte-reversed vs wire LE.
	for i := 0; i < 32; i++ {
		out[i] = b[31-i]
	}
	return out, nil
}
