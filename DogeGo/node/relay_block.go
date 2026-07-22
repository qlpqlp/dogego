// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"

	"dogego/applog"
)

// RelayBlockToPeer sends a full P2P `block` message to the active outbound peer (mining submit / relay).
func RelayBlockToPeer(mw *MsgWriter, payload []byte) error {
	if mw == nil || len(payload) < 81 {
		return fmt.Errorf("relay block: missing writer or payload")
	}
	if err := mw.Write("block", payload); err != nil {
		return err
	}
	applog.Line("block", fmt.Sprintf("relayed block to peer (%d bytes)", len(payload)))
	return nil
}
