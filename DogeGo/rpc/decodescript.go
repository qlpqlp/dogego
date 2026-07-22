// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"

	"dogego/chain"
	"dogego/consensus"

	"golang.org/x/crypto/ripemd160"
)

func scriptToASM(b []byte) string {
	return consensus.ScriptToASM(b)
}

// execDecodeScript implements decodescript (Core-shaped asm via consensus.ScriptToASM).
func execDecodeScript(chainName string, params []json.RawMessage) (map[string]interface{}, int, string) {
	if len(params) < 1 {
		return nil, -8, "decodescript: hex string required"
	}
	var hexStr string
	if err := json.Unmarshal(params[0], &hexStr); err != nil {
		return nil, -8, "decodescript: bad hex param"
	}
	hexStr = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(hexStr), "0x"))
	var script []byte
	if hexStr != "" {
		var err error
		script, err = hex.DecodeString(hexStr)
		if err != nil {
			return nil, -8, "decodescript: invalid hex"
		}
	}

	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return nil, -8, err.Error()
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, -8, err.Error()
	}

	out := scriptPubKeyDecode(script, p)
	if t, _ := out["type"].(string); t != "scripthash" {
		if len(script) > 0 {
			h := scriptHash160(script)
			addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
			if addr != "" {
				out["p2sh"] = addr
			}
		}
	}
	return out, 0, ""
}

func scriptHash160(script []byte) [20]byte {
	s := sha256.Sum256(script)
	r := ripemd160.New()
	_, _ = r.Write(s[:])
	var out [20]byte
	copy(out[:], r.Sum(nil))
	return out
}

func scriptPubKeyDecode(script []byte, p chain.Params) map[string]interface{} {
	hexEnc := hex.EncodeToString(script)
	asm := consensus.ScriptToASM(script)
	addrs := []interface{}{}
	typ := "nonstandard"
	reqSigs := 1

	if addr := chain.PayToPubKeyHashAddress(script, p.PubkeyHashAddrID); addr != "" {
		typ = "pubkeyhash"
		addrs = append(addrs, addr)
	} else if isDecodeP2PKScript(script) {
		typ = "pubkey"
		reqSigs = 1
	} else if len(script) == 23 && script[0] == 0xa9 && script[1] == 0x14 && script[22] == 0x87 {
		typ = "scripthash"
		var h [20]byte
		copy(h[:], script[2:22])
		addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
		if addr != "" {
			addrs = append(addrs, addr)
		}
	} else if len(script) >= 1 && script[0] == 0x6a {
		typ = "nulldata"
		reqSigs = 0
	} else if nReq, pubs, err := consensus.ParseMultisigRedeemScript(script); err == nil {
		typ = "multisig"
		reqSigs = nReq
		for _, pub := range pubs {
			h := pubkeyHash160(pub)
			if addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, h[:]); addr != "" {
				addrs = append(addrs, addr)
			}
		}
	}

	out := map[string]interface{}{
		"asm":       asm,
		"hex":       hexEnc,
		"type":      typ,
		"reqSigs":   reqSigs,
		"addresses": addrs,
	}
	if pq := consensus.PQCommitmentFields(script); pq != nil {
		out["type"] = "nulldata"
		out["reqSigs"] = 0
		for k, v := range pq {
			out[k] = v
		}
	}
	if meta := consensus.RedeemScriptMeta(script); meta != nil {
		for k, v := range meta {
			out[k] = v
		}
	}
	return out
}

func isDecodeP2PKScript(s []byte) bool {
	if len(s) < 3 || s[len(s)-1] != 0xac {
		return false
	}
	switch s[0] {
	case 0x21:
		return len(s) == 35
	case 0x41:
		return len(s) == 67
	default:
		return false
	}
}
