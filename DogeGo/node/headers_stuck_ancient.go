// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/store"
)

// ancientHeaderTipBlockTime is the latest header nTime we still treat as a partial/stale IBD journal
// (mainnet headers before ~2016 imply the chain never reached modern network height).
const ancientHeaderTipBlockTimeUnix = 1451606400 // 2016-01-01 UTC

const stuckAncientPeerLead = 1_000_000

// MaybeResetStuckAncientHeaderChain truncates headers.bin to genesis when the journal is far behind
// the network, has no valid block bodies yet, and automatic rewinds are not converging.
// Safe to call from startup sweep, periodic recovery, and the header-advance watchdog.
func MaybeResetStuckAncientHeaderChain(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx, peerStart int32) (bool, error) {
	if j == nil || p.Net != chain.MainnetDogecoin {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip <= 0 || tip >= badNBitsGenesisResetTipCeiling {
		return false, nil
	}
	if bs != nil && !hasNoStoredBlockBodies(bs) {
		return false, nil
	}
	if bad, reason := JournalFailsKnownCheckpoint(j, p.Net); bad {
		applog.Line("headers", "stuck ancient chain: "+reason)
		return resetHeaderJournalToGenesis(j, aux, bs, tip, "checkpoint mismatch below tip")
	}
	tipTime := headerTipBlockTimeUnix(j, tip)
	ancientTip := tipTime > 0 && tipTime < ancientHeaderTipBlockTimeUnix
	farBehindPeer := peerStart > 0 && int64(peerStart) > tip+stuckAncientPeerLead
	if !ancientTip && !farBehindPeer {
		return false, nil
	}
	reason := fmt.Sprintf("header tip %d is ancient (block time %d) with no stored block bodies", tip, tipTime)
	if farBehindPeer {
		reason = fmt.Sprintf("header tip %d stuck while peers report height %d and no block bodies are stored", tip, peerStart)
	}
	applog.Line("headers", "auto recovery: "+reason+" - resetting headers.bin to genesis for clean IBD")
	return resetHeaderJournalToGenesis(j, aux, bs, tip, reason)
}

func resetHeaderJournalToGenesis(j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, wasTip int64, reason string) (bool, error) {
	if err := TruncateChainToHeight(j, aux, bs, 0); err != nil {
		return false, err
	}
	if bs != nil {
		bs.lastBadNBitsRewind = -1
		bs.badNBitsRepeatHeight = -1
		bs.badNBitsRepeatCount = 0
		if bs.ChainWork != nil {
			bs.ChainWork.Invalidate()
			bs.ChainWork.Warm(j)
		}
	}
	newTip, _ := j.TipHeight()
	applog.Line("headers", fmt.Sprintf("header journal reset to height %d (was %d); %s", newTip, wasTip, reason))
	setHeaderSyncRecoveryHint("Partial header chain was reset - syncing headers from peers.")
	return true, fmt.Errorf("headers: rewound journal to height 0 after stuck ancient chain (%s)", reason)
}

func hasNoStoredBlockBodies(bs *BlockStoreCtx) bool {
	if bs == nil || bs.Journal == nil {
		return true
	}
	if bs.Raw == nil {
		return true
	}
	return NeedsGenesisBlock(bs)
}

// RecordWatchdogHeaderStall tracks repeated stalls at the same tip; returns whether to log and whether to genesis-reset.
func RecordWatchdogHeaderStall(tip int64, peerH int32) (logNow, genesisReset bool) {
	syncActivity.mu.Lock()
	defer syncActivity.mu.Unlock()
	now := time.Now()
	if syncActivity.watchdogStallTip != tip {
		syncActivity.watchdogStallTip = tip
		syncActivity.watchdogStallCount = 1
	} else {
		syncActivity.watchdogStallCount++
	}
	logNow = now.Sub(syncActivity.lastWatchdogLog) >= 2*time.Minute
	if logNow {
		syncActivity.lastWatchdogLog = now
	}
	// Only suggest reset when the network is far ahead (unknown peer height must not trigger reset).
	tryReset := tip > 0 && tip < badNBitsGenesisResetTipCeiling &&
		syncActivity.watchdogStallCount >= 3 &&
		peerH > 0 && int64(peerH) > tip+stuckAncientPeerLead
	return logNow, tryReset
}

// maybeResetStuckAncientInSweep is the periodic autoRecoverSweep hook (skips tiny test/solo chains).
func maybeResetStuckAncientInSweep(j *store.HeaderJournal, aux *store.HeaderAuxJournal, p chain.Params, bs *BlockStoreCtx) (bool, error) {
	if j == nil {
		return false, nil
	}
	tip, err := j.TipHeight()
	if err != nil || tip < 1000 {
		return false, nil
	}
	return MaybeResetStuckAncientHeaderChain(j, aux, p, bs, 0)
}
