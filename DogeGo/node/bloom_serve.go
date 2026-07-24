// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
	"dogego/bloom"
)

// HandleFilterLoad installs a peer BIP37 bloom filter.
func HandleFilterLoad(pm *PeerMgr, peerAddr string, payload []byte, mb *MisbehaviorTracker) error {
	f, err := bloom.ParseFilterLoad(payload)
	if err != nil {
		if mb != nil && peerAddr != "" {
			mb.Note(peerAddr, 100, "bad-filterload")
		}
		return err
	}
	if pm != nil {
		pm.SetPeerBloom(peerAddr, f)
	}
	applog.Line("net", fmt.Sprintf("filterload from %s (%d payload bytes)", peerAddr, len(payload)))
	return nil
}

// HandleFilterAdd inserts data into the peer's existing bloom filter.
func HandleFilterAdd(pm *PeerMgr, peerAddr string, payload []byte, mb *MisbehaviorTracker) error {
	data, err := bloom.ParseFilterAdd(payload)
	if err != nil {
		if mb != nil && peerAddr != "" {
			mb.Note(peerAddr, 100, "bad-filteradd")
		}
		return err
	}
	if pm == nil {
		return nil
	}
	f := pm.PeerBloom(peerAddr)
	if f == nil {
		if mb != nil && peerAddr != "" {
			mb.Note(peerAddr, 100, "filteradd-no-filter")
		}
		return fmt.Errorf("filteradd without filterload")
	}
	f.Insert(data)
	return nil
}

// HandleFilterClear removes the peer's bloom filter.
func HandleFilterClear(pm *PeerMgr, peerAddr string) {
	if pm != nil {
		pm.ClearPeerBloom(peerAddr)
	}
}
