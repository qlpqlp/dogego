// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

import (
	"net"
	"testing"
)

func TestCountingConnReadWrite(t *testing.T) {
	ctr := newNetByteCounter()
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()

	cc := &countingConn{Conn: cli, ctr: ctr}
	done := make(chan error, 1)
	go func() {
		buf := make([]byte, 4)
		if _, err := cc.Read(buf); err != nil {
			done <- err
			return
		}
		if _, err := cc.Write([]byte("pong")); err != nil {
			done <- err
			return
		}
		done <- nil
	}()

	// Client must be blocked in Read before server Write, or net.Pipe deadlocks.
	if _, err := srv.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := srv.Read(buf); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if ctr.Recv() != 4 {
		t.Fatalf("recv want 4 got %d", ctr.Recv())
	}
	if ctr.Sent() != 4 {
		t.Fatalf("sent want 4 got %d", ctr.Sent())
	}
}
