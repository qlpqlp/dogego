// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"
)

func TestVerifyPinnedCert(t *testing.T) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		t.Fatal(err)
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "test-relay"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	fp := certFingerprintDER(der)
	if err := verifyPinnedCert([][]byte{der}, []string{fp}); err != nil {
		t.Fatal(err)
	}
	if err := verifyPinnedCert([][]byte{der}, []string{"deadbeef"}); err == nil {
		t.Fatal("expected pin mismatch")
	}
	if err := verifyPinnedCert(nil, []string{fp}); err == nil {
		t.Fatal("expected missing cert error")
	}
}

func TestServerCertFingerprint(t *testing.T) {
	fp, err := ServerCertFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	if len(fp) != 64 {
		t.Fatalf("fingerprint len %d", len(fp))
	}
}

func TestClientTLSConfigWithPins(t *testing.T) {
	cfg, err := clientTLSConfigWithPins(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VerifyPeerCertificate != nil {
		t.Fatal("expected no verify hook without pins")
	}
	fp, err := ServerCertFingerprint()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err = clientTLSConfigWithPins([]string{fp})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.VerifyPeerCertificate == nil {
		t.Fatal("expected verify hook with pins")
	}
}
