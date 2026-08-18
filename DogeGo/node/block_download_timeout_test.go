// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"testing"
	"time"
)

func TestEffectiveBlockDownloadTimeoutEarlyIBDCap(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 0
	bs.contiguousMu.Unlock()
	d := EffectiveBlockDownloadTimeout(bs, 4)
	if d != earlyIBDBlockDownloadTimeout {
		t.Fatalf("early IBD timeout=%v want %v", d, earlyIBDBlockDownloadTimeout)
	}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 5000
	bs.contiguousMu.Unlock()
	d = EffectiveBlockDownloadTimeout(bs, 4)
	if d != bodyIBDBlockDownloadTimeout {
		t.Fatalf("body IBD timeout=%v want %v (must not wait Core 17m while claims leak)", d, bodyIBDBlockDownloadTimeout)
	}
}

func TestBlockDownloadTimeoutMatchesCoreFormula(t *testing.T) {
	// Core: max(nPowTargetSpacing,10) * (BASE + PER_PEER * otherDownloaders)
	// BASE=5e6, PER_PEER=2.5e6 millionths → 5 min + 2.5 min/peer at 60s spacing.
	if got := BlockDownloadTimeout(0, 60); got != 5*time.Minute {
		t.Fatalf("1 downloader %v want 5m", got)
	}
	if got := BlockDownloadTimeout(5, 60); got != 1050*time.Second {
		t.Fatalf("6 lanes %v want 1050s (Core nCalculatedDlWindow)", got)
	}
}

func TestEffectiveBlockDownloadTimeoutAt50kUsesCoreWindow(t *testing.T) {
	bs := &BlockStoreCtx{}
	bs.contiguousMu.Lock()
	bs.contiguousTip = 51_000
	bs.contiguousMu.Unlock()
	d := EffectiveBlockDownloadTimeout(bs, 6)
	if d != bodyIBDBlockDownloadTimeout {
		t.Fatalf("body IBD at 51k timeout=%v want %v (not Core 1050s)", d, bodyIBDBlockDownloadTimeout)
	}
}
