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
	"time"
)

// EncodeAddrPayload builds an "addr" message body from network addresses (Core CADDR_TIME_VERSION).
func EncodeAddrPayload(addrs []NetAddress) ([]byte, error) {
	if len(addrs) > MaxAddrPerMessage {
		addrs = addrs[:MaxAddrPerMessage]
	}
	var b bytes.Buffer
	if err := WriteCompactSize(&b, uint64(len(addrs))); err != nil {
		return nil, err
	}
	now := uint32(time.Now().Unix())
	for _, a := range addrs {
		t := a.Time
		if t == 0 {
			t = now
		}
		if err := binary.Write(&b, binary.LittleEndian, t); err != nil {
			return nil, err
		}
		svc := a.Services
		if svc == 0 {
			svc = 1 // NODE_NETWORK
		}
		if err := binary.Write(&b, binary.LittleEndian, svc); err != nil {
			return nil, err
		}
		ip := a.IP.To16()
		if ip == nil {
			ip = net.IPv6zero
		}
		if _, err := b.Write(ip); err != nil {
			return nil, err
		}
		if err := binary.Write(&b, binary.BigEndian, a.Port); err != nil {
			return nil, err
		}
	}
	return b.Bytes(), nil
}
