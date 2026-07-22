// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"unicode"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
	"dogego/wire"
)

// dogecoinMessageMagic matches strMessageMagic in Dogecoin Core (src/validation.cpp).
const dogecoinMessageMagic = "Dogecoin Signed Message:\n"

func messageHashForSignVerify(msg string) [32]byte {
	var buf bytes.Buffer
	magic := []byte(dogecoinMessageMagic)
	_ = wire.WriteCompactSize(&buf, uint64(len(magic)))
	buf.Write(magic)
	mb := []byte(msg)
	_ = wire.WriteCompactSize(&buf, uint64(len(mb)))
	buf.Write(mb)
	s := sha256.Sum256(buf.Bytes())
	s2 := sha256.Sum256(s[:])
	return s2
}

func pubkeyHash160(pub []byte) [20]byte {
	sh := sha256.Sum256(pub)
	r := ripemd160.New()
	_, _ = r.Write(sh[:])
	var out [20]byte
	copy(out[:], r.Sum(nil))
	return out
}

func normalizeMessageSigBase64(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

// execVerifyMessage implements verifymessage (P2PKH only, Core-shaped message hash and compact ECDSA).
func execVerifyMessage(chainName string, params []json.RawMessage) (bool, int, string) {
	if len(params) < 3 {
		return false, -8, "verifymessage: address, signature, and message required"
	}
	var addr, sigB64, msg string
	if err := json.Unmarshal(params[0], &addr); err != nil {
		return false, -8, "verifymessage: bad address"
	}
	if err := json.Unmarshal(params[1], &sigB64); err != nil {
		return false, -8, "verifymessage: bad signature"
	}
	if err := json.Unmarshal(params[2], &msg); err != nil {
		return false, -8, "verifymessage: bad message"
	}
	addr = strings.TrimSpace(addr)
	sigB64 = normalizeMessageSigBase64(sigB64)

	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return false, -8, "verifymessage: unknown chain"
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return false, -8, "verifymessage: chain params"
	}

	v, wantH160, err := chain.Base58CheckDecode(addr)
	if err != nil {
		return false, -8, "verifymessage: invalid address"
	}
	if v == p.ScriptHashAddrID {
		return false, -8, "verifymessage: address does not refer to pubkey"
	}
	if v != p.PubkeyHashAddrID {
		return false, -8, "verifymessage: invalid address"
	}

	rawSig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(rawSig) != 65 {
		return false, -8, "verifymessage: malformed base64 encoding"
	}

	h := messageHashForSignVerify(msg)
	pub, _, err := ecdsa.RecoverCompact(rawSig, h[:])
	if err != nil {
		return false, 0, ""
	}

	// Match Core: HASH160(pub) must equal the P2PKH payload for the recovered key serialization.
	serC := pub.SerializeCompressed()
	serU := pub.SerializeUncompressed()
	if pubkeyHash160(serC) == wantH160 || pubkeyHash160(serU) == wantH160 {
		return true, 0, ""
	}
	return false, 0, ""
}

// execSignMessageWithPrivkey implements signmessagewithprivkey (WIF + Core message hash, compact ECDSA, base64).
func execSignMessageWithPrivkey(chainName string, params []json.RawMessage) (string, int, string) {
	if len(params) < 2 {
		return "", -8, "signmessagewithprivkey: message and private key required"
	}
	var msg, wif string
	if err := json.Unmarshal(params[0], &msg); err != nil {
		return "", -8, "signmessagewithprivkey: bad message"
	}
	if err := json.Unmarshal(params[1], &wif); err != nil {
		return "", -8, "signmessagewithprivkey: bad private key"
	}
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", -8, "signmessagewithprivkey: unknown chain"
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", -8, "signmessagewithprivkey: chain params"
	}
	keyBytes, compressed, err := chain.DecodeWIF(strings.TrimSpace(wif), p.PrivKeyWIFVersion)
	if err != nil {
		return "", -8, "signmessagewithprivkey: invalid private key"
	}
	sk, _ := secp256k1.PrivKeyFromBytes(keyBytes)
	h := messageHashForSignVerify(msg)
	sig := ecdsa.SignCompact(sk, h[:], compressed)
	return base64.StdEncoding.EncodeToString(sig), 0, ""
}
