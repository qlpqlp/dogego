// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"io"
	"net"
	"strings"
	"syscall"
)

// isPermanentFetchErr reports errors that mean the P2P stream is dead or unusable
// (stop batching further getdata on this connection).
func isPermanentFetchErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	if errors.Is(err, net.ErrClosed) {
		return true
	}
	es := strings.TrimSpace(err.Error())
	if strings.EqualFold(es, "eof") || strings.EqualFold(es, "unexpected eof") {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "forcibly closed") ||
		strings.Contains(msg, "connection aborted") ||
		strings.Contains(msg, "established connection was aborted") ||
		strings.Contains(msg, "use of closed network connection") ||
		strings.Contains(msg, "wsasend") && strings.Contains(msg, "abort") {
		return true
	}
	var oe *net.OpError
	if errors.As(err, &oe) && oe.Err != nil {
		switch {
		case errors.Is(oe.Err, syscall.ECONNRESET):
			return true
		case errors.Is(oe.Err, syscall.EPIPE):
			return true
		case errors.Is(oe.Err, syscall.ECONNABORTED):
			return true
		}
	}
	return false
}
