// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"encoding/hex"
	"fmt"
	"strings"
)

func (s *SubprocessExtension) hasP2P() bool {
	return s.manifest.HasPermission("p2p_extension")
}

func (s *SubprocessExtension) hasIndexer() bool {
	for _, c := range s.manifest.Capabilities {
		if c == "indexer" {
			return true
		}
	}
	return s.manifest.HasPermission("chain_index")
}

func (s *SubprocessExtension) hasPeerSync() bool {
	for _, c := range s.manifest.Capabilities {
		if c == "l2_sync" {
			return true
		}
	}
	return s.hasP2P()
}

// ProtocolID implements P2PExtension for subprocess extensions with p2p_extension.
func (s *SubprocessExtension) ProtocolID() string {
	if s.p2pMeta != nil && s.p2pMeta.ProtocolID != "" {
		return s.p2pMeta.ProtocolID
	}
	for _, c := range s.manifest.Capabilities {
		if strings.HasPrefix(c, "zkproof-") {
			return c
		}
	}
	return ""
}

type subprocessP2PMeta struct {
	ProtocolID string
	Commands   []string
}

// P2PCommands implements P2PExtension.
func (s *SubprocessExtension) P2PCommands() []string {
	if s.p2pMeta != nil && len(s.p2pMeta.Commands) > 0 {
		return append([]string(nil), s.p2pMeta.Commands...)
	}
	return nil
}

// HandleP2P forwards overlay messages to the subprocess binary.
func (s *SubprocessExtension) HandleP2P(cmd string, payload []byte, peer string, send func(string, []byte) error) error {
	s.mu.Lock()
	alive := s.alive
	bridge := s.bridge
	s.mu.Unlock()
	if !alive || bridge == nil {
		return nil
	}
	out, p2pSend, err := bridge.Call("dogego_p2p", []interface{}{
		cmd, hex.EncodeToString(payload), peer,
	})
	if err != nil {
		return err
	}
	_ = out
	return s.dispatchP2PSend(p2pSend, send)
}

func (s *SubprocessExtension) dispatchP2PSend(msgs []p2pOutboundMsg, send func(string, []byte) error) error {
	if len(msgs) == 0 {
		return nil
	}
	s.mu.Lock()
	host := s.host
	s.mu.Unlock()
	for _, m := range msgs {
		payload, err := hex.DecodeString(strings.TrimSpace(m.PayloadHex))
		if err != nil {
			continue
		}
		if strings.TrimSpace(m.Peer) != "" {
			if mh, ok := host.(*managerHost); ok {
				_ = mh.sendOverlayPeer(m.Peer, m.Cmd, payload)
			}
			continue
		}
		if m.ProtocolID != "" && host != nil {
			if oh, ok := host.(OverlayHost); ok {
				_ = oh.BroadcastOverlay(m.ProtocolID, m.Cmd, payload, "")
				continue
			}
		}
		if send != nil {
			if err := send(m.Cmd, payload); err != nil {
				return err
			}
		}
	}
	return nil
}

// OnBlockConnected implements BlockIndexExtension.
func (s *SubprocessExtension) OnBlockConnected(height int64, host Host) error {
	if !s.hasIndexer() {
		return nil
	}
	s.mu.Lock()
	alive := s.alive
	bridge := s.bridge
	s.mu.Unlock()
	if !alive || bridge == nil {
		return nil
	}
	_, _, err := bridge.Call("dogego_block_connected", []interface{}{height})
	return err
}

// OnBlockDisconnected implements BlockDisconnectExtension.
func (s *SubprocessExtension) OnBlockDisconnected(height int64, host Host) error {
	if !s.hasIndexer() {
		return nil
	}
	s.mu.Lock()
	alive := s.alive
	bridge := s.bridge
	s.mu.Unlock()
	if !alive || bridge == nil {
		return nil
	}
	_, _, err := bridge.Call("dogego_block_disconnected", []interface{}{height})
	return err
}

// OnPeerConnected implements PeerSyncExtension.
func (s *SubprocessExtension) OnPeerConnected(peerAddr string, protocols []string, send func(string, []byte) error) {
	s.mu.Lock()
	alive := s.alive
	bridge := s.bridge
	s.mu.Unlock()
	if !alive || bridge == nil {
		return
	}
	_, p2pSend, err := bridge.Call("dogego_peer_connected", []interface{}{peerAddr, protocols})
	if err != nil {
		return
	}
	_ = s.dispatchP2PSend(p2pSend, send)
}

func (s *SubprocessExtension) loadP2PMeta() error {
	s.mu.Lock()
	bridge := s.bridge
	s.mu.Unlock()
	if bridge == nil {
		return fmt.Errorf("subprocess not running")
	}
	raw, _, err := bridge.Call("dogego_p2p_meta", nil)
	if err != nil {
		return err
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid p2p meta")
	}
	meta := &subprocessP2PMeta{}
	if v, ok := m["protocol_id"].(string); ok {
		meta.ProtocolID = v
	}
	if arr, ok := m["commands"].([]interface{}); ok {
		for _, x := range arr {
			if c, ok := x.(string); ok {
				meta.Commands = append(meta.Commands, c)
			}
		}
	}
	s.p2pMeta = meta
	return nil
}

func (s *SubprocessExtension) manifestAdvertisesRPC(name string) bool {
	for _, rm := range s.manifest.AdvertisedRPCMethods() {
		if rm.Name == name {
			return true
		}
	}
	return false
}
