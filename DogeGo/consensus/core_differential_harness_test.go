// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"strings"
	"testing"
)

type coreDifficultyVector struct {
	Name             string `json:"name"`
	Network          string `json:"network"`
	Mode             string `json:"mode"`
	PrevHeight       int64  `json:"prev_height"`
	Tip0             int64  `json:"tip0"`
	LastRetargetTime int64  `json:"last_retarget_time"`
	PrevTime         uint32 `json:"prev_time"`
	PrevBits         string `json:"prev_bits"`
	CandidateTime    uint32 `json:"candidate_time"`
	WantBits         string `json:"want_bits"`
}

func loadCoreDifficultyVectors(t *testing.T) []coreDifficultyVector {
	t.Helper()
	var vecs []coreDifficultyVector
	loadJSONFixture(t, "core_difficulty_vectors.json", &vecs)
	if len(vecs) == 0 {
		t.Fatal("no differential vectors loaded")
	}
	return vecs
}

// TestCoreDifficultyDifferentialVectors is a fixture-driven scaffold for Core differential checks.
// It replays difficulty vectors and compares DogeGo outputs against expected Core values.
func TestCoreDifficultyDifferentialVectors(t *testing.T) {
	for _, v := range loadCoreDifficultyVectors(t) {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			net, err := networkFromFixture(v.Network)
			if err != nil {
				t.Fatal(err)
			}
			prevBits, err := parseU32Hex(v.PrevBits)
			if err != nil {
				t.Fatalf("prev_bits parse: %v", err)
			}
			wantBits, err := parseU32Hex(v.WantBits)
			if err != nil {
				t.Fatalf("want_bits parse: %v", err)
			}

			dc := LookupConsensus(net, v.PrevHeight)
			var got uint32
			switch strings.ToLower(strings.TrimSpace(v.Mode)) {
			case "get_next_work":
				times := make([]uint32, 241)
				bits := make([]uint32, 241)
				times[0] = uint32(v.LastRetargetTime)
				times[240] = v.PrevTime
				bits[240] = prevBits
				view := testBatchView(v.Tip0, times, bits)
				got, err = getNextWorkRequired(view, v.PrevHeight, v.CandidateTime, dc)
			case "calculate_next_work":
				view := testBatchView(v.Tip0, []uint32{v.PrevTime}, []uint32{prevBits})
				got, err = calculateDogecoinNextWorkRequired(view, v.PrevHeight, v.LastRetargetTime, dc)
			default:
				t.Fatalf("unsupported vector mode %q", v.Mode)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != wantBits {
				t.Fatalf("vector %s mismatch: got 0x%x want 0x%x", v.Name, got, wantBits)
			}
		})
	}
}
