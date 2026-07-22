// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"time"

	"dogego/autostart"
	"dogego/config"
)

// AutostartLoginProbeResult is returned by GET /api/core-autostart-probe.
type AutostartLoginProbeResult struct {
	CheckedAt string                `json:"checked_at"`
	OK        bool                  `json:"ok"`
	WantLogin bool                  `json:"want_login"`
	Status    autostart.Status      `json:"status"`
	Verify    autostart.VerifyResult `json:"verify"`
	Issues    []string              `json:"issues,omitempty"`
	Warnings  []string              `json:"warnings,omitempty"`
	Notes     []string              `json:"notes,omitempty"`
	CLI       string                `json:"cli,omitempty"`
}

// ProbeAutostartLogin checks OS login autostart vs dogecoinconf.json (mirrors dogego cert autostart).
func ProbeAutostartLogin(conf config.File) AutostartLoginProbeResult {
	want := conf.AutostartOnLogin()
	vr := autostart.VerifyLogin(want)
	return AutostartLoginProbeResult{
		CheckedAt: time.Now().UTC().Format(time.RFC3339),
		OK:        vr.OK,
		WantLogin: want,
		Status:    vr.Status,
		Verify:    vr,
		Issues:    vr.Issues,
		Warnings:  vr.Warnings,
		Notes:     vr.Notes,
		CLI:       "dogego cert autostart",
	}
}
