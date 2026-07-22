// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"sync"
	"time"
)

const p2pWireHeaderLen = 24

func p2pFrameBytes(payloadLen int) int64 {
	return int64(p2pWireHeaderLen + payloadLen)
}

// peerMsgStats tracks per-command P2P wire bytes (24-byte header + payload).
type peerMsgStats struct {
	mu   sync.Mutex
	recv map[string]int64
	sent map[string]int64
}

func newPeerMsgStats() *peerMsgStats {
	return &peerMsgStats{
		recv: make(map[string]int64),
		sent: make(map[string]int64),
	}
}

func (s *peerMsgStats) addRecv(cmd string, n int64) {
	if s == nil || cmd == "" || n <= 0 {
		return
	}
	s.mu.Lock()
	s.recv[cmd] += n
	s.mu.Unlock()
}

func (s *peerMsgStats) addSent(cmd string, n int64) {
	if s == nil || cmd == "" || n <= 0 {
		return
	}
	s.mu.Lock()
	s.sent[cmd] += n
	s.mu.Unlock()
}

func (s *peerMsgStats) recvMap() map[string]int64 {
	return s.copyMap(s.recv)
}

func (s *peerMsgStats) sentMap() map[string]int64 {
	return s.copyMap(s.sent)
}

func (s *peerMsgStats) copyMap(m map[string]int64) map[string]int64 {
	if s == nil {
		return map[string]int64{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int64, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func attachPeerMsgStats(link *peerLink, mw *MsgWriter) {
	stats := newPeerMsgStats()
	if link != nil {
		link.msgStats = stats
	}
	if mw != nil {
		mw.msgStats = stats
		if link != nil {
			mw.onSent = link.noteSend
		}
	}
}

// AttachWriterMsgStats enables Core getpeerinfo bytesrecv_per_msg/bytessent_per_msg on a lone sync link.
func AttachWriterMsgStats(mw *MsgWriter) {
	if mw == nil {
		return
	}
	if mw.msgStats == nil {
		mw.msgStats = newPeerMsgStats()
	}
}

func noteWriterRecv(mw *MsgWriter, cmd string, payloadLen int) {
	if mw == nil || mw.msgStats == nil || cmd == "" {
		return
	}
	mw.msgStats.addRecv(cmd, p2pFrameBytes(payloadLen))
}

func (l *peerLink) noteSend() {
	if l != nil {
		l.lastSend = time.Now()
	}
}
