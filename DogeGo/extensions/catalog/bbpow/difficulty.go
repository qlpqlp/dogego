// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package bbpow

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	"dogego/pow"
)

// Lane IDs for dual-algorithm research difficulty.
const (
	LaneScrypt = "scrypt_auxpow"
	LaneSHA256 = "sha256_bbpow"
)

// DualDifficultyModel is a research-only dual-lane difficulty sketch.
// A real hard-fork design would keep separate adjusters so one algo cannot starve the other.
type DualDifficultyModel struct {
	mu sync.Mutex

	ScryptBits uint32 `json:"scrypt_bits"`
	SHA256Bits uint32 `json:"sha256_bits"`

	ScryptBlocks int64 `json:"scrypt_blocks"`
	SHA256Blocks int64 `json:"sha256_blocks"`

	LastScryptUnix int64 `json:"last_scrypt_unix"`
	LastSHA256Unix int64 `json:"last_sha256_unix"`

	TargetSpacingSec int64   `json:"target_spacing_sec"`
	ShareCap         float64 `json:"share_cap"` // soft research cap: max fraction one lane should mine (e.g. 0.7)
}

// NewDualDifficultyModel returns defaults inspired by ~1 min Dogecoin spacing and dual-algo fairness.
func NewDualDifficultyModel() *DualDifficultyModel {
	return &DualDifficultyModel{
		ScryptBits:       0x1e0ffff0, // easy research default (not mainnet)
		SHA256Bits:       0x1d00ffff, // Bitcoin-era compact style research default
		TargetSpacingSec: 60,
		ShareCap:         0.70,
	}
}

// Snapshot returns a JSON-friendly copy without the mutex.
func (m *DualDifficultyModel) Snapshot() map[string]interface{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	scDiff, _ := pow.DifficultyFromCompact(m.ScryptBits)
	// SHA256 difficulty vs Bitcoin limit (same helper uses doge limit - report raw bits + work instead).
	shaWork, _ := pow.BlockProofFromBits(m.SHA256Bits)
	scWork, _ := pow.BlockProofFromBits(m.ScryptBits)
	total := m.ScryptBlocks + m.SHA256Blocks
	var scShare, shaShare float64
	if total > 0 {
		scShare = float64(m.ScryptBlocks) / float64(total)
		shaShare = float64(m.SHA256Blocks) / float64(total)
	}
	return map[string]interface{}{
		"model":               "independent_lanes_v0",
		"note":                "Research sketch only. Not applied to Dogecoin L1. Real design needs separate adjusters + anti-dominance rules.",
		"scrypt_bits":         fmt.Sprintf("%08x", m.ScryptBits),
		"sha256_bits":         fmt.Sprintf("%08x", m.SHA256Bits),
		"scrypt_blocks":       m.ScryptBlocks,
		"sha256_blocks":       m.SHA256Blocks,
		"scrypt_share":        scShare,
		"sha256_share":        shaShare,
		"share_cap":           m.ShareCap,
		"target_spacing_sec":  m.TargetSpacingSec,
		"scrypt_difficulty":   scDiff,
		"scrypt_work_hex":     bigHex(scWork),
		"sha256_work_hex":     bigHex(shaWork),
		"last_scrypt_unix":    m.LastScryptUnix,
		"last_sha256_unix":    m.LastSHA256Unix,
		"open_questions": []string{
			"Does 1 unit of Bitcoin work equal 1 unit of Dogecoin work?",
			"Separate difficulty adjustments per algorithm?",
			"How to prevent one mining class from overwhelming the other?",
			"How to incentivize Bitcoin miners to embed Dogecoin commitments?",
		},
	}
}

// RecordLaneBlock records a research block on one lane (does not affect L1).
func (m *DualDifficultyModel) RecordLaneBlock(lane string, bits uint32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().Unix()
	switch lane {
	case LaneSHA256:
		m.SHA256Blocks++
		m.LastSHA256Unix = now
		if bits != 0 {
			m.SHA256Bits = bits
		}
	default:
		m.ScryptBlocks++
		m.LastScryptUnix = now
		if bits != 0 {
			m.ScryptBits = bits
		}
	}
}

// DominanceWarning returns a soft warning when one lane exceeds ShareCap.
func (m *DualDifficultyModel) DominanceWarning() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	total := m.ScryptBlocks + m.SHA256Blocks
	if total < 10 {
		return ""
	}
	sc := float64(m.ScryptBlocks) / float64(total)
	sh := float64(m.SHA256Blocks) / float64(total)
	if sc > m.ShareCap {
		return fmt.Sprintf("scrypt lane share %.0f%% exceeds research cap %.0f%%", sc*100, m.ShareCap*100)
	}
	if sh > m.ShareCap {
		return fmt.Sprintf("sha256 lane share %.0f%% exceeds research cap %.0f%%", sh*100, m.ShareCap*100)
	}
	return ""
}

func bigHex(n *big.Int) string {
	if n == nil {
		return ""
	}
	return n.Text(16)
}

// CompareToAuxPoW returns educational comparison (not a consensus rule).
func CompareToAuxPoW() map[string]interface{} {
	return map[string]interface{}{
		"experimental":            true,
		"name_recommendation":     "Bitcoin-Backed Proof-of-Work (BBPoW) or Cross-Algorithm Auxiliary PoW (CAuxPoW)",
		"why_not_auxpow":          "Classic AuxPoW requires the same PoW algorithm on both chains. Bitcoin is SHA-256; Dogecoin AuxPoW parent PoW is Scrypt. Bitcoin ASICs cannot produce valid Scrypt AuxPoW.",
		"asics": map[string]interface{}{
			"bitcoin_sha256":        "Same SHA-256 ASICs as today; under a hard-fork BBPoW design they embed a Dogecoin commitment while mining Bitcoin. No Scrypt ASIC required for that lane.",
			"scrypt_auxpow":         "Litecoin/Dogecoin merge miners still need Scrypt ASICs for classic AuxPoW.",
			"one_asic_both_algos":   false,
			"need_two_asic_types":   "Only if the same operator wants to mine both lanes themselves.",
		},
		"reuse_auxpow_wire": map[string]interface{}{
			"possible":              true,
			"meaning":               "Parent header + coinbase + merkle branches can stay CAuxPow-shaped with Bitcoin as parent.",
			"avoids_new_field":      true,
			"avoids_hard_fork":      false,
			"why_still_hard_fork":   "L1 CheckAuxPow requires Scrypt(parent) vs child nBits. Accepting SHA-256d(parent) instead/in-OR expands validity → hard fork.",
		},
		"soft_fork": map[string]interface{}{
			"or_bitcoin_instead_of_scrypt": false,
			"and_extra_commitment_only":    true,
			"and_useful_for_btc_only":      false,
			"why": "Soft forks tighten rules. OR-accepting Bitcoin work expands valid blocks; legacy nodes reject → hard fork. AND still requires Scrypt work.",
		},
		"valid_block_rule_sketch": "DogecoinBlockValid = ValidScryptAuxPoW() OR ValidBitcoinProof()",
		"bitcoin_unchanged":       true,
		"dogecoin_change":         "hard_fork",
		"existing_auxpow":         "Litecoin → Dogecoin Scrypt AuxPoW remains lane 1.",
		"extension_scope":         "This DogeGo extension verifies BBPoW off-chain on testnet only. It does not accept BBPoW blocks on L1.",
		"docs":                    []string{"extensions/catalog/bbpow/docs/PROTOCOL.md", "extensions/catalog/bbpow/docs/HARD_FORK.md", "extensions/catalog/bbpow/docs/USER_GUIDE.md"},
	}
}
