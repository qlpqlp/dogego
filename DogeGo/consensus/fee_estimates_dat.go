// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"dogego/store"
)

// WriteCoreFeeEstimatesDat writes Core CBlockPolicyEstimator fee_estimates.dat layout:
// int32 nBestSeenHeight, then TxConfirmStats (decay, buckets, avg, txCtAvg, confAvg).
func WriteCoreFeeEstimatesDat(path string, bestSeen int32, stats *TxConfirmStats) error {
	if stats == nil || len(stats.txCtAvg) == 0 {
		return nil
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, bestSeen); err != nil {
		return err
	}
	if err := writeCoreTxConfirmStats(&buf, stats); err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	return store.AtomicWriteFile(path, buf.Bytes(), 0o600)
}

// ReadCoreFeeEstimatesDat loads Core fee_estimates.dat (missing file returns nil stats, nil error).
func ReadCoreFeeEstimatesDat(path string) (bestSeen int32, stats *TxConfirmStats, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil, nil
		}
		return 0, nil, err
	}
	br := bytes.NewReader(raw)
	if err := binary.Read(br, binary.LittleEndian, &bestSeen); err != nil {
		return 0, nil, fmt.Errorf("fee_estimates.dat: best seen height: %w", err)
	}
	stats, err = readCoreTxConfirmStats(br)
	if err != nil {
		return 0, nil, err
	}
	// Core pre-139900 files append deprecated priority TxConfirmStats; skip when present.
	if br.Len() > 0 {
		_, _ = readCoreTxConfirmStats(br)
	}
	if stats != nil && bestSeen >= 0 {
		stats.SetBestSeenHeight(int64(bestSeen))
	}
	return bestSeen, stats, nil
}

// SaveCoreFeeEstimatesDat writes fee_estimates.dat beside fee_history.json when confirm stats exist.
func (h *FeeHistory) SaveCoreFeeEstimatesDat(path string) error {
	if h == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	stats := h.confirmStats
	var best int32
	if stats != nil {
		if stats.bestSeenHeight > math.MaxInt32 {
			best = math.MaxInt32
		} else if stats.bestSeenHeight >= 0 {
			best = int32(stats.bestSeenHeight)
		}
	}
	return WriteCoreFeeEstimatesDat(path, best, stats)
}

// ApplyCoreConfirmStats replaces in-memory TxConfirmStats from a Core dat load.
func (h *FeeHistory) ApplyCoreConfirmStats(bestSeen int32, stats *TxConfirmStats) {
	if h == nil || stats == nil {
		return
	}
	h.mu.Lock()
	h.confirmStats = stats
	if bestSeen >= 0 {
		h.confirmStats.SetBestSeenHeight(int64(bestSeen))
	}
	h.mu.Unlock()
}

func writeCoreTxConfirmStats(w io.Writer, s *TxConfirmStats) error {
	if err := binary.Write(w, binary.LittleEndian, s.decay); err != nil {
		return err
	}
	buckets := make([]float64, len(s.buckets))
	for i, b := range s.buckets {
		buckets[i] = float64(b)
	}
	if err := writeCoreVecDouble(w, buckets); err != nil {
		return err
	}
	if err := writeCoreVecDouble(w, s.avg); err != nil {
		return err
	}
	if err := writeCoreVecDouble(w, s.txCtAvg); err != nil {
		return err
	}
	nConf := uint64(len(s.confAvg))
	if err := writeCoreCompactSize(w, nConf); err != nil {
		return err
	}
	for _, row := range s.confAvg {
		if err := writeCoreVecDouble(w, row); err != nil {
			return err
		}
	}
	return nil
}

func readCoreTxConfirmStats(r io.Reader) (*TxConfirmStats, error) {
	var decay float64
	if err := binary.Read(r, binary.LittleEndian, &decay); err != nil {
		return nil, fmt.Errorf("fee_estimates.dat: decay: %w", err)
	}
	if decay <= 0 || decay >= 1 {
		return nil, fmt.Errorf("fee_estimates.dat: corrupt decay")
	}
	fileBuckets, err := readCoreVecDouble(r)
	if err != nil {
		return nil, err
	}
	numBuckets := len(fileBuckets)
	if numBuckets <= 1 || numBuckets > 1000 {
		return nil, fmt.Errorf("fee_estimates.dat: invalid bucket count %d", numBuckets)
	}
	fileAvg, err := readCoreVecDouble(r)
	if err != nil {
		return nil, err
	}
	if len(fileAvg) != numBuckets {
		return nil, fmt.Errorf("fee_estimates.dat: avg bucket mismatch")
	}
	fileTxCtAvg, err := readCoreVecDouble(r)
	if err != nil {
		return nil, err
	}
	if len(fileTxCtAvg) != numBuckets {
		return nil, fmt.Errorf("fee_estimates.dat: tx count bucket mismatch")
	}
	maxConfirms, err := readCoreCompactSize(r)
	if err != nil {
		return nil, err
	}
	if maxConfirms == 0 || maxConfirms > 6*24*7 {
		return nil, fmt.Errorf("fee_estimates.dat: invalid confirm rows %d", maxConfirms)
	}
	fileConfAvg := make([][]float64, maxConfirms)
	for i := uint64(0); i < maxConfirms; i++ {
		row, err := readCoreVecDouble(r)
		if err != nil {
			return nil, err
		}
		if len(row) != numBuckets {
			return nil, fmt.Errorf("fee_estimates.dat: conf row %d bucket mismatch", i)
		}
		fileConfAvg[i] = row
	}
	bounds := make([]uint64, numBuckets)
	for i, b := range fileBuckets {
		if b < 0 || b > float64(math.MaxUint64) {
			return nil, fmt.Errorf("fee_estimates.dat: invalid bucket boundary")
		}
		bounds[i] = uint64(b)
	}
	s := newTxConfirmStats(bounds, int(maxConfirms), decay)
	copy(s.avg, fileAvg)
	copy(s.txCtAvg, fileTxCtAvg)
	for i := range s.confAvg {
		if i < len(fileConfAvg) {
			copy(s.confAvg[i], fileConfAvg[i])
		}
	}
	return s, nil
}

func writeCoreCompactSize(w io.Writer, n uint64) error {
	switch {
	case n < 253:
		_, err := w.Write([]byte{byte(n)})
		return err
	case n < 0x10000:
		b := []byte{0xfd, byte(n), byte(n >> 8)}
		_, err := w.Write(b)
		return err
	case n < 0x100000000:
		var buf [5]byte
		buf[0] = 0xfe
		binary.LittleEndian.PutUint32(buf[1:], uint32(n))
		_, err := w.Write(buf[:])
		return err
	default:
		var buf [9]byte
		buf[0] = 0xff
		binary.LittleEndian.PutUint64(buf[1:], n)
		_, err := w.Write(buf[:])
		return err
	}
}

func readCoreCompactSize(r io.Reader) (uint64, error) {
	var b [1]byte
	if _, err := io.ReadFull(r, b[:]); err != nil {
		return 0, err
	}
	switch b[0] {
	case 253:
		var buf [2]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint16(buf[:])), nil
	case 254:
		var buf [4]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		return uint64(binary.LittleEndian.Uint32(buf[:])), nil
	case 255:
		var buf [8]byte
		if _, err := io.ReadFull(r, buf[:]); err != nil {
			return 0, err
		}
		return binary.LittleEndian.Uint64(buf[:]), nil
	default:
		return uint64(b[0]), nil
	}
}

func writeCoreVecDouble(w io.Writer, v []float64) error {
	if err := writeCoreCompactSize(w, uint64(len(v))); err != nil {
		return err
	}
	for _, f := range v {
		if err := binary.Write(w, binary.LittleEndian, f); err != nil {
			return err
		}
	}
	return nil
}

func readCoreVecDouble(r io.Reader) ([]float64, error) {
	n, err := readCoreCompactSize(r)
	if err != nil {
		return nil, err
	}
	if n > 1000 {
		return nil, fmt.Errorf("fee_estimates.dat: vector too long")
	}
	out := make([]float64, n)
	for i := uint64(0); i < n; i++ {
		if err := binary.Read(r, binary.LittleEndian, &out[i]); err != nil {
			return nil, fmt.Errorf("fee_estimates.dat: vector element: %w", err)
		}
	}
	return out, nil
}
