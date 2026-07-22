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
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/consensus"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const maxInvTxFetchPerMessage = 8

// TxInvMempoolCtx groups dependencies for admitting txs fetched via inv/getdata.
type TxInvMempoolCtx struct {
	Network                chain.Network
	AllowUnverifiedMempool bool
	FullRBF                bool
	Standard               consensus.StandardPolicy
	MempoolLimits          consensus.MempoolRelayLimits
}

// HandleInvTxFetch requests unknown transactions advertised by inv (Core inv→getdata→tx).
func HandleInvTxFetch(ctx context.Context, w *MsgWriter, p chain.Params, pool *mempool.Pool, orphans *mempool.OrphanPool, txIx *store.TxIndex, raw *store.RawBlockStore, j consensus.HeaderChain, peerFeeFilter uint64, mc TxInvMempoolCtx, peerAddr string, mb *MisbehaviorTracker, pm *PeerMgr, invPayload []byte) {
	if w == nil || pool == nil || mc.AllowUnverifiedMempool {
		return
	}
	entries, err := wire.DecodeInvPayload(invPayload)
	if err != nil {
		return
	}
	fetched := 0
	for _, e := range entries {
		if fetched >= maxInvTxFetchPerMessage {
			return
		}
		if e.Type == wire.InvTypeWitnessTx {
			if mb != nil && peerAddr != "" {
				mb.Note(peerAddr, misbehaviorWitnessTx, "witness tx inv")
			}
			applog.Line("mempool", fmt.Sprintf("inv witness tx ignored from %s", peerAddr))
			continue
		}
		if e.Type != wire.InvTypeTx {
			continue
		}
		id := pow.BlockHashHex(e.Hash[:])
		if pool.ContainsTxID(id) {
			continue
		}
		rawTx, err := fetchTxViaGetData(ctx, w, p, e.Hash, e.Type)
		if err != nil {
			if !errors.Is(err, context.Canceled) {
				applog.Line("mempool", fmt.Sprintf("inv tx fetch %s: %v", id[:16], err))
			}
			continue
		}
		fetched++
		if err := AdmitInboundTx(rawTx, peerAddr, w, mb, pool, orphans, txIx, raw, j, mc.Network, peerFeeFilter, mc.FullRBF, mc.Standard, mc.MempoolLimits); err != nil {
			if errors.Is(err, ErrWitnessTxRejected) {
				continue
			}
			if errors.Is(err, consensus.ErrOrphanTx) {
				applog.Line("mempool", fmt.Sprintf("inv tx orphan %s (pending parents)", id[:16]))
			} else {
				HandleInboundTxAdmissionFailure(rawTx, peerAddr, w, mb, err)
			}
			continue
		}
		applog.Line("mempool", fmt.Sprintf("inv tx accepted %s (%d bytes)", id[:16], len(rawTx)))
		if pm != nil {
			pm.NotePeerTx(peerAddr)
		}
	}
}

func fetchTxViaGetData(ctx context.Context, w *MsgWriter, p chain.Params, want [32]byte, invType uint32) ([]byte, error) {
	pl, err := wire.EncodeGetData([]wire.InvEntry{{Type: invType, Hash: want}})
	if err != nil {
		return nil, err
	}
	if err := w.Write("getdata", pl); err != nil {
		return nil, err
	}
	conn := w.Conn()
	for i := 0; i < 200; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(deadlineFromCtx(ctx, 45*time.Second))
		cmd, payload, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			return nil, err
		}
		switch cmd {
		case "ping":
			if err := w.Write("pong", payload); err != nil {
				return nil, err
			}
		case "tx":
			return payload, nil
		case "notfound":
			return nil, fmt.Errorf("notfound")
		case "reject":
			rj, err := wire.DecodeRejectPayload(payload)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("reject: %s", rj.String())
		}
	}
	return nil, fmt.Errorf("timeout waiting for tx")
}
