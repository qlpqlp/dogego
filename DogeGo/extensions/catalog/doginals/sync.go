// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"strings"
	"time"

	"dogego/extensions"
)

func (e *Extension) startBackgroundSync() {
	e.mu.Lock()
	if e.syncStop != nil {
		e.mu.Unlock()
		return
	}
	e.syncStop = make(chan struct{})
	stop := e.syncStop
	e.mu.Unlock()
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				e.backgroundSyncTick()
			}
		}
	}()
}

func (e *Extension) stopBackgroundSync() {
	e.mu.Lock()
	stop := e.syncStop
	e.syncStop = nil
	e.mu.Unlock()
	if stop != nil {
		close(stop)
	}
}

func (e *Extension) backgroundSyncTick() {
	e.mu.Lock()
	host := e.host
	st := e.store
	e.mu.Unlock()
	if host == nil || st == nil {
		return
	}
	oh, ok := host.(extensions.OverlayHost)
	if !ok {
		return
	}
	ids, err := st.ListAssetIDs(200)
	if err != nil || len(ids) == 0 {
		return
	}
	_ = oh.BroadcastOverlay(ProtocolID, CmdDInv, EncodeInv(ids), "")
	oh.EachOverlayPeer(ProtocolID, func(_ string, send func(string, []byte) error) {
		if send != nil && len(ids) > 0 {
			_ = send(CmdDInv, EncodeInv(ids))
		}
	})
}

func (e *Extension) broadcastAsset(host extensions.Host, id string) {
	if host == nil || id == "" {
		return
	}
	oh, ok := host.(extensions.OverlayHost)
	if !ok {
		return
	}
	_ = oh.BroadcastOverlay(ProtocolID, CmdDInv, EncodeInv([]string{id}), "")
}

func protocolSupported(protocols []string) bool {
	for _, p := range protocols {
		if p == ProtocolID {
			return true
		}
	}
	return false
}

func cleanPeerAddr(a string) string {
	return strings.TrimSpace(a)
}
