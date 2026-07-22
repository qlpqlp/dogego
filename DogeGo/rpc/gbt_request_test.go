// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "testing"

func TestGbtLegacyMaxVersion(t *testing.T) {
	req := map[string]interface{}{"maxversion": float64(3)}
	v, ok := gbtLegacyMaxVersion(req, nil)
	if !ok || v != 3 {
		t.Fatalf("v=%d ok=%v", v, ok)
	}
	_, ok = gbtLegacyMaxVersion(req, map[string]struct{}{"csv": {}})
	if ok {
		t.Fatal("rules present should ignore maxversion")
	}
}

func TestGbtLegacyVersionForce(t *testing.T) {
	if !gbtLegacyVersionForce(map[string]interface{}{"maxversion": float64(2)}, nil) {
		t.Fatal("expected force")
	}
	if gbtLegacyVersionForce(map[string]interface{}{"maxversion": float64(1)}, nil) {
		t.Fatal("maxversion 1")
	}
}
