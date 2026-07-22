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
	"strings"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/clock"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/rpc"
	"dogego/store"
	"dogego/wire"
)

// RelayEnv holds node services shared by relay peer read loops.
type RelayEnv struct {
	Network                chain.Network
	FullNode               bool
	AllowUnverifiedMempool bool
	FullRBF                bool
	Standard               consensus.StandardPolicy
	MempoolLimits          consensus.MempoolRelayLimits

	Journal     *store.HeaderJournal
	Aux         *store.HeaderAuxJournal
	ChainPolicy *store.ChainPolicy
	BlockStore  *BlockStoreCtx
	Pool        *mempool.Pool
	Orphans     *mempool.OrphanPool
	TxIndex   *store.TxIndex
	RawBlocks *store.RawBlockStore
	BlockFilters  *store.BlockFilterIndex
	PeerFeeFilter *FeeFilterSet
	TipWait     *rpc.TipWaiter
	RawFill     *progressiveRawState
	Misbehavior *MisbehaviorTracker
	DGRFanIn    func(cmd string, payload []byte)
}

func runRelayPeerSession(ctx context.Context, env RelayEnv, pm *PeerMgr, link *peerLink) {
	defer pm.removeSession(link.addr)
	p := pm.params
	mw := link.mw
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		readTO := 90 * time.Second
		link.ping.maybePing(mw)
		if env.FullNode && env.RawFill != nil && env.BlockStore != nil && env.RawFill.bodiesDownloadActive(env.BlockStore) {
			readTO = 4 * time.Second
			lane := env.RawFill.laneForAddr(link.addr)
			n, perr := MaybePumpLaneBodyIBDDownload(ctx, mw, p, env.BlockStore, env.RawFill, lane, pm.blockScorer, addrBookFromPeerMgr(pm), &link.lastBodyPump)
			if perr != nil {
				if errors.Is(perr, ErrBlockDownloadStall) || errors.Is(perr, ErrBlockDownloadTimeout) || sessionFailureHardFromFetchErr(perr) {
					applog.Line("block", "relay block disconnect: "+perr.Error())
					_ = pm.DisconnectPeer(link.addr)
					return
				}
			} else if n > 0 && pm.blockScorer != nil {
				pm.blockScorer.NoteBlocksDelivered(link.addr, n)
			}
		}
		_ = mw.Conn().SetReadDeadline(time.Now().Add(readTO))
		cmd, pl, err := wire.ReadMessage(mw.Conn(), p.Magic)
		if err != nil {
			if isNetTimeout(err) && env.FullNode && env.RawFill != nil && env.BlockStore != nil && env.RawFill.bodiesDownloadActive(env.BlockStore) {
				lane := env.RawFill.laneForAddr(link.addr)
				n, ferr := env.RawFill.tryFetchMissingBatches(ctx, mw, p, env.BlockStore, lane, IdleFetchBatchesPerRound(env.BlockStore), pm.blockScorer, addrBookFromPeerMgr(pm))
				if errors.Is(ferr, ErrBlockDownloadStall) || errors.Is(ferr, ErrBlockDownloadTimeout) {
					applog.Line("block", "relay block disconnect: "+ferr.Error())
					_ = pm.DisconnectPeer(link.addr)
					return
				}
				if ferr != nil {
					hard := sessionFailureHardFromFetchErr(ferr)
					penalizeBlockPeer(pm.blockScorer, addrBookFromPeerMgr(pm), link.addr, hard)
					if hard {
						_ = pm.DisconnectPeer(link.addr)
						return
					}
				} else if n > 0 && pm.blockScorer != nil {
					pm.blockScorer.NoteBlocksDelivered(link.addr, n)
				}
				continue
			}
			applog.Line("net", fmt.Sprintf("relay peer %s closed: %v", link.addr, err))
			if pm.blockScorer != nil {
				pm.blockScorer.NoteSessionFailure(link.addr, false)
			}
			return
		}
		pm.NotePeerRecv(link.addr)
		pm.NotePeerMsg(link.addr, cmd, len(pl))
		handleRelayP2PMessage(ctx, env, pm, link, p, mw, cmd, pl)
	}
}

func handleRelayP2PMessage(ctx context.Context, env RelayEnv, pm *PeerMgr, link *peerLink, p chain.Params, mw *MsgWriter, cmd string, pl []byte) {
	switch cmd {
	case "ping":
		_ = replyPing(mw, pl)
	case "pong":
		link.ping.notePong(pl)
	case "getaddr":
		addrs := pm.AddrSample(25)
		if len(addrs) == 0 {
			return
		}
		body, err := wire.EncodeAddrPayload(addrs)
		if err != nil {
			return
		}
		_ = mw.Write("addr", body)
	case "addr":
		nets, err := wire.DecodeAddrPayload(pl)
		if err != nil {
			return
		}
		pm.NoteAddrsFromPeer(link.addr, nets)
	case "feefilter":
		fee, err := wire.DecodeFeeFilterPayload(pl)
		if err == nil {
			pm.NotePeerFeeFilter(link.addr, fee)
		}
	case "sendcmpct":
		if announce, _, err := NegotiateSendCmpct(pm, link, mw, pl, nil); err != nil {
			applog.Line("net", fmt.Sprintf("relay %s sendcmpct: %v", link.addr, err))
		} else if announce {
			link.cmpctHBFrom = true
		}
	case "sendheaders":
		_ = mw.Write("sendheaders", nil)
	case "getcfilters":
		if err := HandleInboundGetCFilters(mw, env.Journal, env.RawBlocks, env.TxIndex, env.BlockFilters, pl); err != nil {
			applog.Line("net", fmt.Sprintf("relay %s getcfilters: %v", link.addr, err))
		}
	case "getcfheaders":
		if err := HandleInboundGetCFHeaders(mw, env.Journal, env.RawBlocks, env.TxIndex, env.BlockFilters, pl); err != nil {
			applog.Line("net", fmt.Sprintf("relay %s getcfheaders: %v", link.addr, err))
		}
	case "getcfcheckpt":
		if err := HandleInboundGetCFCheckpt(mw, env.Journal, env.RawBlocks, env.TxIndex, env.BlockFilters, pl); err != nil {
			applog.Line("net", fmt.Sprintf("relay %s getcfcheckpt: %v", link.addr, err))
		}
	case "cmpctblock":
		cmpctEnv := CmpctServeEnv{Raw: env.RawBlocks, Pool: env.Pool, Block: env.BlockStore}
		HandleInboundCmpctBlock(mw, cmpctEnv, link, pl)
	case "getblocktxn":
		if err := HandleInboundGetBlockTxn(mw, env.RawBlocks, pl); err != nil {
			applog.Line("net", fmt.Sprintf("relay %s getblocktxn: %v", link.addr, err))
		}
	case "blocktxn":
		cmpctEnv := CmpctServeEnv{Raw: env.RawBlocks, Pool: env.Pool, Block: env.BlockStore}
		HandleInboundBlockTxn(mw, cmpctEnv, link, pl)
	case "tx":
		if env.AllowUnverifiedMempool {
			if env.Pool != nil {
				_ = env.Pool.Add(pl)
			}
			break
		}
		if env.Pool == nil {
			break
		}
		feeHint := uint64(0)
		if env.PeerFeeFilter != nil {
			feeHint = env.PeerFeeFilter.Max()
		}
		if err := AdmitInboundTx(pl, link.addr, mw, env.Misbehavior, env.Pool, env.Orphans, env.TxIndex, env.RawBlocks, env.Journal, env.Network, feeHint, env.FullRBF, env.Standard, env.MempoolLimits); err == nil {
			pm.NotePeerTx(link.addr)
			pm.BroadcastTx(pl, link.addr, env.Pool, env.TxIndex, env.RawBlocks)
			fanInDGR(env, "tx", pl)
		} else if !errors.Is(err, ErrWitnessTxRejected) && !errors.Is(err, consensus.ErrOrphanTx) {
			HandleInboundTxAdmissionFailure(pl, link.addr, mw, env.Misbehavior, err)
		}
	case "getdata":
		serve := GetDataServeEnv{Raw: env.RawBlocks, Pool: env.Pool, TxIx: env.TxIndex}
		if serve.Raw != nil || serve.Pool != nil || serve.TxIx != nil {
			_ = HandleInboundGetData(ctx, mw, serve, pl)
		}
	case "getheaders":
		if env.Journal != nil {
			_ = HandleInboundGetHeaders(ctx, mw, GetHeadersServeEnv{Journal: env.Journal, Aux: env.Aux}, pl)
		}
	case "inv":
		if env.BlockStore != nil {
			HandleInvBlockFetch(ctx, mw, p, env.BlockStore, pl)
		}
		if !env.AllowUnverifiedMempool && env.Pool != nil {
			feeHint := uint64(0)
			if env.PeerFeeFilter != nil {
				feeHint = env.PeerFeeFilter.Max()
			}
			HandleInvTxFetch(ctx, mw, p, env.Pool, env.Orphans, env.TxIndex, env.RawBlocks, env.Journal,
				feeHint, TxInvMempoolCtx{
					Network: env.Network, AllowUnverifiedMempool: env.AllowUnverifiedMempool,
					FullRBF: env.FullRBF, Standard: env.Standard, MempoolLimits: env.MempoolLimits,
				}, link.addr, env.Misbehavior, pm, pl)
		}
		fanInDGR(env, "inv", pl)
	case "block":
		if env.BlockStore != nil {
			HandleBroadcastBlock(mw, env.BlockStore, link.addr, env.Misbehavior, pl)
			RelayStoredBlock(env.BlockStore, pl, link.addr)
			if pm.blockScorer != nil {
				pm.blockScorer.NoteBlocksDelivered(link.addr, 1)
			}
		}
		fanInDGR(env, "block", pl)
	case "headers":
		if env.Journal == nil {
			break
		}
		if env.BlockStore != nil && ShouldDeferInboundHeaders(env.BlockStore) {
			break
		}
		nowUnix := clock.UnixNow()
		if env.BlockStore != nil {
			nowUnix = env.BlockStore.NetworkTimeUnix()
		}
		n, _, err := ApplyHeadersMessage(env.Journal, env.Aux, p, pl, nowUnix, env.BlockStore)
		if err != nil {
			if strings.Contains(err.Error(), "fork deferred (marginal chain work") {
				break
			}
			_, _, misbehave := InboundHeadersErrorPolicy(err)
			if misbehave && env.Misbehavior != nil {
				env.Misbehavior.Note(link.addr, misbehaviorInvalidHeaders, "invalid headers: "+err.Error())
			} else if IsHeaderRewindRetryErr(err) {
				applog.Line("headers", fmt.Sprintf("relay %s: %s", link.addr, err.Error()))
			}
			break
		}
		if n > 0 {
			tip, _ := env.Journal.TipHeight()
			tipHash := ""
			if tip >= 0 && env.Journal != nil {
				if h80, err := env.Journal.ReadHeaderAt(tip); err == nil {
					tipHash = pow.BlockHashHex(h80)
				}
			}
			pm.NotePeerHeaders(link.addr, tip, tipHash)
			applog.Line("headers", fmt.Sprintf("relay %s: +%d header(s) (tip %d)", link.addr, n, tip))
			if pm.blockScorer != nil {
				pm.blockScorer.NoteHeadersDelivered(link.addr, n)
			}
			var utxo *store.UtxoCache
			if env.BlockStore != nil {
				utxo = env.BlockStore.Utxo
			}
			NotifyRPCTip(env.Journal, env.RawBlocks, utxo, env.TipWait)
			if env.RawFill != nil {
				env.RawFill.OnTipChanged(tip)
			}
		}
		fanInDGR(env, "headers", pl)
	case "reject":
		if rj, err := wire.DecodeRejectPayload(pl); err == nil {
			applog.Line("net", "relay "+link.addr+" reject: "+rj.String())
			if env.Misbehavior != nil {
				env.Misbehavior.Note(link.addr, misbehaviorReject, "reject: "+rj.String())
			}
		}
	default:
		if extMgr := pm.extensionManager(); extMgr != nil && handleExtensionP2P(extMgr, link.addr, cmd, pl, extensionSendFunc(mw, p.Magic)) {
			return
		}
	}
}

func fanInDGR(env RelayEnv, cmd string, pl []byte) {
	if env.DGRFanIn != nil && len(pl) > 0 {
		env.DGRFanIn(cmd, pl)
	}
}
