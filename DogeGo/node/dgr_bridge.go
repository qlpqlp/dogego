// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"

	"dogego/applog"
	"dogego/chain"
	"dogego/clock"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/node/dgr"
	"dogego/store"
	"dogego/wire"
)

// DGRBridgeEnv holds node state for phase-4 bidirectional DGR relay.
type DGRBridgeEnv struct {
	Ctx     context.Context
	Params  chain.Params
	Network chain.Network

	MW          **MsgWriter
	PeerMgr     **PeerMgr
	Pool        *mempool.Pool
	Orphans     *mempool.OrphanPool
	TxIndex     *store.TxIndex
	RawBlocks   *store.RawBlockStore
	Journal     *store.HeaderJournal
	BlockStore  *BlockStoreCtx
	Aux         *store.HeaderAuxJournal
	Misbehavior *MisbehaviorTracker

	AllowUnverifiedMempool bool
	FullRBF                bool
	Standard               consensus.StandardPolicy
	MempoolLimits          consensus.MempoolRelayLimits
	PeerFeeFilters         *FeeFilterSet
	ConnectedAddr          *string

	mu sync.Mutex
}

// NewDGRBridge builds publish/push handlers for a DGR manager.
func NewDGRBridge(env *DGRBridgeEnv) *dgr.P2PBridge {
	if env == nil {
		return nil
	}
	return &dgr.P2PBridge{
		Publish: env.publishToNetwork,
		OnPush:  env.ingestFromRelay,
	}
}

func (e *DGRBridgeEnv) publishToNetwork(cmd string, payload []byte) error {
	if e == nil || cmd == "" || len(payload) == 0 {
		return nil
	}
	e.mu.Lock()
	mw := e.mw()
	pm := e.peerMgr()
	e.mu.Unlock()
	if pm != nil {
		pm.BroadcastCmd(cmd, payload, "")
	}
	if mw != nil {
		if err := mw.Write(cmd, payload); err != nil {
			return err
		}
	}
	applog.Line("dgr", "relayed client "+cmd+" to Dogecoin P2P ("+strconv.Itoa(len(payload))+" bytes)")
	return nil
}

func (e *DGRBridgeEnv) ingestFromRelay(cmd string, payload []byte) {
	if e == nil || cmd == "" || len(payload) == 0 {
		return
	}
	e.mu.Lock()
	ctx := e.Ctx
	p := e.Params
	mw := e.mw()
	pm := e.peerMgr()
	pool := e.Pool
	orphans := e.Orphans
	txIx := e.TxIndex
	rb := e.RawBlocks
	j := e.Journal
	bs := e.BlockStore
	mb := e.Misbehavior
	auxJ := e.Aux
	peerAddr := e.connectedAddr()
	e.mu.Unlock()

	switch cmd {
	case "inv":
		if bs == nil || !BodiesBehindHeaders(bs) {
			HandleInvBlockFetch(ctx, mw, p, bs, payload)
		}
		if !e.AllowUnverifiedMempool && pool != nil {
			HandleInvTxFetch(ctx, mw, p, pool, orphans, txIx, rb, j, e.maxPeerFeeFilter(), TxInvMempoolCtx{
				Network:                e.Network,
				AllowUnverifiedMempool: e.AllowUnverifiedMempool,
				FullRBF:                e.FullRBF,
				Standard:               e.Standard,
				MempoolLimits:          e.MempoolLimits,
			}, peerAddr, mb, pm, payload)
		}
	case "tx":
		if e.AllowUnverifiedMempool {
			if pool != nil {
				if err := pool.Add(payload); err != nil {
					applog.Line("mempool", "DGR tx not stored: "+err.Error())
				} else {
					applog.Line("mempool", "DGR tx accepted unverified")
				}
			}
			return
		}
		if err := AdmitInboundTx(payload, "dgr-relay", mw, mb, pool, orphans, txIx, rb, j, e.Network, e.maxPeerFeeFilter(), e.FullRBF, e.Standard, e.MempoolLimits); err != nil {
			if !errors.Is(err, ErrWitnessTxRejected) && !errors.Is(err, consensus.ErrOrphanTx) {
				applog.Line("mempool", "DGR tx rejected: "+err.Error())
			}
			return
		}
		applog.Line("mempool", "DGR tx accepted via relay")
	case "block":
		if bs != nil {
			HandleBroadcastBlock(mw, bs, "dgr-relay", mb, payload)
		}
	case "headers":
		if j == nil {
			return
		}
		if bs != nil && ShouldDeferInboundHeaders(bs) {
			return
		}
		nowUnix := clock.UnixNow()
		if bs != nil {
			nowUnix = bs.NetworkTimeUnix()
		}
		n, _, err := ApplyHeadersMessage(j, auxJ, p, payload, nowUnix, bs)
		if err != nil {
			return
		}
		if n > 0 {
			applog.Line("headers", fmt.Sprintf("DGR headers push: +%d header(s)", n))
		}
	default:
		applog.Line("dgr", "ignored relay push cmd "+cmd)
	}
}

func (e *DGRBridgeEnv) mw() *MsgWriter {
	if e.MW == nil || *e.MW == nil {
		return nil
	}
	return *e.MW
}

func (e *DGRBridgeEnv) peerMgr() *PeerMgr {
	if e.PeerMgr == nil || *e.PeerMgr == nil {
		return nil
	}
	return *e.PeerMgr
}

func (e *DGRBridgeEnv) connectedAddr() string {
	if e.ConnectedAddr == nil {
		return ""
	}
	return *e.ConnectedAddr
}

func (e *DGRBridgeEnv) maxPeerFeeFilter() uint64 {
	if e.PeerFeeFilters == nil {
		return 0
	}
	return e.PeerFeeFilters.Max()
}

// PublishViaDGR relays a P2P message through an active outbound DGR session.
func PublishViaDGR(mgr *dgr.Manager, cmd string, payload []byte) bool {
	if mgr == nil || !mgr.UsingRelay() || len(payload) == 0 {
		return false
	}
	return mgr.Publish(cmd, payload)
}

// PublishTxViaDGR advertises and relays a raw transaction through DGR when active.
func PublishTxViaDGR(mgr *dgr.Manager, raw []byte) bool {
	if mgr == nil || !mgr.UsingRelay() || len(raw) == 0 {
		return false
	}
	tx, err := wire.DeserializeTx(raw)
	if err != nil {
		return false
	}
	invBody, err := wire.EncodeInvPayload([]wire.InvEntry{{Type: wire.InvTypeTx, Hash: tx.TxHash()}})
	if err != nil {
		return false
	}
	ok := mgr.Publish("inv", invBody)
	if mgr.Publish("tx", raw) {
		ok = true
	}
	return ok
}

// FanInViaDGR pushes operator-received P2P traffic to registered CGNAT clients.
func FanInViaDGR(mgr *dgr.Manager, cmd string, payload []byte) {
	if mgr == nil || !mgr.InboundRelay() || len(payload) == 0 {
		return
	}
	mgr.FanIn(cmd, payload)
}
