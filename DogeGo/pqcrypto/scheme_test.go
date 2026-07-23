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
	if !s.Available() {
		t.Skip("Raccoon-G-44 requires CGO_ENABLED=1 -tags raccoon_g (libgmp+libmpfr)")
	}
	seed := DeriveSeed([]byte("rcg"), "raccoon")
	pk, sk, err := s.GenerateKey(seed[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(pk) != raccoonPKLen {
		t.Fatalf("pk len=%d want %d", len(pk), raccoonPKLen)
	}
	if len(sk) != raccoonSKLen {
		t.Fatalf("sk len=%d want %d (libdogecoin wire)", len(sk), raccoonSKLen)
	}
	msg := DeriveSeed([]byte("m3"), "sighash")
	sig, err := s.Sign(sk, msg[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) != raccoonSigLen {
		t.Fatalf("sig len=%d want %d", len(sig), raccoonSigLen)
	}
	if !s.Verify(pk, msg[:], sig) {
		t.Fatal("raccoon verify failed")
	}
	if !s.LibdogecoinCompatible() {
		t.Fatal("expected libdogecoin-compatible raccoon backend")
	}
	if s.UpstreamRef() == "" {
		t.Fatal("expected upstream ref")
	}
}

func TestRaccoonUnavailableWithoutCGO(t *testing.T) {
	s := RaccoonG44{}
	if s.Available() {
		t.Skip("raccoon backend linked in this build")
	}
	if s.Backend() != "unavailable" {
		t.Fatalf("backend=%q want unavailable", s.Backend())
	}
	seed := DeriveSeed([]byte("x"), "raccoon")
	_, _, err := s.GenerateKey(seed[:])
	if err == nil {
		t.Fatal("expected GenerateKey error without raccoon_g CGO")
	}
}
