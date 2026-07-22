// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
)

func TestIsPermanentFetchErr(t *testing.T) {
	if !isPermanentFetchErr(io.EOF) {
		t.Fatal("EOF")
	}
	if !isPermanentFetchErr(errors.New("EOF")) {
		t.Fatal("bare EOF string")
	}
	if !isPermanentFetchErr(errors.New("read tcp: wsasend: An established connection was aborted by the software in your host machine")) {
		t.Fatal("wsasend abort")
	}
	if !isPermanentFetchErr(&net.OpError{Err: syscall.ECONNRESET}) {
		t.Fatal("ECONNRESET")
	}
	if isPermanentFetchErr(errors.New("timeout: no valid block")) {
		t.Fatal("application timeout string should not be treated as transport EOF")
	}
}
