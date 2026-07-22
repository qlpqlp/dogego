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
)

// MaxUserAgentLen is the maximum accepted user-agent string length (Core-style cap).
const MaxUserAgentLen = 256

// DecodedVersion is the subset of the P2P "version" message body DogeGo decodes for RPC / logging.
type DecodedVersion struct {
	ProtocolVersion int32
	Services        uint64
	Timestamp       int64 // peer nTime from version (Unix seconds)
	UserAgent       string
	StartHeight     int32
	RelayTxes       bool // BIP37 optional relay byte (default true when omitted)
}

// TimeOffsetSeconds returns peer nTime minus now (Core getpeerinfo timeoffset).
func TimeOffsetSeconds(dv *DecodedVersion, nowUnix int64) int32 {
	if dv == nil || dv.Timestamp == 0 {
		return 0
	}
	if nowUnix <= 0 {
		nowUnix = 0
	}
	off := dv.Timestamp - nowUnix
	const max = int64(1<<31 - 1)
	const min = -int64(1 << 31)
	if off > max {
		return int32(max)
	}
	if off < min {
		return int32(min)
	}
	return int32(off)
}

// ParseVersionPayload decodes a standard post-INIT_PROTO_VERSION layout (version, services, time,
// addrYou, addrMe, nonce, user_agent, start_height, optional relay byte).
func ParseVersionPayload(pl []byte) (*DecodedVersion, error) {
	r := bytes.NewReader(pl)
	var ver int32
	if err := binary.Read(r, binary.LittleEndian, &ver); err != nil {
		return nil, fmt.Errorf("version field: %w", err)
	}
	var services uint64
	if err := binary.Read(r, binary.LittleEndian, &services); err != nil {
		return nil, fmt.Errorf("services: %w", err)
	}
	var ts int64
	if err := binary.Read(r, binary.LittleEndian, &ts); err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	if _, err := io.CopyN(io.Discard, r, 26); err != nil { // addrYou
		return nil, fmt.Errorf("addrYou: %w", err)
	}
	if _, err := io.CopyN(io.Discard, r, 26); err != nil { // addrMe
		return nil, fmt.Errorf("addrMe: %w", err)
	}
	var nonce uint64
	if err := binary.Read(r, binary.LittleEndian, &nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	n, err := ReadCompactSize(r)
	if err != nil {
		return nil, fmt.Errorf("user_agent length: %w", err)
	}
	if n > MaxUserAgentLen {
		return nil, fmt.Errorf("user_agent length %d exceeds max %d", n, MaxUserAgentLen)
	}
	ua := make([]byte, n)
	if _, err := io.ReadFull(r, ua); err != nil {
		return nil, fmt.Errorf("user_agent: %w", err)
	}
	var startH int32
	if err := binary.Read(r, binary.LittleEndian, &startH); err != nil {
		return nil, fmt.Errorf("start_height: %w", err)
	}
	relayTxes := true
	if r.Len() >= 1 {
		var relay byte
		if err := binary.Read(r, binary.LittleEndian, &relay); err != nil {
			return nil, fmt.Errorf("relay: %w", err)
		}
		relayTxes = relay != 0
	}
	return &DecodedVersion{
		ProtocolVersion: ver,
		Services:        services,
		Timestamp:       ts,
		UserAgent:       string(ua),
		StartHeight:     startH,
		RelayTxes:       relayTxes,
	}, nil
}
