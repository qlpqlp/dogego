// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type rotationPending struct {
	newSeed []byte
	newAddr string
}

// RotationStatus reports in-progress key rotation for the web UI.
type RotationStatus struct {
	Active    bool   `json:"active"`
	NewAddress string `json:"new_address,omitempty"`
}

// BeginKeyRotation reserves a fresh HD seed and returns its first receive address.
// Funds must be swept to that address before FinalizeKeyRotation; signing still uses the current keys until then.
func (w *Disk) BeginKeyRotation() (newAddr string, err error) {
	if err := w.requireUnlocked(); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.hdEnabled() {
		return "", fmt.Errorf("key rotation requires an HD wallet")
	}
	seed, err := newHDSeed()
	if err != nil {
		return "", err
	}
	d0, err := deriveHDAt(seed, w.addrVer, bip44ReceivePath(0, 0))
	if err != nil {
		return "", err
	}
	w.rotatePending = &rotationPending{newSeed: seed, newAddr: d0.Addr}
	return d0.Addr, nil
}

// RotationState returns whether a rotation is pending and the sweep destination address.
func (w *Disk) RotationState() RotationStatus {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.rotatePending == nil {
		return RotationStatus{}
	}
	return RotationStatus{Active: true, NewAddress: w.rotatePending.newAddr}
}

// PendingRotationAddress returns the sweep destination when rotation is active.
func (w *Disk) PendingRotationAddress() (string, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.rotatePending == nil {
		return "", false
	}
	return w.rotatePending.newAddr, true
}

// CancelKeyRotation clears a pending rotation without changing the active wallet.
func (w *Disk) CancelKeyRotation() {
	w.mu.Lock()
	w.rotatePending = nil
	w.mu.Unlock()
}

// FinalizeKeyRotation applies the pending HD seed after funds were swept to PendingRotationAddress.
// A timestamped copy of wallet.json is written beside the live file before the new seed is saved.
func (w *Disk) FinalizeKeyRotation() (archivePath string, err error) {
	if err := w.requireUnlocked(); err != nil {
		return "", err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.rotatePending == nil {
		return "", fmt.Errorf("no key rotation in progress")
	}
	pending := w.rotatePending
	w.rotatePending = nil

	var ap string
	ap, err = w.archiveWalletLocked()
	if err != nil {
		return "", err
	}
	archivePath = ap

	w.hdSeed = append([]byte(nil), pending.newSeed...)
	w.hdChangeNext = 0
	w.hdChangeKeypool = w.hdChangeKeypool[:0]
	w.extraPrivHex = nil
	w.importedDesc = nil
	w.usedRecvScripts = make(map[string]struct{})
	w.scannedTx = nil
	w.prunedImports = nil
	w.replacements = make(map[string]string)
	w.abandoned = nil

	d0, err := w.deriveReceive(0)
	if err != nil {
		return "", err
	}
	w.priv = d0.Priv
	w.addr = d0.Addr
	w.seedInitialReceiveKeypoolLocked()
	if err := w.topUpChangeKeypoolLocked(defaultKeypoolSize); err != nil {
		return "", err
	}
	if err := w.saveLocked(); err != nil {
		return "", err
	}
	return archivePath, nil
}

func (w *Disk) archiveWalletLocked() (string, error) {
	if w.path == "" {
		return "", fmt.Errorf("wallet path not set")
	}
	stamp := time.Now().UTC().Format("20060102-150405")
	archive := w.path + ".pre-rotate-" + stamp
	data, err := os.ReadFile(w.path)
	if err != nil {
		return "", fmt.Errorf("read wallet for archive: %w", err)
	}
	if err := os.WriteFile(archive, data, 0o600); err != nil {
		return "", fmt.Errorf("archive wallet: %w", err)
	}
	return archive, nil
}

// RemoveRotationArchive deletes a pre-rotate wallet.json backup after the operator confirms migration.
func RemoveRotationArchive(path string) error {
	path = filepath.Clean(path)
	if path == "" {
		return fmt.Errorf("empty archive path")
	}
	base := filepath.Base(path)
	if len(base) < 12 || base[:12] != "wallet.json." {
		return fmt.Errorf("refusing to delete non-wallet archive")
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
