// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestMergeBlockFilterDiagnostics(t *testing.T) {
	res := map[string]interface{}{}
	mergeBlockFilterDiagnostics(res, 100, 200)
	if res["dogego_filter_index_through"] != int64(100) {
		t.Fatalf("through=%v", res["dogego_filter_index_through"])
	}
	if res["dogego_filter_index_lag"] != int64(100) {
		t.Fatalf("lag=%v", res["dogego_filter_index_lag"])
	}
	mergeBlockFilterDiagnostics(res, -1, 200)
	if _, ok := res["dogego_filter_index_through"]; ok && res["dogego_filter_index_through"] == int64(-1) {
		t.Fatal("negative through should not merge")
	}
}
