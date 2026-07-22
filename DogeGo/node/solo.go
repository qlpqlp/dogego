// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/mempool"
	"dogego/rpc"
	"dogego/store"
)

const soloMinerInterval = 15 * time.Second

func canEnterSoloMode(p chain.Params, j *store.HeaderJournal) bool {
	if !p.IsRebootTestnet() || j == nil {
		return false
	}
	n, err := j.Count()
	return err == nil && n > 0
}

func logSoloMode(reason string, detail error) {
	msg := fmt.Sprintf("DogeGo: solo mode - %s", reason)
	if detail != nil {
		msg += fmt.Sprintf(" (%v)", detail)
	}
	msg += "; continuing as reboot testnet founder (auto-mining to wallet until peers supply blocks)"
	fmt.Fprintln(os.Stderr, msg)
	applog.Line("net", msg)
}

// SoloMinerOpts configures the background generate loop.
type SoloMinerOpts struct {
	ChainName  string
	Journal    *store.HeaderJournal
	Aux        *store.HeaderAuxJournal
	Raw        *store.RawBlockStore
	Paths      *rpc.DataPaths
	Pool       *mempool.Pool
	TxIndex    *store.TxIndex
	MiningAddr string
	BlockStore *BlockStoreCtx
	TipWait    *rpc.TipWaiter
	Active     *atomic.Bool
	// MineKick optional; a send on this channel triggers an immediate block attempt (solo testnet).
	MineKick <-chan struct{}
}

// RunSoloMiner periodically mines one block (reboot testnet founder; scrypt PoW).
func RunSoloMiner(ctx context.Context, o SoloMinerOpts) {
	if o.Active != nil {
		o.Active.Store(true)
		defer o.Active.Store(false)
	}
	h160, err := rpc.P2PKHScriptFromAddress(o.ChainName, o.MiningAddr)
	if err != nil {
		applog.Line("mining", "solo miner: invalid mining address: "+err.Error())
		return
	}
	applog.Line("mining", fmt.Sprintf("solo miner active (payout %s, interval %s)", o.MiningAddr, soloMinerInterval))
	tick := time.NewTicker(soloMinerInterval)
	defer tick.Stop()
	var mineMu sync.Mutex
	mineOne := func() {
		mineMu.Lock()
		defer mineMu.Unlock()
		display, err := rpc.MineAndSubmitLegacyBlock(o.Journal, o.Aux, o.Raw, o.Paths, o.Pool, o.TxIndex, o.ChainName, h160, rpc.DefaultGenerateMaxTries())
		if err != nil {
			applog.Line("mining", "solo mine: "+err.Error())
			return
		}
		if o.BlockStore != nil {
			_ = o.BlockStore.SyncUtxoCache()
		}
		var utxo *store.UtxoCache
		if o.BlockStore != nil {
			utxo = o.BlockStore.Utxo
		} else if o.Paths != nil {
			utxo = o.Paths.Utxo
		}
		if o.BlockStore != nil && o.Paths != nil && utxo != nil {
			if dir := strings.TrimSpace(o.Paths.ChainDataDir); dir != "" {
				MaybeSaveCaughtUpUtxoSnapshot(o.BlockStore, utxo, dir)
			}
		}
		NotifyRPCTip(o.Journal, o.Raw, utxo, o.TipWait)
		tip, _ := o.Journal.TipHeight()
		applog.Line("mining", fmt.Sprintf("mined block %s (tip height %d)", display, tip))
	}
	mineOne()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			mineOne()
		case <-o.MineKick:
			mineOne()
		}
	}
}
