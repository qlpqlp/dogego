// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package dgr

import "testing"

func TestTokenBucketAllow(t *testing.T) {
	b := newTokenBucket(10, 2)
	if !b.allow() || !b.allow() {
		t.Fatal("expected burst allowance")
	}
	if b.allow() {
		t.Fatal("expected bucket empty after burst")
	}
}

func TestRegisterLimiter(t *testing.T) {
	l := newRegisterLimiter(2)
	if !l.allow("1.2.3.4") || !l.allow("1.2.3.4") {
		t.Fatal("expected first two registers")
	}
	if l.allow("1.2.3.4") {
		t.Fatal("expected register rate limit")
	}
	if !l.allow("5.6.7.8") {
		t.Fatal("expected separate IP allowance")
	}
}
