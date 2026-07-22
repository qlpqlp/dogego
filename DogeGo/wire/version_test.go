// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire_test

import (
	"net"
	"testing"

	"dogego/wire"
)

func TestParseVersionPayloadRoundtrip(t *testing.T) {
	ua := "/TestPeer:1.0/"
	pl := wire.BuildVersionPayload(70015, 1, net.IPv4(8, 8, 8, 8), 8333, 0x1122334455667788, ua, 12345, true)
	got, err := wire.ParseVersionPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProtocolVersion != 70015 {
		t.Fatalf("version %d", got.ProtocolVersion)
	}
	if got.Services != 1 {
		t.Fatalf("services %d", got.Services)
	}
	if got.UserAgent != ua {
		t.Fatalf("ua %q", got.UserAgent)
	}
	if got.StartHeight != 12345 {
		t.Fatalf("start %d", got.StartHeight)
	}
	if got.Timestamp <= 0 {
		t.Fatal("expected version timestamp")
	}
	off := wire.TimeOffsetSeconds(got, got.Timestamp)
	if off != 0 {
		t.Fatalf("offset at peer time want 0 got %d", off)
	}
	if !got.RelayTxes {
		t.Fatal("relay true")
	}
}

func TestParseVersionPayloadNoRelay(t *testing.T) {
	pl := wire.BuildVersionPayload(70015, 1, net.IPv4(1, 1, 1, 1), 1, 1, "/", 0, false)
	pl = pl[:len(pl)-1] // strip relay byte
	got, err := wire.ParseVersionPayload(pl)
	if err != nil {
		t.Fatal(err)
	}
	if !got.RelayTxes {
		t.Fatal("missing relay byte defaults to true")
	}
	plOff := wire.BuildVersionPayload(70015, 1, net.IPv4(1, 1, 1, 1), 1, 1, "/", 0, false)
	gotOff, err := wire.ParseVersionPayload(plOff)
	if err != nil || gotOff.RelayTxes {
		t.Fatalf("relay=0 got %v err %v", gotOff.RelayTxes, err)
	}
}

func TestParseVersionPayloadUserAgentTooLong(t *testing.T) {
	long := make([]byte, wire.MaxUserAgentLen+1)
	for i := range long {
		long[i] = 'x'
	}
	pl := wire.BuildVersionPayload(1, 0, net.IPv4zero, 0, 0, string(long), 0, false)
	_, err := wire.ParseVersionPayload(pl)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestParseVersionPayloadTooShort(t *testing.T) {
	_, err := wire.ParseVersionPayload([]byte{1, 2, 3})
	if err == nil {
		t.Fatal("expected error")
	}
}
