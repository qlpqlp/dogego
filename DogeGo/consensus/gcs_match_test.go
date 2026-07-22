// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"testing"

	"dogego/pow"
	"dogego/wire"
)

func TestBasicFilterMayContainRoundTrip(t *testing.T) {
	for _, v := range loadCoreBlockFilterVectors(t) {
		t.Run(v.BlockHash, func(t *testing.T) {
			blockRaw, err := hex.DecodeString(v.Block)
			if err != nil {
				t.Fatal(err)
			}
			pb, err := wire.ParseBlock(blockRaw)
			if err != nil {
				t.Fatal(err)
			}
			hashLE := pow.BlockHashLE(blockRaw[:80])
			enc, err := buildBasicFilterFromCoreVector(v)
			if err != nil {
				t.Fatal(err)
			}
			outs := CollectBasicFilterOutputScripts(pb)
			if len(outs) == 0 {
				return // empty filter block (e.g. witness-only / no standard outputs)
			}
			spk := outs[0]
			ok2, err := BasicFilterMayContainScript(hashLE, enc, spk)
			if err != nil || !ok2 {
				t.Fatalf("contain=%v err=%v spk=%x", ok2, err, spk)
			}
			ok3, _ := BasicFilterMayContainScript(hashLE, enc, []byte{0x99})
			if ok3 {
				t.Fatal("unexpected match")
			}
			// Matcher must work on Core-encoded bytes too (same as build output).
			coreEnc, err := hex.DecodeString(v.BasicFilter)
			if err != nil {
				t.Fatal(err)
			}
			if string(coreEnc) != string(enc) {
				t.Fatal("build != core filter bytes")
			}
			ok4, err := BasicFilterMayContainScript(hashLE, coreEnc, spk)
			if err != nil || !ok4 {
				t.Fatalf("core enc contain=%v err=%v", ok4, err)
			}
		})
	}
}
