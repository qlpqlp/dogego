// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"strings"
)

func certFingerprintDER(der []byte) string {
	sum := sha256.Sum256(der)
	return hex.EncodeToString(sum[:])
}

func certFingerprint(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	return certFingerprintDER(cert.Raw)
}

func normalizeTLSPins(pins []string) []string {
	if len(pins) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	var out []string
	for _, pin := range pins {
		p := strings.ToLower(strings.TrimSpace(pin))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func verifyPinnedCert(rawCerts [][]byte, pins []string) error {
	norm := normalizeTLSPins(pins)
	if len(norm) == 0 {
		return nil
	}
	if len(rawCerts) == 0 {
		return fmt.Errorf("dgr: no peer certificate")
	}
	fp := certFingerprintDER(rawCerts[0])
	for _, pin := range norm {
		if fp == pin {
			return nil
		}
	}
	return fmt.Errorf("dgr: TLS certificate pin mismatch")
}

func clientTLSConfigWithPins(pins []string) (*tls.Config, error) {
	if err := ensureTLS(); err != nil {
		return nil, err
	}
	cfg := cliTLS.Clone()
	norm := normalizeTLSPins(pins)
	if len(norm) == 0 {
		return cfg, nil
	}
	cfg.VerifyPeerCertificate = func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		return verifyPinnedCert(rawCerts, norm)
	}
	return cfg, nil
}

// ServerCertFingerprint returns the SHA-256 hex fingerprint of the inbound relay TLS certificate.
func ServerCertFingerprint() (string, error) {
	if err := ensureTLS(); err != nil {
		return "", err
	}
	if srvTLS == nil || len(srvTLS.Certificates) == 0 {
		return "", fmt.Errorf("dgr: server TLS not initialized")
	}
	cert, err := x509.ParseCertificate(srvTLS.Certificates[0].Certificate[0])
	if err != nil {
		return "", err
	}
	return certFingerprint(cert), nil
}
