// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package walletmigration

import "testing"

func TestLiveProbeOK(t *testing.T) {
	if !LiveProbeOK(&LiveImportResult{Status: "probe_passed"}, true, false) {
		t.Fatal("expected probe_passed ok")
	}
	if LiveProbeOK(&LiveImportResult{Status: "probe_needs_passphrase"}, true, false) {
		t.Fatal("expected encrypted probe fail without extract")
	}
	if !LiveProbeOK(&LiveImportResult{Status: "probe_needs_passphrase"}, true, true) {
		t.Fatal("expected encrypted probe ok with extract")
	}
}

func TestLiveImportOK(t *testing.T) {
	cases := []struct {
		name    string
		live    *LiveImportResult
		require bool
		want    bool
	}{
		{name: "not configured optional", want: true},
		{name: "not configured required", require: true, want: false},
		{name: "plain import", live: &LiveImportResult{Status: "passed"}, require: true, want: true},
		{name: "encrypted import", live: &LiveImportResult{Status: "passed_encrypted"}, require: true, want: true},
		{name: "needs passphrase optional", live: &LiveImportResult{Status: "skipped_needs_passphrase"}, want: true},
		{name: "needs passphrase required", live: &LiveImportResult{Status: "skipped_needs_passphrase"}, require: true, want: false},
		{name: "failed", live: &LiveImportResult{Status: "import_failed"}, want: false},
		{name: "missing", live: &LiveImportResult{Status: "required_missing"}, require: true, want: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := LiveImportOK(c.live, c.require); got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestRPCClientForHostPort(t *testing.T) {
	c := RPCClientForHostPort("127.0.0.1", 44556)
	if c.BaseURL != "http://127.0.0.1:44556" {
		t.Fatalf("base=%q", c.BaseURL)
	}
}

func TestWalletDatProbeOptional(t *testing.T) {
	if WalletDatProbeOptional(true, false) {
		t.Fatal("explicit path should not optional-skip")
	}
	if !WalletDatProbeOptional(false, false) {
		t.Fatal("auto-discovered optional should skip")
	}
	if WalletDatProbeOptional(false, true) {
		t.Fatal("required should not optional-skip")
	}
}
