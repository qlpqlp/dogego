// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import "dogego/config"

// InteractiveSession reports whether DogeGo likely runs on an interactive desktop
// (GUI session with a browser), as opposed to SSH/headless servers.
func InteractiveSession() bool {
	return interactiveSession()
}

// ApplyWizardDefaults sets setup-wizard defaults for desktop installs.
func ApplyWizardDefaults(f *config.File) {
	if f == nil || !InteractiveSession() {
		return
	}
	if TraySupported() {
		f.Tray = config.TrayPtr(true)
	}
	// Desktop manual starts should open the dashboard unless the user opts out later.
	f.NoBrowser = false
}

// OpenWizardInBrowser reports whether the setup wizard should auto-open a browser tab.
func OpenWizardInBrowser(noBrowser bool) bool {
	if noBrowser {
		return false
	}
	return InteractiveSession()
}
