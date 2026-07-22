// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletimport

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"dogego/walletmigration"
)

// OfflineSuite is one go test gate for wallet import certification.
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

// DefaultOfflineSuites returns cross-platform offline tests (delegated from scripts/wallet_import_cert.ps1 via cert wallet-import).
func DefaultOfflineSuites() []OfflineSuite {
	return []OfflineSuite{
		{Name: "wallet BIP39/BIP38", Args: []string{"test", "./wallet", "-run", "BIP39|BIP38|RestoreFromMnemonic|ImportBIP38", "-count=1"}},
		{Name: "bdb + corewallet", Args: []string{"test", "./wallet/bdb", "./wallet/corewallet", "-count=1"}},
		{Name: "rpc import/probe/signers", Args: []string{"test", "./rpc", "-run", "TestExecDogegoImport|TestExecDogegoListWalletAddresses|ImportWalletDat|ProbeWalletDat|ImportWalletMulti|ImportWalletCallsKeypool|ImportWalletDatCallsKeypool|EnumerateSigners|SignerDisplay|Berkeley|MockSigner|MixedPool|NativePool", "-count=1"}},
		{Name: "signer package", Args: []string{"test", "./signer", "-count=1"}},
		{Name: "ui wallet import", Args: []string{"test", "./ui", "-run", "TestWalletImport|TestWalletAddress|TestProbeCoreWallet|TestWalletProbeWalletDatAPI|TestWalletImportWalletDatAPI|TestWalletImportWalletDatPassphraseAPI", "-count=1"}},
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

// SuiteCommandLine returns the go test invocation string used in wallet_import_cert.ps1.
func SuiteCommandLine(s OfflineSuite) string {
	return "go test " + strings.Join(s.Args[1:], " ")
}

// SuiteScriptNeedles returns command strings that wallet_import_cert.ps1 may use
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

// RunOffline executes wallet import certification from module root (includes wallet-migration suites).
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
	walletmigration.SetOutput(testOut, testErr)
	if err := walletmigration.RunOffline(root); err != nil {
		return fmt.Errorf("wallet-migration: %w", err)
	}
	return nil
}
