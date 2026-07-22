// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
)

const maxP2PFrameWire = 2 << 20 // 2 MiB cap per proxied P2P message

// P2P frame proxy status bytes (phase 2).
const (
	P2PProxyOK         byte = 0
	P2PProxyDialFail   byte = 1
	P2PProxyTimeout    byte = 2
	P2PProxyWireErr    byte = 3
	P2PProxyNoResponse byte = 4 // write delivered; peer had nothing to send yet
)

var (
	errP2PFrameTooLarge = errors.New("dgr: p2p frame too large")
	errP2PFrameShort    = errors.New("dgr: p2p frame too short")
)

// encodeP2PFrameRequest builds a client→relay P2P proxy request.
// wireMsg must be a full Dogecoin P2P frame (24-byte header + payload).
func encodeP2PFrameRequest(requestID uint32, peer string, wireMsg []byte) ([]byte, error) {
	peer = strings.TrimSpace(peer)
	if len(peer) == 0 {
		return nil, fmt.Errorf("dgr: empty peer target")
	}
	if len(peer) > 65535 {
		return nil, fmt.Errorf("dgr: peer target too long")
	}
	if len(wireMsg) == 0 {
		return nil, fmt.Errorf("dgr: empty wire message")
	}
	if len(wireMsg) > maxP2PFrameWire {
		return nil, errP2PFrameTooLarge
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, requestID)
	writeString(&buf, string(peer))
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(wireMsg)))
	if _, err := buf.Write(wireMsg); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeP2PFrameRequest(payload []byte) (requestID uint32, peer string, wireMsg []byte, ok bool) {
	if len(payload) < 4+2 {
		return 0, "", nil, false
	}
	r := bytes.NewReader(payload)
	if err := binary.Read(r, binary.BigEndian, &requestID); err != nil {
		return 0, "", nil, false
	}
	var err error
	peer, err = readString(r)
	if err != nil || peer == "" {
		return 0, "", nil, false
	}
	var n uint32
	if err = binary.Read(r, binary.BigEndian, &n); err != nil || n == 0 || n > maxP2PFrameWire {
		return 0, "", nil, false
	}
	if r.Len() < int(n) {
		return 0, "", nil, false
	}
	wireMsg = make([]byte, n)
	if _, err = r.Read(wireMsg); err != nil {
		return 0, "", nil, false
	}
	return requestID, peer, wireMsg, true
}

func encodeP2PFrameResponse(requestID uint32, status byte, wireMsg []byte) ([]byte, error) {
	if status == P2PProxyOK {
		if len(wireMsg) == 0 {
			return nil, fmt.Errorf("dgr: empty ok response")
		}
		if len(wireMsg) > maxP2PFrameWire {
			return nil, errP2PFrameTooLarge
		}
	} else if status == P2PProxyNoResponse {
		wireMsg = nil
	} else {
		wireMsg = nil
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, requestID)
	_ = buf.WriteByte(status)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(wireMsg)))
	if len(wireMsg) > 0 {
		_, _ = buf.Write(wireMsg)
	}
	return buf.Bytes(), nil
}

func decodeP2PFrameResponse(payload []byte) (requestID uint32, status byte, wireMsg []byte, ok bool) {
	if len(payload) < 4+1+4 {
		return 0, 0, nil, false
	}
	r := bytes.NewReader(payload)
	if err := binary.Read(r, binary.BigEndian, &requestID); err != nil {
		return 0, 0, nil, false
	}
	statusByte, err := r.ReadByte()
	if err != nil {
		return 0, 0, nil, false
	}
	var n uint32
	if err = binary.Read(r, binary.BigEndian, &n); err != nil {
		return 0, 0, nil, false
	}
	if n > maxP2PFrameWire || r.Len() < int(n) {
		return 0, 0, nil, false
	}
	if n > 0 {
		wireMsg = make([]byte, n)
		if _, err = r.Read(wireMsg); err != nil {
			return 0, 0, nil, false
		}
	}
	return requestID, statusByte, wireMsg, true
}
