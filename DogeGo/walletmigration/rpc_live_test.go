// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLiveImportViaRPCUnencrypted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		switch req.Method {
		case "dogego_probewalletdat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"is_bdb": true, "encrypted": false, "key_count": 2,
					"can_import": true, "needs_passphrase": false,
				},
			})
		case "dogego_importwalletdat":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"keys_imported": 2,
					"via_native_bdb": true,
					"pool_count":     1,
					"pool_indices_replayed": false,
					"keypool_hint":          "Core keypool entries detected - spend keys import via ckey/key; native import replays matched HD receive pubkeys into hd_keypool (pool_indices_replayed); run keypoolrefill for additional receive keys; pool-only pubkeys without spend keys stay unmatched",
					"pool_unmatched_hint":   "1 Core pool pubkey(s) have no spend key in wallet.dat",
					"keypool_refill_size":   100,
				},
			})
		default:
			t.Fatalf("method %q", req.Method)
		}
	}))
	defer srv.Close()

	c := RPCClient{BaseURL: srv.URL}
	live, err := LiveImportViaRPC(c, "C:\\wallet.dat", "")
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "passed" || live.KeysImported != 2 {
		t.Fatalf("live=%#v", live)
	}
	if live.KeypoolHint == "" || !strings.Contains(live.KeypoolHint, "keypoolrefill") {
		t.Fatalf("keypool_hint=%q", live.KeypoolHint)
	}
	if live.PoolIndicesReplayed == nil || *live.PoolIndicesReplayed {
		t.Fatalf("pool_indices_replayed=%v", live.PoolIndicesReplayed)
	}
	if live.PoolUnmatchedHint == "" || !strings.Contains(live.PoolUnmatchedHint, "no spend key") {
		t.Fatalf("pool_unmatched_hint=%q", live.PoolUnmatchedHint)
	}
	if live.KeypoolRefillSize == nil || *live.KeypoolRefillSize != 100 {
		t.Fatalf("keypool_refill_size=%v", live.KeypoolRefillSize)
	}
}

func TestLiveProbeViaRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"is_bdb": true, "encrypted": false, "key_count": 1,
				"can_import": true, "needs_passphrase": false,
			},
		})
	}))
	defer srv.Close()

	live, err := LiveProbeViaRPC(RPCClient{BaseURL: srv.URL}, "w.dat")
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "probe_passed" {
		t.Fatalf("live=%#v", live)
	}
}

func TestLiveImportViaRPCEncryptedNeedsPassphrase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{
				"is_bdb": true, "encrypted": true, "encrypted_keys": 1,
				"needs_passphrase": true, "can_import": true,
			},
		})
	}))
	defer srv.Close()

	live, err := LiveImportViaRPC(RPCClient{BaseURL: srv.URL}, "w.dat", "")
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "skipped_needs_passphrase" {
		t.Fatalf("live=%#v", live)
	}
}

func TestLiveImportViaRPCEncryptedWithPassphrase(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Method == "dogego_probewalletdat" {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"result": map[string]any{
					"is_bdb": true, "encrypted": true, "encrypted_keys": 1,
					"needs_passphrase": true, "can_import": true,
				},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"result": map[string]any{"keys_imported": 1},
		})
	}))
	defer srv.Close()

	live, err := LiveImportViaRPC(RPCClient{BaseURL: srv.URL}, "w.dat", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if live.Status != "passed_encrypted" || live.KeysImported != 1 {
		t.Fatalf("live=%#v", live)
	}
}
