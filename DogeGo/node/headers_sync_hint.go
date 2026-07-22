// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"os"
	"strings"

	"dogego/store"
)

// printHeaderSyncBadNBitsHint explains recoverable vs consensus failures after all header peers fail.
func printHeaderSyncBadNBitsHint(j *store.HeaderJournal, headersPath string, err error) {
	tip, tipErr := j.TipHeight()
	if tipErr != nil {
		tip = -1
	}
	fmt.Fprintf(os.Stderr, "DogeGo: local header tip height %d (%s).\n", tip, headersPath)
	fmt.Fprintf(os.Stderr, "  DogeGo auto-rewinds damaged header periods, prunes rawblocks/txindex above the rewind height, "+
		"and retries getheaders without exiting - no manual delete required in most cases.\n")
	fmt.Fprintf(os.Stderr, "  After a force-kill, DogeGo drops a torn partial header at the end of headers.bin on startup "+
		"and resumes from the last complete record.\n")
	fmt.Fprintf(os.Stderr, "  If recovery repeats at the same height, DogeGo steps back additional 240-block windows automatically.\n")
	fmt.Fprintf(os.Stderr, "  Last resort only: stop the node, delete %s (and headers_aux.bin / rawblocks/ for a full redownload), "+
		"then restart.\n", headersPath)
	fmt.Fprintf(os.Stderr, "  If the node is running with RPC: invalidateblock <hash> on the first bad height truncates "+
		"the chain and marks descendants invalid (same as Core).\n")
	if strings.Contains(err.Error(), "240-block") || strings.Contains(err.Error(), "retarget") {
		fmt.Fprintf(os.Stderr, "  Mainnet pre-Digishield retargets are still being aligned with Core; reboot testnet "+
			"(-network testnet) is the path DogeGo exercises most.\n")
	}
}
