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

const maxP2PPublishPayload = 2 << 20 // 2 MiB (same cap as P2P_FRAME proxy)

var (
	errPublishEmpty   = errors.New("dgr: empty publish")
	errPublishTooLarge = errors.New("dgr: publish payload too large")
)

// encodeP2PPayload builds cmd + payload for P2P_PUBLISH / P2P_PUSH frames.
func encodeP2PPayload(cmd string, payload []byte) ([]byte, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, errPublishEmpty
	}
	if len(payload) > maxP2PPublishPayload {
		return nil, errPublishTooLarge
	}
	var buf bytes.Buffer
	writeString(&buf, cmd)
	_ = binary.Write(&buf, binary.BigEndian, uint32(len(payload)))
	if len(payload) > 0 {
		_, _ = buf.Write(payload)
	}
	return buf.Bytes(), nil
}

func decodeP2PPayload(frame []byte) (cmd string, payload []byte, ok bool) {
	if len(frame) < 2+4 {
		return "", nil, false
	}
	r := bytes.NewReader(frame)
	var err error
	cmd, err = readString(r)
	if err != nil || cmd == "" {
		return "", nil, false
	}
	var n uint32
	if err = binary.Read(r, binary.BigEndian, &n); err != nil {
		return "", nil, false
	}
	if n > maxP2PPublishPayload {
		return "", nil, false
	}
	if r.Len() < int(n) {
		return "", nil, false
	}
	if n == 0 {
		return cmd, nil, true
	}
	payload = make([]byte, n)
	if _, err = r.Read(payload); err != nil {
		return "", nil, false
	}
	return cmd, payload, true
}

// encodeTunnelData builds peer + wire frame for P2P_TUNNEL push.
func encodeTunnelData(peer string, wireMsg []byte) ([]byte, error) {
	return encodeP2PPayload(peer, wireMsg)
}

func decodeTunnelData(frame []byte) (peer string, wireMsg []byte, ok bool) {
	return decodeP2PPayload(frame)
}

func p2pPublishStatusText(cmd string) string {
	return fmt.Sprintf("publish %s", cmd)
}
