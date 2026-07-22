// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"

	"dogego/wallet"
)

// walletLoaded reports whether a wallet.json instance is attached to this node.
func walletLoaded(cfg StartConfig) bool {
	return cfg.Wallet != nil
}

// walletRPCReady reports whether send/list wallet RPC bridges are wired (full node).
func walletRPCReady(cfg StartConfig) bool {
	return cfg.WalletSend != nil
}

// walletAddressReady reports whether getnewaddress can run (wallet disk + in-process RPC).
func walletAddressReady(cfg StartConfig) bool {
	return cfg.Wallet != nil && cfg.RPCInvoke != nil
}

// walletStatusJSON is returned by GET /api/wallet for Receive/Send tabs.
func walletStatusJSON(w *wallet.Disk, network string) map[string]any {
	if w == nil {
		return map[string]any{"enabled": false, "network": network}
	}
	addr := w.DefaultAddress()
	if addr == "" {
		addr = w.Address()
	}
	hdpath := ""
	if addr != "" {
		if p, _, ok := w.AddressHDPath(addr); ok {
			hdpath = p
		}
	}
	out := map[string]any{
		"enabled":                 true,
		"address":                 addr,
		"path":                    hdpath,
		"encrypted":               w.IsEncrypted(),
		"unlocked":                w.IsUnlocked(),
		"private_keys_enabled":    !w.IsEncrypted() || w.IsUnlocked(),
		"network":                 network,
		"pq_commitments_enabled":  w.PqCommitmentsEnabled(),
		"avoid_reuse":             w.AvoidReuse(),
	}
	if until := w.UnlockUntil(); until > 0 && w.IsEncrypted() && w.IsUnlocked() {
		out["unlocked_until"] = until
	}
	for k, v := range w.PQStatus() {
		out[k] = v
	}
	attachWalletKeypoolStatus(out, w)
	if w.HDSeedIDHex() != "" {
		out["hd_wallet"] = true
	}
	if wallet.MainnetRequiresEncryption(network) && !w.IsEncrypted() {
		out["mainnet_encryption_required"] = true
	}
	return out
}

// attachWalletEncryptionStatus adds disk encryption flags for dashboard polling before GET /api/wallet.
func attachWalletEncryptionStatus(summary map[string]any, w *wallet.Disk) {
	if summary == nil || w == nil {
		return
	}
	summary["wallet_encrypted"] = w.IsEncrypted()
	summary["wallet_unlocked"] = w.IsUnlocked()
	summary["wallet_private_keys_enabled"] = !w.IsEncrypted() || w.IsUnlocked()
	if until := w.UnlockUntil(); until > 0 && w.IsEncrypted() && w.IsUnlocked() {
		summary["wallet_unlocked_until"] = until
	}
}

func attachWalletKeypoolStatus(out map[string]any, w *wallet.Disk) {
	if out == nil || w == nil {
		return
	}
	recv := w.KeypoolSize()
	chg := w.ChangeKeypoolSize()
	if recv > 0 {
		out["keypool_size"] = recv
	}
	if chg > 0 {
		out["change_keypool_size"] = chg
	}
	entries := w.HDKeypoolCoreIndexEntries()
	if len(entries) == 0 {
		return
	}
	out["pool_core_indices_stored"] = len(entries)
	idx := make(map[string]int64, len(entries))
	for _, e := range entries {
		idx[fmt.Sprintf("%d", e.ReceiveIndex)] = e.CoreIndex
	}
	out["hd_keypool_core_index"] = idx
}
