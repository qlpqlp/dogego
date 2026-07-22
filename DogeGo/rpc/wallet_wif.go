// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"crypto/sha256"
	"strings"

	"dogego/secp256k1"
	"golang.org/x/crypto/ripemd160"

	"dogego/chain"
)

// addressFromWIF derives the P2PKH address for a WIF on the RPC chain.
func addressFromWIF(chainName, wif string) (string, error) {
	net, err := networkFromRPCChainName(chainName)
	if err != nil {
		return "", err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", err
	}
	secret, compressed, err := chain.DecodeWIF(strings.TrimSpace(wif), p.PrivKeyWIFVersion)
	if err != nil {
		return "", err
	}
	priv, _ := secp256k1.PrivKeyFromBytes(secret)
	pub := priv.PubKey().SerializeCompressed()
	if !compressed {
		pub = priv.PubKey().SerializeUncompressed()
	}
	h := sha256.Sum256(pub)
	r := ripemd160.New()
	_, _ = r.Write(h[:])
	return chain.Base58CheckEncode(p.PubkeyHashAddrID, r.Sum(nil)), nil
}
