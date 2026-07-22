// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"
	"net"
	"time"

	"dogego/applog"
	"dogego/chain"
	"dogego/clock"
	"dogego/store"
	"dogego/wire"
)

const headerTopUpReadRounds = 48

// TopUpHeadersRound requests one getheaders batch on an existing primary link (steady state / after body IBD).
// Returns the number of headers appended, or 0 when the peer reports the chain is caught up.
func TopUpHeadersRound(ctx context.Context, w *MsgWriter, p chain.Params, j *store.HeaderJournal, aux *store.HeaderAuxJournal, bs *BlockStoreCtx, raw *progressiveRawState, feed *PeerDiscoveryFeed) (int, error) {
	if w == nil || j == nil {
		return 0, nil
	}
	if bs != nil && ShouldPauseHeaderCatchUpForBodyIBD(bs, 0) {
		return 0, nil
	}
	payload, err := encodeTopUpGetHeaders(j, p)
	if err != nil {
		return 0, err
	}
	if err := RequestHeadersTopUp(w, payload); err != nil {
		return 0, err
	}
	conn := w.Conn()
	total := 0
	for i := 0; i < headerTopUpReadRounds; i++ {
		select {
		case <-ctx.Done():
			return total, ctx.Err()
		default:
		}
		_ = conn.SetReadDeadline(time.Now().Add(12 * time.Second))
		cmd, pl, err := wire.ReadMessage(conn, p.Magic)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				return total, nil
			}
			return total, err
		}
		switch cmd {
		case "ping":
			_ = replyPing(w, pl)
		case "headers":
			nowUnix := clock.UnixNow()
			if bs != nil {
				nowUnix = bs.NetworkTimeUnix()
			}
			count, partial, err := ApplyHeadersMessage(j, aux, p, pl, nowUnix, bs)
			if err != nil {
				return total, err
			}
			if count == 0 {
				return total, nil
			}
			total += count
			tip, _ := j.TipHeight()
			applog.Line("headers", fmt.Sprintf("header top-up: +%d headers (tip %d)", count, tip))
			if raw != nil {
				raw.OnTipChanged(tip)
			}
			if bs != nil {
				bs.maybeBackfillAuxAfterHeaderAdvance()
			}
			if !partial {
				return total, nil
			}
		case "reject":
			rj, err := wire.DecodeRejectPayload(pl)
			if err != nil {
				return total, fmt.Errorf("reject: %w", err)
			}
			return total, fmt.Errorf("reject: %s", rj.String())
		case "addr":
			if feed != nil {
				feed.NoteFromAddrPayload(pl)
			}
		default:
			if !isBenignHeaderSyncNoise(cmd) && cmd != "feefilter" && cmd != "sendcmpct" && cmd != "inv" {
				// ignore chatter
			}
		}
	}
	return total, nil
}
