// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const subprocessPIDFile = ".subprocess.pid"

func killStaleSubprocess(dataDir string) {
	killPIDFile(dataDir)
}

func killPIDFile(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	pidPath := filepath.Join(dir, subprocessPIDFile)
	raw, err := os.ReadFile(pidPath)
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || pid <= 0 {
		_ = os.Remove(pidPath)
		return
	}
	forceKillPID(pid)
	_ = os.Remove(pidPath)
}

func writeSubprocessPIDTo(dir string, pid int) {
	if strings.TrimSpace(dir) == "" || pid <= 0 {
		return
	}
	pidPath := filepath.Join(dir, subprocessPIDFile)
	_ = os.WriteFile(pidPath, []byte(strconv.Itoa(pid)), 0o644)
}

func clearSubprocessPID(dirs ...string) {
	for _, dir := range dirs {
		if strings.TrimSpace(dir) == "" {
			continue
		}
		_ = os.Remove(filepath.Join(dir, subprocessPIDFile))
	}
}

// forceKillPID terminates pid (and on Windows, its process tree).
func forceKillPID(pid int) {
	if pid <= 0 {
		return
	}
	if runtime.GOOS == "windows" {
		// /T kills child tree; /F forces. Needed so the .exe image unlocks for uninstall.
		cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
		_ = cmd.Run()
		// Brief pause for the kernel to release the executable mapping.
		time.Sleep(50 * time.Millisecond)
	}
	if proc, err := os.FindProcess(pid); err == nil && proc != nil {
		_ = proc.Kill()
		done := make(chan struct{})
		go func() {
			_, _ = proc.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
		}
	}
}

// forceKillExtensionBinary stops any still-running copy of the extension exe under extDir.
// Used on disable/uninstall so Windows can delete the locked .exe.
func forceKillExtensionBinary(extDir, binaryName string) {
	extDir = strings.TrimSpace(extDir)
	if extDir == "" {
		return
	}
	killPIDFile(extDir)
	killPIDFile(filepath.Join(extDir, "data"))

	bin := strings.TrimSpace(binaryName)
	if bin == "" {
		return
	}
	path, err := resolveSubprocessBinary(extDir, bin)
	if err != nil {
		path = filepath.Join(extDir, bin)
		if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(path), ".exe") {
			path += ".exe"
		}
	}
	base := filepath.Base(path)
	if runtime.GOOS == "windows" && base != "" && base != "." && base != string(os.PathSeparator) {
		// Image-name kill as last resort when PID file was cleared but process survived.
		_ = exec.Command("taskkill", "/F", "/IM", base).Run()
		time.Sleep(50 * time.Millisecond)
	}
}

// removePathRetry deletes path, retrying on Windows sharing violations after kills.
func removePathRetry(path string) error {
	var last error
	for i := 0; i < 8; i++ {
		last = os.RemoveAll(path)
		if last == nil {
			return nil
		}
		if runtime.GOOS == "windows" {
			time.Sleep(time.Duration(50*(i+1)) * time.Millisecond)
			continue
		}
		return last
	}
	return last
}
