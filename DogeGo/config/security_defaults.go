// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package config

// ApplyRecommendedSecurityDefaults sets HTTPS and local CA trust for new wizard installs.
func ApplyRecommendedSecurityDefaults(f *File) {
	if f == nil {
		return
	}
	f.WebUITLSLocal = true
	f.LocalTLSTrustCA = true
}
