// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"encoding/json"
	"time"

	"github.com/cockroachdb/pebble"
)

const maxMetricSamples = 720 // ~5h at 25s ticks; enough for dashboard timelines

// MetricSample is one sidecar observation for charts.
type MetricSample struct {
	RecordedUnix        int64 `json:"recorded_unix"`
	MempoolTxs          int   `json:"mempool_txs"`
	MempoolBytes        int64 `json:"mempool_bytes"`
	MaxRecentBlockBytes int64 `json:"max_recent_block_bytes"`
	ChainDataBytes      int64 `json:"chain_data_bytes"`
	HeadersBytes        int64 `json:"headers_bytes"`
	RawBlocksBytes      int64 `json:"rawblocks_bytes"`
	TxIndexBytes        int64 `json:"txindex_bytes"`
}

// LiveMetrics is sampled by the embedded sidecar each tick.
type LiveMetrics struct {
	MempoolTxs          int
	MempoolBytes        int64
	MaxRecentBlockBytes int64
	HeadersBytes        int64
	RawBlocksBytes      int64
	TxIndexBytes        int64
	ChainDataBytes      int64
}

type metricSampleValue struct {
	RecordedUnix        int64 `json:"recorded_unix"`
	MempoolTxs          int   `json:"mempool_txs"`
	MempoolBytes        int64 `json:"mempool_bytes"`
	MaxRecentBlockBytes int64 `json:"max_recent_block_bytes"`
	ChainDataBytes      int64 `json:"chain_data_bytes"`
	HeadersBytes        int64 `json:"headers_bytes"`
	RawBlocksBytes      int64 `json:"rawblocks_bytes"`
	TxIndexBytes        int64 `json:"txindex_bytes"`
}

// RecordMetricSample appends one observation and prunes old rows.
func RecordMetricSample(db *DB, m LiveMetrics) error {
	if db == nil || db.db == nil {
		return nil
	}
	seq, err := nextMetricSeq(db)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	v := metricSampleValue{
		RecordedUnix: now, MempoolTxs: m.MempoolTxs, MempoolBytes: m.MempoolBytes,
		MaxRecentBlockBytes: m.MaxRecentBlockBytes, ChainDataBytes: m.ChainDataBytes,
		HeadersBytes: m.HeadersBytes, RawBlocksBytes: m.RawBlocksBytes, TxIndexBytes: m.TxIndexBytes,
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if err := db.db.Set(sampleKey(seq), b, pebble.Sync); err != nil {
		return err
	}
	if seq > maxMetricSamples {
		return pruneMetricSamples(db, seq-maxMetricSamples+1)
	}
	return nil
}

// ReadMetricSamples returns recent samples oldest-first.
func ReadMetricSamples(db *DB, limit int) ([]MetricSample, error) {
	if db == nil || db.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > maxMetricSamples {
		limit = maxMetricSamples
	}
	it, err := db.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixSample,
		UpperBound: prefixEnd(prefixSample),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var rev []MetricSample
	for ok := it.Last(); ok && len(rev) < limit; ok = it.Prev() {
		var v metricSampleValue
		if err := json.Unmarshal(it.Value(), &v); err != nil {
			return nil, err
		}
		rev = append(rev, MetricSample(v))
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	out := make([]MetricSample, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out, nil
}
