// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"strings"
	"testing"
)

func TestMetricSamplesCSV(t *testing.T) {
	csv := string(MetricSamplesCSV([]MetricSample{
		{RecordedUnix: 1_700_000_000, MempoolTxs: 2, MempoolBytes: 100},
	}))
	if !strings.Contains(csv, "recorded_unix") {
		t.Fatal("missing header")
	}
	if !strings.Contains(csv, "1700000000") {
		t.Fatal("missing row")
	}
}
