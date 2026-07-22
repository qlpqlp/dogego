// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package store

import (
	"errors"
	"time"
)

// errAbortBeforeRawPutRename is returned when abortBeforeRawPutRename simulates force-kill after .tmp write.
var errAbortBeforeRawPutRename = errors.New("simulated kill before raw block rename")

// abortBeforeRawPutRename when true aborts Put after writing .tmp but before rename (tests only).
var abortBeforeRawPutRename bool

// stallAfterRawPutTmpWrite blocks Put after .tmp write (subprocess kill tests).
var stallAfterRawPutTmpWrite time.Duration

// SetAbortBeforeRawPutRenameForTest configures one-shot abort before rename (store + node tests).
func SetAbortBeforeRawPutRenameForTest(v bool) { abortBeforeRawPutRename = v }

// StallAfterRawPutTmpWriteForTest blocks Put after .tmp write until the process is killed.
func StallAfterRawPutTmpWriteForTest(d time.Duration) { stallAfterRawPutTmpWrite = d }
