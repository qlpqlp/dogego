// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"errors"
	"strings"
	"testing"
)

func TestAutostartApplyWarningOnError(t *testing.T) {
	got := autostartApplyWarning("", errors.New("schtasks create: exit status 1 (ERROR: Access is denied.)"))
	if !strings.Contains(got, "Login autostart could not be registered") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Access is denied") {
		t.Fatalf("got %q", got)
	}
}

func TestAutostartApplyWarningPassesSyncWarn(t *testing.T) {
	got := autostartApplyWarning("Runs at Windows sign-in", nil)
	if got != "Runs at Windows sign-in" {
		t.Fatalf("got %q", got)
	}
}

func TestAutostartApplyWarningEmpty(t *testing.T) {
	if got := autostartApplyWarning("", nil); got != "" {
		t.Fatalf("got %q", got)
	}
}
