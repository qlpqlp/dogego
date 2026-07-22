//go:build windows

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

const createNoWindow = 0x08000000
const detachedProcess = 0x00000008

func spawnRestartChild(exePath string, args []string) error {
	cmd := exec.Command(exePath, args...)
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	if wd, err := os.Getwd(); err == nil && wd != "" {
		cmd.Dir = wd
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: createNoWindow | detachedProcess,
		HideWindow:    true,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func spawnReplacementNode(parentPID int) error {
	exePath, err := os.Executable()
	if err != nil {
		exePath = os.Args[0]
	}
	return spawnReplacementFrom(exePath, parentPID, "")
}

func spawnReplacementFrom(exePath string, parentPID int, replaceTarget string) error {
	args := buildApplyChildArgs(parentPID, replaceTarget)
	if err := spawnRestartChild(exePath, args); err != nil {
		return fmt.Errorf("spawn: %w", err)
	}
	return nil
}
