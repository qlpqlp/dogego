// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"fmt"
	"os"
	"strings"

	"dogego/autostart"
	"dogego/config"
)

// autostartApplyWarning returns a user-facing warning when login autostart could not be registered.
// Config is already saved; callers should still return HTTP 200 with autostart_warning set.
func autostartApplyWarning(syncWarn string, syncErr error) string {
	if syncErr != nil {
		return fmt.Sprintf("Login autostart could not be registered: %s", strings.TrimSpace(syncErr.Error()))
	}
	return strings.TrimSpace(syncWarn)
}

func applyAutostart(f config.File, confPath string) string {
	warn, err := syncAutostart(f, confPath)
	return autostartApplyWarning(warn, err)
}

func syncAutostart(f config.File, confPath string) (warning string, err error) {
	st := autostart.Current()
	if !st.Supported {
		if f.AutostartOnLogin() {
			return "Autostart is not supported on this operating system.", nil
		}
		return "", nil
	}
	sub := "node"
	if f.NodeMode == "spv" {
		sub = "spvnode"
	}
	opts := autostart.Options{
		ConfPath:   confPath,
		DataDir:    f.DataDir,
		Subcommand: sub,
		Tray:       f.TrayEnabled(os.Getenv("DOGEGO_HEADLESS") != "1"),
	}
	if exe, e := os.Executable(); e == nil {
		opts.ExePath = exe
	}
	if f.AutostartOnLogin() {
		if err := autostart.Apply(opts); err != nil {
			return "", err
		}
		st = autostart.Current()
		return st.Detail, nil
	}
	if err := autostart.Remove(); err != nil {
		return "", err
	}
	return "", nil
}
