// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

// Package autostart registers DogeGo to run at OS user login (Windows Task Scheduler,
// Linux systemd user unit or XDG autostart, macOS LaunchAgent).
package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	// ValueLogin enables login/boot autostart.
	ValueLogin = "login"
	// ValueDisable turns autostart off.
	ValueDisable = "disable"
)

// Options describes how to launch dogego at login.
type Options struct {
	ExePath    string // absolute path to dogego binary
	ConfPath   string // absolute path to dogecoinconf.json
	DataDir    string // optional; passed as -datadir when set
	Subcommand string // node or spvnode
	Tray       bool   // pass -tray when starting at login
}

// Status reports whether OS autostart is installed on this machine.
type Status struct {
	Supported bool   `json:"supported"`
	Installed bool   `json:"installed"`
	Platform  string `json:"platform"`
	Method    string `json:"method,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// OnLogin reports whether the config value enables autostart.
func OnLogin(autostart string) bool {
	return strings.ToLower(strings.TrimSpace(autostart)) == ValueLogin
}

// NormalizeConfig trims and lowercases persisted autostart values.
func NormalizeConfig(v *string) {
	if v == nil {
		return
	}
	*v = strings.ToLower(strings.TrimSpace(*v))
	if *v == "" {
		*v = ValueDisable
	}
}

// Apply installs or refreshes OS login autostart for the given options.
func Apply(opts Options) error {
	if err := normalizeOpts(&opts); err != nil {
		return err
	}
	return applyPlatform(opts)
}

// Remove uninstalls OS login autostart when present.
func Remove() error {
	return removePlatform()
}

// Current returns installation status for the running platform.
func Current() Status {
	st := Status{Platform: platformName()}
	st.Supported = st.Platform != "unsupported"
	if !st.Supported {
		st.Detail = "autostart is only supported on Windows, Linux, and macOS"
		return st
	}
	installed, method, detail := statusPlatform()
	st.Installed = installed
	st.Method = method
	st.Detail = detail
	return st
}

func normalizeOpts(opts *Options) error {
	if opts == nil {
		return fmt.Errorf("autostart: nil options")
	}
	if strings.TrimSpace(opts.ExePath) == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("autostart: executable path: %w", err)
		}
		opts.ExePath = exe
	}
	abs, err := filepath.Abs(opts.ExePath)
	if err != nil {
		return fmt.Errorf("autostart: executable path: %w", err)
	}
	opts.ExePath = abs
	if strings.TrimSpace(opts.ConfPath) == "" {
		return fmt.Errorf("autostart: config path required")
	}
	confAbs, err := filepath.Abs(opts.ConfPath)
	if err != nil {
		return fmt.Errorf("autostart: config path: %w", err)
	}
	opts.ConfPath = confAbs
	if strings.TrimSpace(opts.DataDir) != "" {
		dd, err := filepath.Abs(opts.DataDir)
		if err != nil {
			return fmt.Errorf("autostart: datadir: %w", err)
		}
		opts.DataDir = dd
	}
	sub := strings.ToLower(strings.TrimSpace(opts.Subcommand))
	if sub == "" || sub == "full" {
		sub = "node"
	}
	if sub != "node" && sub != "spvnode" {
		return fmt.Errorf("autostart: subcommand must be node or spvnode")
	}
	opts.Subcommand = sub
	return nil
}

func nodeArgv(opts Options) []string {
	args := []string{opts.Subcommand, "-nobrowser"}
	if opts.Tray {
		args = append(args, "-tray")
	}
	if strings.TrimSpace(opts.DataDir) != "" {
		args = append(args, "-datadir", opts.DataDir)
	}
	return args
}
