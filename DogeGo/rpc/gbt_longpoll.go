// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"sync"
	"time"
)

// gbtLongpollTimeout matches Core’s practical longpoll wait window (~1 minute).
const gbtLongpollTimeout = 60 * time.Second

var gbtWake = struct {
	mu sync.Mutex
	ch chan struct{}
}{ch: make(chan struct{})}

// NotifyGBTWake unblocks getblocktemplate longpoll waiters (tip or mempool change).
func NotifyGBTWake() {
	gbtWake.mu.Lock()
	defer gbtWake.mu.Unlock()
	select {
	case <-gbtWake.ch:
		// already closed
	default:
		close(gbtWake.ch)
	}
	gbtWake.ch = make(chan struct{})
}

func gbtWakeChan() <-chan struct{} {
	gbtWake.mu.Lock()
	ch := gbtWake.ch
	gbtWake.mu.Unlock()
	return ch
}

// waitGBTLongpoll blocks until stillSame returns false or timeout elapses (Core longpoll).
func waitGBTLongpoll(timeout time.Duration, stillSame func() bool) {
	if timeout <= 0 {
		timeout = gbtLongpollTimeout
	}
	deadline := time.Now().Add(timeout)
	for stillSame() && time.Now().Before(deadline) {
		remain := time.Until(deadline)
		if remain <= 0 {
			return
		}
		slice := 1 * time.Second
		if remain < slice {
			slice = remain
		}
		ch := gbtWakeChan()
		timer := time.NewTimer(slice)
		select {
		case <-ch:
		case <-timer.C:
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}
}
