// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPairValidateRequiresBoth(t *testing.T) {
	if err := (Pair{CertFile: "a.pem"}).Validate(); err == nil {
		t.Fatal("expected error for key only")
	}
}

func TestListenPlain(t *testing.T) {
	ln, scheme, err := Listen("127.0.0.1:0", Pair{})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if scheme != "http" {
		t.Fatalf("scheme %q", scheme)
	}
}

func TestListenLocalhostBindsLoopback(t *testing.T) {
	ln, scheme, err := Listen("localhost:0", Pair{})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if scheme != "http" {
		t.Fatalf("scheme %q", scheme)
	}
	go func() {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err == nil {
			_ = c.Close()
		}
	}()
	c, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	_ = c.Close()
}

func TestListenTLS(t *testing.T) {
	cert, key := writeTestTLSPair(t)
	ln, scheme, err := Listen("127.0.0.1:0", Pair{CertFile: cert, KeyFile: key})
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	if scheme != "https" {
		t.Fatalf("scheme %q", scheme)
	}
}

func writeTestTLSPair(t *testing.T) (certPath, keyPath string) {
	t.Helper()
	dir := t.TempDir()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "DogeGo Test"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPath = filepath.Join(dir, "cert.pem")
	keyPath = filepath.Join(dir, "key.pem")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b}), 0o600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}
