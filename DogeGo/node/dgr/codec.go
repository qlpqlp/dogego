// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import (
	"bytes"
	"encoding/binary"
	"net"
	"strings"
)

func encodeRegister(network, authToken string, p2pPort int) []byte {
	var buf bytes.Buffer
	writeString(&buf, network)
	writeString(&buf, authToken)
	_ = binary.Write(&buf, binary.BigEndian, uint16(p2pPort))
	return buf.Bytes()
}

func decodeRegister(payload []byte) (network, authToken string, p2pPort int, ok bool) {
	r := bytes.NewReader(payload)
	var err error
	network, err = readString(r)
	if err != nil {
		return "", "", 0, false
	}
	authToken, err = readString(r)
	if err != nil {
		return "", "", 0, false
	}
	var port16 uint16
	if err = binary.Read(r, binary.BigEndian, &port16); err != nil {
		return "", "", 0, false
	}
	return network, authToken, int(port16), true
}

func encodeRegisterOK(sessionID uint64) []byte {
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, sessionID)
	return buf.Bytes()
}

func decodeRegisterOK(payload []byte) (sessionID uint64, ok bool) {
	if len(payload) < 8 {
		return 0, false
	}
	sessionID = binary.BigEndian.Uint64(payload[:8])
	return sessionID, true
}

func encodePeerHint(hostports ...string) []byte {
	var buf bytes.Buffer
	for _, hp := range hostports {
		writeString(&buf, strings.TrimSpace(hp))
	}
	return buf.Bytes()
}

func decodePeerHints(payload []byte) []string {
	r := bytes.NewReader(payload)
	var out []string
	for r.Len() > 0 {
		s, err := readString(r)
		if err != nil {
			break
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func writeString(w *bytes.Buffer, s string) {
	b := []byte(s)
	if len(b) > 65535 {
		b = b[:65535]
	}
	_ = binary.Write(w, binary.BigEndian, uint16(len(b)))
	_, _ = w.Write(b)
}

func readString(r *bytes.Reader) (string, error) {
	var n uint16
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return "", err
	}
	if n == 0 {
		return "", nil
	}
	b := make([]byte, n)
	if _, err := r.Read(b); err != nil {
		return "", err
	}
	return string(b), nil
}

func clientAllowed(remote net.Addr, allow []string) bool {
	if len(allow) == 0 {
		return true
	}
	host, _, err := net.SplitHostPort(remote.String())
	if err != nil {
		host = strings.TrimSpace(remote.String())
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, rule := range allow {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}
		if !strings.Contains(rule, "/") {
			if ip.Equal(net.ParseIP(rule)) {
				return true
			}
			continue
		}
		_, cidr, err := net.ParseCIDR(rule)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}
