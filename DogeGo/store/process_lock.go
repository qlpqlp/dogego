// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ProcessLock holds an exclusive lock for one DogeGo node process per chain datadir (Core .lock analogue).
type ProcessLock struct {
	f *os.File
}

// AcquireProcessLock opens chainDataDir/.dogego-process.lock exclusively.
// Clears a stale lock left by a crashed process (pid in file no longer running).
func AcquireProcessLock(chainDataDir string) (*ProcessLock, error) {
	lock, err := tryAcquireProcessLock(chainDataDir)
	if err == nil {
		return lock, nil
	}
	if tryClearStaleProcessLock(chainDataDir) {
		return tryAcquireProcessLock(chainDataDir)
	}
	return nil, err
}

func tryAcquireProcessLock(chainDataDir string) (*ProcessLock, error) {
	if chainDataDir == "" {
		return nil, fmt.Errorf("empty chain data dir")
	}
	path := filepath.Join(chainDataDir, ".dogego-process.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := lockProcessFile(f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("another DogeGo process may already be using this datadir (%s): %w", chainDataDir, err)
	}
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "pid=%d\n", os.Getpid())
	return &ProcessLock{f: f}, nil
}

func tryClearStaleProcessLock(chainDataDir string) bool {
	path := filepath.Join(chainDataDir, ".dogego-process.lock")
	b, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	pid := parseLockPID(string(b))
	if pid > 0 && processPIDAlive(pid) {
		return false
	}
	_ = os.Remove(path)
	return true
}

func parseLockPID(text string) int {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "pid=") {
			if n, err := strconv.Atoi(strings.TrimPrefix(line, "pid=")); err == nil {
				return n
			}
		}
	}
	return 0
}

// Release unlocks and closes the lock file.
func (l *ProcessLock) Release() {
	if l == nil || l.f == nil {
		return
	}
	unlockProcessFile(l.f)
	_ = l.f.Close()
	l.f = nil
}
