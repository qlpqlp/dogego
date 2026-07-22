// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"context"
	"fmt"

	"dogego/applog"
	"dogego/store"
	"dogego/wire"
)

// GetHeadersServeEnv holds the header chain served to peers.
type GetHeadersServeEnv struct {
	Journal *store.HeaderJournal
	Aux     *store.HeaderAuxJournal
}

// HandleInboundGetHeaders answers getheaders with a headers message (Core ProcessGetHeaders subset).
func HandleInboundGetHeaders(ctx context.Context, mw *MsgWriter, env GetHeadersServeEnv, payload []byte) error {
	if mw == nil || env.Journal == nil {
		return nil
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	req, err := wire.DecodeGetHeaders(payload)
	if err != nil {
		return err
	}
	_ = req.Version // accepted for wire compatibility; we serve our best chain regardless
	fork, err := store.FindLocatorForkHeight(env.Journal, req.Locator)
	if err != nil {
		return err
	}
	headers, err := store.HeadersAfterFork(env.Journal, env.Aux, fork, req.HashStop, store.MaxHeadersPerMessage)
	if err != nil {
		return err
	}
	if len(headers) == 0 {
		pl, err := wire.EncodeHeadersPayload(nil)
		if err != nil {
			return err
		}
		return mw.Write("headers", pl)
	}
	pl, err := wire.EncodeHeadersPayload(headers)
	if err != nil {
		return err
	}
	if err := mw.Write("headers", pl); err != nil {
		return err
	}
	applog.Line("headers", fmt.Sprintf("getheaders reply: %d header(s) after fork height %d", len(headers), fork))
	return nil
}
