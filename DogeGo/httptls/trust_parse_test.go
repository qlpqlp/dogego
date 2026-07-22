// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package httptls

import "testing"

func TestStoreTrustsFingerprint(t *testing.T) {
	sample := `
================ Certificate 3 ================
Serial Number: 01
Issuer: CN=DogeGo Local CA, O=DogeGo
Subject: CN=DogeGo Local CA, O=DogeGo
Signature Algorithm:
Cert Hash(sha1): ab cd ef 01 23 45
`
	if !storeTrustsFingerprint(sample, localCACommonName, "abcdef012345") {
		t.Fatal("expected fingerprint match")
	}
	if storeTrustsFingerprint(sample, localCACommonName, "000000000000") {
		t.Fatal("expected fingerprint mismatch")
	}
	if storeTrustsFingerprint(sample, "Other CA", "abcdef012345") {
		t.Fatal("wrong subject should not match")
	}
}
