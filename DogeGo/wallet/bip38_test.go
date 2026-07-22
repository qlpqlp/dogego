// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"testing"

	"dogego/chain"
)

func TestBIP38NonECCompressed(t *testing.T) {
	enc := "6PYNKZ1EAgYgmQfmNVamxyXVWHzK5s6DGhwP4J5o44cvXdoY7sRzhtpUeo"
	secret, compressed, err := DecryptBIP38(enc, "TestingOneTwoThree", 0x00)
	if err != nil {
		t.Fatal(err)
	}
	if !compressed {
		t.Fatal("expected compressed")
	}
	want, _ := hex.DecodeString("cbf4b9f70470856bb4f40f80b87edb90865997ffee6df315ab166d713af433a5")
	if hex.EncodeToString(secret) != hex.EncodeToString(want) {
		t.Fatalf("got %x want %x", secret, want)
	}
}

func TestBIP38ECMultiply(t *testing.T) {
	enc := "6PfQu77ygVyJLZjfvMLyhLMQbYnu5uguoJJ4kMCLqWwPEdfpwANVS76gTX"
	secret, _, err := DecryptBIP38(enc, "TestingOneTwoThree", 0x00)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := hex.DecodeString("a43a940577f4e97f5c4d39eb14ff083a98187c64ea7c99ef7ce460833959a519")
	if hex.EncodeToString(secret) != hex.EncodeToString(want) {
		t.Fatalf("got %x want %x", secret, want)
	}
}

func TestBIP38ECMultiplySatoshi(t *testing.T) {
	enc := "6PfLGnQs6VZnrNpmVKfjotbnQuaJK4KZoPFrAjx1JMJUa1Ft8gnf5WxfKd"
	secret, compressed, err := DecryptBIP38(enc, "Satoshi", 0x00)
	if err != nil {
		t.Fatal(err)
	}
	if compressed {
		t.Fatal("expected uncompressed")
	}
	want, _, err := chain.DecodeWIF("5KJ51SgxWaAYR13zd9ReMhJpwrcX47xTJh2D3fGPG9CM8vkv5sH", 0x80)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(secret) != hex.EncodeToString(want) {
		t.Fatalf("got %x want %x", secret, want)
	}
}

func TestBIP38ECLotSequence(t *testing.T) {
	enc := "6PgNBNNzDkKdhkT6uJntUXwwzQV8Rr2tZcbkDcuC9DZRsS6AtHts4Ypo1j"
	secret, compressed, err := DecryptBIP38(enc, "MOLON LABE", 0x00)
	if err != nil {
		t.Fatal(err)
	}
	if compressed {
		t.Fatal("expected uncompressed")
	}
	want, _, err := chain.DecodeWIF("5JLdxTtcTHcfYcmJsNVy1v2PMDx432JPoYcBTVVRHpPaxUrdtf8", 0x80)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(secret) != hex.EncodeToString(want) {
		t.Fatalf("got %x want %x", secret, want)
	}
}

func TestBIP38ECLotSequenceUnicodePassphrase(t *testing.T) {
	enc := "6PgGWtx25kUg8QWvwuJAgorN6k9FbE25rv5dMRwu5SKMnfpfVe5mar2ngH"
	secret, _, err := DecryptBIP38(enc, "ΜΟΛΩΝ ΛΑΒΕ", 0x00)
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := chain.DecodeWIF("5KMKKuUmAkiNbA3DazMQiLfDq47qs8MAEThm4yL8R2PhV1ov33D", 0x80)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(secret) != hex.EncodeToString(want) {
		t.Fatalf("got %x want %x", secret, want)
	}
}
