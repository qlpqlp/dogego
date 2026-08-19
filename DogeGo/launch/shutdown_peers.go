// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package launch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dogego/config"
)

// ShutdownDualPeers gracefully stops sibling instances before the coordinator exits.
// After a graceful wait, force-stops any peer that is still listening (dual mainnet+testnet).
func ShutdownDualPeers(dataDir, currentNetwork string) {
	dataDir = resolveAbsDataDir(dataDir)
	if !shouldManageSiblingSpawns(dataDir, currentNetwork) {
		return
	}
	inst, err := config.LoadInstances(dataDir)
	if err != nil || len(inst.Instances) < 2 {
		return
	}
	cur := strings.ToLower(strings.TrimSpace(currentNetwork))
	var peers []config.InstanceEntry
	for _, e := range inst.Instances {
		netSlug := strings.ToLower(strings.TrimSpace(e.Network))
		if netSlug == "" || netSlug == cur {
			continue
		}
		peers = append(peers, e)
	}
	if len(peers) == 0 {
		return
	}
	for _, e := range peers {
		requestPeerShutdown(e.WebUI, e.Network)
	}
	// Retry once â€” peers under IBD may take a moment to accept the control request.
	time.Sleep(500 * time.Millisecond)
	for _, e := range peers {
		if TCPListenOpen(e.WebUI, 300*time.Millisecond) {
			requestPeerShutdown(e.WebUI, e.Network)
		}
	}
	waitPeersStopped(peers, 8*time.Second)
	for _, e := range peers {
		if !TCPListenOpen(e.WebUI, 300*time.Millisecond) {
			continue
		}
		fmt.Fprintf(os.Stderr, "DogeGo: %s peer still up after graceful shutdown; forcing stop\n", e.Network)
		forceStopPeer(dataDir, e)
	}
	waitPeersStopped(peers, 3*time.Second)
}

func peerShutdownURL(webui string) string {
	webui = strings.TrimSpace(webui)
	if webui == "" {
		return ""
	}
	if !strings.HasPrefix(webui, "http://") && !strings.HasPrefix(webui, "https://") {
		webui = "http://" + webui
	}
	return strings.TrimSuffix(webui, "/") + "/api/control/shutdown"
}

func requestPeerShutdown(webui, network string) {
	if !TCPListenOpen(webui, 400*time.Millisecond) {
		return
	}
	url := peerShutdownURL(webui)
	if url == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "DogeGo: could not shut down %s peer (%s): %v\n", network, webui, err)
		return
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "DogeGo: %s peer shutdown HTTP %d (%s)\n", network, resp.StatusCode, webui)
	}
}

func waitPeersStopped(peers []config.InstanceEntry, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		anyUp := false
		for _, e := range peers {
			if TCPListenOpen(e.WebUI, 300*time.Millisecond) {
				anyUp = true
				break
			}
		}
		if !anyUp {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
}

func forceStopPeer(dataDir string, e config.InstanceEntry) {
	netSlug := strings.ToLower(strings.TrimSpace(e.Network))
	if netSlug == "" {
		return
	}
	chainDir := filepath.Join(dataDir, netSlug)
	pid := readProcessLockPID(chainDir)
	if pid <= 0 || pid == os.Getpid() {
		return
	}
	if err := terminatePID(pid); err != nil {
		fmt.Fprintf(os.Stderr, "DogeGo: force stop %s peer pid %d: %v\n", netSlug, pid, err)
		return
	}
	fmt.Fprintf(os.Stderr, "DogeGo: force-stopped %s peer (pid %d)\n", netSlug, pid)
}

func readProcessLockPID(chainDir string) int {
	b, err := os.ReadFile(filepath.Join(chainDir, ".dogego-process.lock"))
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid=") {
			n, _ := strconv.Atoi(strings.TrimPrefix(line, "pid="))
			return n
		}
	}
	return 0
}
