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

func TestSyncActivityNotStalledDuringBodyIBD(t *testing.T) {
	syncActivity.mu.Lock()
	syncActivity.lastProgressAt = time.Now().Add(-10 * time.Minute)
	syncActivity.lastProgress = "validated headers"
	syncActivity.lastKind = "headers"
	syncActivity.mu.Unlock()
	t.Cleanup(func() {
		syncActivity.mu.Lock()
		syncActivity.lastProgressAt = time.Time{}
		syncActivity.lastProgress = ""
		syncActivity.lastKind = ""
		syncActivity.mu.Unlock()
	})

	snap := BuildSyncActivitySnapshot(SyncActivityInput{
		HeaderTip:        534_000,
		ContiguousBodies: 400,
		BlocksPerMinute:  7.2,
		LowestMissing:    401,
		BlockAssistActive: true,
	})
	if stalled, _ := snap["stalled"].(bool); stalled {
		t.Fatal("expected stalled=false while body download is active")
	}
}

func TestSyncActivityHeadlineBodyIBDPause(t *testing.T) {
	snap := BuildSyncActivitySnapshot(SyncActivityInput{
		HeaderTip:            534_000,
		PeerStartHeight:      6_264_746,
		HeaderCatchUpPending: true,
		BodyIBDHeaderPaused:  true,
		ContiguousBodies:     744,
		ChainActiveHeight:    744,
		LowestMissing:        745,
		BlocksPerMinute:      2.4,
		InFlightBatches:      1,
	})
	headline, _ := snap["headline"].(string)
	if headline != "Downloading block bodies from height 745" {
		t.Fatalf("headline %q want body download, not header catch-up", headline)
	}
	tasks, _ := snap["tasks"].([]map[string]string)
	for _, task := range tasks {
		if task["name"] == "header_sync" && task["state"] != "paused" {
			t.Fatalf("header_sync task state %q want paused", task["state"])
		}
	}
}

func TestSyncActivityStalledWhenNoBodyProgress(t *testing.T) {
	syncActivity.mu.Lock()
	syncActivity.lastProgressAt = time.Now().Add(-10 * time.Minute)
	syncActivity.lastProgress = "validated headers"
	syncActivity.mu.Unlock()
	t.Cleanup(func() {
		syncActivity.mu.Lock()
		syncActivity.lastProgressAt = time.Time{}
		syncActivity.lastProgress = ""
		syncActivity.mu.Unlock()
	})

	snap := BuildSyncActivitySnapshot(SyncActivityInput{
		HeaderTip:        534_000,
		ContiguousBodies: 400,
		LowestMissing:    -1,
	})
	if stalled, _ := snap["stalled"].(bool); !stalled {
		t.Fatal("expected stalled=true with large header/body gap and no body activity")
	}
}
