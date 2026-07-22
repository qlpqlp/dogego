// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "fmt"

// rbfMockPool is a minimal MempoolRBFPool for differential tests and harness fixtures.
type rbfMockPool struct {
	spend map[string]string
	raw   map[string][]byte
}

func (m *rbfMockPool) SpendsOutpoint(txid string, vout uint32) bool {
	_, ok := m.spend[rpcOutpointKey(txid, vout)]
	return ok
}

func (m *rbfMockPool) SpenderOfOutpoint(txid string, vout uint32) string {
	return m.spend[rpcOutpointKey(txid, vout)]
}

func (m *rbfMockPool) RemoveCluster(id string) ([]string, error) {
	delete(m.raw, id)
	for k, v := range m.spend {
		if v == id {
			delete(m.spend, k)
		}
	}
	return []string{id}, nil
}

func (m *rbfMockPool) GetRawByTxID(id string) ([]byte, error) {
	return m.raw[id], nil
}

func rpcOutpointKey(txid string, vout uint32) string {
	return fmt.Sprintf("%s:%d", txid, vout)
}

// fixedPrevOutView resolves prevouts by RPC display txid for RBF differential fixtures.
type fixedPrevOutView map[string]PrevOut

func (v fixedPrevOutView) Lookup(prevHash [32]byte, idx uint32) (PrevOut, bool) {
	o, ok := v[rpcOutpointKey(txidDisplayFromLE(prevHash), idx)]
	return o, ok
}
