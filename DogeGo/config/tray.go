// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

// TrayEnabled resolves the tray flag: explicit JSON value wins; otherwise use desktopDefault.
func (f File) TrayEnabled(desktopDefault bool) bool {
	if f.Tray != nil {
		return *f.Tray
	}
	return desktopDefault
}

// TrayPtr returns a bool pointer for persisting tray in JSON.
func TrayPtr(v bool) *bool {
	b := v
	return &b
}
