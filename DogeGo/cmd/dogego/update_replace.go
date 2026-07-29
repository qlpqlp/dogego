// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const replaceTargetFlag = "-replacetarget"

func parseReplaceTargetArg(args []string) string {
	for _, arg := range args {
		if strings.HasPrefix(arg, replaceTargetFlag+"=") {
			return strings.TrimPrefix(arg, replaceTargetFlag+"=")
		}
	}
	return ""
}

func maybeReplaceInstallBinary(args []string) {
	target := parseReplaceTargetArg(args)
	if target == "" {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "DogeGo: replace target skipped: %v\n", err)
		return
	}
	if err := replaceExecutable(exe, target); err != nil {
		fmt.Fprintf(os.Stderr, "DogeGo: could not replace install binary at %s: %v\n", target, err)
		return
	}
	fmt.Fprintf(os.Stderr, "DogeGo: install binary updated at %s\n", target)
}

func replaceExecutable(src, dst string) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}
	if strings.EqualFold(srcAbs, dstAbs) {
		return nil
	}
	var lastErr error
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 400 * time.Millisecond)
		}
		lastErr = replaceExecutableOnce(srcAbs, dstAbs)
		if lastErr == nil {
			return nil
		}
	}
	return lastErr
}

func replaceExecutableOnce(srcAbs, dstAbs string) error {
	backup := dstAbs + ".bak"
	_ = os.Remove(backup)
	if _, err := os.Stat(dstAbs); err == nil {
		if err := os.Rename(dstAbs, backup); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	if err := copyFileAtomic(srcAbs, dstAbs); err != nil {
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, dstAbs)
		}
		return err
	}
	_ = os.Remove(backup)
	return nil
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".new"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
