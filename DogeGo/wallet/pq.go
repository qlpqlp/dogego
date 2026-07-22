// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"dogego/consensus"
)

const defaultPQTag = consensus.PQTagFalcon

// ensurePQMaterialLocked derives and persists PQ commitment root material (Phase-1 OP_RETURN carrier).
func (w *Disk) ensurePQMaterialLocked() error {
	if len(w.pqCommitSeed) == 32 && w.pqTag != "" {
		return nil
	}
	var material []byte
	switch {
	case len(w.hdSeed) >= 16:
		material = append([]byte("dogego/pq/v1/hd/"), w.hdSeed...)
	case w.priv != nil:
		material = append([]byte("dogego/pq/v1/legacy/"), w.priv.Serialize()...)
	default:
		return fmt.Errorf("wallet has no key material for PQ commitments")
	}
	sum := sha256.Sum256(material)
	w.pqCommitSeed = append(w.pqCommitSeed[:0], sum[:]...)
	if w.pqTag == "" {
		w.pqTag = defaultPQTag
	}
	w.pqCommitmentsEnabled = true
	return w.saveLocked()
}

// EnsurePQReady generates PQ carrier material on first use (new and upgraded wallets).
func (w *Disk) EnsurePQReady() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.ensurePQMaterialLocked()
}

// NextPQCommitment returns tag + 64-hex commitment for the next wallet send.
func (w *Disk) NextPQCommitment() (tag string, commitHex string, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.ensurePQMaterialLocked(); err != nil {
		return "", "", err
	}
	w.pqSendCounter++
	var buf [8]byte
	for i := 0; i < 8; i++ {
		buf[7-i] = byte(w.pqSendCounter >> (8 * i))
	}
	h := sha256.Sum256(append(w.pqCommitSeed, buf[:]...))
	if err := w.saveLocked(); err != nil {
		return "", "", err
	}
	return w.pqTag, hex.EncodeToString(h[:]), nil
}

// PQStatus reports wallet PQ readiness for the web UI.
func (w *Disk) PQStatus() map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	ready := len(w.pqCommitSeed) == 32
	tag := w.pqTag
	if tag == "" {
		tag = defaultPQTag
	}
	return map[string]any{
		"pq_commitments_enabled": w.pqCommitmentsEnabled,
		"pq_carrier_enabled":     w.pqCarrierEnabled,
		"pq_ready":               ready,
		"pq_tag":                 tag,
		"pq_auto_on_send":        w.pqCommitmentsEnabled,
	}
}
