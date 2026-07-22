// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"fmt"
)

// ZKAnchorTag is the DogeGo optional L2 anchor OP_RETURN tag (not consensus-enforced).
const ZKAnchorTag = "ZKDG"

const zkAnchorPayloadLen = 36 // 4-byte tag + 32-byte anchor hash

// ZKAnchor describes an optional DogeGo ZK L2 anchor in OP_RETURN form.
type ZKAnchor struct {
	Tag        string `json:"tag"`
	AnchorHash string `json:"anchor_hash"` // 64 hex chars (SHA256 digest)
}

// DetectZKAnchor reports canonical OP_RETURN ZKDG anchor: OP_RETURN OP_PUSH36 <ZKDG><32-byte hash>.
func DetectZKAnchor(script []byte) (ZKAnchor, bool) {
	if len(script) != 38 || script[0] != 0x6a || script[1] != 0x24 {
		return ZKAnchor{}, false
	}
	if string(script[2:6]) != ZKAnchorTag {
		return ZKAnchor{}, false
	}
	return ZKAnchor{
		Tag:        ZKAnchorTag,
		AnchorHash: hex.EncodeToString(script[6:38]),
	}, true
}

// ZKAnchorFields returns explorer/RPC extra fields for a scriptPubKey map, or nil.
func ZKAnchorFields(script []byte) map[string]interface{} {
	a, ok := DetectZKAnchor(script)
	if !ok {
		return nil
	}
	return map[string]interface{}{
		"dogego_zkl2_tag":         a.Tag,
		"dogego_zkl2_anchor_hash": a.AnchorHash,
		"dogego_zkl2_note":        "Optional DogeGo ZK L2 anchor (extension-only; not consensus-validated on L1)",
	}
}

// ValidateZKAnchorHashHex checks a claimed 32-byte anchor hash.
func ValidateZKAnchorHashHex(hashHex string) error {
	b, err := hex.DecodeString(hashHex)
	if err != nil || len(b) != 32 {
		return fmt.Errorf("zkl2 anchor hash: want 64 hex chars (32 bytes)")
	}
	return nil
}

// BuildZKAnchorScript returns canonical OP_RETURN ZKDG anchor scriptPubKey.
func BuildZKAnchorScript(anchorHash32 []byte) ([]byte, error) {
	if len(anchorHash32) != 32 {
		return nil, fmt.Errorf("zkl2 anchor hash: want 32 bytes")
	}
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(ZKAnchorTag))
	copy(script[6:38], anchorHash32)
	return script, nil
}

// VerifyZKAnchorScriptHex validates ZKDG OP_RETURN anchor form off-chain (format only).
func VerifyZKAnchorScriptHex(scriptHex string) (map[string]interface{}, error) {
	b, err := hex.DecodeString(scriptHex)
	if err != nil {
		return nil, fmt.Errorf("invalid script hex")
	}
	a, ok := DetectZKAnchor(b)
	if !ok {
		return nil, fmt.Errorf("not a canonical ZKDG anchor")
	}
	return map[string]interface{}{
		"valid":       true,
		"tag":         a.Tag,
		"anchor_hash": a.AnchorHash,
		"note":        "format check only; enable dogego.zkl2 extension for L2 verification",
	}, nil
}
