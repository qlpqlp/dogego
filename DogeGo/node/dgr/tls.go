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
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"sync"
	"time"
)

var (
	tlsOnce sync.Once
	srvTLS  *tls.Config
	cliTLS  *tls.Config
	tlsErr  error
)

func ensureTLS() error {
	tlsOnce.Do(func() {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			tlsErr = err
			return
		}
		serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		if err != nil {
			tlsErr = err
			return
		}
		tmpl := x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: "dogego-relay-cgnat"},
			NotBefore:    time.Now().Add(-time.Hour),
			NotAfter:     time.Now().Add(365 * 24 * time.Hour),
			KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}
		der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
		if err != nil {
			tlsErr = err
			return
		}
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		keyDER, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			tlsErr = err
			return
		}
		keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			tlsErr = err
			return
		}
		srvTLS = &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"dogego-relay-cgnat"},
			MinVersion:   tls.VersionTLS13,
		}
		cliTLS = &tls.Config{
			InsecureSkipVerify: true, // optional relay_tls_pins enforce trust when set
			NextProtos:         []string{"dogego-relay-cgnat"},
			MinVersion:         tls.VersionTLS13,
		}
	})
	return tlsErr
}

func serverTLSConfig() (*tls.Config, error) {
	if err := ensureTLS(); err != nil {
		return nil, err
	}
	return srvTLS.Clone(), nil
}

func clientTLSConfig() (*tls.Config, error) {
	if err := ensureTLS(); err != nil {
		return nil, err
	}
	return cliTLS.Clone(), nil
}
