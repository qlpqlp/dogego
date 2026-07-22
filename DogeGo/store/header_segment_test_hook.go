// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import "time"

// stallAfterHeaderSegTmpWrite blocks header segment append after .tmp write (subprocess kill tests).
var stallAfterHeaderSegTmpWrite time.Duration

// StallAfterHeaderSegTmpWriteForTest blocks segment append after .tmp write until process kill.
func StallAfterHeaderSegTmpWriteForTest(d time.Duration) { stallAfterHeaderSegTmpWrite = d }
