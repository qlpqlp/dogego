//go:build !windows

// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

func spawnReplacementNode(parentPID int) error {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	return spawnReplacementFrom(exePath, parentPID, "")
}

func spawnReplacementFrom(exePath string, parentPID int, replaceTarget string) error {
	args := buildApplyChildArgs(parentPID, replaceTarget)
	cmd := exec.Command(exePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	return nil
}
