// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package analytics

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cockroachdb/pebble"
)

const (
	maxReorgEvents    = 500
	reorgSchemaNoteKey = "reorg_events"
)

var (
	prefixReorg  = []byte("r/")
	keyReorgSeq  = []byte("m/reorg_seq")
	keyReorgNote = []byte("m/reorg_events")
)

// ReorgBlockDetail is one block on the displaced or incoming fork branch.
type ReorgBlockDetail struct {
	Height        int64  `json:"height"`
	Hash          string `json:"hash"`
	TimeUnix      int64  `json:"time_unix"`
	Bits          uint32 `json:"bits"`
	AuxPow        bool   `json:"auxpow"`
	ParentHash    string `json:"parent_hash,omitempty"`
	MinerAddress  string `json:"miner_address,omitempty"`
	MinerKind     string `json:"miner_kind,omitempty"` // p2pkh | non_p2pkh | unknown
	BodyAvailable bool   `json:"body_available"`
}

// ReorgEvent is one chain reorg (or operator truncate) recorded for analytics.
type ReorgEvent struct {
	Seq                   uint64             `json:"seq"`
	RecordedUnix          int64              `json:"recorded_unix"`
	Network               string             `json:"network,omitempty"`
	Kind                  string             `json:"kind"` // header_reorg | truncate
	ForkAt                int64              `json:"fork_at"`
	OldTipHeight          int64              `json:"old_tip_height"`
	Depth                 int64              `json:"depth"`
	IncomingCount         int                `json:"incoming_count"`
	IncomingWork          string             `json:"incoming_work,omitempty"`
	DisplacedWork         string             `json:"displaced_work,omitempty"`
	WorkDelta             string             `json:"work_delta,omitempty"`
	Precious              bool               `json:"precious,omitempty"`
	Truncated             bool               `json:"truncated,omitempty"`
	HourUTC               int                `json:"hour_utc"`
	DisplacedAuxPowCount  int                `json:"displaced_auxpow_count"`
	IncomingAuxPowCount   int                `json:"incoming_auxpow_count"`
	DisplacedMinerCounts  map[string]int     `json:"displaced_miner_counts,omitempty"`
	IncomingMinerCounts   map[string]int     `json:"incoming_miner_counts,omitempty"`
	Displaced             []ReorgBlockDetail `json:"displaced,omitempty"`
	Incoming              []ReorgBlockDetail `json:"incoming,omitempty"`
}

func reorgKey(seq uint64) []byte {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], seq)
	out := make([]byte, 0, len(prefixReorg)+8)
	out = append(out, prefixReorg...)
	return append(out, b[:]...)
}

func nextReorgSeq(db *DB) (uint64, error) {
	val, closer, err := db.db.Get(keyReorgSeq)
	seq := uint64(0)
	if err == nil {
		defer closer.Close()
		_, _ = fmt.Sscanf(string(val), "%d", &seq)
	}
	seq++
	if err := db.db.Set(keyReorgSeq, []byte(fmt.Sprintf("%d", seq)), pebble.Sync); err != nil {
		return 0, err
	}
	return seq, nil
}

func pruneReorgEvents(db *DB, keepFrom uint64) error {
	if keepFrom <= 1 {
		return nil
	}
	return db.db.DeleteRange(reorgKey(1), reorgKey(keepFrom), pebble.Sync)
}

// RecordReorgEvent appends one reorg observation and prunes old rows.
func RecordReorgEvent(db *DB, ev ReorgEvent) error {
	if db == nil || db.db == nil {
		return nil
	}
	seq, err := nextReorgSeq(db)
	if err != nil {
		return err
	}
	if ev.RecordedUnix == 0 {
		ev.RecordedUnix = time.Now().Unix()
	}
	if ev.Kind == "" {
		ev.Kind = "header_reorg"
	}
	ev.Seq = seq
	ev.HourUTC = int(time.Unix(ev.RecordedUnix, 0).UTC().Hour())
	b, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	if err := db.db.Set(reorgKey(seq), b, pebble.Sync); err != nil {
		return err
	}
	_ = db.db.Set(keyReorgNote, []byte(reorgSchemaNoteKey), pebble.NoSync)
	if seq > maxReorgEvents {
		return pruneReorgEvents(db, seq-maxReorgEvents+1)
	}
	return nil
}

// ReadReorgEvents returns recent reorg events oldest-first (limit <= 0 → all kept).
func ReadReorgEvents(db *DB, limit int) ([]ReorgEvent, error) {
	if db == nil || db.db == nil {
		return nil, nil
	}
	if limit <= 0 || limit > maxReorgEvents {
		limit = maxReorgEvents
	}
	it, err := db.db.NewIter(&pebble.IterOptions{
		LowerBound: prefixReorg,
		UpperBound: prefixEnd(prefixReorg),
	})
	if err != nil {
		return nil, err
	}
	defer it.Close()
	var rev []ReorgEvent
	for ok := it.Last(); ok && len(rev) < limit; ok = it.Prev() {
		var ev ReorgEvent
		if err := json.Unmarshal(it.Value(), &ev); err != nil {
			return nil, err
		}
		rev = append(rev, ev)
	}
	if err := it.Error(); err != nil {
		return nil, err
	}
	out := make([]ReorgEvent, len(rev))
	for i := range rev {
		out[len(rev)-1-i] = rev[i]
	}
	return out, nil
}

// ReorgSummary aggregates events for dashboard charts.
type ReorgSummary struct {
	Total              int            `json:"total"`
	LastRecordedUnix   int64          `json:"last_recorded_unix,omitempty"`
	MaxDepth           int64          `json:"max_depth"`
	TotalDepth         int64          `json:"total_depth"`
	AuxPowInvolved     int            `json:"auxpow_involved"`
	ByHourUTC          [24]int        `json:"by_hour_utc"`
	MinerOnDisplaced   map[string]int `json:"miner_on_displaced,omitempty"`
	MinerOnIncoming    map[string]int `json:"miner_on_incoming,omitempty"`
}

// SummarizeReorgEvents builds chart-friendly aggregates (newest events first optional).
func SummarizeReorgEvents(events []ReorgEvent) ReorgSummary {
	var s ReorgSummary
	s.MinerOnDisplaced = map[string]int{}
	s.MinerOnIncoming = map[string]int{}
	for _, ev := range events {
		s.Total++
		if ev.RecordedUnix > s.LastRecordedUnix {
			s.LastRecordedUnix = ev.RecordedUnix
		}
		if ev.Depth > s.MaxDepth {
			s.MaxDepth = ev.Depth
		}
		s.TotalDepth += ev.Depth
		if ev.DisplacedAuxPowCount > 0 || ev.IncomingAuxPowCount > 0 {
			s.AuxPowInvolved++
		}
		h := ev.HourUTC
		if h >= 0 && h < 24 {
			s.ByHourUTC[h]++
		}
		for addr, n := range ev.DisplacedMinerCounts {
			s.MinerOnDisplaced[addr] += n
		}
		for addr, n := range ev.IncomingMinerCounts {
			s.MinerOnIncoming[addr] += n
		}
	}
	if len(s.MinerOnDisplaced) == 0 {
		s.MinerOnDisplaced = nil
	}
	if len(s.MinerOnIncoming) == 0 {
		s.MinerOnIncoming = nil
	}
	return s
}
