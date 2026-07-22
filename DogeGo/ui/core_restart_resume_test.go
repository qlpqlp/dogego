// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"testing"

	"dogego/config"
)

func TestProbeCoreRestartResumeNoInvoke(t *testing.T) {
	r := ProbeCoreRestartResume("mainnet", "", config.File{}, nil)
	if len(r.Issues) == 0 || r.Issues[0] != "dogego_rpc_not_ready" {
		t.Fatalf("issues=%v", r.Issues)
	}
}

func TestAppendAutostartCheckDisabled(t *testing.T) {
	var out CoreRestartResumeResult
	appendAutostartCheck(&out, config.File{Autostart: "disable"})
	if out.AutostartWant {
		t.Fatal("want false")
	}
	found := false
	for _, c := range out.Checks {
		if c.Name == "os_autostart" && c.Status == "skipped" {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks=%v", out.Checks)
	}
}

func TestAppendAutostartCheckLogin(t *testing.T) {
	var out CoreRestartResumeResult
	appendAutostartCheck(&out, config.File{Autostart: "login"})
	if !out.AutostartWant {
		t.Fatal("want true")
	}
	found := false
	for _, c := range out.Checks {
		if c.Name == "os_autostart" {
			found = true
			if c.Status != "ok" && c.Status != "issue" && c.Status != "warning" {
				t.Fatalf("status=%q", c.Status)
			}
		}
	}
	if !found {
		t.Fatalf("checks=%v", out.Checks)
	}
}
