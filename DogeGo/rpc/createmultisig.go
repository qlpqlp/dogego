// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"dogego/secp256k1"

	"dogego/chain"
)

const maxMultisigRedeemScriptBytes = 520 // MAX_SCRIPT_ELEMENT_SIZE (src/script/script.h)

func multisigEncodeOpN(n int) (byte, error) {
	if n < 1 || n > 16 {
		return 0, fmt.Errorf("multisig OP_N out of range: %d", n)
	}
	return byte(0x50 + n), nil
}

func buildMultisigRedeemScript(nRequired int, pubkeys [][]byte) ([]byte, error) {
	if nRequired < 1 {
		return nil, fmt.Errorf("a multisignature address must require at least one key to redeem")
	}
	if len(pubkeys) < nRequired {
		return nil, fmt.Errorf("not enough keys supplied (got %d keys, but need at least %d to redeem)", len(pubkeys), nRequired)
	}
	if len(pubkeys) > 16 {
		return nil, fmt.Errorf("number of addresses involved in the multisignature address creation > 16")
	}
	seen := make(map[string]struct{}, len(pubkeys))
	for _, pk := range pubkeys {
		k := string(pk)
		if _, dup := seen[k]; dup {
			return nil, fmt.Errorf("duplicate key")
		}
		seen[k] = struct{}{}
	}
	opM, err := multisigEncodeOpN(nRequired)
	if err != nil {
		return nil, err
	}
	var out []byte
	out = append(out, opM)
	for _, pk := range pubkeys {
		if len(pk) != 33 && len(pk) != 65 {
			return nil, fmt.Errorf("invalid public key")
		}
		out = append(out, byte(len(pk)))
		out = append(out, pk...)
	}
	opN, err := multisigEncodeOpN(len(pubkeys))
	if err != nil {
		return nil, err
	}
	out = append(out, opN, 0xae) // OP_CHECKMULTISIG
	if len(out) > maxMultisigRedeemScriptBytes {
		return nil, fmt.Errorf("redeemScript exceeds size limit: %d > %d", len(out), maxMultisigRedeemScriptBytes)
	}
	return out, nil
}

// execCreateMultisig implements createmultisig for hex-encoded pubkeys (Core non-wallet path).
func execCreateMultisig(chainName string, params []json.RawMessage) (map[string]interface{}, int, string) {
	if len(params) < 2 {
		return nil, -8, "createmultisig: nrequired and keys array required"
	}
	var nReq float64
	if err := json.Unmarshal(params[0], &nReq); err != nil || nReq < 1 || nReq > 16 || nReq != float64(int(nReq)) {
		return nil, -8, "createmultisig: bad nrequired (integer 1..16)"
	}
	nRequired := int(nReq)
	var keys []json.RawMessage
	if err := json.Unmarshal(params[1], &keys); err != nil {
		return nil, -8, "createmultisig: bad keys array"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}
	pubser := make([][]byte, 0, len(keys))
	for i, kr := range keys {
		var ks string
		if err := json.Unmarshal(kr, &ks); err != nil {
			return nil, -8, fmt.Sprintf("createmultisig: bad key at index %d", i)
		}
		ks = strings.TrimSpace(ks)
		if ks == "" {
			return nil, -8, fmt.Sprintf("createmultisig: empty key at index %d", i)
		}
		if !isHexPubKeyString(ks) {
			return nil, -8, "Invalid public key: " + ks
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(ks), "0x"))
		if err != nil {
			return nil, -8, "Invalid public key: " + ks
		}
		pk, err := secp256k1.ParsePubKey(raw)
		if err != nil {
			return nil, -8, "Invalid public key: " + ks
		}
		var ser []byte
		switch len(raw) {
		case 33:
			ser = pk.SerializeCompressed()
		case 65:
			ser = pk.SerializeUncompressed()
		default:
			return nil, -8, "Invalid public key: " + ks
		}
		pubser = append(pubser, ser)
	}
	redeem, err := buildMultisigRedeemScript(nRequired, pubser)
	if err != nil {
		return nil, -8, "createmultisig: " + err.Error()
	}
	h := scriptHash160(redeem)
	addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
	if addr == "" {
		return nil, -8, "createmultisig: P2SH encode failed"
	}
	return map[string]interface{}{
		"address":      addr,
		"redeemScript": hex.EncodeToString(redeem),
	}, 0, ""
}

func isHexPubKeyString(s string) bool {
	s = strings.TrimPrefix(strings.ToLower(s), "0x")
	if len(s) != 130 && len(s) != 66 {
		return false
	}
	if len(s)%2 != 0 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9':
		case c >= 'a' && c <= 'f':
		default:
			return false
		}
	}
	return true
}
