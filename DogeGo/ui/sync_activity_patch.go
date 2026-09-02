// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import "fmt"

// patchSyncActivityForHeaderTip fixes dashboard sync_activity text when the P2P snapshot
// was built with a stale in-memory header cache (tip is already synced from disk in summary).
func patchSyncActivityForHeaderTip(act map[string]any, tip, peerStart, contiguous, chainActive, blocksBehind int64, blocksPerMin float64, lowestMissing int64, inFlight int, bodyIBDHeaderPaused bool) {
	if act == nil || tip < 0 {
		return
	}
	headline := "Downloading block bodies"
	if lowestMissing >= 0 {
		headline = fmt.Sprintf("Downloading block bodies from height %d", lowestMissing)
	} else if contiguous >= 0 && tip > contiguous {
		headline = fmt.Sprintf("Downloading block bodies from height %d", contiguous+1)
	}
	act["headline"] = headline

	var detailParts []string
	if tip > 0 {
		detailParts = append(detailParts, fmt.Sprintf("headers through %d", tip))
	}
	if chainActive >= 0 {
		detailParts = append(detailParts, fmt.Sprintf("connected through %d", chainActive))
	} else if contiguous >= 0 {
		detailParts = append(detailParts, fmt.Sprintf("stored through %d", contiguous))
	}
	if blocksPerMin > 0 {
		detailParts = append(detailParts, fmt.Sprintf("%.1f blocks/min", blocksPerMin))
	}
	if inFlight > 0 {
		detailParts = append(detailParts, fmt.Sprintf("%d batch(es) in flight", inFlight))
	}
	if !bodyIBDHeaderPaused && peerStart > tip+32 {
		act["headline"] = fmt.Sprintf("Catching up headers (%d / ~%d)", tip, peerStart)
	}
	detail := ""
	if len(detailParts) > 0 {
		detail = stringsJoinParts(detailParts, " · ")
	}
	if blocksBehind > 0 && tip > chainActive && chainActive >= 0 {
		if detail != "" {
			detail += " · "
		}
		detail += fmt.Sprintf("%d header(s) ahead of connected bodies", blocksBehind)
	}
	if detail != "" {
		act["detail"] = detail
	}
	act["last_progress_message"] = fmt.Sprintf("Validated headers (local tip height %d)", tip)
}

func stringsJoinParts(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	s := parts[0]
	for i := 1; i < len(parts); i++ {
		s += sep + parts[i]
	}
	return s
}
