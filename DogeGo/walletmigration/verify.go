// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"dogego/chain"
	"dogego/wallet/corewallet"
)

// OfflineSuite is one go test gate for wallet.dat migration certification.
type OfflineSuite struct {
	Name string
	Args []string
}

var (
	testOut io.Writer = os.Stdout
	testErr io.Writer = os.Stderr
)

// SetOutput redirects go test stdout/stderr (for CLI/tests).
func SetOutput(stdout, stderr io.Writer) {
	if stdout != nil {
		testOut = stdout
	}
	if stderr != nil {
		testErr = stderr
	}
}

// DefaultOfflineSuites returns cross-platform offline tests (delegated from scripts/wallet_migration_cert.ps1 via cert wallet-migration -offline-only).
func DefaultOfflineSuites() []OfflineSuite {
	return []OfflineSuite{
		{Name: "bdb + corewallet", Args: []string{"test", "./wallet/bdb", "./wallet/corewallet", "-count=1"}},
		{Name: "walletmigration probe", Args: []string{"test", "./walletmigration", "-count=1"}},
		{Name: "rpc import/probe", Args: []string{"test", "./rpc", "-run", "TestExecDogegoImportWalletDat|TestExecProbeWalletDat|ImportWalletDat|ProbeWalletDat|ImportWalletMulti|ImportWalletCallsKeypool|ImportWalletDatCallsKeypool|SyntheticFixture|Berkeley|MultiPool|NativePool|MixedPool", "-count=1"}},
		{Name: "ui wallet.dat API", Args: []string{"test", "./ui", "-run", "TestWalletImportWalletDatAPI|TestWalletImportWalletDatPassphraseAPI|TestWalletProbeWalletDatAPI", "-count=1"}},
	}
}

// RunRegex returns the -run filter for a suite, or "" if the suite has none.
func RunRegex(s OfflineSuite) string {
	for i := 0; i < len(s.Args); i++ {
		if s.Args[i] == "-run" && i+1 < len(s.Args) {
			return s.Args[i+1]
		}
	}
	return ""
}

// SuiteCommandLine returns the go test invocation string used in wallet_migration_cert.ps1.
func SuiteCommandLine(s OfflineSuite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that wallet_migration_cert.ps1 may use
// (PowerShell double-quotes the -run regex filter).
func SuiteScriptNeedles(s OfflineSuite) []string {
	tail := s.Args[1:]
	needles := []string{SuiteCommandLine(s)}
	for i := 0; i < len(tail); i++ {
		if tail[i] != "-run" || i+1 >= len(tail) {
			continue
		}
		quoted := append([]string(nil), tail...)
		quoted[i+1] = `""` + tail[i+1] + `""`
		needles = append(needles, "go test "+strings.Join(quoted, " "))
	}
	return needles
}

// RunOffline executes offline certification suites from module root.
func RunOffline(root string) error {
	for _, s := range DefaultOfflineSuites() {
		cmd := exec.Command("go", s.Args...)
		cmd.Dir = root
		cmd.Stdout = testOut
		cmd.Stderr = testErr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("%s: %w", s.Name, err)
		}
	}
	return nil
}

// LiveProbeResult summarizes an on-disk wallet.dat probe (no RPC required).
type LiveProbeResult struct {
	Path          string                    `json:"path"`
	Network       string                    `json:"network"`
	Probe         *corewallet.ProbeResult   `json:"probe,omitempty"`
	ExtractedKeys int                       `json:"extracted_keys,omitempty"`
	ExtractOK     bool                      `json:"extract_ok,omitempty"`
	ExtractError  string                    `json:"extract_error,omitempty"`
}

// ProbeFile inspects wallet.dat on disk; when passphrase is non-empty, attempts native decrypt (dry-run extract).
func ProbeFile(path, passphrase, network string) (*LiveProbeResult, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("wallet path required")
	}
	net, err := chain.ParseNetwork(network)
	if err != nil {
		return nil, err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return nil, err
	}
	out := &LiveProbeResult{Path: path, Network: network}
	probe, err := corewallet.ProbeWalletDat(path, p.PrivKeyWIFVersion)
	if err != nil {
		return out, err
	}
	out.Probe = probe
	if passphrase != "" {
		ex, err := corewallet.ExtractDumpLinesWithPassphrase(path, p.PrivKeyWIFVersion, passphrase)
		if err != nil {
			out.ExtractError = err.Error()
			return out, nil
		}
		out.ExtractOK = true
		out.ExtractedKeys = ex.KeyCount
	} else if probe != nil && probe.CanImport && !probe.NeedsPassphrase {
		ex, err := corewallet.ExtractDumpLines(path, p.PrivKeyWIFVersion)
		if err != nil {
			out.ExtractError = err.Error()
			return out, nil
		}
		out.ExtractOK = true
		out.ExtractedKeys = ex.KeyCount
	}
	return out, nil
}
