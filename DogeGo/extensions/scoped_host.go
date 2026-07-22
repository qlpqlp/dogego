// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/json"
	"fmt"
)

// scopedHost enforces manifest permissions for one active extension.
type scopedHost struct {
	inner *managerHost
	id    string
	man   Manifest
}

func (m *Manager) hostFor(id string, man Manifest) Host {
	if m == nil {
		return nil
	}
	var inner *managerHost
	if mh, ok := m.host.(*managerHost); ok && mh != nil {
		inner = mh
	} else {
		inner = &managerHost{m: m}
		m.host = inner
	}
	return &scopedHost{inner: inner, id: id, man: man}
}

func (h *scopedHost) require(perm string) error {
	if h == nil || h.inner == nil {
		return fmt.Errorf("extensions unwired")
	}
	if !h.man.HasPermission(perm) {
		return fmt.Errorf("extension %q lacks %q permission", h.id, perm)
	}
	return nil
}

func (h *scopedHost) Network() string {
	if h.inner == nil {
		return ""
	}
	return h.inner.Network()
}

func (h *scopedHost) TipHeight() (int64, error) {
	if err := h.require("chain_read"); err != nil {
		return -1, err
	}
	return h.inner.TipHeight()
}

func (h *scopedHost) GetRawBlockByHeight(height int64) ([]byte, error) {
	if err := h.require("chain_read"); err != nil {
		return nil, err
	}
	return h.inner.GetRawBlockByHeight(height)
}

func (h *scopedHost) LookupTxHex(txid string) (string, int64, bool) {
	if err := h.require("chain_read"); err != nil {
		return "", 0, false
	}
	return h.inner.LookupTxHex(txid)
}

func (h *scopedHost) BlockHashAtHeight(height int64) (string, error) {
	if err := h.require("chain_read"); err != nil {
		return "", err
	}
	return h.inner.BlockHashAtHeight(height)
}

func (h *scopedHost) ConfirmedTxInBlock(blockHash, txid string) (uint32, bool) {
	if err := h.require("chain_index"); err != nil {
		return 0, false
	}
	return h.inner.ConfirmedTxInBlock(blockHash, txid)
}

func (h *scopedHost) DataDir() string {
	if h.inner == nil {
		return ""
	}
	return h.inner.DataDir()
}

func (h *scopedHost) ExtensionDataDir(id string) (string, error) {
	if err := h.require("datadir_write"); err != nil {
		return "", err
	}
	if id != h.id {
		return "", fmt.Errorf("extension %q may only access its own data dir", h.id)
	}
	return h.inner.ExtensionDataDir(id)
}

func (h *scopedHost) Log(line string) {
	if h.inner != nil {
		h.inner.Log(line)
	}
}

func (h *scopedHost) BroadcastOverlay(protocolID, cmd string, payload []byte, excludePeer string) error {
	if err := h.require("p2p_extension"); err != nil {
		return err
	}
	return h.inner.BroadcastOverlay(protocolID, cmd, payload, excludePeer)
}

func (h *scopedHost) EachOverlayPeer(protocolID string, fn func(peer string, send func(string, []byte) error)) {
	if err := h.require("p2p_extension"); err != nil {
		return
	}
	h.inner.EachOverlayPeer(protocolID, fn)
}

func (h *scopedHost) OverlayPeerCount(protocolID string) int {
	if err := h.require("p2p_extension"); err != nil {
		return 0
	}
	return h.inner.OverlayPeerCount(protocolID)
}

func (h *scopedHost) CallWalletRPC(method string, params []json.RawMessage) (interface{}, error) {
	if err := h.require("wallet_rpc"); err != nil {
		return nil, err
	}
	if h.inner == nil || h.inner.m == nil {
		return nil, fmt.Errorf("extensions unwired")
	}
	return CallWalletRPC(h.inner.m.walletCaller, method, params)
}
