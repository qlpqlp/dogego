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
)

// SendCmpct is Dogecoin Core's sendcmpct body (not Bitcoin BIP152's 1-byte mode).
// See src/net_processing.cpp and qa/rpc-tests/test_framework/mininode.py msg_sendcmpct.
type SendCmpct struct {
	Announce bool   // fAnnounceUsingCMPCTBLOCK - peer wants cmpctblock inv announcements
	Version  uint64 // compact block version (1 legacy, 2 witness); Dogecoin uses 1
}

// DecodeSendCmpct parses sendcmpct (bool announce + uint64 version, 9 bytes).
func DecodeSendCmpct(payload []byte) (SendCmpct, error) {
	if len(payload) != 9 {
		return SendCmpct{}, fmt.Errorf("sendcmpct: want 9 bytes, got %d", len(payload))
	}
	r := bytes.NewReader(payload)
	var flag byte
	if err := binary.Read(r, binary.LittleEndian, &flag); err != nil {
		return SendCmpct{}, err
	}
	if flag > 1 {
		return SendCmpct{}, fmt.Errorf("sendcmpct: invalid announce byte %d", flag)
	}
	var ver uint64
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return SendCmpct{}, err
	}
	return SendCmpct{Announce: flag != 0, Version: ver}, nil
}

// EncodeSendCmpct builds sendcmpct for outbound negotiation.
func EncodeSendCmpct(announce bool, version uint64) ([]byte, error) {
	var buf [9]byte
	if announce {
		buf[0] = 1
	}
	binary.LittleEndian.PutUint64(buf[1:], version)
	return buf[:], nil
}

// DefaultSendCmpctDecline is used on short-lived header-sync / block-assist links (no compact relay).
func DefaultSendCmpctDecline() ([]byte, error) {
	return EncodeSendCmpct(false, 1)
}
