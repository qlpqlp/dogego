// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import "fmt"

const nodeTipLabel = "node tip"

// NodeTipEnabled reports whether the dedicated node-tip HD key is tracked for spends.
func (w *Disk) NodeTipEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.hdNodeTipEnabled
}

// NodeTipAddress returns the dedicated node-tip P2PKH address when enabled.
func (w *Disk) NodeTipAddress() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() || !w.hdNodeTipEnabled {
		return ""
	}
	d, err := w.deriveNodeTip(0)
	if err != nil {
		return ""
	}
	return d.Addr
}

// PreviewNodeTipAddress derives the node-tip address without enabling tracking.
func (w *Disk) PreviewNodeTipAddress() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return "", fmt.Errorf("hd wallet required")
	}
	d, err := w.deriveNodeTip(0)
	if err != nil {
		return "", err
	}
	return d.Addr, nil
}

// EnableNodeTip activates the dedicated node-tip key and labels it for the wallet UI.
func (w *Disk) EnableNodeTip() (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return "", fmt.Errorf("hd wallet required")
	}
	d, err := w.deriveNodeTip(0)
	if err != nil {
		return "", err
	}
	w.hdNodeTipEnabled = true
	if w.labels == nil {
		w.labels = make(map[string]string)
	}
	w.labels[d.Addr] = nodeTipLabel
	return d.Addr, w.saveLocked()
}

// PreviewNodeTipFromPath opens or creates wallet.json and returns the node-tip address.
func PreviewNodeTipFromPath(path string, addrVer byte) (string, error) {
	w, err := LoadOrCreate(path, addrVer)
	if err != nil {
		return "", err
	}
	return w.PreviewNodeTipAddress()
}

// EnableNodeTipFromPath enables node-tip tracking on an existing wallet file.
func EnableNodeTipFromPath(path string, addrVer byte) (string, error) {
	w, err := LoadOrCreate(path, addrVer)
	if err != nil {
		return "", err
	}
	return w.EnableNodeTip()
}
