// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"testing"
)

func TestSessionFailureHardFromBlockStall(t *testing.T) {
	if !sessionFailureHardFromFetchErr(blockStallError("1.2.3.4:22556")) {
		t.Fatal("block stall should use hard cooldown")
	}
	if sessionFailureHardFromFetchErr(ErrBlockDownloadStall) != true {
		t.Fatal("bare stall error should be hard")
	}
	if !errors.Is(blockStallError("peer"), ErrBlockDownloadStall) {
		t.Fatal("wrapped stall")
	}
}

func TestSessionFailureHardFromBlockDownloadTimeout(t *testing.T) {
	if !sessionFailureHardFromFetchErr(blockDownloadTimeoutError("1.2.3.4:22556")) {
		t.Fatal("download timeout should use hard cooldown")
	}
	if !errors.Is(blockDownloadTimeoutError("peer"), ErrBlockDownloadTimeout) {
		t.Fatal("wrapped download timeout")
	}
}
