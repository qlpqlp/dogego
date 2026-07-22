// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"errors"
	"testing"

	"dogego/chain"
)

type vbJournal struct {
	headers []header80
}

type header80 struct {
	version uint32
	time    uint32
}

func (j *vbJournal) TipHeight() (int64, error) {
	if len(j.headers) == 0 {
		return -1, nil
	}
	return int64(len(j.headers) - 1), nil
}

func (j *vbJournal) ReadHeaderAt(h int64) ([]byte, error) {
	if h < 0 || int(h) >= len(j.headers) {
		return nil, errors.New("height")
	}
	return j.headers[h].bytes(), nil
}

func (j *vbJournal) HeightByDisplayHash(string) (int64, error) {
	return 0, errors.New("height")
}

func (h header80) bytes() []byte {
	b := make([]byte, 80)
	binary.LittleEndian.PutUint32(b[0:4], h.version)
	binary.LittleEndian.PutUint32(b[68:72], h.time)
	return b
}

func vbVersion(signal bool) uint32 {
	v := uint32(VersionBitsTopBits)
	if signal {
		v |= 1
	}
	return v
}

func TestEvaluateBIP9CSVLockedInToActive(t *testing.T) {
	const period = 8
	const threshold = 6
	dep := BIP9Deployment{Name: "csv", Bit: 0, StartTime: 100, Timeout: 999999}
	mask := uint32(1)
	var headers []header80
	for i := 0; i < period*4; i++ {
		tm := uint32(50)
		if i >= period {
			tm = 200
		}
		sig := i >= period
		headers = append(headers, header80{version: vbVersion(sig), time: tm})
	}
	j := &vbJournal{headers: headers}
	tip := int64(len(headers) - 1)
	st := computeThresholdStateForward(j, tip, dep, period, threshold, mask)
	if st != ThresholdActive {
		t.Fatalf("state %s want active", st)
	}
}

func TestEvaluateBIP9DisabledSegwitMainnet(t *testing.T) {
	p := BIP9ParamsForNetwork(chain.MainnetDogecoin)
	var seg BIP9Deployment
	for _, d := range p.Deployments {
		if d.Name == "segwit" {
			seg = d
			break
		}
	}
	if seg.Timeout != 0 {
		t.Fatal("expected disabled segwit timeout 0")
	}
	res, err := EvaluateBIP9AtTip(nil, chain.MainnetDogecoin, seg, p.Period, p.Threshold)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != ThresholdDefined {
		t.Fatalf("segwit status %s", res.Status)
	}
}

func TestBIP9ParamsMainnetCSVHeightFallback(t *testing.T) {
	dc := LookupConsensus(chain.MainnetDogecoin, 419328)
	if dc.CSVHeight != 419328 {
		t.Fatalf("csv height %d", dc.CSVHeight)
	}
}
