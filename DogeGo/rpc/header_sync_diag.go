// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"fmt"
	"strings"

	"dogego/consensus"
)

// HeaderSyncDiagnostics returns operator fields for header/body lag and stale tip time (Core IBD hints).
func HeaderSyncDiagnostics(j HeaderJournal, headers, blocks int64, paths *DataPaths) map[string]interface{} {
	if j == nil || headers < 0 {
		return nil
	}
	out := map[string]interface{}{}
	if headers > blocks && blocks >= 0 {
		gap := headers - blocks
		out["dogego_headers_ahead_of_chainactive"] = gap
		if gap > 512 {
			note := "headers ahead of connected block bodies; forward getdata fills from the lowest missing height (orphan raw files ahead of the frontier are normal until the gap closes)"
			if gap > 50_000 && headers > 0 {
				bodyPct := float64(blocks) / float64(headers) * 100
				if bodyPct > 100 {
					bodyPct = 100
				}
				// Deep body IBD may pause getheaders only after assumevalid height is on tip (mainnet 5.05M).
				avMin := consensus.DefaultAssumeValidHeight("mainnet")
				if avMin <= 0 {
					avMin = 500_000
				}
				if headers >= avMin {
					note = fmt.Sprintf("header tip at %d; downloading block bodies (chainActive %d, ~%.2f%% of header height). Header getheaders paused until bodies catch up - normal on mainnet IBD", headers, blocks, bodyPct)
					out["dogego_body_ibd_header_paused"] = true
				} else {
					note = fmt.Sprintf("header tip at %d; downloading block bodies (chainActive %d, ~%.2f%% of header height). Headers keep syncing toward assumevalid height %d so script-skip can unlock", headers, blocks, bodyPct, avMin)
				}
			}
			out["dogego_body_sync_note"] = note
		}
	}
	headerTipTime, err := headerBlockTime(j, headers)
	if err != nil {
		return out
	}
	out["dogego_header_tip_time"] = headerTipTime
	if blocks >= 0 {
		if t, err := headerBlockTime(j, blocks); err == nil {
			out["dogego_chainactive_tip_time"] = t
		}
	}
	maxTipAge, nowUnix := ibdTimeParams(paths)
	if nowUnix > 0 && headerTipTime > 0 {
		age := nowUnix - headerTipTime
		if age < 0 {
			age = 0
		}
		out["dogego_header_tip_age_sec"] = age
		catchUp := paths != nil && paths.HeaderCatchUpPending != nil && paths.HeaderCatchUpPending()
		if maxTipAge > 0 && age > maxTipAge && !catchUp {
			out["dogego_header_tip_stale"] = true
		}
	}
	if paths != nil && paths.HeaderSyncRecoveryHint != nil {
		if hint := strings.TrimSpace(paths.HeaderSyncRecoveryHint()); hint != "" {
			out["dogego_header_sync_recovery"] = hint
		}
	}
	const postAuxStallMin, postAuxStallMax int64 = 509_500, 510_500
	catchUp := paths != nil && paths.HeaderCatchUpPending != nil && paths.HeaderCatchUpPending()
	if catchUp && headers >= postAuxStallMin && headers <= postAuxStallMax {
		out["dogego_post_aux_era_header_stall"] = true
	}
	if _, has := out["dogego_header_sync_recovery"]; !has {
		if v, ok := out["dogego_header_tip_stale"].(bool); ok && v {
			catchUp := paths != nil && paths.HeaderCatchUpPending != nil && paths.HeaderCatchUpPending()
			if catchUp || (headers >= 0 && headers < 5_000_000) {
				out["dogego_header_sync_recovery"] = "Downloading historical headers; tip block time lags wall clock until the header chain reaches the network tip (normal during IBD, not a corrupt chain)"
			} else {
				out["dogego_header_sync_recovery"] = "header tip block time is older than -maxtipage; node stays in initialblockdownload until the chain tip is fresher"
			}
		}
	}
	return out
}
