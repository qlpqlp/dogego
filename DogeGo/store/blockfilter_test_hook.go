// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "time"

var stallAfterBlockFilterPutTmpWrite time.Duration

// StallAfterBlockFilterPutTmpWriteForTest blocks filter Put after .tmp write until process kill.
func StallAfterBlockFilterPutTmpWriteForTest(d time.Duration) {
	stallAfterBlockFilterPutTmpWrite = d
}
