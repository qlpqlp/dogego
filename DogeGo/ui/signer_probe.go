// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"strings"
	"time"

	"dogego/config"
	"dogego/signer"
)

// SignerTestResult is returned by POST /api/signer-test (HWI enumeratesigners reachability).
type SignerTestResult struct {
	OK              bool     `json:"ok"`
	SignerConfigured bool    `json:"signer_configured"`
	SignerCmd       string   `json:"signer_cmd,omitempty"`
	DeviceCount     int      `json:"device_count,omitempty"`
	Devices         []any    `json:"devices,omitempty"`
	Errors          []string `json:"errors,omitempty"`
	Hint            string   `json:"hint,omitempty"`
	TestedAt        string   `json:"tested_at"`
}

// ApplySignerCmdFormOverride merges optional Settings-form signer_cmd into probe config.
func ApplySignerCmdFormOverride(conf config.File, formCmd string) config.File {
	if v := strings.TrimSpace(formCmd); v != "" {
		conf.SignerCmd = v
	}
	return conf
}

// ResolveSignerCommand returns argv for an HWI-compatible signer from form override or config.
func ResolveSignerCommand(conf config.File, formCmd string) []string {
	conf = ApplySignerCmdFormOverride(conf, formCmd)
	if v := strings.TrimSpace(conf.SignerCmd); v != "" {
		return signer.ParseCommandLine(v)
	}
	return nil
}

// ProbeSignerTest runs enumeratesigners against the configured or form signer_cmd.
func ProbeSignerTest(conf config.File, formSignerCmd string) SignerTestResult {
	argv := ResolveSignerCommand(conf, formSignerCmd)
	out := SignerTestResult{
		SignerCmd: strings.TrimSpace(ApplySignerCmdFormOverride(conf, formSignerCmd).SignerCmd),
		TestedAt:  time.Now().UTC().Format(time.RFC3339),
		Hint:      "Quick HWI-compatible signer check (enumerate). Save Settings and restart to persist signer_cmd.",
	}
	if len(argv) == 0 {
		out.Errors = append(out.Errors, "signer_cmd is empty")
		out.Hint = "Set External signer command in Settings, for example: python hwi.py --chain dogecoin --stdin"
		return out
	}
	out.SignerCmd = strings.Join(argv, " ")
	c, err := signer.New(argv)
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		return out
	}
	devs, err := c.Enumerate()
	if err != nil {
		out.Errors = append(out.Errors, err.Error())
		out.Hint = "Check signer_cmd path, Python/HWI install, and that the hardware wallet is connected and unlocked."
		return out
	}
	out.DeviceCount = len(devs)
	for _, d := range devs {
		out.Devices = append(out.Devices, d)
	}
	out.SignerConfigured = out.DeviceCount > 0
	out.OK = out.SignerConfigured
	if !out.SignerConfigured {
		out.Hint = "Signer process responded but returned 0 devices. Connect/unlock the device or check USB permissions."
	}
	return out
}
