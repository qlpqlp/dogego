// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package upnp

import "strings"

// Mode controls UPnP / NAT-PMP port mapping (Core -upnp analogue).
type Mode int

const (
	ModeNever Mode = iota
	ModeEnable
	ModeAuto
)

// ParseMode interprets dogecoinconf.json / -upnp (empty → auto).
func ParseMode(s string) Mode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "enable", "enabled", "1", "yes", "true", "on":
		return ModeEnable
	case "disable", "disabled", "never", "0", "no", "false", "off":
		return ModeNever
	default:
		return ModeAuto
	}
}

// ShouldMap returns whether to attempt router port mapping.
// Core default: on when listening and no -proxy (DogeGo has no Tor proxy yet).
func ShouldMap(mode Mode, listen bool) bool {
	switch mode {
	case ModeNever:
		return false
	case ModeEnable:
		return true
	default:
		return listen
	}
}
