// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package autostart

// VerifyResult reports whether OS autostart matches operator intent.
type VerifyResult struct {
	WantLogin bool   `json:"want_login"`
	OK        bool   `json:"ok"`
	Status    Status `json:"status"`
	Issues    []string `json:"issues,omitempty"`
	Warnings  []string `json:"warnings,omitempty"`
	Notes     []string `json:"notes,omitempty"`
}

// VerifyLogin checks autostart registration against config intent (login vs disable).
func VerifyLogin(wantLogin bool) VerifyResult {
	r := VerifyResult{WantLogin: wantLogin, Status: Current()}
	if !wantLogin {
		r.OK = true
		if r.Status.Installed {
			r.Warnings = append(r.Warnings, "autostart_installed_but_disabled_in_config")
		} else {
			r.Notes = append(r.Notes, "autostart_not_configured")
		}
		return r
	}
	if !r.Status.Supported {
		r.OK = true
		r.Warnings = append(r.Warnings, "autostart_login_unsupported_platform")
		return r
	}
	if !r.Status.Installed {
		r.Issues = append(r.Issues, "autostart_login_not_registered")
		r.OK = false
		return r
	}
	r.OK = true
	r.Notes = append(r.Notes, "autostart_registered")
	return r
}
