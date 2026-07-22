// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package zkl2

import (
	"strings"
	"time"

	"dogego/extensions"
)

// OnBlockConnected indexes new L1 blocks for ZKDG anchors.
func (e *Extension) OnBlockConnected(height int64, host extensions.Host) error {
	return e.indexBlockHeight(host, height)
}

// OnPeerConnected requests missing proofs from a newly negotiated zkproof-v1 peer.
func (e *Extension) OnPeerConnected(peerAddr string, protocols []string, send func(string, []byte) error) {
	if send == nil {
		return
	}
	hasProto := false
	for _, p := range protocols {
		if p == ProtocolID {
			hasProto = true
			break
		}
	}
	if !hasProto {
		return
	}
	go e.syncWithPeer(peerAddr, send)
}

func (e *Extension) syncWithPeer(peerAddr string, send func(string, []byte) error) {
	e.mu.Lock()
	host := e.host
	st := e.store
	e.mu.Unlock()
	if host == nil || st == nil || send == nil {
		return
	}
	tip, err := host.TipHeight()
	if err != nil || tip < 0 {
		return
	}
	start := tip - 512
	if start < 0 {
		start = 0
	}
	req := EncodeGetZKHeaders(start, 256)
	_ = send(CmdGetZKHeaders, req)
}

func (e *Extension) announceProof(host extensions.Host, proofHash string, excludePeer string) {
	if host == nil || proofHash == "" {
		return
	}
	oh, ok := host.(extensions.OverlayHost)
	if !ok {
		return
	}
	inv := EncodeZKInv([]string{proofHash})
	_ = oh.BroadcastOverlay(ProtocolID, CmdZKInv, inv, excludePeer)
}

func (e *Extension) relayZKInv(host extensions.Host, hashes []string, excludePeer string) {
	if host == nil || len(hashes) == 0 {
		return
	}
	oh, ok := host.(extensions.OverlayHost)
	if !ok {
		return
	}
	inv := EncodeZKInv(hashes)
	_ = oh.BroadcastOverlay(ProtocolID, CmdZKInv, inv, excludePeer)
}

func (e *Extension) requestMissingFromPeer(peer string, hashes []string, send func(string, []byte) error) {
	if send == nil || len(hashes) == 0 {
		return
	}
	e.mu.Lock()
	st := e.store
	e.mu.Unlock()
	if st == nil {
		return
	}
	var need []string
	for _, h := range hashes {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "" {
			continue
		}
		if _, ok, _ := st.GetProof(h); ok {
			continue
		}
		need = append(need, h)
		if len(need) >= 32 {
			break
		}
	}
	if len(need) == 0 {
		return
	}
	raw := EncodeGetZKProof(need)
	_ = send(CmdGetZKProof, raw)
}

func (e *Extension) runBackgroundSync() {
	ticker := time.NewTicker(90 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-e.syncStop:
			return
		case <-ticker.C:
			e.backgroundSyncTick()
		}
	}
}

func (e *Extension) backgroundSyncTick() {
	e.mu.Lock()
	host := e.host
	st := e.store
	stop := e.syncStop
	e.mu.Unlock()
	if host == nil || st == nil || stop == nil {
		return
	}
	oh, ok := host.(extensions.OverlayHost)
	if !ok {
		return
	}
	oh.EachOverlayPeer(ProtocolID, func(_ string, send func(string, []byte) error) {
		if send != nil {
			e.syncWithPeer("", send)
		}
	})
	heights, _, err := st.ProofHeightSummary(8)
	if err != nil || len(heights) == 0 {
		return
	}
	var inv []string
	for _, h := range heights {
		hashes, err := st.ListProofHashesAtHeight(h, 4)
		if err != nil {
			continue
		}
		inv = append(inv, hashes...)
		if len(inv) >= 16 {
			break
		}
	}
	if len(inv) > 0 {
		e.relayZKInv(host, inv, "")
	}
}
