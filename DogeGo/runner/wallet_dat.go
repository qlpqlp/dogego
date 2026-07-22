// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"dogego/walletmigration"
)

// ResolveWalletDatPath returns an existing Core wallet.dat path.
// explicit takes precedence; when explicit is non-empty but missing, returns "" (no fallback).
// When explicit is empty, common Core datadir locations are scanned.
func ResolveWalletDatPath(explicit string) string {
	if path, _ := ResolveWalletDatPathConfigured(explicit); path != "" {
		return path
	}
	if strings.TrimSpace(explicit) != "" {
		return ""
	}
	if strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT")) != "" {
		return ""
	}
	for _, p := range walletDatCandidates() {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// ResolveWalletDatPathConfigured returns a wallet.dat path and whether it was explicitly configured
// (-wallet-dat flag, DOGEGO_WALLET_DAT env). Auto-discovered paths return explicit=false.
func ResolveWalletDatPathConfigured(explicit string) (path string, configured bool) {
	if p := strings.TrimSpace(explicit); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		return "", true
	}
	if p := strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
		return "", true
	}
	return "", false
}

func walletDatCandidates() []string {
	var out []string
	if d := strings.TrimSpace(os.Getenv("DOGEGO_CORE_DATADIR")); d != "" {
		out = append(out, filepath.Join(d, "wallet.dat"))
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		out = append(out,
			filepath.Join(appData, "Dogecoin", "testnet3", "wallet.dat"),
			filepath.Join(appData, "Dogecoin", "wallet.dat"),
		)
	}
	if home := os.Getenv("HOME"); home != "" {
		out = append(out,
			filepath.Join(home, ".dogecoin", "testnet3", "wallet.dat"),
			filepath.Join(home, ".dogecoin", "wallet.dat"),
		)
	}
	return out
}

// WalletDatImportEnabled reports whether live wallet.dat import/probe should run:
// when required, or when wallet.dat was explicitly configured (flag/env) and exists.
func WalletDatImportEnabled(require bool) bool {
	if require {
		return true
	}
	path, configured := ResolveWalletDatPathConfigured("")
	return configured && path != ""
}

// WalletDatLiveImportNeeded reports whether weekly live should RPC-import after setup
// (mirrors ci_scheduled_weekly_live.ps1 wallet_migration_cert.ps1 -SkipOffline block).
func WalletDatLiveImportNeeded(require bool) bool {
	if require {
		return true
	}
	return strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT")) != ""
}

// ImportWalletDatLive RPC-imports Core wallet.dat on a running DogeGo node.
func ImportWalletDatLive(host string, port int, require bool) (*walletmigration.LiveImportResult, error) {
	if !WalletDatLiveImportNeeded(require) {
		return nil, nil
	}
	path, _ := ResolveWalletDatPathConfigured("")
	if path == "" {
		if require {
			return nil, fmt.Errorf("wallet_dat_required_missing")
		}
		return nil, nil
	}
	pass := os.Getenv("DOGEGO_WALLET_DAT_PASSPHRASE")
	client := walletmigration.RPCClientForHostPort(host, port)
	live, err := walletmigration.LiveImportViaRPC(client, path, pass)
	if err != nil {
		live = &walletmigration.LiveImportResult{Path: path, Status: "import_failed", Error: err.Error()}
	}
	if !walletmigration.LiveImportOK(live, require) {
		if live != nil && live.Error != "" {
			return live, fmt.Errorf("%s", live.Error)
		}
		return live, fmt.Errorf("wallet_dat_import_failed")
	}
	return live, nil
}
