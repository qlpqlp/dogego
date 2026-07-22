// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"

	"dogego/chain"
)

// SetNetAddrVersions stores P2PKH/P2SH version bytes for address↔script mapping (node wires after load).
func (w *Disk) SetNetAddrVersions(pkhVer, shVer byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pkhVer = pkhVer
	w.shVer = shVer
}

// RebuildUsedRecvScripts marks scriptPubKeys that have received wallet payments (avoid_reuse index).
func (w *Disk) RebuildUsedRecvScripts(pkhVer, shVer byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if pkhVer != 0 || shVer != 0 {
		w.pkhVer, w.shVer = pkhVer, shVer
	}
	w.rebuildUsedRecvScriptsLocked()
}

func (w *Disk) rebuildUsedRecvScriptsLocked() {
	w.usedRecvScripts = make(map[string]struct{})
	if !w.avoidReuse {
		return
	}
	pkh, sh := w.pkhVer, w.shVer
	if pkh == 0 && sh == 0 {
		pkh = w.addrVer
	}
	addrToScript := make(map[string]string)
	for _, pk := range w.trackedScriptsForReuseLocked() {
		addr := chain.ScriptPubKeyAddress(pk, pkh, sh)
		if addr == "" {
			continue
		}
		addrToScript[addr] = hex.EncodeToString(pk)
	}
	for _, tx := range w.scannedTx {
		if tx.Category != "receive" {
			continue
		}
		if h, ok := addrToScript[tx.Address]; ok {
			w.usedRecvScripts[h] = struct{}{}
		}
	}
}

func (w *Disk) trackedScriptsForReuseLocked() [][]byte {
	seen := make(map[string]struct{})
	var out [][]byte
	add := func(pk []byte) {
		if len(pk) == 0 {
			return
		}
		k := string(pk)
		if _, ok := seen[k]; ok {
			return
		}
		seen[k] = struct{}{}
		out = append(out, append([]byte(nil), pk...))
	}
	for _, s := range w.spendScriptsLocked() {
		add(s)
	}
	if len(out) == 0 {
		add(w.p2pkhScriptLocked())
	}
	for _, pk := range w.watchScripts {
		add(pk)
	}
	return out
}

// IsRecvScriptReused reports whether pkScript received a payment while avoid_reuse is enabled.
func (w *Disk) IsRecvScriptReused(pkScript []byte) bool {
	if len(pkScript) == 0 {
		return false
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.avoidReuse {
		return false
	}
	_, ok := w.usedRecvScripts[hex.EncodeToString(pkScript)]
	return ok
}
