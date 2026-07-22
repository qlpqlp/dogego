// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "testing"

func TestBIP9PeriodStatsAt(t *testing.T) {
	const period = 4
	j := &vbJournal{headers: make([]header80, 4)}
	for h := range j.headers {
		j.headers[h] = header80{version: vbVersion(h%2 == 0), time: uint32(1000 + h)}
	}
	dep := BIP9Deployment{Name: "csv", Bit: 0}
	st, err := BIP9PeriodStatsAt(j, 3, dep, period, 3)
	if err != nil {
		t.Fatal(err)
	}
	if st.Elapsed != 4 || st.Count != 2 {
		t.Fatalf("elapsed=%d count=%d", st.Elapsed, st.Count)
	}
	if st.Possible {
		t.Fatal("should not be possible with count 2 threshold 3")
	}
}
