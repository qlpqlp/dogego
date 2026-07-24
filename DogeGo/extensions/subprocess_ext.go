// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	subprocessMaxLineBytes = 4 << 20
	subprocessRPCVersion  = 1
)

// SubprocessExtension runs entry.binary with line-delimited JSON-RPC (sandboxed; no wallet).
type SubprocessExtension struct {
	manifest Manifest
	extDir   string
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	bridge   *subprocessBridge
	host     Host
	dataDir  string
	p2pMeta  *subprocessP2PMeta
	alive    bool
}

// NewSubprocessExtension builds a subprocess-backed extension from an on-disk install dir.
func NewSubprocessExtension(dir string, man Manifest) (*SubprocessExtension, error) {
	if strings.TrimSpace(man.ID) == "" {
		return nil, fmt.Errorf("subprocess: empty id")
	}
	name := strings.TrimSpace(man.Entry.Binary)
	if name == "" {
		return nil, fmt.Errorf("entry.binary required")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return nil, fmt.Errorf("invalid binary name")
	}
	return &SubprocessExtension{manifest: man, extDir: dir}, nil
}

func resolveSubprocessBinary(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("entry.binary required")
	}
	if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("invalid binary name")
	}
	removeForeignSubprocessBinaries(dir, name)
	path, ok := findSubprocessBinaryPath(dir, name)
	if !ok || !hostNativeExecutable(path) {
		return "", fmt.Errorf("subprocess binary %q not found for %s/%s", name, runtime.GOOS, runtime.GOARCH)
	}
	clean, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(clean, absDir+string(filepath.Separator)) && clean != absDir {
		return "", fmt.Errorf("binary path escape")
	}
	return clean, nil
}

func (s *SubprocessExtension) Manifest() Manifest { return s.manifest }

func (s *SubprocessExtension) OnEnable(ctx context.Context, host Host) error {
	if host == nil {
		return fmt.Errorf("subprocess: host required")
	}
	s.mu.Lock()
	if s.alive {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	if err := materializePlatformBinary(s.extDir, s.manifest); err != nil {
		return err
	}
	if err := buildSubprocessIfNeeded(s.extDir, s.manifest); err != nil {
		return err
	}
	bin, err := resolveSubprocessBinary(s.extDir, s.manifest.Entry.Binary)
	if err != nil {
		return err
	}
	dataDir, err := host.ExtensionDataDir(s.manifest.ID)
	if err != nil {
		return err
	}
	killStaleSubprocess(dataDir)

	cmd := exec.Command(bin)
	cmd.Dir = s.extDir
	cmd.Env = subprocessEnv(host, s.manifest.ID, dataDir)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		err = fmt.Errorf("subprocess start: %w", err)
		if strings.Contains(err.Error(), "exec format error") {
			err = fmt.Errorf("%w (wrong OS/arch binary  -  uninstall and reinstall from catalog so the universal zip materializes %s)", err, CurrentPlatformKey())
		}
		return err
	}
	reader := bufio.NewReaderSize(stdout, 64*1024)
	bridge := newSubprocessBridge(stdin, reader, host, s.manifest)
	s.mu.Lock()
	s.cmd = cmd
	s.stdin = stdin
	s.reader = reader
	s.host = host
	s.dataDir = dataDir
	s.bridge = bridge
	s.alive = true
	writeSubprocessPIDTo(dataDir, cmd.Process.Pid)
	writeSubprocessPIDTo(s.extDir, cmd.Process.Pid)
	s.mu.Unlock()

	host.Log(fmt.Sprintf("subprocess %s started pid=%d", s.manifest.ID, cmd.Process.Pid))
	init := map[string]interface{}{
		"protocol": subprocessRPCVersion,
		"network":  host.Network(),
		"data_dir": dataDir,
	}
	_, _, err = bridge.Call("dogego_on_enable", []interface{}{init})
	if err != nil {
		s.mu.Lock()
		_ = s.stopLocked()
		s.mu.Unlock()
		return err
	}
	if s.hasP2P() {
		go func() {
			if err := s.loadP2PMeta(); err != nil {
				host.Log(fmt.Sprintf("subprocess %s p2p meta: %v", s.manifest.ID, err))
			}
		}()
	}
	return nil
}

func (s *SubprocessExtension) OnDisable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopLocked()
}

func (s *SubprocessExtension) stopLocked() error {
	dataDir := s.dataDir
	extDir := s.extDir
	binName := s.manifest.Entry.Binary
	pid := 0
	if s.cmd != nil && s.cmd.Process != nil {
		pid = s.cmd.Process.Pid
	}

	// Prefer a fast graceful disable; never block uninstall behind a long RPC timeout.
	if s.alive && s.bridge != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _, _ = s.bridge.Call("dogego_on_disable", nil)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
		}
		s.bridge.Close()
	}
	if s.stdin != nil {
		_ = s.stdin.Close()
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
		waitDone := make(chan struct{})
		go func() {
			_, _ = s.cmd.Process.Wait()
			close(waitDone)
		}()
		select {
		case <-waitDone:
		case <-time.After(2 * time.Second):
		}
	}
	if pid > 0 {
		forceKillPID(pid)
	}
	forceKillExtensionBinary(extDir, binName)
	killStaleSubprocess(dataDir)

	s.alive = false
	s.cmd = nil
	s.stdin = nil
	s.reader = nil
	s.bridge = nil
	s.host = nil
	s.p2pMeta = nil
	clearSubprocessPID(dataDir, extDir)
	s.dataDir = ""
	return nil
}

func (s *SubprocessExtension) HandleRPC(method string, params []json.RawMessage, host Host) (interface{}, error) {
	if !s.manifest.HasPermission("rpc_register") {
		return nil, fmt.Errorf("extension lacks rpc_register permission")
	}
	switch method {
	case "info":
		if s.manifestAdvertisesRPC("info") {
			break
		}
		s.mu.Lock()
		pid := 0
		if s.cmd != nil && s.cmd.Process != nil {
			pid = s.cmd.Process.Pid
		}
		alive := s.alive
		s.mu.Unlock()
		out := map[string]interface{}{
			"id":      s.manifest.ID,
			"runtime": "subprocess",
			"version": s.manifest.Version,
			"alive":   alive,
			"pid":     pid,
		}
		if s.manifest.HasPermission("ui_panel") {
			ui := map[string]interface{}{}
			if panel := s.subprocessUIPanelLocked(); panel != nil {
				for k, v := range panel {
					ui[k] = v
				}
			}
			tools := ToolsFromManifest(s.manifest)
			if len(tools) > 0 {
				ui["tools"] = tools
			}
			if len(ui) > 0 {
				out["ui"] = ui
			}
		}
		return out, nil
	case "chain_tip":
		if !s.manifest.HasPermission("chain_read") {
			return nil, fmt.Errorf("extension lacks chain_read permission")
		}
		if host == nil {
			return nil, fmt.Errorf("host unavailable")
		}
		tip, err := host.TipHeight()
		if err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"network":    host.Network(),
			"tip_height": tip,
		}, nil
	case "wallet_call":
		if !s.manifest.HasPermission("wallet_rpc") {
			return nil, fmt.Errorf("extension lacks wallet_rpc permission")
		}
		wh, ok := host.(WalletRPCHost)
		if !ok {
			return nil, fmt.Errorf("wallet rpc host unavailable")
		}
		if len(params) < 1 {
			return nil, fmt.Errorf("want [method, ...params]")
		}
		var method string
		if err := json.Unmarshal(params[0], &method); err != nil {
			return nil, err
		}
		return wh.CallWalletRPC(method, params[1:])
	}
	s.mu.Lock()
	bridge := s.bridge
	alive := s.alive
	s.mu.Unlock()
	if !alive || bridge == nil {
		return nil, fmt.Errorf("subprocess not running")
	}
	out, _, err := bridge.Call(method, rawParamsToIface(params))
	return out, err
}

func (s *SubprocessExtension) subprocessUIPanelLocked() map[string]interface{} {
	s.mu.Lock()
	bridge := s.bridge
	alive := s.alive
	s.mu.Unlock()
	if !alive || bridge == nil {
		return nil
	}
	raw, _, err := bridge.Call("ui_status", nil)
	if err != nil {
		return nil
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	ui, ok := m["ui"].(map[string]interface{})
	if !ok || len(ui) == 0 {
		return nil
	}
	return ui
}

func rawParamsToIface(params []json.RawMessage) []interface{} {
	if len(params) == 0 {
		return nil
	}
	out := make([]interface{}, 0, len(params))
	for _, p := range params {
		var v interface{}
		if json.Unmarshal(p, &v) == nil {
			out = append(out, v)
		}
	}
	return out
}

func readSubprocessLine(r *bufio.Reader) ([]byte, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("subprocess read: %w", err)
	}
	if len(line) > subprocessMaxLineBytes {
		return nil, fmt.Errorf("subprocess response too large")
	}
	return bytesTrimSpace(line), nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func subprocessEnv(host Host, id, dataDir string) []string {
	base := []string{
		"DOGEGO_EXT_ID=" + id,
		"DOGEGO_EXT_RPC=" + strconvItoa(subprocessRPCVersion),
	}
	if host != nil {
		base = append(base,
			"DOGEGO_NETWORK="+host.Network(),
			"DOGEGO_DATA_DIR="+dataDir,
		)
	}
	// Inherit PATH only; strip wallet-like vars from parent env.
	var out []string
	out = append(out, base...)
	for _, kv := range os.Environ() {
		upper := strings.ToUpper(kv)
		if strings.HasPrefix(upper, "DOGEGO_WALLET") ||
			strings.HasPrefix(upper, "WALLET") ||
			strings.Contains(upper, "PRIVATE_KEY") {
			continue
		}
		if strings.HasPrefix(upper, "PATH=") ||
			strings.HasPrefix(upper, "SYSTEMROOT=") ||
			strings.HasPrefix(upper, "TEMP=") ||
			strings.HasPrefix(upper, "TMP=") ||
			strings.HasPrefix(upper, "HOME=") ||
			strings.HasPrefix(upper, "USERPROFILE=") {
			out = append(out, kv)
		}
	}
	return out
}

func strconvItoa(n int) string {
	return fmt.Sprintf("%d", n)
}

// RPCMethods advertises subprocess methods from the manifest.
func (s *SubprocessExtension) RPCMethods() []RPCMethod {
	return s.manifest.AdvertisedRPCMethods()
}
