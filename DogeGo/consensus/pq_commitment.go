// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// PQ scheme tags (libdogecoin / draft BIP post-quantum signature commitments, Phase 1).
const (
	PQTagFalcon    = "FLC1"
	PQTagDilithium = "DIL2"
	PQTagRaccoon   = "RCG4"
)

const pqCanonicalPayloadLen = 36 // 4-byte ASCII tag + 32-byte commitment

// PQCommitment describes a canonical Phase-1 OP_RETURN PQ commitment output.
type PQCommitment struct {
	Scheme     string `json:"scheme"`
	Tag        string `json:"tag"`
	Commitment string `json:"commitment"` // 64 hex chars
}

// DetectPQCommitment reports whether scriptPubKey is canonical OP_RETURN PQ commitment form:
// OP_RETURN OP_PUSH36 <4-byte tag><32-byte commitment> (FLC1 / DIL2 / RCG4).
func DetectPQCommitment(script []byte) (PQCommitment, bool) {
	if len(script) != 38 || script[0] != 0x6a || script[1] != 0x24 {
		return PQCommitment{}, false
	}
	tag := string(script[2:6])
	var scheme string
	switch tag {
	case PQTagFalcon:
		scheme = "falcon-512"
	case PQTagDilithium:
		scheme = "dilithium2"
	case PQTagRaccoon:
		scheme = "raccoon-g-44"
	default:
		return PQCommitment{}, false
	}
	return PQCommitment{
		Scheme:     scheme,
		Tag:        tag,
		Commitment: hex.EncodeToString(script[6:38]),
	}, true
}

// PQCommitmentFields returns RPC/explorer-shaped extra fields for a scriptPubKey map, or nil.
func PQCommitmentFields(script []byte) map[string]interface{} {
	c, ok := DetectPQCommitment(script)
	if !ok {
		return nil
	}
	return map[string]interface{}{
		"dogego_pqc_scheme":     c.Scheme,
		"dogego_pqc_tag":        c.Tag,
		"dogego_pqc_commitment": c.Commitment,
		"dogego_pqc_note":       "Phase-1 OP_RETURN commitment (recognition only; not consensus-validated PQ proof)",
	}
}

// ValidatePQCommitmentHex checks a claimed 32-byte commitment hex string.
func ValidatePQCommitmentHex(commitHex string) error {
	b, err := hex.DecodeString(commitHex)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("pq commitment: want 64 hex chars (32 bytes)")
	}
	return nil
}

// BuildPQCommitmentScript returns canonical OP_RETURN PQ commitment scriptPubKey (tag FLC1 / DIL2 / RCG4).
func BuildPQCommitmentScript(tag string, commitment32 []byte) ([]byte, error) {
	if len(commitment32) != 32 {
		return nil, fmt.Errorf("pq commitment: want 32 bytes")
	}
	var tagBytes []byte
	switch tag {
	case PQTagFalcon, PQTagDilithium, PQTagRaccoon:
		tagBytes = []byte(tag)
	default:
		return nil, fmt.Errorf("pq commitment: unknown tag %q", tag)
	}
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], tagBytes)
	copy(script[6:38], commitment32)
	return script, nil
}

// VerifyPQCommitmentScriptHex validates Phase-1 OP_RETURN PQ commitment form off-chain (format only; no PQ crypto).
func VerifyPQCommitmentScriptHex(scriptHex string) (map[string]interface{}, error) {
	scriptHex = strings.TrimSpace(scriptHex)
	if scriptHex == "" {
		return nil, fmt.Errorf("pq commitment: empty script")
	}
	b, err := hex.DecodeString(scriptHex)
	if err != nil {
		return nil, fmt.Errorf("pq commitment: invalid hex")
	}
	c, ok := DetectPQCommitment(b)
	if !ok {
		return nil, fmt.Errorf("pq commitment: not a canonical Phase-1 OP_RETURN commitment (FLC1/DIL2/RCG4)")
	}
	out := map[string]interface{}{
		"valid":       true,
		"scheme":      c.Scheme,
		"tag":         c.Tag,
		"commitment":  c.Commitment,
		"script_hex":  strings.ToLower(scriptHex),
		"verify_note": "Off-chain format check only - not a post-quantum signature proof",
	}
	return out, nil
}
