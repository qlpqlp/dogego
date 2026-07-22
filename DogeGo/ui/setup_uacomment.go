// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"dogego/chain"
	"dogego/config"
	"dogego/wallet"
)

func registerSetupUAComment(mux *http.ServeMux) {
	mux.HandleFunc("/api/setup/node-tip-preview", func(w http.ResponseWriter, r *http.Request) {
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
		net, err := chain.ParseNetwork(strings.TrimSpace(req.Network))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		p, err := chain.ParamsFor(net)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		addr, err := wallet.PreviewNodeTipFromPath(wpath, p.PubkeyHashAddrID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"address": addr})
	})
}

func applySetupUACommentTip(f *config.File) error {
	if f == nil {
		return fmt.Errorf("nil config")
	}
	publish := f.UACommentPublishTip()
	if !publish {
		f.UACommentTipAddress = ""
		f.UACommentUseNodeTip = nil
		return nil
	}
	if f.UACommentUseNodeTipEnabled() {
		if f.NoWallet {
			return fmt.Errorf("node tip address requires wallet")
		}
		wpath, err := ensureSetupWallet(f.DataDir, f.Network)
		if err != nil {
			return err
		}
		net, err := chain.ParseNetwork(strings.TrimSpace(f.Network))
		if err != nil {
			return err
		}
		p, err := chain.ParamsFor(net)
		if err != nil {
			return err
		}
		addr, err := wallet.EnableNodeTipFromPath(wpath, p.PubkeyHashAddrID)
		if err != nil {
			return err
		}
		f.UACommentTipAddress = addr
		on := true
		f.UACommentUseNodeTip = &on
		return nil
	}
	f.UACommentUseNodeTip = nil
	if err := config.ValidateUACommentTip(f.UACommentTipAddress, f.Network); err != nil {
		return err
	}
	return nil
}

// resolveUACommentTipForConfig updates uacomment tip fields for Settings save without
// mutating wallet.json (encrypted wallets cannot derive HD keys until unlock).
func resolveUACommentTipForConfig(f *config.File, existing config.File) (warn string, err error) {
	if f == nil {
		return "", fmt.Errorf("nil config")
	}
	if !f.UACommentPublishTip() {
		f.UACommentTipAddress = ""
		f.UACommentUseNodeTip = nil
		return "", nil
	}
	if !f.UACommentUseNodeTipEnabled() {
		f.UACommentUseNodeTip = nil
		if err := config.ValidateUACommentTip(f.UACommentTipAddress, f.Network); err != nil {
			return "", err
		}
		return "", nil
	}
	if f.NoWallet {
		return "", fmt.Errorf("node tip address requires wallet")
	}
	wpath, err := walletPathForConfig(f)
	if err != nil {
		return "", err
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(f.Network))
	if err != nil {
		return "", err
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		return "", err
	}
	addr, prevErr := wallet.PreviewNodeTipFromPath(wpath, p.PubkeyHashAddrID)
	if prevErr == nil {
		f.UACommentTipAddress = addr
		on := true
		f.UACommentUseNodeTip = &on
		return "", nil
	}
	if strings.TrimSpace(existing.UACommentTipAddress) != "" {
		f.UACommentTipAddress = strings.TrimSpace(existing.UACommentTipAddress)
		on := true
		f.UACommentUseNodeTip = &on
		return "Using saved node-tip address (wallet locked or HD seed unavailable during save).", nil
	}
	if strings.Contains(strings.ToLower(prevErr.Error()), "hd wallet") {
		off := false
		f.UACommentUseNodeTip = &off
		f.UACommentTipAddress = ""
		return "Node tip requires an HD wallet; tip publishing was turned off for this save.", nil
	}
	return "", prevErr
}

func walletPathForConfig(f *config.File) (string, error) {
	dataDir := strings.TrimSpace(f.DataDir)
	if dataDir == "" {
		return "", fmt.Errorf("datadir required")
	}
	net, err := chain.ParseNetwork(strings.TrimSpace(f.Network))
	if err != nil {
		return "", err
	}
	sub, err := chain.ChainDataDirName(net)
	if err != nil {
		return "", err
	}
	wpath := filepath.Join(dataDir, sub, "wallet.json")
	if _, err := os.Stat(wpath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("wallet not found at %s", wpath)
		}
		return "", err
	}
	return wpath, nil
}
