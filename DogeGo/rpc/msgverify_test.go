// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"dogego/secp256k1"
	"dogego/secp256k1/ecdsa"

	"dogego/chain"
)

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return json.RawMessage(b)
}

func TestSignVerifyMessageRoundTrip(t *testing.T) {
	sk, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pk := sk.PubKey()
	h160 := pubkeyHash160(pk.SerializeCompressed())
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, h160[:])
	if addr == "" {
		t.Fatal("encode address")
	}
	wif, err := chain.EncodeWIF(sk.Serialize(), p.PrivKeyWIFVersion, true)
	if err != nil {
		t.Fatal(err)
	}
	msg := "hello doge"
	sigB64, code, errStr := execSignMessageWithPrivkey("test", []json.RawMessage{
		mustJSON(t, msg),
		mustJSON(t, wif),
	})
	if code != 0 {
		t.Fatalf("sign code=%d msg=%s", code, errStr)
	}
	ok, code, errStr := execVerifyMessage("test", []json.RawMessage{
		mustJSON(t, addr),
		mustJSON(t, sigB64),
		mustJSON(t, msg),
	})
	if code != 0 {
		t.Fatalf("verify code=%d msg=%s", code, errStr)
	}
	if !ok {
		t.Fatal("expected verify true")
	}
}

func TestVerifyMessageWrongSig(t *testing.T) {
	sk, err := secp256k1.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	pk := sk.PubKey()
	h160 := pubkeyHash160(pk.SerializeCompressed())
	addr := chain.Base58CheckEncode(p.PubkeyHashAddrID, h160[:])
	other, _ := secp256k1.NewPrivateKey()
	h := messageHashForSignVerify("foo")
	badSig := ecdsa.SignCompact(other, h[:], true)
	sigB64 := base64.StdEncoding.EncodeToString(badSig)
	ok, code, errStr := execVerifyMessage("test", []json.RawMessage{
		mustJSON(t, addr),
		mustJSON(t, sigB64),
		mustJSON(t, "foo"),
	})
	if code != 0 {
		t.Fatalf("verify code=%d msg=%s", code, errStr)
	}
	if ok {
		t.Fatal("expected verify false")
	}
}

func TestVerifyMessageP2SHRejected(t *testing.T) {
	p, err := chain.ParamsFor(chain.RebootTestnet)
	if err != nil {
		t.Fatal(err)
	}
	var h [20]byte
	h[0] = 0xab
	addr := chain.Base58CheckEncode(p.ScriptHashAddrID, h[:])
	if addr == "" {
		t.Fatal("p2sh encode")
	}
	_, code, msg := execVerifyMessage("test", []json.RawMessage{
		mustJSON(t, addr),
		mustJSON(t, "abcd"),
		mustJSON(t, "x"),
	})
	if code == 0 {
		t.Fatal("expected error for P2SH")
	}
	if msg == "" {
		t.Fatal("expected message")
	}
}
