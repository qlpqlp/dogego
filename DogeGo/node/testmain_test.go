// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	WaitAutoRecoverSweepAsync()
	os.Exit(code)
}
