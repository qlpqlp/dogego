// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// TrustResult reports local CA installation outcome.
type TrustResult struct {
	OK      bool   `json:"ok"`
	Trusted bool   `json:"trusted"`
	Detail  string `json:"detail,omitempty"`
	Hint    string `json:"hint,omitempty"`
	OS      string `json:"os"`
}

// TrustStatus reports whether the local CA appears trusted.
type TrustStatus struct {
	Trusted bool
	Detail  string
}

// TrustLocalCA installs the DogeGo local CA into the user trust store when possible.
// Stale CAs from a previous datadir (same common name, different key) are replaced.
func TrustLocalCA(caCertPath string) TrustResult {
	return trustLocalCA(caCertPath, false)
}

// TrustLocalCAForce reinstalls the CA even when a same-name cert is already trusted.
func TrustLocalCAForce(caCertPath string) TrustResult {
	return trustLocalCA(caCertPath, true)
}

func trustLocalCA(caCertPath string, force bool) TrustResult {
	caCertPath = strings.TrimSpace(caCertPath)
	res := TrustResult{OS: runtime.GOOS}
	if caCertPath == "" {
		res.Detail = "local CA not generated yet"
		res.Hint = "Enable webui_tls_local or rpc_tls_local and restart, then try again"
		return res
	}
	if _, err := os.Stat(caCertPath); err != nil {
		res.Detail = "local CA file missing"
		return res
	}
	st := CATrustStatus(caCertPath)
	if st.Trusted && !force {
		res.OK = true
		res.Trusted = true
		res.Detail = st.Detail
		return res
	}
	if err := removeLocalCAFromTrustStore(); err != nil {
		res.Detail = "remove stale CA: " + err.Error()
		res.Hint = trustManualHint(runtime.GOOS, caCertPath)
		return res
	}
	var installErr error
	switch runtime.GOOS {
	case "windows":
		installErr = trustWindowsUser(caCertPath)
	case "darwin":
		installErr = trustDarwinUser(caCertPath)
	default:
		installErr = trustLinuxUser(caCertPath)
	}
	if installErr != nil {
		res.Detail = installErr.Error()
		res.Hint = trustManualHint(runtime.GOOS, caCertPath)
		return res
	}
	st = CATrustStatus(caCertPath)
	res.Trusted = st.Trusted
	res.OK = st.Trusted
	res.Detail = st.Detail
	if !res.OK {
		res.Hint = trustManualHint(runtime.GOOS, caCertPath)
	}
	return res
}

// CATrustStatus checks whether the on-disk local CA is trusted (fingerprint match).
func CATrustStatus(caCertPath string) TrustStatus {
	caCertPath = strings.TrimSpace(caCertPath)
	if caCertPath == "" {
		return TrustStatus{Detail: "local CA path empty"}
	}
	want, err := CACertSHA1Hex(caCertPath)
	if err != nil {
		return TrustStatus{Detail: "local CA file unreadable"}
	}
	switch runtime.GOOS {
	case "windows":
		return trustStatusWindows(want)
	case "darwin":
		return trustStatusDarwin(want)
	default:
		return trustStatusLinux(want)
	}
}

func removeLocalCAFromTrustStore() error {
	switch runtime.GOOS {
	case "windows":
		return untrustWindowsUser()
	case "darwin":
		return untrustDarwinUser()
	default:
		return untrustLinuxUser()
	}
}

func trustWindowsUser(caCertPath string) error {
	cmd := exec.Command("certutil", "-addstore", "-user", "Root", caCertPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func untrustWindowsUser() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("certutil", "-delstore", "-user", "Root", localCACommonName)
		out, err := cmd.CombinedOutput()
		text := strings.ToLower(string(out))
		if err != nil {
			if strings.Contains(text, "cannot find") || strings.Contains(text, "not found") {
				return nil
			}
			return fmt.Errorf("certutil delstore: %s", strings.TrimSpace(string(out)))
		}
	}
	return nil
}

func trustStatusWindows(wantSHA1 string) TrustStatus {
	cmd := exec.Command("certutil", "-store", "-user", "Root")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return TrustStatus{Detail: "could not query user Root store"}
	}
	store := string(out)
	if !strings.Contains(store, localCACommonName) {
		return TrustStatus{Detail: "DogeGo Local CA not found in Windows user Root store"}
	}
	if storeTrustsFingerprint(store, localCACommonName, wantSHA1) {
		return TrustStatus{Trusted: true, Detail: "DogeGo Local CA is in the Windows user Root store"}
	}
	return TrustStatus{Detail: "DogeGo Local CA in Windows store does not match datadir/tls/local-ca.crt (reinstalling on next start)"}
}

func trustDarwinUser(caCertPath string) error {
	home, _ := os.UserHomeDir()
	kc := filepath.Join(home, "Library", "Keychains", "login.keychain-db")
	cmd := exec.Command("security", "add-trusted-cert", "-d", "-r", "trustRoot", "-k", kc, caCertPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("security: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func untrustDarwinUser() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("security", "delete-certificate", "-c", localCACommonName)
		if err := cmd.Run(); err != nil {
			return nil
		}
	}
	return nil
}

func trustStatusDarwin(wantSHA1 string) TrustStatus {
	cmd := exec.Command("security", "find-certificate", "-c", localCACommonName, "-Z")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), localCACommonName) {
		return TrustStatus{Detail: "DogeGo Local CA not found in login keychain"}
	}
	got := parseSecurityFindCertSHA1(string(out))
	if got != "" && strings.EqualFold(got, wantSHA1) {
		return TrustStatus{Trusted: true, Detail: "DogeGo Local CA found in login keychain"}
	}
	if got != "" {
		return TrustStatus{Detail: "DogeGo Local CA in keychain does not match datadir/tls/local-ca.crt"}
	}
	return TrustStatus{Detail: "DogeGo Local CA found in login keychain but fingerprint could not be verified"}
}

func parseSecurityFindCertSHA1(out string) string {
	for _, line := range strings.Split(out, "\n") {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.HasPrefix(lower, "sha-1 hash:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(parts[1]), " ", ""))
			}
		}
	}
	return ""
}

func trustLinuxUser(caCertPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	nssDB := filepath.Join(home, ".pki", "nssdb")
	if err := os.MkdirAll(nssDB, 0o700); err != nil {
		return err
	}
	if _, err := exec.LookPath("certutil"); err != nil {
		return fmt.Errorf("certutil not found (install libnss3-tools)")
	}
	cmd := exec.Command("certutil", "-d", "sql:"+nssDB, "-A", "-t", "C,,", "-n", localCACommonName, "-i", caCertPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("certutil: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func untrustLinuxUser() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if _, err := exec.LookPath("certutil"); err != nil {
		return nil
	}
	nssDB := filepath.Join(home, ".pki", "nssdb")
	for i := 0; i < 5; i++ {
		cmd := exec.Command("certutil", "-d", "sql:"+nssDB, "-D", "-n", localCACommonName)
		if err := cmd.Run(); err != nil {
			return nil
		}
	}
	return nil
}

func trustStatusLinux(wantSHA1 string) TrustStatus {
	home, err := os.UserHomeDir()
	if err != nil {
		return TrustStatus{Detail: "could not resolve home directory"}
	}
	if _, err := exec.LookPath("certutil"); err != nil {
		return TrustStatus{Detail: "certutil not installed (libnss3-tools)"}
	}
	nssDB := filepath.Join(home, ".pki", "nssdb")
	cmd := exec.Command("certutil", "-d", "sql:"+nssDB, "-L")
	out, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(out), localCACommonName) {
		return TrustStatus{Detail: "DogeGo Local CA not found in NSS user database"}
	}
	dump := exec.Command("certutil", "-d", "sql:"+nssDB, "-L", "-n", localCACommonName, "-a")
	dout, derr := dump.CombinedOutput()
	if derr == nil {
		for _, line := range strings.Split(string(dout), "\n") {
			if strings.Contains(strings.ToLower(line), "sha1 fingerprint") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 {
					got := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(parts[1]), ":", ""))
					if got == wantSHA1 {
						return TrustStatus{Trusted: true, Detail: "DogeGo Local CA found in NSS user database (Chrome/Chromium)"}
					}
					return TrustStatus{Detail: "DogeGo Local CA in NSS does not match datadir/tls/local-ca.crt"}
				}
			}
		}
	}
	return TrustStatus{Detail: "DogeGo Local CA found in NSS user database but fingerprint could not be verified"}
}

func trustManualHint(goos, caPath string) string {
	switch goos {
	case "windows":
		return fmt.Sprintf("Run in PowerShell: certutil -delstore -user Root %q; certutil -addstore -user Root %q", localCACommonName, caPath)
	case "darwin":
		return fmt.Sprintf("Run: security delete-certificate -c %q; security add-trusted-cert -d -r trustRoot -k ~/Library/Keychains/login.keychain-db %q", localCACommonName, caPath)
	default:
		return fmt.Sprintf("Remove then re-add: certutil -d sql:$HOME/.pki/nssdb -D -n %q; certutil -d sql:$HOME/.pki/nssdb -A -t \"C,,\" -n %q -i %q", localCACommonName, localCACommonName, caPath)
	}
}
