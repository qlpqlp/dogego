// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/binary"
	"strings"
	"testing"
	"time"

	"dogego/chain"
)

func TestHeaderSyncDiagnosticsStaleTip(t *testing.T) {
	h := make([]byte, 80)
	stale := uint32(time.Now().Unix() - chain.DefaultMaxTipAge - 7200)
	binary.LittleEndian.PutUint32(h[68:72], stale)
	j := &memJournal{tip: 10, best: "b", gen: "g", count: 11, hdrs: repeatHdr(h, 11)}
	d := HeaderSyncDiagnostics(j, 10, 10, nil)
	if d["dogego_header_tip_stale"] != true {
		t.Fatalf("want stale %#v", d)
	}
	if _, ok := d["dogego_header_sync_recovery"].(string); !ok {
		t.Fatal("want recovery hint")
	}
}

func TestHeaderSyncDiagnosticsHeadersAhead(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[68:72], uint32(time.Now().Unix()))
	j := &memJournal{tip: 2000, best: "b", gen: "g", count: 2001, hdrs: repeatHdr(h, 2001)}
	d := HeaderSyncDiagnostics(j, 2000, 100, nil)
	if d["dogego_headers_ahead_of_chainactive"].(int64) != 1900 {
		t.Fatalf("ahead %#v", d["dogego_headers_ahead_of_chainactive"])
	}
	if _, ok := d["dogego_body_sync_note"].(string); !ok {
		t.Fatal("want body IBD note when headers far ahead of bodies")
	}
	if _, ok := d["dogego_header_sync_recovery"].(string); ok {
		t.Fatal("large header/body gap during forward IBD must not set recovery hint")
	}
}

func TestHeaderSyncDiagnosticsPostAuxEraStallFlag(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[68:72], uint32(time.Now().Unix()))
	j := &memJournal{tip: 510_000, best: "b", gen: "g", count: 510_001, hdrs: repeatHdr(h, 510_001)}
	paths := &DataPaths{HeaderCatchUpPending: func() bool { return true }}
	d := HeaderSyncDiagnostics(j, 510_000, 3000, paths)
	if d["dogego_post_aux_era_header_stall"] != true {
		t.Fatalf("stall flag %#v", d)
	}
}

func TestHeaderSyncDiagnosticsLargeBodyIBDGap(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[68:72], uint32(time.Now().Unix()))
	// Below assumevalid height: headers keep syncing (not paused).
	j := &memJournal{tip: 534_000, best: "b", gen: "g", count: 534_001, hdrs: repeatHdr(h, 534_001)}
	d := HeaderSyncDiagnostics(j, 534_000, 5966, nil)
	if d["dogego_body_ibd_header_paused"] == true {
		t.Fatalf("headers below assumevalid must not report pause, got %#v", d["dogego_body_ibd_header_paused"])
	}
	note, ok := d["dogego_body_sync_note"].(string)
	if !ok || !strings.Contains(note, "534000") || !strings.Contains(note, "assumevalid") {
		t.Fatalf("note %q", note)
	}
	// At/after assumevalid height with large body gap: pause flag may show.
	j2 := &memJournal{tip: 5_100_000, best: "b", gen: "g", count: 100, hdrs: repeatHdr(h, 100)}
	d2 := HeaderSyncDiagnostics(j2, 5_100_000, 5966, nil)
	if d2["dogego_body_ibd_header_paused"] != true {
		t.Fatalf("paused flag %#v", d2["dogego_body_ibd_header_paused"])
	}
}

func TestHeaderSyncDiagnosticsRecoveryHintFromPaths(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[68:72], uint32(time.Now().Unix()))
	j := &memJournal{tip: 10, best: "b", gen: "g", count: 11, hdrs: repeatHdr(h, 11)}
	paths := &DataPaths{HeaderSyncRecoveryHint: func() string { return "recovering headers" }}
	d := HeaderSyncDiagnostics(j, 10, 5, paths)
	if d["dogego_header_sync_recovery"] != "recovering headers" {
		t.Fatalf("hint %#v", d["dogego_header_sync_recovery"])
	}
}

func repeatHdr(h []byte, n int) [][]byte {
	out := make([][]byte, n)
	for i := range out {
		b := make([]byte, 80)
		copy(b, h)
		binary.LittleEndian.PutUint32(b[0:4], uint32(i))
		out[i] = b
	}
	return out
}
