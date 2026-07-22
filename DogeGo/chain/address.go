// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"strings"

	"golang.org/x/crypto/ripemd160"
)

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

// Base58CheckEncode encodes version byte + payload + 4-byte double-SHA256 checksum (Bitcoin/Dogecoin style).
func Base58CheckEncode(version byte, payload []byte) string {
	if len(payload) != 20 {
		return ""
	}
	buf := make([]byte, 0, 1+len(payload)+4)
	buf = append(buf, version)
	buf = append(buf, payload...)
	h := sha256.Sum256(buf)
	h2 := sha256.Sum256(h[:])
	buf = append(buf, h2[0:4]...)
	return base58Encode(buf)
}

func base58Encode(input []byte) string {
	leadingOnes := 0
	for leadingOnes < len(input) && input[leadingOnes] == 0 {
		leadingOnes++
	}
	n := new(big.Int).SetBytes(input[leadingOnes:])
	if n.Sign() == 0 {
		return strings.Repeat("1", leadingOnes)
	}
	radix := big.NewInt(58)
	zero := big.NewInt(0)
	rem := new(big.Int)
	var rev []byte
	for n.Cmp(zero) > 0 {
		n.DivMod(n, radix, rem)
		rev = append(rev, base58Alphabet[rem.Int64()])
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return strings.Repeat("1", leadingOnes) + string(rev)
}

func base58Decode(s string) ([]byte, error) {
	zeroCount := 0
	for zeroCount < len(s) && s[zeroCount] == '1' {
		zeroCount++
	}
	s = s[zeroCount:]
	if len(s) == 0 {
		return bytes.Repeat([]byte{0}, zeroCount), nil
	}
	val := big.NewInt(0)
	radix := big.NewInt(58)
	for i := 0; i < len(s); i++ {
		idx := strings.IndexByte(base58Alphabet, s[i])
		if idx < 0 {
			return nil, fmt.Errorf("invalid base58 character")
		}
		val.Mul(val, radix)
		val.Add(val, big.NewInt(int64(idx)))
	}
	buf := val.Bytes()
	out := make([]byte, zeroCount+len(buf))
	copy(out[zeroCount:], buf)
	return out, nil
}

// Base58CheckDecode decodes a Base58Check string (version + 20-byte payload + 4-byte checksum).
func Base58CheckDecode(s string) (version byte, hash160 [20]byte, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, hash160, fmt.Errorf("empty address")
	}
	raw, err := base58Decode(s)
	if err != nil {
		return 0, hash160, err
	}
	if len(raw) < 5 {
		return 0, hash160, fmt.Errorf("too short")
	}
	payload := raw[:len(raw)-4]
	cs := raw[len(raw)-4:]
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	if !bytes.Equal(cs, h2[:4]) {
		return 0, hash160, fmt.Errorf("checksum mismatch")
	}
	if len(payload) != 21 {
		return 0, hash160, fmt.Errorf("unexpected payload length %d", len(payload))
	}
	version = payload[0]
	copy(hash160[:], payload[1:21])
	return version, hash160, nil
}

// DecodeBase58CheckBytes returns the full payload (without the 4-byte checksum) from a Base58Check string.
func DecodeBase58CheckBytes(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, fmt.Errorf("empty string")
	}
	raw, err := base58Decode(s)
	if err != nil {
		return nil, err
	}
	if len(raw) < 5 {
		return nil, fmt.Errorf("too short")
	}
	payload := raw[:len(raw)-4]
	cs := raw[len(raw)-4:]
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	if !bytes.Equal(cs, h2[:4]) {
		return nil, fmt.Errorf("checksum mismatch")
	}
	return payload, nil
}

// RandomP2PKHAddress returns a random P2PKH address string for params (visual / testing only).
func RandomP2PKHAddress(p Params) (string, error) {
	var h [20]byte
	if _, err := rand.Read(h[:]); err != nil {
		return "", err
	}
	s := Base58CheckEncode(p.PubkeyHashAddrID, h[:])
	if s == "" {
		return "", fmt.Errorf("encode failed")
	}
	return s, nil
}

// PayToPubKeyHashAddress returns the base58-check P2PKH address for a standard
// OP_DUP OP_HASH160 <20> OP_EQUALVERIFY OP_CHECKSIG script, or "" if the script is not P2PKH.
func PayToPubKeyHashAddress(pkScript []byte, pubkeyHashAddrVersion byte) string {
	if len(pkScript) == 25 && pkScript[0] == 0x76 && pkScript[1] == 0xa9 && pkScript[2] == 0x14 &&
		pkScript[23] == 0x88 && pkScript[24] == 0xac {
		return Base58CheckEncode(pubkeyHashAddrVersion, pkScript[3:23])
	}
	return ""
}

// PayToScriptHashAddress returns the base58-check P2SH address for OP_HASH160 <20> OP_EQUAL, or "".
func PayToScriptHashAddress(pkScript []byte, scriptHashAddrVersion byte) string {
	if len(pkScript) == 23 && pkScript[0] == 0xa9 && pkScript[1] == 0x14 && pkScript[22] == 0x87 {
		return Base58CheckEncode(scriptHashAddrVersion, pkScript[2:22])
	}
	return ""
}

// ScriptPubKeyAddress returns the display address for a P2PKH or P2SH scriptPubKey.
func ScriptPubKeyAddress(pkScript []byte, pubkeyHashAddrVersion, scriptHashAddrVersion byte) string {
	if a := PayToPubKeyHashAddress(pkScript, pubkeyHashAddrVersion); a != "" {
		return a
	}
	return PayToScriptHashAddress(pkScript, scriptHashAddrVersion)
}

// P2PKHScriptFromPubKeyHash builds standard pay-to-pubkey-hash scriptPubKey.
func P2PKHScriptFromPubKeyHash(h160 [20]byte) []byte {
	pk := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	return append(pk, 0x88, 0xac)
}

// P2SHScriptFromScriptHash builds standard P2SH scriptPubKey (redeem hash only).
func P2SHScriptFromScriptHash(h160 [20]byte) []byte {
	pk := append([]byte{0xa9, 0x14}, h160[:]...)
	return append(pk, 0x87)
}

// Hash160 computes RIPEMD160(SHA256(b)).
func Hash160(b []byte) []byte {
	h := sha256.Sum256(b)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	return r.Sum(nil)
}
