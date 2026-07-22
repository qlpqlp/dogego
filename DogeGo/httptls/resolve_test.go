// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostsForListenAddrs(t *testing.T) {
	hosts := HostsForListenAddrs("127.0.0.1:2013", "0.0.0.0:22555")
	seen := map[string]bool{}
	for _, h := range hosts {
		seen[h] = true
	}
	for _, want := range []string{"127.0.0.1", "localhost"} {
		if !seen[want] {
			t.Fatalf("missing host %q in %v", want, hosts)
		}
	}
	if seen["0.0.0.0"] {
		t.Fatalf("0.0.0.0 should not be a SAN: %v", hosts)
	}
}

func TestResolveLocalTLS(t *testing.T) {
	dir := t.TempDir()
	opts := LocalTLSOptions{
		BaseDataDir:   dir,
		WebUITLSLocal: true,
		RpcTLSLocal:   true,
		WebUIListen:   "127.0.0.1:2013",
		RPCListen:     "127.0.0.1:22555",
	}
	res, err := ResolveLocalTLS(opts)
	if err != nil {
		t.Fatal(err)
	}
	if !res.WebUI.Enabled() || !res.RPC.Enabled() {
		t.Fatalf("expected webui and rpc TLS pairs: %+v", res)
	}
	if res.Local == nil {
		t.Fatal("expected local material")
	}
	for _, p := range []string{res.WebUI.CertFile, res.WebUI.KeyFile, res.RPC.CertFile, res.RPC.KeyFile, res.Local.CACertPath} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing %s: %v", p, err)
		}
	}
	if filepath.Base(res.Local.Dir) != "tls" {
		t.Fatalf("unexpected tls dir: %s", res.Local.Dir)
	}
	if !res.Local.CAGenerated {
		t.Fatal("expected new CA on first resolve")
	}
	st := Status(opts, res.WebUI, res.RPC, res.Local)
	if st["webui_https"] != true || st["rpc_https"] != true {
		t.Fatalf("status: %+v", st)
	}
}
