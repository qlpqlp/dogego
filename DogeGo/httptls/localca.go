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
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	localCACommonName   = "DogeGo Local CA"
	localLeafCommonName = "DogeGo Local HTTPS"
	localCATTL          = 10 * 365 * 24 * time.Hour
	localLeafTTL        = 825 * 24 * time.Hour
)

// LocalMaterial holds generated or loaded local TLS PEM paths under datadir/tls/.
type LocalMaterial struct {
	Dir         string
	CACertPath  string
	CAKeyPath   string
	CAGenerated bool // true when EnsureLocalMaterial created a new CA this run
}

// HostsForListenAddrs collects DNS/IP SAN entries for certificate generation.
func HostsForListenAddrs(addrs ...string) []string {
	seen := make(map[string]struct{})
	add := func(s string) {
		s = strings.TrimSpace(strings.Trim(s, "[]"))
		if s == "" || s == "0.0.0.0" || s == "::" || s == "*" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
	}
	add("localhost")
	add("127.0.0.1")
	add("::1")
	if hn, err := os.Hostname(); err == nil {
		add(hn)
	}
	for _, addr := range addrs {
		host, _, err := net.SplitHostPort(strings.TrimSpace(addr))
		if err != nil {
			host = strings.TrimSpace(addr)
		}
		add(host)
	}
	out := make([]string, 0, len(seen))
	for h := range seen {
		out = append(out, h)
	}
	return out
}

// EnsureLocalMaterial creates or loads the DogeGo local CA under baseDataDir/tls/.
func EnsureLocalMaterial(baseDataDir string) (*LocalMaterial, error) {
	baseDataDir = strings.TrimSpace(baseDataDir)
	if baseDataDir == "" {
		return nil, fmt.Errorf("empty datadir")
	}
	dir := filepath.Join(baseDataDir, "tls")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	mat := &LocalMaterial{
		Dir:        dir,
		CACertPath: filepath.Join(dir, "local-ca.crt"),
		CAKeyPath:  filepath.Join(dir, "local-ca.key"),
	}
	if fileExists(mat.CACertPath) && fileExists(mat.CAKeyPath) {
		return mat, nil
	}
	// Datadir reset: drop leaf certs signed by a previous CA so they are reissued.
	for _, name := range []string{"webui.crt", "webui.key", "rpc.crt", "rpc.key"} {
		_ = os.Remove(filepath.Join(dir, name))
	}
	if err := generateCA(mat.CACertPath, mat.CAKeyPath); err != nil {
		return nil, err
	}
	mat.CAGenerated = true
	return mat, nil
}

// EnsureLeafPair returns PEM paths for a server leaf signed by the local CA.
func EnsureLeafPair(mat *LocalMaterial, leafName string, hosts []string) (Pair, error) {
	if mat == nil {
		return Pair{}, fmt.Errorf("missing local TLS material")
	}
	leafName = strings.TrimSpace(leafName)
	if leafName == "" {
		return Pair{}, fmt.Errorf("empty leaf name")
	}
	certPath := filepath.Join(mat.Dir, leafName+".crt")
	keyPath := filepath.Join(mat.Dir, leafName+".key")
	if fileExists(certPath) && fileExists(keyPath) {
		if leafStillValid(certPath, hosts) {
			return Pair{CertFile: certPath, KeyFile: keyPath}, nil
		}
	}
	caCert, caKey, err := loadCA(mat.CACertPath, mat.CAKeyPath)
	if err != nil {
		return Pair{}, err
	}
	if err := generateLeaf(certPath, keyPath, caCert, caKey, hosts); err != nil {
		return Pair{}, err
	}
	return Pair{CertFile: certPath, KeyFile: keyPath}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func leafStillValid(certPath string, hosts []string) bool {
	b, err := os.ReadFile(certPath)
	if err != nil {
		return false
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}
	if time.Now().After(cert.NotAfter.Add(-72 * time.Hour)) {
		return false
	}
	want := hostSet(hosts)
	for _, name := range cert.DNSNames {
		if want[name] {
			delete(want, name)
		}
	}
	for _, ip := range cert.IPAddresses {
		if want[ip.String()] {
			delete(want, ip.String())
		}
	}
	return len(want) == 0 || len(want) < len(hostSet(hosts))/2
}

func hostSet(hosts []string) map[string]bool {
	m := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		m[h] = true
	}
	return m
}

func generateCA(certPath, keyPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   localCACommonName,
			Organization: []string{"DogeGo"},
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(localCATTL),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
}

// LoadCertificateFile reads the first PEM certificate from path.
func LoadCertificateFile(path string) (*x509.Certificate, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, fmt.Errorf("bad certificate PEM in %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

// CertificateSHA1Hex returns the lowercase hex SHA-1 fingerprint (certutil-style).
func CertificateSHA1Hex(cert *x509.Certificate) string {
	if cert == nil {
		return ""
	}
	sum := sha1.Sum(cert.Raw)
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

// CACertSHA1Hex returns the SHA-1 fingerprint of a CA certificate file.
func CACertSHA1Hex(path string) (string, error) {
	cert, err := LoadCertificateFile(path)
	if err != nil {
		return "", err
	}
	return CertificateSHA1Hex(cert), nil
}

func loadCA(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	cb, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	block, _ := pem.Decode(cb)
	if block == nil {
		return nil, nil, fmt.Errorf("bad CA cert PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, err
	}
	kb, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	kblock, _ := pem.Decode(kb)
	if kblock == nil {
		return nil, nil, fmt.Errorf("bad CA key PEM")
	}
	key, err := x509.ParseECPrivateKey(kblock.Bytes)
	if err != nil {
		return nil, nil, err
	}
	return cert, key, nil
}

func generateLeaf(certPath, keyPath string, caCert *x509.Certificate, caKey *ecdsa.PrivateKey, hosts []string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	dnsNames := make([]string, 0)
	var ips []net.IP
	for _, h := range hosts {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if ip := net.ParseIP(h); ip != nil {
			ips = append(ips, ip)
			continue
		}
		dnsNames = append(dnsNames, h)
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   localLeafCommonName,
			Organization: []string{"DogeGo"},
		},
		NotBefore:   now.Add(-time.Hour),
		NotAfter:    now.Add(localLeafTTL),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames,
		IPAddresses: ips,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644); err != nil {
		return err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	return os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600)
}
