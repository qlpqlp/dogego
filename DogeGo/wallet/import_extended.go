// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"strings"

	"dogego/chain"
	"dogego/secp256k1"
)

// RestoreFromMnemonic replaces the HD wallet with keys derived from a BIP39 mnemonic.
func (w *Disk) RestoreFromMnemonic(mnemonic, passphrase string) error {
	if err := w.requireUnlocked(); err != nil {
		return err
	}
	seed, err := MnemonicToSeed(mnemonic, passphrase)
	if err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.hdSeed = append(w.hdSeed[:0], seed...)
	w.hdChangeNext = 0
	w.hdChangeKeypool = w.hdChangeKeypool[:0]
	w.seedInitialReceiveKeypoolLocked()
	d0, err := w.deriveReceive(0)
	if err != nil {
		return err
	}
	w.priv = d0.Priv
	w.addr = d0.Addr
	if err := w.topUpChangeKeypoolLocked(defaultKeypoolSize); err != nil {
		return err
	}
	return w.saveLocked()
}

// ImportBIP38 decrypts a BIP38 key and imports it as the spend key (paper wallet sweep).
func (w *Disk) ImportBIP38(encrypted, passphrase string, wifVer, addrVer byte) error {
	if err := w.requireUnlocked(); err != nil {
		return err
	}
	secret, compressed, err := DecryptBIP38(encrypted, passphrase, addrVer)
	if err != nil {
		return err
	}
	wif, err := chain.EncodeWIF(secret, wifVer, compressed)
	if err != nil {
		return err
	}
	return w.ImportSpendPrivKey(wif, wifVer, addrVer)
}

// AddressEntry describes one tracked wallet address for the UI / RPC.
type AddressEntry struct {
	Address            string `json:"address"`
	HDPath             string `json:"hdpath,omitempty"`
	IsChange           bool   `json:"ischange,omitempty"`
	Label              string `json:"label,omitempty"`
	WatchOnly          bool   `json:"watchonly,omitempty"`
	IsCosigner         bool   `json:"cosigner,omitempty"`
	IsKeypool          bool   `json:"iskeypool,omitempty"`
	IsNodeTip          bool   `json:"isnodetip,omitempty"`
	HDKeypoolCoreIndex *int64 `json:"hd_keypool_core_index,omitempty"`
}

// ListAddressEntries returns HD, labeled, watch-only, and cosigner addresses.
func (w *Disk) ListAddressEntries(pkhVer, shVer byte) []AddressEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	seen := make(map[string]AddressEntry)
	add := func(e AddressEntry) {
		e.Address = strings.TrimSpace(e.Address)
		if e.Address == "" {
			return
		}
		if prev, ok := seen[e.Address]; ok {
			if e.Label != "" && prev.Label == "" {
				prev.Label = e.Label
			}
			if e.HDPath != "" && prev.HDPath == "" {
				prev.HDPath = e.HDPath
			}
			prev.WatchOnly = prev.WatchOnly || e.WatchOnly
			prev.IsCosigner = prev.IsCosigner || e.IsCosigner
			prev.IsChange = prev.IsChange || e.IsChange
			prev.IsKeypool = prev.IsKeypool || e.IsKeypool
			prev.IsNodeTip = prev.IsNodeTip || e.IsNodeTip
			if e.HDKeypoolCoreIndex != nil && prev.HDKeypoolCoreIndex == nil {
				prev.HDKeypoolCoreIndex = e.HDKeypoolCoreIndex
			}
			seen[e.Address] = prev
			return
		}
		seen[e.Address] = e
	}
	if w.hdEnabled() && w.priv != nil {
		for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
			d, err := w.deriveReceive(i)
			if err != nil {
				continue
			}
			entry := AddressEntry{
				Address: d.Addr,
				HDPath:  formatBIP32Path(bip44ReceivePath(0, i)),
				Label:   w.labels[d.Addr],
			}
			for _, k := range w.hdKeypool {
				if k == i {
					entry.IsKeypool = true
					break
				}
			}
			if core, ok := w.hdKeypoolCoreIdx[i]; ok {
				v := core
				entry.HDKeypoolCoreIndex = &v
			}
			add(entry)
		}
		for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
			d, err := w.deriveChange(i)
			if err != nil {
				continue
			}
			entry := AddressEntry{
				Address:  d.Addr,
				HDPath:   formatBIP32Path(bip44ChangePath(0, i)),
				IsChange: true,
				Label:    w.labels[d.Addr],
			}
			for _, k := range w.hdChangeKeypool {
				if k == i {
					entry.IsKeypool = true
					break
				}
			}
			add(entry)
		}
		if w.hdNodeTipEnabled {
			d, err := w.deriveNodeTip(0)
			if err == nil {
				add(AddressEntry{
					Address:   d.Addr,
					HDPath:    formatBIP32Path(bip44NodeTipPath(0, 0)),
					IsNodeTip: true,
					Label:     w.labels[d.Addr],
				})
			}
		}
	} else if w.addr != "" {
		add(AddressEntry{Address: w.addr, Label: w.labels[w.addr]})
	}
	for addr, lbl := range w.labels {
		add(AddressEntry{Address: addr, Label: lbl})
	}
	for _, pk := range w.watchScripts {
		addr := chain.ScriptPubKeyAddress(pk, pkhVer, shVer)
		add(AddressEntry{Address: addr, WatchOnly: true, Label: w.labels[addr]})
	}
	for _, hexKey := range w.extraPrivHex {
		raw, err := hex.DecodeString(strings.TrimSpace(hexKey))
		if err != nil || len(raw) != 32 {
			continue
		}
		priv, pub := secp256k1.PrivKeyFromBytes(raw)
		if priv == nil || pub == nil {
			continue
		}
		addr := p2pkh(w.addrVer, pub)
		add(AddressEntry{Address: addr, IsCosigner: true, Label: w.labels[addr]})
	}
	out := make([]AddressEntry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	SortAddressEntries(out)
	return out
}
