// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"math/big"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

// ForkProbeFunc requests competing headers from other peers before a local reorg (Core: compare tips).
type ForkProbeFunc func(forkAt int64, forkHash [32]byte)

func encodeForkProbeGetHeaders(j *store.HeaderJournal, forkAt int64, p chain.Params) ([]byte, error) {
	var zero [32]byte
	loc, err := j.BuildBlockLocatorFromHeight(forkAt, 101)
	if err != nil {
		return nil, err
	}
	return wire.EncodeGetHeaders(p.ProtocolVersion, loc, zero)
}

// RequestForkProbeFromRelays asks relay peers for headers building on forkAt (best-effort; replies on relay read loops).
func (pm *PeerMgr) RequestForkProbeFromRelays(p chain.Params, j *store.HeaderJournal, forkAt int64) int {
	if pm == nil || j == nil || forkAt < 0 {
		return 0
	}
	payload, err := encodeForkProbeGetHeaders(j, forkAt, p)
	if err != nil {
		return 0
	}
	addrs := pm.RelayAddrsOrdered(maxRelayHeaderTopUpPeers)
	sent := 0
	for _, addr := range addrs {
		mw := pm.LinkByAddr(addr)
		if mw == nil {
			continue
		}
		if err := RequestHeadersTopUp(mw, payload); err != nil {
			applog.Line("headers", fmt.Sprintf("fork probe %s: %v", addr, err))
			continue
		}
		sent++
		applog.Line("headers", fmt.Sprintf("fork probe getheaders → relay %s (from height %d)", addr, forkAt))
	}
	return sent
}

func logReorgChainWork(forkAt, tipH int64, inc, cur *big.Int, precious bool) {
	if inc == nil || cur == nil {
		return
	}
	delta := new(big.Int).Sub(inc, cur)
	if precious && inc.Cmp(cur) < 0 {
		applog.Line("headers", fmt.Sprintf("reorg at fork height %d: precious block overrides chain work (incoming − current = %s)", forkAt, delta.String()))
		return
	}
	applog.Line("headers", fmt.Sprintf("reorg at fork height %d: truncating %d→%d, incoming chain work beats fork window by %s", forkAt, tipH, forkAt, delta.String()))
}
