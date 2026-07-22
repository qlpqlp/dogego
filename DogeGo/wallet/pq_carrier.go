// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wallet

import (
	"encoding/hex"
	"fmt"

	"dogego/consensus"
	"dogego/pqcrypto"
)

// PqCarrierEnabled reports whether TX_C/TX_R carrier sends are allowed.
func (w *Disk) PqCarrierEnabled() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.pqCarrierEnabled
}

// SetPqCarrierEnabled persists the pq_carrier_enabled wallet flag.
func (w *Disk) SetPqCarrierEnabled(v bool) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.pqCarrierEnabled = v
	return w.saveLocked()
}

func (w *Disk) ensurePQCarrierKeysLocked(tag string) (pqcrypto.Scheme, []byte, []byte, error) {
	if err := w.ensurePQMaterialLocked(); err != nil {
		return nil, nil, nil, err
	}
	if tag == "" {
		tag = w.pqTag
	}
	if tag == "" {
		tag = consensus.PQTagFalcon
	}
	scheme, err := pqcrypto.ByOPReturnTag(tag)
	if err != nil {
		return nil, nil, nil, err
	}
	if w.pqKeys == nil {
		w.pqKeys = make(map[string]pqKeyPair)
	}
	if kp, ok := w.pqKeys[tag]; ok && len(kp.pk) > 0 && len(kp.sk) > 0 {
		return scheme, append([]byte(nil), kp.pk...), append([]byte(nil), kp.sk...), nil
	}
	seed := pqcrypto.DeriveSeed(append([]byte("dogego/pq/carrier/v1/"), w.pqCommitSeed...), tag)
	pk, sk, err := scheme.GenerateKey(seed[:])
	if err != nil {
		return nil, nil, nil, err
	}
	w.pqKeys[tag] = pqKeyPair{pk: pk, sk: sk}
	w.pqTag = tag
	if err := w.saveLocked(); err != nil {
		return nil, nil, nil, err
	}
	return scheme, pk, sk, nil
}

// PQCarrierKeyMaterial returns persisted PQ pk/sk for a tag (generates on first use).
func (w *Disk) PQCarrierKeyMaterial(tag string) (opTag string, pk, sk []byte, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	scheme, pk, sk, err := w.ensurePQCarrierKeysLocked(tag)
	if err != nil {
		return "", nil, nil, err
	}
	return scheme.OPReturnTag(), pk, sk, nil
}

// PQCarrierStatus reports carrier readiness for RPC/UI.
func (w *Disk) PQCarrierStatus() map[string]any {
	w.mu.Lock()
	defer w.mu.Unlock()
	tag := w.pqTag
	if tag == "" {
		tag = consensus.PQTagFalcon
	}
	ready := false
	if kp, ok := w.pqKeys[tag]; ok {
		ready = len(kp.pk) > 0
	}
	return map[string]any{
		"pq_carrier_enabled": w.pqCarrierEnabled,
		"pq_carrier_ready":   ready,
		"pq_tag":             tag,
	}
}

type pqKeyPair struct {
	pk []byte
	sk []byte
}

type pqKeyPairJSON struct {
	PKHex string `json:"pk_hex"`
	SKHex string `json:"sk_hex"`
}

func loadPQKeys(m map[string]pqKeyPairJSON) map[string]pqKeyPair {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]pqKeyPair, len(m))
	for tag, row := range m {
		pk, err1 := hex.DecodeString(row.PKHex)
		sk, err2 := hex.DecodeString(row.SKHex)
		if err1 != nil || err2 != nil || len(pk) == 0 || len(sk) == 0 {
			continue
		}
		out[tag] = pqKeyPair{pk: pk, sk: sk}
	}
	return out
}

func savePQKeys(m map[string]pqKeyPair) map[string]pqKeyPairJSON {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]pqKeyPairJSON, len(m))
	for tag, kp := range m {
		out[tag] = pqKeyPairJSON{
			PKHex: hex.EncodeToString(kp.pk),
			SKHex: hex.EncodeToString(kp.sk),
		}
	}
	return out
}

func pqKeysTagReady(m map[string]pqKeyPair, tag string) bool {
	if tag == "" {
		tag = consensus.PQTagFalcon
	}
	kp, ok := m[tag]
	return ok && len(kp.pk) > 0
}

func (w *Disk) pqCarrierReadyLocked() bool {
	tag := w.pqTag
	if tag == "" {
		tag = consensus.PQTagFalcon
	}
	return pqKeysTagReady(w.pqKeys, tag)
}

// ErrPQCarrierDisabled is returned when carrier mode is off.
var ErrPQCarrierDisabled = fmt.Errorf("wallet: pq_carrier_enabled is false")
