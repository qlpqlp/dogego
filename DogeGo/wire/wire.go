// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

const headerLen = 24

func checksum(payload []byte) [4]byte {
	h := sha256.Sum256(payload)
	h2 := sha256.Sum256(h[:])
	var c [4]byte
	copy(c[:], h2[:4])
	return c
}

// WriteMessage writes a full P2P frame (header + payload).
func WriteMessage(w io.Writer, magic [4]byte, cmd string, payload []byte) error {
	if len(cmd) > 12 {
		return fmt.Errorf("command too long")
	}
	var hdr [headerLen]byte
	copy(hdr[0:4], magic[:])
	copy(hdr[4:16], cmd)
	for i := len(cmd); i < 12; i++ {
		hdr[4+i] = 0
	}
	binary.LittleEndian.PutUint32(hdr[16:20], uint32(len(payload)))
	cs := checksum(payload)
	copy(hdr[20:24], cs[:])
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if len(payload) > 0 {
		_, err := w.Write(payload)
		return err
	}
	return nil
}

// ReadMessage reads one frame; returns command (trimmed) and payload.
func ReadMessage(r io.Reader, magic [4]byte) (string, []byte, error) {
	var hdr [headerLen]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return "", nil, err
	}
	if !bytes.Equal(hdr[0:4], magic[:]) {
		return "", nil, fmt.Errorf("bad magic %x, expected %x", hdr[0:4], magic[:])
	}
	cmd := string(bytes.TrimRight(hdr[4:16], "\x00"))
	n := binary.LittleEndian.Uint32(hdr[16:20])
	if n > 32*1024*1024 {
		return "", nil, fmt.Errorf("oversized message %d", n)
	}
	payload := make([]byte, n)
	if n > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return "", nil, err
		}
	}
	cs := checksum(payload)
	if !bytes.Equal(hdr[20:24], cs[:]) {
		return "", nil, fmt.Errorf("checksum mismatch for %s", cmd)
	}
	return cmd, payload, nil
}

// IPv4-mapped IPv6 (::ffff:a.b.c.d) on the wire.
func encodeAddr(services uint64, ip net.IP, port uint16) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, services)
	var ip16 [16]byte
	if v4 := ip.To4(); v4 != nil {
		ip16[10] = 0xff
		ip16[11] = 0xff
		copy(ip16[12:16], v4)
	} else if v6 := ip.To16(); v6 != nil {
		copy(ip16[:], v6)
	}
	b.Write(ip16[:])
	_ = binary.Write(&b, binary.BigEndian, port)
	return b.Bytes()
}

// BuildVersionPayload builds the version message body (INIT_PROTO_VERSION addr layout).
func BuildVersionPayload(protocolVersion int32, nodeNetwork uint64, remoteIP net.IP, remotePort uint16, nonce uint64, userAgent string, startHeight int32, relay bool) []byte {
	var b bytes.Buffer
	_ = binary.Write(&b, binary.LittleEndian, protocolVersion)
	_ = binary.Write(&b, binary.LittleEndian, nodeNetwork)
	_ = binary.Write(&b, binary.LittleEndian, time.Now().Unix())
	// addrYou
	b.Write(encodeAddr(nodeNetwork, remoteIP, remotePort))
	// addrMe
	b.Write(encodeAddr(nodeNetwork, net.IPv4zero, 0))
	_ = binary.Write(&b, binary.LittleEndian, nonce)
	_ = WriteCompactSize(&b, uint64(len(userAgent)))
	b.WriteString(userAgent)
	_ = binary.Write(&b, binary.LittleEndian, startHeight)
	if relay {
		b.WriteByte(1)
	} else {
		b.WriteByte(0)
	}
	return b.Bytes()
}
