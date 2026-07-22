// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"bytes"
	"fmt"
	"time"
)

// MetricSamplesCSV exports metric timeline rows as CSV (RFC 4180-ish).
func MetricSamplesCSV(samples []MetricSample) []byte {
	var b bytes.Buffer
	b.WriteString("recorded_unix,recorded_iso,mempool_txs,mempool_bytes,max_recent_block_bytes,chain_data_bytes,headers_bytes,rawblocks_bytes,txindex_bytes\n")
	for _, s := range samples {
		iso := time.Unix(s.RecordedUnix, 0).UTC().Format(time.RFC3339)
		fmt.Fprintf(&b, "%d,%s,%d,%d,%d,%d,%d,%d,%d\n",
			s.RecordedUnix, iso, s.MempoolTxs, s.MempoolBytes, s.MaxRecentBlockBytes,
			s.ChainDataBytes, s.HeadersBytes, s.RawBlocksBytes, s.TxIndexBytes)
	}
	return b.Bytes()
}
