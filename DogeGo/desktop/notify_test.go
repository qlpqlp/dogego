// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package desktop

import "testing"

func TestNotifyUpdateAvailableEmptyVersion(t *testing.T) {
	NotifyUpdateAvailable("", "https://example.com")
	NotifyUpdateAvailable("  ", "")
}
