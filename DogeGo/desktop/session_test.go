// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import (
	"testing"

	"dogego/config"
)

func TestApplyWizardDefaultsDesktop(t *testing.T) {
	if !InteractiveSession() {
		t.Skip("non-interactive environment")
	}
	var f config.File
	ApplyWizardDefaults(&f)
	if TraySupported() && (f.Tray == nil || !*f.Tray) {
		t.Fatal("expected tray default on desktop")
	}
}

func TestOpenWizardInBrowserRespectsNoBrowser(t *testing.T) {
	if OpenWizardInBrowser(true) {
		t.Fatal("nobrowser should win")
	}
}

func TestHeadlessEnv(t *testing.T) {
	t.Setenv("DOGEGO_HEADLESS", "1")
	if InteractiveSession() {
		t.Fatal("DOGEGO_HEADLESS=1 should disable interactive session")
	}
}
