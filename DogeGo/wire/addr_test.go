// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestDecodeAddrPayload_empty(t *testing.T) {
	addrs, err := DecodeAddrPayload([]byte{0})
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 0 {
		t.Fatalf("got %d addrs", len(addrs))
	}
}

func TestDecodeAddrPayload_oneIPv4(t *testing.T) {
	var b bytes.Buffer
	_ = WriteCompactSize(&b, 1)
	_ = binary.Write(&b, binary.LittleEndian, uint32(0x5c6a7b8a))
	_ = binary.Write(&b, binary.LittleEndian, uint64(1))
	var ip16 [16]byte
	ip16[10] = 0xff
	ip16[11] = 0xff
	copy(ip16[12:], net.IPv4(8, 8, 8, 8).To4())
	_, _ = b.Write(ip16[:])
	_ = binary.Write(&b, binary.BigEndian, uint16(22556))

	addrs, err := DecodeAddrPayload(b.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if len(addrs) != 1 {
		t.Fatalf("len %d", len(addrs))
	}
	a := addrs[0]
	if a.Time != 0x5c6a7b8a || a.Services != 1 || !a.IP.Equal(net.IPv4(8, 8, 8, 8)) || a.Port != 22556 {
		t.Fatalf("%+v", a)
	}
	if a.HostPort() != "8.8.8.8:22556" {
		t.Fatalf("HostPort %q", a.HostPort())
	}
}

func TestDecodeAddrPayload_tooMany(t *testing.T) {
	var b bytes.Buffer
	_ = WriteCompactSize(&b, MaxAddrPerMessage+1)
	_, err := DecodeAddrPayload(b.Bytes())
	if err == nil {
		t.Fatal("expected error")
	}
}
