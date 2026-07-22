// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestProbeCoreAddrmanNoRPCSkipped(t *testing.T) {
	out := ProbeCoreAddrman(nil)
	if !out.Skipped || out.Reason != "rpc_not_ready" {
		t.Fatalf("out=%+v", out)
	}
}

func TestProbeCoreAddrmanP2PDisabledSkipped(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "getaddrmaninfo" {
			return map[string]interface{}{
				"error": map[string]interface{}{"code": float64(-9), "message": "P2P disabled"},
			}
		}
		return map[string]interface{}{"result": nil}
	}
	out := ProbeCoreAddrman(invoke)
	if !out.Skipped || out.Reason != "p2p_disabled" || !out.OK {
		t.Fatalf("out=%+v", out)
	}
}

func TestProbeCoreAddrmanOK(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		switch method {
		case "getaddrmaninfo":
			return map[string]interface{}{
				"result": map[string]interface{}{
					"all": map[string]interface{}{"total": 5, "new": 3, "tried": 2},
					"dogego_buckets": map[string]interface{}{
						"n_key_set":             true,
						"tried_buckets_total":   256,
						"new_buckets_total":     1024,
						"bucket_slot_cap":       64,
						"tried_buckets_used":    2,
						"new_buckets_used":      3,
						"tried_bucket_max_fill": 1,
						"new_bucket_max_fill":   2,
					},
				},
			}
		case "getblockchaininfo":
			return map[string]interface{}{
				"result": map[string]interface{}{
					"dogego_addrbook_tried":     2,
					"dogego_addrbook_new":       3,
					"dogego_addrbook_n_key_set": true,
				},
			}
		default:
			return map[string]interface{}{"result": nil}
		}
	}
	out := ProbeCoreAddrman(invoke)
	if !out.OK {
		t.Fatalf("out=%+v", out)
	}
	if out.Tried == nil || *out.Tried != 2 {
		t.Fatalf("tried=%v", out.Tried)
	}
	if !out.BucketSchemaOK || !out.CountsMatchChainInfo {
		t.Fatalf("schema=%v match=%v", out.BucketSchemaOK, out.CountsMatchChainInfo)
	}
	if !strings.Contains(strings.Join(out.Notes, " "), "addrman_n_key_persisted") {
		t.Fatalf("notes=%v", out.Notes)
	}
}

func TestProbeCoreAddrmanBucketSchemaMismatch(t *testing.T) {
	invoke := func(method string, _ []json.RawMessage) map[string]interface{} {
		if method == "getaddrmaninfo" {
			return map[string]interface{}{
				"result": map[string]interface{}{
					"all": map[string]interface{}{"total": 1, "new": 1, "tried": 0},
					"dogego_buckets": map[string]interface{}{
						"tried_buckets_total": 128,
						"new_buckets_total":   512,
						"bucket_slot_cap":     32,
					},
				},
			}
		}
		return map[string]interface{}{"result": map[string]interface{}{}}
	}
	out := ProbeCoreAddrman(invoke)
	if out.OK || out.BucketSchemaOK {
		t.Fatalf("out=%+v", out)
	}
}
