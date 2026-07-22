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
	"dogego/consensus"
)

// DescriptorRow is one entry for listdescriptors.
type DescriptorRow struct {
	Desc      string
	Timestamp int64
	Active    bool
	Internal  bool
}

// ListDescriptors returns pkh / sh(pkh) / sh(multi) descriptors for HD and watch-only scripts.
func (w *Disk) ListDescriptors(pkhVer, shVer byte) []DescriptorRow {
	w.mu.Lock()
	defer w.mu.Unlock()
	impByDesc := make(map[string]ImportedDescriptor, len(w.importedDesc))
	for _, imp := range w.importedDesc {
		impByDesc[imp.Desc] = imp
	}
	var out []DescriptorRow
	add := func(desc string, internal bool) {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			return
		}
		ts := int64(0)
		if imp, ok := impByDesc[desc]; ok {
			ts = imp.Timestamp
			internal = imp.Internal
		}
		out = append(out, DescriptorRow{
			Desc: desc, Timestamp: ts, Active: true, Internal: internal,
		})
	}
	if w.hdEnabled() && w.priv != nil {
		for i := uint32(0); i <= w.hdMaxReceiveIndexLocked(); i++ {
			if d, err := w.deriveReceive(i); err == nil {
				add("pkh("+d.Addr+")", false)
			}
		}
		for i := uint32(0); i <= w.hdMaxChangeIndexLocked(); i++ {
			if d, err := w.deriveChange(i); err == nil {
				add("pkh("+d.Addr+")", true)
			}
		}
	} else if w.addr != "" {
		add("pkh("+w.addr+")", false)
	}
	seen := make(map[string]struct{})
	for _, pk := range w.watchScripts {
		if desc, ok := consensus.MultiDescriptorFromRedeem(pk); ok {
			add(desc, false)
			continue
		}
		addr := chain.ScriptPubKeyAddress(pk, pkhVer, shVer)
		if addr == "" {
			continue
		}
		if _, ok := seen[addr]; ok {
			continue
		}
		seen[addr] = struct{}{}
		if len(pk) == 25 && pk[0] == 0x76 {
			add("pkh("+addr+")", false)
			continue
		}
		if len(pk) == 23 && pk[0] == 0xa9 {
			if redeemHex, ok := w.watchRedeems[hex.EncodeToString(pk)]; ok && redeemHex != "" {
				if inner, err := hex.DecodeString(strings.TrimSpace(redeemHex)); err == nil {
					if desc, ok := consensus.P2SHRedeemDescriptor(inner, pkhVer); ok {
						add(desc, false)
						continue
					}
					if len(inner) == 25 && inner[0] == 0x76 {
						if innerAddr := chain.ScriptPubKeyAddress(inner, pkhVer, shVer); innerAddr != "" {
							add("sh(pkh("+innerAddr+"))", false)
							continue
						}
					}
				}
			}
			add("sh(pkh("+addr+"))", false)
		}
	}
	seenDesc := make(map[string]struct{}, len(out))
	for _, r := range out {
		seenDesc[r.Desc] = struct{}{}
	}
	for _, imp := range w.importedDesc {
		if _, ok := seenDesc[imp.Desc]; ok {
			continue
		}
		seenDesc[imp.Desc] = struct{}{}
		out = append(out, DescriptorRow{
			Desc: imp.Desc, Timestamp: imp.Timestamp, Active: true, Internal: imp.Internal,
		})
	}
	return out
}
