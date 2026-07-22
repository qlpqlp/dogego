// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"fmt"
	"io"
	"net"
	"syscall"
	"testing"
)

func TestRecoverableHeaderPeerErr(t *testing.T) {
	if !recoverableHeaderPeerErr(io.EOF) {
		t.Fatal("EOF")
	}
	if !recoverableHeaderPeerErr(io.ErrUnexpectedEOF) {
		t.Fatal("unexpected eof")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("timeout waiting for headers")) {
		t.Fatal("timeout string")
	}
	if !recoverableHeaderPeerErr(&net.OpError{Err: syscall.ECONNRESET}) {
		t.Fatal("op ECONNRESET")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("reject during headers sync: x")) {
		t.Fatal("reject during headers should retry")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("headers: bad magic fdd00701, expected c0c0c0c0")) {
		t.Fatal("bad magic should be recoverable")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("header batch index 79 (chain height 4080 on mainnet): bad nBits want 0x1d00ba8a got 0x1d00e52d")) {
		t.Fatal("bad nBits should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("header batch index 1583 (chain height 371337 on mainnet): legacy scrypt header after auxpow activation (mainnet merge-mining from height 371337)")) {
		t.Fatal("legacy at auxpow fork should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("header batch index 0 (chain height 371336 on mainnet): auxpow header before activation (mainnet legacy scrypt through height 371336)")) {
		t.Fatal("early auxpow should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("read tcp 192.168.1.1:7417->51.79.177.13:22556: use of closed network connection")) {
		t.Fatal("closed network connection should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("write tcp [2001:818:e91b:3300:595a:f830:fe7c:477d]:62245->[2001:41d0:2:ae2::]:22556: wsasend: An established connection was aborted by the software in your host machine")) {
		t.Fatal("Windows wsasend write abort should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("header sync incomplete at height 371337 (peer reports 6200000)")) {
		t.Fatal("header sync incomplete should retry next peer")
	}
	if !recoverableHeaderPeerErr(fmt.Errorf("header 1418 aux: aux hash block mismatch")) {
		t.Fatal("invalid auxpow from peer should retry next peer")
	}
}
