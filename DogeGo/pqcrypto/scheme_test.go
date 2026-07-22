// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package pqcrypto

import "testing"

func TestFalconRoundTrip(t *testing.T) {
	s := Falcon512{}
	seed := DeriveSeed([]byte("test"), "falcon")
	pk, sk, err := s.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	msg := DeriveSeed([]byte("msg"), "sighash")
	sig, err := s.Sign(sk, msg[:])
	if err != nil {
		t.Fatal(err)
	}
	if !s.Verify(pk, msg[:], sig) {
		t.Fatal("falcon verify failed")
	}
	c := s.Commit(pk, sig)
	if c != Commit(pk, sig) {
		t.Fatal("commit mismatch")
	}
}

func TestDilithiumRoundTrip(t *testing.T) {
	s := Dilithium2{}
	seed := DeriveSeed([]byte("dil"), "dilithium")
	pk, sk, err := s.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	msg := DeriveSeed([]byte("m2"), "sighash")
	sig, err := s.Sign(sk, msg[:])
	if err != nil {
		t.Fatal(err)
	}
	if !s.Verify(pk, msg[:], sig) {
		t.Fatal("dilithium verify failed")
	}
}

func TestRaccoonRoundTrip(t *testing.T) {
	s := RaccoonG44{}
	seed := DeriveSeed([]byte("rcg"), "raccoon")
	pk, sk, err := s.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(pk) != raccoonPKLen {
		t.Fatalf("pk len=%d", len(pk))
	}
	msg := DeriveSeed([]byte("m3"), "sighash")
	sig, err := s.Sign(sk, msg[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != raccoonSigLen {
		t.Fatalf("sig len=%d", len(sig))
	}
	if !s.Verify(pk, msg[:], sig) {
		t.Fatal("raccoon verify failed")
	}
}
