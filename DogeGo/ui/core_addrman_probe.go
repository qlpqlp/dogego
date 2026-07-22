// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const (
	coreAddrmanTriedBuckets = 256
	coreAddrmanNewBuckets   = 1024
	coreAddrmanBucketSlotCap = 64
)

// CoreAddrmanProbeResult is returned by GET /api/core-addrman-probe.
type CoreAddrmanProbeResult struct {
	CheckedAt              string `json:"checked_at"`
	OK                     bool   `json:"ok"`
	Skipped                bool   `json:"skipped,omitempty"`
	Reason                 string `json:"reason,omitempty"`
	AddrmanInfo            any    `json:"addrman_info,omitempty"`
	Tried                  *int   `json:"tried,omitempty"`
	NewAddrs               *int   `json:"new,omitempty"`
	Total                  *int   `json:"total,omitempty"`
	NKeySet                *bool  `json:"n_key_set,omitempty"`
	TriedBucketsUsed       *int   `json:"tried_buckets_used,omitempty"`
	NewBucketsUsed         *int   `json:"new_buckets_used,omitempty"`
	TriedBucketMaxFill     *int   `json:"tried_bucket_max_fill,omitempty"`
	NewBucketMaxFill       *int   `json:"new_bucket_max_fill,omitempty"`
	BucketSlotCap          *int   `json:"bucket_slot_cap,omitempty"`
	ChainInfoTried         *int   `json:"chaininfo_tried,omitempty"`
	ChainInfoNew           *int   `json:"chaininfo_new,omitempty"`
	BucketSchemaOK         bool   `json:"bucket_schema_ok"`
	CountsMatchChainInfo   bool   `json:"counts_match_chaininfo,omitempty"`
	Issues                 []string `json:"issues,omitempty"`
	Notes                  []string `json:"notes,omitempty"`
	Hint                   string `json:"hint,omitempty"`
}

// ProbeCoreAddrman runs getaddrmaninfo and cross-checks dogego_addrbook_* on getblockchaininfo.
func ProbeCoreAddrman(invoke func(string, []json.RawMessage) map[string]interface{}) CoreAddrmanProbeResult {
	out := CoreAddrmanProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		Hint:      "Partial Core addrman: 256 tried + 1024 new hash buckets, 64-deep slot caps, Core-scale table capacity (16384/65536), nKey persistence (learned_addrs.json v3), TriedSlot/NewRefs slot indices + multi-ref new. Mirrors scripts/core_addrman_workflow.ps1.",
	}
	if invoke == nil {
		out.Skipped = true
		out.Reason = "rpc_not_ready"
		return out
	}
	infoRaw, err := invokeDogeGoRPCAny(invoke, "getaddrmaninfo", nil)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "p2p disabled") || strings.Contains(msg, "not enabled") {
			out.Skipped = true
			out.Reason = "p2p_disabled"
			out.OK = true
			out.Notes = append(out.Notes, "enable P2P (listen or outbound peers) for addrman probe")
			return out
		}
		out.Issues = append(out.Issues, "getaddrmaninfo_failed")
		out.Reason = err.Error()
		return out
	}
	info, ok := infoRaw.(map[string]interface{})
	if !ok {
		out.Issues = append(out.Issues, "getaddrmaninfo_bad_shape")
		return out
	}
	out.AddrmanInfo = info
	if all, ok := info["all"].(map[string]interface{}); ok {
		if n := probeJSONInt(all["tried"]); n >= 0 {
			out.Tried = &n
		}
		if n := probeJSONInt(all["new"]); n >= 0 {
			out.NewAddrs = &n
		}
		if n := probeJSONInt(all["total"]); n >= 0 {
			out.Total = &n
		}
	}
	if buckets, ok := info["dogego_buckets"].(map[string]interface{}); ok {
		if n := probeJSONInt(buckets["tried_buckets_used"]); n >= 0 {
			out.TriedBucketsUsed = &n
		}
		if n := probeJSONInt(buckets["new_buckets_used"]); n >= 0 {
			out.NewBucketsUsed = &n
		}
		if n := probeJSONInt(buckets["tried_bucket_max_fill"]); n >= 0 {
			out.TriedBucketMaxFill = &n
		}
		if n := probeJSONInt(buckets["new_bucket_max_fill"]); n >= 0 {
			out.NewBucketMaxFill = &n
		}
		if n := probeJSONInt(buckets["bucket_slot_cap"]); n >= 0 {
			out.BucketSlotCap = &n
		}
		if v, ok := buckets["n_key_set"].(bool); ok {
			out.NKeySet = &v
		}
		tbTotal := probeJSONInt(buckets["tried_buckets_total"])
		nbTotal := probeJSONInt(buckets["new_buckets_total"])
		slotCap := probeJSONInt(buckets["bucket_slot_cap"])
		out.BucketSchemaOK = tbTotal == coreAddrmanTriedBuckets &&
			nbTotal == coreAddrmanNewBuckets &&
			slotCap == coreAddrmanBucketSlotCap
		if !out.BucketSchemaOK {
			out.Issues = append(out.Issues, "bucket_schema_mismatch")
		}
	} else {
		out.Issues = append(out.Issues, "dogego_buckets_missing")
	}
	if chain, chainErr := invokeDogeGoRPC(invoke, "getblockchaininfo", nil); chainErr == nil && chain != nil {
		if n := probeJSONInt(chain["dogego_addrbook_tried"]); n >= 0 {
			out.ChainInfoTried = &n
		}
		if n := probeJSONInt(chain["dogego_addrbook_new"]); n >= 0 {
			out.ChainInfoNew = &n
		}
		if out.Tried != nil && out.ChainInfoTried != nil && out.NewAddrs != nil && out.ChainInfoNew != nil {
			out.CountsMatchChainInfo = *out.Tried == *out.ChainInfoTried && *out.NewAddrs == *out.ChainInfoNew
			if !out.CountsMatchChainInfo {
				out.Issues = append(out.Issues, "addrbook_counts_drift_vs_chaininfo")
			}
		}
		if v, ok := chain["dogego_addrbook_n_key_set"].(bool); ok && out.NKeySet == nil {
			out.NKeySet = &v
		}
	}
	if out.NKeySet != nil && *out.NKeySet {
		out.Notes = append(out.Notes, "addrman_n_key_persisted: bucket assignment uses learned_addrs.json n_key")
	} else {
		out.Notes = append(out.Notes, "addrman_n_key_pending: n_key set after first addrbook save")
	}
	if out.TriedBucketMaxFill != nil && *out.TriedBucketMaxFill > coreAddrmanBucketSlotCap {
		out.Issues = append(out.Issues, "tried_bucket_over_slot_cap")
	}
	if out.NewBucketMaxFill != nil && *out.NewBucketMaxFill > coreAddrmanBucketSlotCap {
		out.Issues = append(out.Issues, "new_bucket_over_slot_cap")
	}
	if out.Tried != nil && out.NewAddrs != nil {
		out.Notes = append(out.Notes, fmt.Sprintf("addrbook tried=%d new=%d", *out.Tried, *out.NewAddrs))
	}
	if out.TriedBucketsUsed != nil && out.NewBucketsUsed != nil {
		out.Notes = append(out.Notes, fmt.Sprintf("buckets used tried=%d new=%d max_fill tried=%d new=%d",
			*out.TriedBucketsUsed, *out.NewBucketsUsed,
			intOrZero(out.TriedBucketMaxFill), intOrZero(out.NewBucketMaxFill)))
	}
	out.Notes = append(out.Notes, "addrman_partial: Core-scale capacity + buckets + slot indices")
	out.OK = len(out.Issues) == 0 && out.BucketSchemaOK
	return out
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
