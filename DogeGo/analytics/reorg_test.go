// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"path/filepath"
	"testing"
)

func TestRecordReorgEventRoundTrip(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "reorg.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}

	ev := ReorgEvent{
		Network:              "mainnet",
		Kind:                 "header_reorg",
		ForkAt:               100,
		OldTipHeight:         103,
		Depth:                3,
		IncomingCount:        4,
		IncomingWork:         "1000",
		DisplacedWork:        "900",
		WorkDelta:            "100",
		DisplacedAuxPowCount: 2,
		IncomingAuxPowCount:  1,
		DisplacedMinerCounts: map[string]int{"DMinerA": 2},
		IncomingMinerCounts:  map[string]int{"DMinerB": 3},
		Displaced: []ReorgBlockDetail{{
			Height: 101, Hash: "aa", TimeUnix: 1700000000, Bits: 0x1e0ffff0,
			AuxPow: true, ParentHash: "ltc1", MinerAddress: "DMinerA", MinerKind: "p2pkh", BodyAvailable: true,
		}},
		Incoming: []ReorgBlockDetail{{
			Height: 101, Hash: "bb", TimeUnix: 1700000060, Bits: 0x1e0ffff0,
			AuxPow: false, MinerAddress: "DMinerB", MinerKind: "p2pkh", BodyAvailable: false,
		}},
	}
	if err := RecordReorgEvent(db, ev); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := RecordReorgEvent(db, ReorgEvent{
		Network: "mainnet", Kind: "header_reorg", ForkAt: 200, OldTipHeight: 201, Depth: 1,
	}); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	got, err := ReadReorgEvents(db, 10)
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if len(got) != 2 {
		_ = db.Close()
		t.Fatalf("len=%d", len(got))
	}
	if got[0].Seq != 1 || got[0].ForkAt != 100 || got[0].Depth != 3 {
		_ = db.Close()
		t.Fatalf("first %#v", got[0])
	}
	if got[0].DisplacedAuxPowCount != 2 || got[0].Displaced[0].ParentHash != "ltc1" {
		_ = db.Close()
		t.Fatalf("aux/miner detail %#v", got[0])
	}
	if got[1].Seq != 2 || got[1].ForkAt != 200 {
		_ = db.Close()
		t.Fatalf("second %#v", got[1])
	}
	sum := SummarizeReorgEvents(got)
	if sum.Total != 2 || sum.MaxDepth != 3 || sum.AuxPowInvolved != 1 {
		_ = db.Close()
		t.Fatalf("summary %#v", sum)
	}
	if sum.MinerOnDisplaced["DMinerA"] != 2 || sum.MinerOnIncoming["DMinerB"] != 3 {
		_ = db.Close()
		t.Fatalf("miners %#v %#v", sum.MinerOnDisplaced, sum.MinerOnIncoming)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	d, err := ReadSideDetail(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if d.Schema != schemaVersion {
		t.Fatalf("schema %d", d.Schema)
	}
	if len(d.ReorgEvents) != 2 || d.ReorgSummary.Total != 2 {
		t.Fatalf("detail events=%d summary=%#v", len(d.ReorgEvents), d.ReorgSummary)
	}
}
