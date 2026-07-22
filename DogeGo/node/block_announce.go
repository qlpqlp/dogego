// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"dogego/wire"
)

// BlockAnnounceEnv holds writers used to announce newly connected blocks.
type BlockAnnounceEnv struct {
	Primary          *MsgWriter
	PeerMgr          *PeerMgr
	PrimaryCmpctHBTo bool // we offered primary peer high-bandwidth cmpctblock
}

// AnnounceBlockHash announces a newly connected block (inv to all; cmpctblock to HB peers when raw is available).
// excludeAddr skips one peer (the block source on inbound relay).
func AnnounceBlockHash(env BlockAnnounceEnv, hash [32]byte, raw []byte, excludeAddr string) {
	if env.Primary == nil && env.PeerMgr == nil {
		return
	}
	cmpctPayload, cmpctOK := ([]byte)(nil), false
	if len(raw) > 0 {
		if pl, err := BuildCmpctBlockPayload(raw); err == nil {
			cmpctPayload, cmpctOK = pl, true
		}
	}
	if cmpctOK {
		if env.PeerMgr != nil {
			for _, mw := range env.PeerMgr.cmpctHBToWriters(excludeAddr) {
				if err := mw.Write("cmpctblock", cmpctPayload); err == nil {
					cmpctMetrics.AnnouncedOut.Add(1)
				}
			}
		}
		if env.Primary != nil && env.PrimaryCmpctHBTo {
			if err := env.Primary.Write("cmpctblock", cmpctPayload); err == nil {
				cmpctMetrics.AnnouncedOut.Add(1)
			}
		}
	}
	invBody, err := wire.EncodeInvPayload([]wire.InvEntry{{Type: wire.InvTypeBlock, Hash: hash}})
	if err != nil {
		return
	}
	if env.PeerMgr != nil {
		if cmpctOK {
			env.PeerMgr.BroadcastInvExceptCmpctHB(invBody, excludeAddr)
		} else {
			env.PeerMgr.BroadcastInvExceptAddr(invBody, excludeAddr)
		}
	}
	if env.Primary != nil && (!cmpctOK || !env.PrimaryCmpctHBTo) {
		_ = env.Primary.Write("inv", invBody)
	}
}

// BroadcastInvExceptAddr sends inv to all relay peers except excludeAddr (including HB when cmpct is unavailable).
func (pm *PeerMgr) BroadcastInvExceptAddr(payload []byte, excludeAddr string) {
	if pm == nil || len(payload) == 0 {
		return
	}
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for addr, l := range pm.sessions {
		if addr == excludeAddr || l.mw == nil {
			continue
		}
		writers = append(writers, l.mw)
	}
	pm.mu.Unlock()
	for _, mw := range writers {
		_ = mw.Write("inv", payload)
	}
}

// BroadcastInv sends an inv payload to all connected peers (including primary).
func (pm *PeerMgr) BroadcastInv(payload []byte) {
	if pm == nil || len(payload) == 0 {
		return
	}
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for _, l := range pm.sessions {
		if l.mw != nil {
			writers = append(writers, l.mw)
		}
	}
	pm.mu.Unlock()
	for _, mw := range writers {
		_ = mw.Write("inv", payload)
	}
}

// BroadcastInvExceptCmpctHB sends inv to peers not receiving high-bandwidth cmpctblock.
func (pm *PeerMgr) BroadcastInvExceptCmpctHB(payload []byte, excludeAddr string) {
	if pm == nil || len(payload) == 0 {
		return
	}
	pm.mu.Lock()
	writers := make([]*MsgWriter, 0, len(pm.sessions))
	for addr, l := range pm.sessions {
		if addr == excludeAddr || l.mw == nil || l.cmpctHBTo {
			continue
		}
		writers = append(writers, l.mw)
	}
	pm.mu.Unlock()
	for _, mw := range writers {
		_ = mw.Write("inv", payload)
	}
}
