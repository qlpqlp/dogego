// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"net"
	"strings"
	"sync"

	"dogego/applog"
	"dogego/rpc"
)

// Misbehavior scoring (Core CConnman::Misbehaving; 100 → ban).
const (
	misbehaviorBanThreshold  = 100
	misbehaviorDefaultBanSec = int64(86400)

	misbehaviorInvalidHeaders = 20
	misbehaviorReject         = 10
	misbehaviorInvalidBlock   = 50
	misbehaviorWitnessTx      = 20
	misbehaviorInvalidTx      = 10
)

// MisbehaviorTracker accumulates per-IP scores and applies setban when threshold is reached.
type MisbehaviorTracker struct {
	mu          sync.Mutex
	ban         rpc.BanManager
	pm          *PeerMgr
	scores      map[string]int
	persistPath string
}

// NewMisbehaviorTracker creates a tracker backed by the node ban manager.
func NewMisbehaviorTracker(ban rpc.BanManager) *MisbehaviorTracker {
	return &MisbehaviorTracker{
		ban:    ban,
		scores: make(map[string]int),
	}
}

// SetPeerMgr enables disconnecting misbehaving sessions (set after PeerMgr is constructed).
func (m *MisbehaviorTracker) SetPeerMgr(pm *PeerMgr) {
	if m != nil {
		m.pm = pm
	}
}

// SetPersistPath enables saving scores to path on each Note and at shutdown (best-effort).
func (m *MisbehaviorTracker) SetPersistPath(path string) {
	if m != nil {
		m.persistPath = path
	}
}

// Note adds score for addr (host:port); bans IP for misbehaviorDefaultBanSec when threshold reached.
func (m *MisbehaviorTracker) Note(addr string, amount int, reason string) {
	if m == nil || m.ban == nil || amount <= 0 || addr == "" {
		return
	}
	ip := peerHostIP(addr)
	if ip == nil {
		return
	}
	key := strings.ToLower(ip.String())
	m.mu.Lock()
	m.scores[key] += amount
	score := m.scores[key]
	m.mu.Unlock()
	if score < misbehaviorBanThreshold {
		applog.Line("net", fmt.Sprintf("misbehavior %s +%d (%s, score %d/%d)", key, amount, reason, score, misbehaviorBanThreshold))
		if m.persistPath != "" {
			_ = SaveMisbehaviorScores(m, m.persistPath)
		}
		return
	}
	if err := m.ban.SetBan(key, "add", misbehaviorDefaultBanSec, false); err != nil {
		applog.Line("net", "misbehavior ban failed: "+err.Error())
		return
	}
	applog.Line("net", fmt.Sprintf("misbehavior ban %s for %d sec (score %d): %s", key, misbehaviorDefaultBanSec, score, reason))
	m.mu.Lock()
	delete(m.scores, key)
	m.mu.Unlock()
	if m.pm != nil && m.ban != nil {
		m.pm.DisconnectBanned(m.ban.IsBanned)
	}
}

// Score returns accumulated misbehavior points for addr (0 if unknown).
func (m *MisbehaviorTracker) Score(addr string) int {
	if m == nil {
		return 0
	}
	ip := peerHostIP(addr)
	if ip == nil {
		return 0
	}
	key := strings.ToLower(ip.String())
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.scores[key]
}

func peerHostIP(hostport string) net.IP {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	return net.ParseIP(host)
}
