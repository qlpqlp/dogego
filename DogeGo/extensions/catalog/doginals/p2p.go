// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package doginals

import (
	"strings"
)

// HandleP2P processes doginals-v1 overlay commands.
func (e *Extension) HandleP2P(cmd string, payload []byte, _ string, send func(cmd string, payload []byte) error) error {
	st, err := e.storeOrErr()
	if err != nil {
		return err
	}
	switch cmd {
	case CmdDInv:
		// Peer announced ids; request missing.
		for _, id := range DecodeInv(payload) {
			id = strings.ToLower(strings.TrimSpace(id))
			if id == "" {
				continue
			}
			if _, ok, _ := st.GetAsset(id); ok {
				continue
			}
			if send != nil {
				_ = send(CmdGetAsset, []byte(id))
			}
		}
	case CmdGetAsset:
		id := strings.ToLower(strings.TrimSpace(string(payload)))
		a, ok, err := st.GetAsset(id)
		if err != nil || !ok {
			return nil
		}
		if send != nil {
			_ = send(CmdAsset, EncodeAssetWire(a))
		}
	case CmdAsset:
		a, err := DecodeAssetWire(payload)
		if err != nil {
			return nil
		}
		a, err = NormalizeAsset(a)
		if err != nil {
			return nil
		}
		_ = st.PutAsset(a)
	}
	return nil
}

// OnPeerConnected announces local L2 inventory.
func (e *Extension) OnPeerConnected(_ string, _ []string, send func(cmd string, payload []byte) error) {
	st, err := e.storeOrErr()
	if err != nil || send == nil {
		return
	}
	ids, err := st.ListAssetIDs(200)
	if err != nil || len(ids) == 0 {
		return
	}
	_ = send(CmdDInv, EncodeInv(ids))
}

// P2PCommands lists overlay commands for dogego_p2p_meta.
func P2PCommands() []string {
	return []string{CmdDInv, CmdGetAsset, CmdAsset}
}
