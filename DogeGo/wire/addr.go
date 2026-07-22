// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// MaxAddrPerMessage bounds how many CAddress entries we decode from one "addr" frame (DoS limit).
const MaxAddrPerMessage = 1000

// caddrWireBytes is the size of one CAddress on the wire for current Dogecoin P2P:
// uint32 nTime + uint64 nServices + 16-byte IPv6 + uint16 port (big-endian), see CAddress::SerializationOp in src/protocol.h.
const caddrWireBytes = 4 + 8 + net.IPv6len + 2

// NetAddress is a CAddress as used in the "addr" message (includes nTime; Core CADDR_TIME_VERSION path).
type NetAddress struct {
	Time     uint32
	Services uint64
	IP       net.IP
	Port     uint16
}

// HostPort returns host:port suitable for net.Dial (IPv6 hosts are bracketed).
func (a NetAddress) HostPort() string {
	if a.IP == nil {
		return ""
	}
	return net.JoinHostPort(a.IP.String(), fmt.Sprintf("%d", a.Port))
}

// DecodeAddrPayload parses the body of a P2P "addr" message: compactSize count, then that many CAddress records.
func DecodeAddrPayload(payload []byte) ([]NetAddress, error) {
	r := bytes.NewReader(payload)
	n, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("addr count: %w", err)
	}
	if n > MaxAddrPerMessage {
		return nil, fmt.Errorf("addr count %d exceeds max %d", n, MaxAddrPerMessage)
	}
	rem := r.Len()
	if rem < int(n)*caddrWireBytes {
		return nil, fmt.Errorf("addr payload truncated: want %d bytes have %d", int(n)*caddrWireBytes, rem)
	}
	out := make([]NetAddress, 0, n)
	for i := uint64(0); i < n; i++ {
		var t uint32
		if err := binary.Read(r, binary.LittleEndian, &t); err != nil {
			return nil, err
		}
		var services uint64
		if err := binary.Read(r, binary.LittleEndian, &services); err != nil {
			return nil, err
		}
		var ipb [net.IPv6len]byte
		if _, err := io.ReadFull(r, ipb[:]); err != nil {
			return nil, err
		}
		var port uint16
		if err := binary.Read(r, binary.BigEndian, &port); err != nil {
			return nil, err
		}
		ip := net.IP(append(net.IP(nil), ipb[:]...))
		out = append(out, NetAddress{Time: t, Services: services, IP: ip, Port: port})
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("addr: %d trailing bytes", r.Len())
	}
	return out, nil
}
