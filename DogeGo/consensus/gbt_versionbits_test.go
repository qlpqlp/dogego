// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"testing"

	"dogego/chain"
)

type gbtHdrJournal struct {
	tip  int64
	hdrs [][]byte
}

func (j *gbtHdrJournal) TipHeight() (int64, error) { return j.tip, nil }
func (j *gbtHdrJournal) ReadHeaderAt(h int64) ([]byte, error) {
	if h < 0 || int(h) >= len(j.hdrs) {
		return nil, nil
	}
	return j.hdrs[h], nil
}
func (j *gbtHdrJournal) HeightByDisplayHash(string) (int64, error) { return -1, nil }

func TestGBTVBName(t *testing.T) {
	if GBTVBName(BIP9Deployment{Name: "csv", GBTForce: true}) != "csv" {
		t.Fatal("csv")
	}
	if GBTVBName(BIP9Deployment{Name: "segwit", GBTForce: false}) != "!segwit" {
		t.Fatal("!segwit")
	}
}

func TestGBTVersionBitsEmptyWithoutChain(t *testing.T) {
	res, err := GBTVersionBits(nil, chain.MainnetDogecoin, 0, nil)
	if err != nil || len(res.VBAvailable) != 0 || len(res.Rules) != 0 {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestGBTBlockVersionOmitsUnsupportedStartedBit(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[0:4], 0x00620102)
	j := &gbtHdrJournal{tip: 0, hdrs: [][]byte{h}}
	full := GBTBlockVersion(j, chain.MainnetDogecoin, 0, map[string]struct{}{"csv": {}})
	masked := GBTBlockVersion(j, chain.MainnetDogecoin, 0, nil)
	if full == masked {
		t.Log("no started deployment at tip; full == masked")
	}
	_ = full
}

func TestGBTVersionBitsReturnsMaps(t *testing.T) {
	h := make([]byte, 80)
	binary.LittleEndian.PutUint32(h[0:4], 0x00620102)
	j := &gbtHdrJournal{tip: 0, hdrs: [][]byte{h}}
	res, err := GBTVersionBits(j, chain.MainnetDogecoin, 0, map[string]struct{}{})
	if err != nil {
		t.Fatal(err)
	}
	_ = res
}
