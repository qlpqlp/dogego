// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"math/big"
	"net"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/p2p"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

const (
	forkElectionSyncTimeout = 3 * time.Second
	maxForkElectionPeers    = 2
)

// ChainElectionFunc runs a short multi-peer chain-work comparison before a header reorg truncate.
type ChainElectionFunc func(ctx context.Context, forkAt int64, forkPrev [32]byte, incoming []wire.DecodedHeader, incomingWork *big.Int) error

func headerPrevHashLE(h80 []byte) [32]byte {
	var p [32]byte
	copy(p[:], h80[4:36])
	return p
}

func headersExtendFork(decoded []wire.DecodedHeader, forkPrev [32]byte) bool {
	if len(decoded) == 0 {
		return false
	}
	return headerPrevHashLE(decoded[0].Header80) == forkPrev
}

// maxAlternateForkWork returns the highest chain work among peer header batches that extend forkPrev
// on a different fork than the incoming batch's first block.
func maxAlternateForkWork(peerBatches [][]wire.DecodedHeader, forkPrev, incomingFirstHash [32]byte) *big.Int {
	maxAlt := big.NewInt(0)
	for _, decoded := range peerBatches {
		if !headersExtendFork(decoded, forkPrev) {
			continue
		}
		peerFirst := pow.BlockHashLE(decoded[0].Header80)
		if peerFirst == incomingFirstHash {
			continue
		}
		w, err := incomingChainWork(decoded)
		if err != nil || w.Cmp(maxAlt) <= 0 {
			continue
		}
		maxAlt = w
	}
	return maxAlt
}

// rejectIncomingForkIfPeerWorkHigher implements Core ProcessHeaders best-chain work comparison
// against peer header batches already fetched for the fork ancestor.
func rejectIncomingForkIfPeerWorkHigher(peerBatches [][]wire.DecodedHeader, forkPrev, incomingFirstHash [32]byte, incomingWork *big.Int) error {
	if incomingWork == nil || incomingWork.Sign() <= 0 || len(peerBatches) == 0 {
		return nil
	}
	maxAlt := maxAlternateForkWork(peerBatches, forkPrev, incomingFirstHash)
	if maxAlt.Cmp(incomingWork) > 0 {
		return fmt.Errorf("headers: fork rejected (peer alternate chain work %s exceeds incoming %s)",
			maxAlt.String(), incomingWork.String())
	}
	return nil
}

// EnsureIncomingForkWins sync-probes relay peers before reorg; rejects when a peer advertises more
// chain work on an alternate fork past the same ancestor (Core ProcessHeaders best-chain check).
func (pm *PeerMgr) EnsureIncomingForkWins(ctx context.Context, j *store.HeaderJournal, p chain.Params, forkAt int64, forkPrev [32]byte, incoming []wire.DecodedHeader, incomingWork *big.Int) error {
	if pm == nil || j == nil || len(incoming) == 0 || incomingWork == nil {
		return nil
	}
	if incomingWork.Sign() <= 0 {
		return nil
	}
	incomingFirst := pow.BlockHashLE(incoming[0].Header80)
	payload, err := encodeForkProbeGetHeaders(j, forkAt, p)
	if err != nil {
		return nil
	}
	addrs := pm.RelayAddrsOrdered(maxForkElectionPeers)
	if len(addrs) == 0 {
		return nil
	}
	probeCtx, cancel := context.WithTimeout(ctx, forkElectionSyncTimeout)
	defer cancel()
	var batches [][]wire.DecodedHeader
	for _, addr := range addrs {
		decoded, err := pm.syncForkProbeHeaders(probeCtx, addr, p, payload, forkPrev)
		if err != nil {
			applog.Line("headers", fmt.Sprintf("fork election probe %s: %v", addr, err))
			continue
		}
		if len(decoded) == 0 {
			continue
		}
		batches = append(batches, decoded)
		applog.Line("headers", fmt.Sprintf("fork election %s: %d headers past fork", addr, len(decoded)))
	}
	if len(batches) == 0 {
		return nil
	}
	if err := rejectIncomingForkIfPeerWorkHigher(batches, forkPrev, incomingFirst, incomingWork); err != nil {
		return err
	}
	maxAlt := maxAlternateForkWork(batches, forkPrev, incomingFirst)
	applog.Line("headers", fmt.Sprintf("fork election: incoming chain work %s beats peer alternates (max %s)",
		incomingWork.String(), maxAlt.String()))
	return nil
}

// syncForkProbeHeaders dials addr, sends getheaders, and returns the first headers reply without mutating the journal.
func (pm *PeerMgr) syncForkProbeHeaders(ctx context.Context, addr string, p chain.Params, payload []byte, forkPrev [32]byte) ([]wire.DecodedHeader, error) {
	if pm == nil || len(payload) == 0 {
		return nil, fmt.Errorf("no probe payload")
	}
	book := addrBookFromPeerMgr(pm)
	RecordOutboundDialTry(book, addr)
	c, err := pm.dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		if p2p.ObserveDialError(addr, err) {
			applog.Line("net", "IPv6 dials disabled (network unreachable); preferring IPv4 peers")
		}
		RecordOutboundHandshakeResult(book, addr, err)
		return nil, err
	}
	defer c.Close()
	dv, err := Handshake(ctx, c, p, pm.userAgent, pm.advertisedServices())
	if err != nil {
		RecordOutboundHandshakeResult(book, addr, err)
		return nil, err
	}
	RecordOutboundHandshakeResult(book, addr, nil)
	mw := NewMsgWriter(c, p.Magic)
	_ = SendCmpctDeclineOnConnect(mw)
	_ = mw.Write("sendheaders", nil)
	if err := mw.Write("getheaders", payload); err != nil {
		return nil, err
	}
	deadline := time.Now().Add(forkElectionSyncTimeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		remain := time.Until(deadline)
		if remain <= 0 {
			break
		}
		_ = c.SetReadDeadline(time.Now().Add(remain))
		cmd, pl, err := wire.ReadMessage(c, p.Magic)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return nil, fmt.Errorf("timeout waiting for headers (peer height %d)", dv.StartHeight)
			}
			return nil, err
		}
		switch cmd {
		case "ping":
			_ = mw.Write("pong", pl)
		case "headers":
			decoded, err := wire.DecodeHeadersPayload(pl)
			if err != nil {
				return nil, err
			}
			if !headersExtendFork(decoded, forkPrev) {
				return nil, fmt.Errorf("headers do not extend fork ancestor")
			}
			return decoded, nil
		case "reject":
			rj, err := wire.DecodeRejectPayload(pl)
			if err != nil {
				return nil, err
			}
			return nil, fmt.Errorf("reject: %s", rj.String())
		}
	}
	return nil, fmt.Errorf("timeout waiting for headers")
}
