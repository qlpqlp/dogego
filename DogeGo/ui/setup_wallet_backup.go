// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dogego/chain"
	"dogego/wallet"
)

type setupWalletBackupRequest struct {
	DataDir  string `json:"datadir"`
	Network  string `json:"network"`
	NoWallet bool   `json:"nowallet"`
}

func ensureSetupWallet(dataDir, network string) (walletPath string, err error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return "", fmt.Errorf("datadir required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return "", err
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return "", err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", err
	}
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return "", err
	}
	chainRoot := filepath.Join(dataDir, sub)
	if err := os.MkdirAll(chainRoot, 0o700); err != nil {
		return "", err
	}
	wpath := filepath.Join(chainRoot, "wallet.json")
	if _, err := wallet.LoadOrCreate(wpath, p.PubkeyHashAddrID); err != nil {
		return "", err
	}
	return wpath, nil
}

func setupWalletExists(dataDir, network string) (bool, error) {
	dataDir = strings.TrimSpace(dataDir)
	if dataDir == "" {
		return false, fmt.Errorf("datadir required")
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(network))
	if err != nil {
		return false, err
	}
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return false, err
	}
	wpath := filepath.Join(dataDir, sub, "wallet.json")
	st, err := os.Stat(wpath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return st.Mode().IsRegular(), nil
}

func registerSetupWalletBackup(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/wallet-backup", func(w http.ResponseWriter, r *http.Request) {
		if !isLoopback(r) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req setupWalletBackupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if req.NoWallet {
			http.Error(w, "wallet disabled", http.StatusBadRequest)
			return
		}
		wpath, err := ensureSetupWallet(req.DataDir, req.Network)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, err := os.Open(wpath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()
		netSlug := strings.TrimSpace(req.Network)
		if netSlug == "" {
			netSlug = "testnet"
		}
		name := "dogego-wallet-" + netSlug + "-" + time.Now().UTC().Format("20060102") + ".json"
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		_, _ = io.Copy(w, f)
	})
}
