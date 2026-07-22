// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package launch

import (
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dogego/config"
)

func TestShutdownDualPeers_requestsSiblingShutdown(t *testing.T) {
	dir := t.TempDir()
	mainPath := filepath.Join(dir, config.DualMainnetConfName)
	testPath := filepath.Join(dir, config.DualTestnetConfName)

	var shutdowns atomic.Int32
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/control/shutdown" && r.Method == http.MethodPost {
			shutdowns.Add(1)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true}`))
			go func() {
				time.Sleep(50 * time.Millisecond)
				srv.Close()
			}()
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	webui := strings.TrimPrefix(srv.URL, "http://")

	inst := config.InstancesFile{
		Instances: []config.InstanceEntry{
			{Network: "mainnet", WebUI: config.DualMainnetWebUI, ConfPath: mainPath},
			{Network: "testnet", WebUI: webui, ConfPath: testPath},
		},
	}
	if err := config.SaveInstances(dir, inst); err != nil {
		t.Fatal(err)
	}

	ShutdownDualPeers(dir, "mainnet")
	if n := shutdowns.Load(); n < 1 {
		t.Fatalf("shutdown requests %d want >= 1", n)
	}
}

func TestPeerShutdownURL(t *testing.T) {
	got := peerShutdownURL("127.0.0.1:2014")
	want := "http://127.0.0.1:2014/api/control/shutdown"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestWaitPeersStopped(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	peers := []config.InstanceEntry{{Network: "testnet", WebUI: addr}}
	done := make(chan struct{})
	go func() {
		waitPeersStopped(peers, 3*time.Second)
		close(done)
	}()
	time.Sleep(200 * time.Millisecond)
	_ = ln.Close()
	select {
	case <-done:
	case <-time.After(4 * time.Second):
		t.Fatal("waitPeersStopped did not return after peer stopped")
	}
}
