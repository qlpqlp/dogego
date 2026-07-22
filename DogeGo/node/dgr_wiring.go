// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package node

import (
	"context"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/config"
	"dogego/node/dgr"
)

// dgrNoInboundGrace is how long a listening node may wait with zero inbound peers
// before autonomously starting the outbound DGR client and using learned operators.
const dgrNoInboundGrace = 10 * time.Minute

// bootDGR starts DGR when enabled and configures global P2P tunnel dialing.
func bootDGR(
	ctx context.Context,
	cfg Config,
	p2pMode string,
	netSlug string,
	p2pPort int,
	p chain.Params,
	addedNodes *AddedNodeStore,
	dgrAdvertiseP2P *string,
	peerSource dgr.P2PPeerSource,
	peerMgr **PeerMgr,
	chainRoot string,
	confFile *config.File,
) *dgr.Manager {
	if !cfg.DogeGoRelayCGNAT.Enabled {
		return nil
	}
	hooks := &dgr.P2PHooks{Magic: p.Magic}
	hooks.LearnedDir = chainRoot
	hooks.OnSeedsMerge = func(seeds []string) {
		if confFile == nil || cfg.ConfSavePath == "" {
			return
		}
		if err := PersistDGRRelaySeeds(cfg.ConfSavePath, confFile, seeds); err != nil {
			applog.Line("dgr", "persist relay_seeds: "+err.Error())
			return
		}
		applog.Line("dgr", "updated Public relay addresses (relay_seeds) from DGR operators")
	}
	hooks.OnPeerHints = func(hints []string) {
		for _, h := range hints {
			addr, err := NormalizeNodeAddr(h, p2pPort)
			if err != nil {
				continue
			}
			addedNodes.Add(addr)
			if peerMgr != nil && *peerMgr != nil {
				(*peerMgr).NoteAddr(addr)
				(*peerMgr).SetPreferredPeers(addedNodes.List())
			}
			applog.Line("dgr", "peer hint addnode "+addr)
		}
	}
	hooks.PeerHints = func() []string {
		out := make([]string, 0, 4+len(addedNodes.List()))
		if dgrAdvertiseP2P != nil && *dgrAdvertiseP2P != "" {
			out = append(out, *dgrAdvertiseP2P)
		}
		out = append(out, addedNodes.List()...)
		return out
	}
	hooks.RelayBook = func() dgr.RelayAddrBook {
		return peerMgrRelayBook(peerMgr)
	}
	hooks.OnTunnelPush = DeliverTunnelPush
	if cfg.DogeGoRelayCGNAT.RoleOutbound(p2pMode) {
		for _, addr := range relaySeedHostP2PAddrs(cfg.DogeGoRelayCGNAT.RelaySeeds, p2pPort) {
			addedNodes.Add(addr)
			if peerMgr != nil && *peerMgr != nil {
				(*peerMgr).NoteAddr(addr)
				(*peerMgr).SetPreferredPeers(addedNodes.List())
			}
			applog.Line("dgr", "relay seed addnode "+addr)
		}
	}
	mgr, err := dgr.Start(ctx, cfg.DogeGoRelayCGNAT, p2pMode, netSlug, p2pPort, peerSource, hooks)
	if err != nil {
		applog.Line("dgr", "start: "+err.Error())
		return nil
	}
	preferFirst := cfg.DogeGoRelayCGNAT.RoleOutbound(p2pMode)
	ConfigureDGRTunnelDial(mgr.RelayP2PFrame, mgr.UsingRelay, preferFirst, p.Magic)
	if preferFirst {
		applog.Line("dgr", "outbound relay client: prefer DGR tunnel before TCP for outbound P2P dials")
	}
	go watchNoInboundAutoDGR(ctx, mgr, peerMgr, p2pMode, p.Magic)
	return mgr
}

// watchNoInboundAutoDGR starts the outbound DGR client when this node is listening
// but still has no inbound peers after a grace period (typical CGNAT / no port-forward).
func watchNoInboundAutoDGR(ctx context.Context, mgr *dgr.Manager, peerMgr **PeerMgr, p2pMode string, magic [4]byte) {
	if mgr == nil {
		return
	}
	if p2pMode == P2PModeCGNAT {
		return // outbound already expected
	}
	timer := time.NewTimer(dgrNoInboundGrace)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return
	case <-timer.C:
	}
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		if peerMgr != nil && *peerMgr != nil {
			total, inbound, outboundRelay := (*peerMgr).ConnectionBreakdown()
			outPeers := outboundRelay
			if outPeers < 1 {
				outPeers = total - inbound
			}
			if inbound == 0 && outPeers >= 1 {
				if mgr.EnsureOutboundClient() {
					ConfigureDGRTunnelDial(mgr.RelayP2PFrame, mgr.UsingRelay, true, magic)
					applog.Line("dgr", "no inbound after grace: outbound DGR client active; using learned operators with secure rotation")
					return
				}
			}
			if inbound > 0 {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
