// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

import (
	"fmt"
	"strings"

	"dogego/chain"
	"dogego/rpc"
)

// ValidateAndNormalize checks persisted node settings before saving to disk.
// It trims strings and defaults empty network to "testnet".
func ValidateAndNormalize(f *File) error {
	if f == nil {
		return fmt.Errorf("nil config")
	}
	f.DataDir = strings.TrimSpace(f.DataDir)
	f.Peer = strings.TrimSpace(f.Peer)
	f.Network = strings.TrimSpace(f.Network)
	f.RPCAddr = strings.TrimSpace(f.RPCAddr)
	f.WebUI = strings.TrimSpace(f.WebUI)
	f.UAComment = strings.TrimSpace(f.UAComment)
	f.UACommentTipAddress = strings.TrimSpace(f.UACommentTipAddress)
	if f.UACommentUseNodeTipEnabled() && f.NoWallet {
		return fmt.Errorf("uacomment_use_node_tip requires wallet (disable nowallet or use a manual tip address)")
	}
	if err := ValidateUACommentTip(f.UACommentTipAddress, f.Network); err != nil {
		return err
	}
	if f.Network == "" {
		f.Network = "testnet"
	}
	if _, err := chain.ParseNetwork(f.Network); err != nil {
		return err
	}
	if f.DataDir == "" {
		return fmt.Errorf("datadir is required")
	}
	if abs, err := ResolveDataDir(f.DataDir); err != nil {
		return err
	} else {
		f.DataDir = abs
	}
	if f.WebUI == "" {
		f.WebUI = DefaultWebUIListen
	}
	f.NodeMode = strings.ToLower(strings.TrimSpace(f.NodeMode))
	if f.NodeMode == "" {
		f.NodeMode = "full"
	}
	if f.NodeMode != "full" && f.NodeMode != "spv" {
		return fmt.Errorf("node_mode must be full or spv")
	}
	f.P2PConnectivity = strings.ToLower(strings.TrimSpace(f.P2PConnectivity))
	if f.P2PConnectivity != "" && f.P2PConnectivity != "classic" && f.P2PConnectivity != "cgnat" && f.P2PConnectivity != "both" {
		return fmt.Errorf("p2p_connectivity must be classic, cgnat, or both")
	}
	f.Firewall = strings.ToLower(strings.TrimSpace(f.Firewall))
	if f.Firewall != "" && f.Firewall != "auto" && f.Firewall != "always" && f.Firewall != "never" {
		return fmt.Errorf("firewall must be auto, always, or never")
	}
	f.Upnp = strings.ToLower(strings.TrimSpace(f.Upnp))
	if f.Upnp != "" && f.Upnp != "auto" && f.Upnp != "enable" && f.Upnp != "disable" {
		return fmt.Errorf("upnp must be auto, enable, or disable")
	}
	f.Autostart = strings.ToLower(strings.TrimSpace(f.Autostart))
	if f.Autostart != "" && f.Autostart != "login" && f.Autostart != "disable" {
		return fmt.Errorf("autostart must be login or disable")
	}
	if f.MaxOutbound < 0 || f.MaxOutbound > 32 {
		return fmt.Errorf("maxoutbound must be 0..32")
	}
	if f.MaxInbound < 0 || f.MaxInbound > 64 {
		return fmt.Errorf("maxinbound must be 0..64")
	}
	if f.BlockSyncWorkers < 0 || f.BlockSyncWorkers > 16 {
		return fmt.Errorf("block_sync_workers must be 0..16")
	}
	if f.MaxOrphanTx < 0 || f.MaxOrphanTx > 1000 {
		return fmt.Errorf("maxorphantx must be 0..1000")
	}
	if f.MaxMempoolMB < 0 || f.MaxMempoolMB > 10000 {
		return fmt.Errorf("maxmempool must be 0..10000 MB")
	}
	if f.DBCacheMB < 0 || f.DBCacheMB > MaxAutoDBCacheMB {
		return fmt.Errorf("dbcache must be 0..%d MB (0 = auto)", MaxAutoDBCacheMB)
	}
	if f.MempoolExpiryHours < 0 || f.MempoolExpiryHours > 8760 {
		return fmt.Errorf("mempoolexpiry must be 0..8760 hours")
	}
	if f.NodeMode == "spv" {
		f.RawBlockBackfill = -1
	}
	f.BlockStorageLayout = strings.ToLower(strings.TrimSpace(f.BlockStorageLayout))
	if f.BlockStorageLayout != "" && f.BlockStorageLayout != "perfile" && f.BlockStorageLayout != "bundled" {
		return fmt.Errorf("block_storage_layout must be perfile or bundled")
	}
	if len(f.RpcAllowIP) > 0 {
		if _, err := rpc.ParseRPCAllowList(f.RpcAllowIP); err != nil {
			return err
		}
	}
	if f.RpclimitPerMin < 0 || f.RpclimitPerMin > 10000 {
		return fmt.Errorf("rpclimit must be 0..10000 requests per minute")
	}
	if f.RpcAuthMaxFail < -1 || f.RpcAuthMaxFail > 1000 {
		return fmt.Errorf("rpcauthmaxfail must be -1..1000 (-1 disables)")
	}
	dgr := f.DogeGoRelayCGNAT
	if dgr.OutboundRelay || dgr.InboundRelay {
		f.DogeGoRelayCGNAT.Enabled = true
	}
	if dgr.RelayPort != 0 && (dgr.RelayPort < 1 || dgr.RelayPort > 65535) {
		return fmt.Errorf("dogego_relay_cgnat.relay_port must be 1..65535")
	}
	if dgr.MaxClients != 0 && (dgr.MaxClients < 1 || dgr.MaxClients > 4096) {
		return fmt.Errorf("dogego_relay_cgnat.max_clients must be 1..4096")
	}
	if dgr.MaxRelayConns != 0 && (dgr.MaxRelayConns < 1 || dgr.MaxRelayConns > 16) {
		return fmt.Errorf("dogego_relay_cgnat.max_relay_conns must be 1..16")
	}
	if dgr.MaxSessionFramesPerSec != 0 && (dgr.MaxSessionFramesPerSec < 1 || dgr.MaxSessionFramesPerSec > 1000) {
		return fmt.Errorf("dogego_relay_cgnat.max_session_frames_per_sec must be 1..1000")
	}
	if dgr.MaxP2PProxyPerSec != 0 && (dgr.MaxP2PProxyPerSec < 1 || dgr.MaxP2PProxyPerSec > 500) {
		return fmt.Errorf("dogego_relay_cgnat.max_p2p_proxy_per_sec must be 1..500")
	}
	if dgr.MaxRegisterPerMin != 0 && (dgr.MaxRegisterPerMin < 1 || dgr.MaxRegisterPerMin > 120) {
		return fmt.Errorf("dogego_relay_cgnat.max_register_per_min must be 1..120")
	}
	for i, pin := range dgr.RelayTLSPins {
		p := strings.ToLower(strings.TrimSpace(pin))
		if len(p) != 64 {
			return fmt.Errorf("dogego_relay_cgnat.relay_tls_pins[%d]: want 64-char SHA-256 hex", i)
		}
		for _, c := range p {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return fmt.Errorf("dogego_relay_cgnat.relay_tls_pins[%d]: invalid hex", i)
			}
		}
		f.DogeGoRelayCGNAT.RelayTLSPins[i] = p
	}
	switch {
	case f.RawBlockBackfill == 0, f.RawBlockBackfill == -1:
		// ok: default / genesis-only
	default:
		if f.RawBlockBackfill < -1 || f.RawBlockBackfill > MaxRawBlockBackfill {
			return fmt.Errorf("rawblock_backfill must be -1, 0, or 1..%d", MaxRawBlockBackfill)
		}
	}
	if f.LimitAncestorCount != 0 && (f.LimitAncestorCount < 1 || f.LimitAncestorCount > 1000) {
		return fmt.Errorf("limitancestorcount must be 1..1000 or omitted")
	}
	if f.LimitDescendantCount != 0 && (f.LimitDescendantCount < 1 || f.LimitDescendantCount > 1000) {
		return fmt.Errorf("limitdescendantcount must be 1..1000 or omitted")
	}
	if f.LimitAncestorSizeKB != 0 && (f.LimitAncestorSizeKB < 1 || f.LimitAncestorSizeKB > 10000) {
		return fmt.Errorf("limitancestorsize must be 1..10000 (kB) or omitted")
	}
	if f.LimitDescendantSizeKB != 0 && (f.LimitDescendantSizeKB < 1 || f.LimitDescendantSizeKB > 10000) {
		return fmt.Errorf("limitdescendantsize must be 1..10000 (kB) or omitted")
	}
	if f.DatacarrierSize != 0 && (f.DatacarrierSize < 1 || f.DatacarrierSize > 10000) {
		return fmt.Errorf("datacarriersize must be 1..10000 bytes or omitted")
	}
	for i, h := range f.DNSSeeds {
		h = strings.TrimSpace(h)
		if h == "" {
			return fmt.Errorf("dnsseed[%d]: empty hostname", i)
		}
		if strings.ContainsAny(h, " \t/\\") {
			return fmt.Errorf("dnsseed[%d]: invalid hostname %q", i, h)
		}
		f.DNSSeeds[i] = h
	}
	f.CoreRPCAddr = strings.TrimSpace(f.CoreRPCAddr)
	f.CoreRPCUser = strings.TrimSpace(f.CoreRPCUser)
	f.CoreRPCPassword = strings.TrimSpace(f.CoreRPCPassword)
	EnsureDGRForP2P(f)
	return nil
}
