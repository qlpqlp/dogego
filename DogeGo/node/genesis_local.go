// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"bytes"
	"fmt"

	"dogego/applog"
	"dogego/chain"
	"dogego/pow"
)

// EnsureLocalGenesis stores height 0 from chainparams when headers exist but rawblocks/ lacks the body.
// Matches Core LoadGenesisBlock / CreateGenesisBlock - no P2P getdata required.
func EnsureLocalGenesis(bs *BlockStoreCtx) error {
	if bs == nil || !NeedsGenesisBlock(bs) {
		return nil
	}
	net := bs.chainNet()
	g80Params, err := pow.Header80FromParams(bs.Params)
	if err != nil {
		return err
	}
	h0, err := bs.Journal.ReadHeaderAt(0)
	if err != nil {
		return err
	}
	if !bytes.Equal(h0, g80Params[:]) {
		return fmt.Errorf("journal genesis header mismatch chainparams (recover headers or truncatetoheight 0)")
	}
	payload, err := chain.GenesisBlockRaw(net)
	if err != nil {
		return fmt.Errorf("genesis block raw: %w", err)
	}
	if len(payload) < 80 {
		return fmt.Errorf("genesis block too short")
	}
	want := pow.BlockHashLE(payload[:80])
	applog.Line("block", fmt.Sprintf("storing local genesis block from chainparams (%s, Core-style)", networkLabel(net)))
	if err := bs.StoreValidatedBlock(want, payload); err != nil {
		return err
	}
	if NeedsGenesisBlock(bs) {
		return fmt.Errorf("genesis fetch: stored payload too small for %s", networkLabel(net))
	}
	return nil
}

// ReconcileGenesisWithContiguous restores chainparams genesis when headers exist but height 0
// is missing while cached contiguous coverage claims progress (stale after purge).
func ReconcileGenesisWithContiguous(bs *BlockStoreCtx) {
	if bs == nil || !NeedsGenesisBlock(bs) {
		return
	}
	if bs.utxoAheadOfStoredBodies() {
		// During UTXO-snapshot body replay, restoring genesis must not reset monotonic
		// contiguous (shrink would force connect replay from height 0).
		if err := EnsureLocalGenesis(bs); err != nil {
			applog.Line("block", "genesis reconcile (replay): "+err.Error())
		}
		return
	}
	if cont := bs.ContiguousRawHeight(); cont > 0 {
		applog.Line("block", fmt.Sprintf("genesis missing with contiguous %d; restoring from chainparams", cont))
		bs.shrinkContiguousTipAfterBodyRemoved(0)
	}
	if err := EnsureLocalGenesis(bs); err != nil {
		applog.Line("block", "genesis reconcile: "+err.Error())
	}
}

func networkLabel(net chain.Network) string {
	switch net {
	case chain.MainnetDogecoin:
		return "mainnet"
	case chain.RebootTestnet:
		return "testnet"
	default:
		return fmt.Sprintf("net=%d", net)
	}
}
