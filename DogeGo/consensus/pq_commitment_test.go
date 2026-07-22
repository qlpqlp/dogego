// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestDetectPQCommitmentFLC1(t *testing.T) {
	script := make([]byte, 38)
	script[0] = 0x6a
	script[1] = 0x24
	copy(script[2:6], []byte(PQTagFalcon))
	for i := 6; i < 38; i++ {
		script[i] = byte(i)
	}
	c, ok := DetectPQCommitment(script)
	if !ok || c.Scheme != "falcon-512" || c.Tag != PQTagFalcon {
		t.Fatalf("%+v ok=%v", c, ok)
	}
	if len(c.Commitment) != 64 {
		t.Fatalf("hex len %d", len(c.Commitment))
	}
}

func TestDetectPQCommitmentRejectsWrongPushLen(t *testing.T) {
	script := []byte{0x6a, 0x20}
	copy(script[2:], []byte(PQTagFalcon))
	if _, ok := DetectPQCommitment(script); ok {
		t.Fatal("expected reject")
	}
}

func TestVerifyPQCommitmentScriptHex(t *testing.T) {
	commit := make([]byte, 32)
	script, err := BuildPQCommitmentScript(PQTagRaccoon, commit)
	if err != nil {
		t.Fatal(err)
	}
	out, err := VerifyPQCommitmentScriptHex(hexEncode(script))
	if err != nil || out["valid"] != true || out["tag"] != PQTagRaccoon {
		t.Fatalf("out=%#v err=%v", out, err)
	}
	_, err = VerifyPQCommitmentScriptHex("6a04deadbeef")
	if err == nil {
		t.Fatal("expected reject")
	}
}

func hexEncode(b []byte) string {
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
