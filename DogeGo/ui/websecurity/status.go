// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package websecurity

// StatusNoPIN is returned when no dashboard PIN is configured (or security gate is unavailable).
func StatusNoPIN() map[string]interface{} {
	return map[string]interface{}{
		"pin_enabled":      false,
		"locked":           false,
		"locked_seconds":   int64(0),
		"failed_attempts":  0,
		"max_failures":     maxPINFailures,
		"unlocked":         true,
		"webauthn_enabled": false,
	}
}
