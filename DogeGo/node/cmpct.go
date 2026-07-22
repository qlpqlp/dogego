// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"

	"dogego/wire"
)

const maxCmpctHBPeers = 3

// NegotiateSendCmpct answers a peer sendcmpct and records compact-block preferences.
// Returns peerAnnouncesCmpct (peer will send us cmpctblock) and weAnnounceCmpct (we will send peer cmpctblock).
func NegotiateSendCmpct(pm *PeerMgr, link *peerLink, mw *MsgWriter, payload []byte, primaryHBTo *bool) (peerAnnouncesCmpct bool, weAnnounceCmpct bool, err error) {
	peer, err := wire.DecodeSendCmpct(payload)
	if err != nil {
		return false, false, err
	}
	if peer.Version != 1 {
		return false, false, nil
	}
	if peer.Announce && link != nil {
		link.cmpctHBFrom = true
	}
	ourAnnounce := false
	if link != nil && link.cmpctHBTo {
		ourAnnounce = true
	} else if pm != nil && pm.cmpctHBSlotsRemaining(primaryHBTo) > 0 {
		ourAnnounce = true
	} else if pm == nil && primaryHBTo != nil && !*primaryHBTo {
		ourAnnounce = true
	}
	body, err := wire.EncodeSendCmpct(ourAnnounce, 1)
	if err != nil {
		return peer.Announce, false, err
	}
	if err := mw.Write("sendcmpct", body); err != nil {
		return peer.Announce, false, err
	}
	if ourAnnounce {
		if link != nil {
			link.cmpctHBTo = true
		}
		if primaryHBTo != nil {
			*primaryHBTo = true
		}
	}
	return peer.Announce, ourAnnounce, nil
}

// SendCmpctOnConnect offers high-bandwidth compact blocks to outbound relay peers.
func SendCmpctOnConnect(pm *PeerMgr, link *peerLink, mw *MsgWriter) error {
	if pm == nil || link == nil || mw == nil {
		return nil
	}
	announce := pm.cmpctHBSlotsRemaining(nil) > 0
	body, err := wire.EncodeSendCmpct(announce, 1)
	if err != nil {
		return err
	}
	if err := mw.Write("sendcmpct", body); err != nil {
		return err
	}
	if announce {
		link.cmpctHBTo = true
	}
	return nil
}

// OfferPrimaryCmpctHB offers high-bandwidth compact blocks to the primary sync peer.
func (pm *PeerMgr) OfferPrimaryCmpctHB(mw *MsgWriter, primaryHBTo *bool) bool {
	if pm == nil || mw == nil || primaryHBTo == nil || *primaryHBTo {
		return *primaryHBTo
	}
	if pm.cmpctHBSlotsRemaining(primaryHBTo) <= 0 {
		return false
	}
	body, err := wire.EncodeSendCmpct(true, 1)
	if err != nil {
		return false
	}
	if err := mw.Write("sendcmpct", body); err != nil {
		return false
	}
	*primaryHBTo = true
	return true
}

func (pm *PeerMgr) cmpctHBSlotsRemaining(primaryHBTo *bool) int {
	if pm == nil {
		return 0
	}
	used := 0
	if primaryHBTo != nil && *primaryHBTo {
		used++
	}
	pm.mu.Lock()
	for _, l := range pm.sessions {
		if l.cmpctHBTo {
			used++
		}
	}
	pm.mu.Unlock()
	if used >= maxCmpctHBPeers {
		return 0
	}
	return maxCmpctHBPeers - used
}

// CmpctHBSessionCounts returns BIP152 HB counts on relay-manager sessions (excludes primary sync link).
func (pm *PeerMgr) CmpctHBSessionCounts() (hbTo, hbFrom int) {
	if pm == nil {
		return 0, 0
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, l := range pm.sessions {
		if l.cmpctHBTo {
			hbTo++
		}
		if l.cmpctHBFrom {
			hbFrom++
		}
	}
	return hbTo, hbFrom
}

func annotateCmpctHBCounts(out map[string]any, pm *PeerMgr, primaryHBTo, primaryHBFrom bool) {
	if out == nil {
		return
	}
	hbTo, hbFrom := 0, 0
	if primaryHBTo {
		hbTo++
	}
	if primaryHBFrom {
		hbFrom++
	}
	if pm != nil {
		t, f := pm.CmpctHBSessionCounts()
		hbTo += t
		hbFrom += f
	}
	out["bip152_hb_to"] = hbTo
	out["bip152_hb_from"] = hbFrom
	out["bip152_hb_max"] = maxCmpctHBPeers
	annotateCmpctRelayMetrics(out)
}

func (pm *PeerMgr) cmpctHBToWriters(exclude string) []*MsgWriter {
	if pm == nil {
		return nil
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	var out []*MsgWriter
	for addr, l := range pm.sessions {
		if addr == exclude || l.mw == nil {
			continue
		}
		if l.cmpctHBTo {
			out = append(out, l.mw)
		}
	}
	return out
}

func randomCmpctNonce() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

// BuildCmpctBlockPayload encodes cmpctblock from a full block payload.
func BuildCmpctBlockPayload(raw []byte) ([]byte, error) {
	nonce, err := randomCmpctNonce()
	if err != nil {
		return nil, err
	}
	hs, err := wire.BuildHeaderAndShortIDsFromBlock(raw, nonce)
	if err != nil {
		return nil, err
	}
	return wire.EncodeHeaderAndShortIDs(hs)
}

// ServeCmpctBlockFromRaw returns cmpctblock bytes for getdata MSG_CMPCT_BLOCK.
func ServeCmpctBlockFromRaw(raw []byte) ([]byte, bool) {
	pl, err := BuildCmpctBlockPayload(raw)
	if err != nil {
		return nil, false
	}
	return pl, true
}

// BlockTxRawsAtIndexes returns serialized txs at the given indexes from a block payload.
func BlockTxRawsAtIndexes(raw []byte, indexes []uint64) ([][]byte, error) {
	offsets, err := wire.BlockTxDiskOffsets(raw)
	if err != nil {
		return nil, err
	}
	out := make([][]byte, 0, len(indexes))
	for _, idx := range indexes {
		if idx >= uint64(len(offsets)) {
			return nil, fmt.Errorf("tx index %d out of range", idx)
		}
		start := int(offsets[idx])
		end := len(raw)
		if int(idx)+1 < len(offsets) {
			end = int(offsets[idx+1])
		}
		out = append(out, append([]byte(nil), raw[start:end]...))
	}
	return out, nil
}
