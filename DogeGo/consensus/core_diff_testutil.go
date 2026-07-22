// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
)

func stripUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

func loadJSONFixture(t *testing.T, name string, dest any) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = stripUTF8BOM(raw)
	if err := json.Unmarshal(raw, dest); err != nil {
		t.Fatal(err)
	}
}

func networkFromFixture(s string) (chain.Network, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "mainnet", "main", "dogecoin":
		return chain.MainnetDogecoin, nil
	case "reboottestnet", "reboot_testnet", "testnet":
		return chain.RebootTestnet, nil
	default:
		return 0, fmt.Errorf("unsupported network %q", s)
	}
}

func parseU32Hex(s string) (uint32, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 0, fmt.Errorf("empty hex value")
	}
	v, err := strconv.ParseUint(s, 0, 32)
	if err != nil {
		return 0, err
	}
	return uint32(v), nil
}

func genesisHeader80(t *testing.T, net chain.Network) []byte {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	g80, err := pow.Header80FromParams(p)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(nil), g80[:]...)
}

func putHeaderTime(h []byte, ts uint32) {
	h[68] = byte(ts)
	h[69] = byte(ts >> 8)
	h[70] = byte(ts >> 16)
	h[71] = byte(ts >> 24)
}

type headerBuildStep struct {
	LinkPrev             bool   `json:"link_prev"`
	Time                 uint32 `json:"time"`
	SetBitsFromConsensus bool   `json:"set_bits_from_consensus"`
	NonceXor             int    `json:"nonce_xor"`
	VersionXor           int    `json:"version_xor"`
}

func buildHeaderChain(t *testing.T, net chain.Network, steps []headerBuildStep) [][]byte {
	t.Helper()
	if len(steps) == 0 {
		t.Fatal("empty header chain steps")
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	out := make([][]byte, 0, len(steps))
	var prev []byte
	for i, st := range steps {
		var h []byte
		if i == 0 {
			h = genesisHeader80(t, net)
		} else {
			h = appendFromStep(t, p, out, prev, st)
		}
		if i == 0 && st.Time != 0 {
			putHeaderTime(h, st.Time)
		}
		prev = h
		out = append(out, h)
	}
	return out
}

func buildHeadersFromJournalTip(t *testing.T, net chain.Network, j *store.HeaderJournal, steps []headerBuildStep) [][]byte {
	t.Helper()
	if len(steps) == 0 {
		return nil
	}
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	prev, err := j.ReadHeaderAt(tip)
	if err != nil {
		t.Fatal(err)
	}
	prefix := make([][]byte, 0, tip+1)
	for h := int64(0); h <= tip; h++ {
		b, err := j.ReadHeaderAt(h)
		if err != nil {
			t.Fatal(err)
		}
		prefix = append(prefix, append([]byte(nil), b...))
	}
	out := make([][]byte, 0, len(steps))
	for _, st := range steps {
		h := appendFromStep(t, p, prefix, prev, st)
		prev = h
		out = append(out, h)
		prefix = append(prefix, h)
	}
	return out
}

func appendFromStep(t *testing.T, p chain.Params, prefix [][]byte, prev []byte, st headerBuildStep) []byte {
	t.Helper()
	h := append([]byte(nil), prev...)
	if st.LinkPrev {
		prevHash := pow.BlockHashLE(prev)
		copy(h[4:36], prevHash[:])
	}
	if st.VersionXor != 0 {
		ver := binary.LittleEndian.Uint32(h[0:4]) ^ uint32(st.VersionXor)
		binary.LittleEndian.PutUint32(h[0:4], ver)
	}
	if st.NonceXor != 0 {
		h[76] ^= byte(st.NonceXor)
	}
	if st.Time != 0 {
		putHeaderTime(h, st.Time)
	}
	if st.SetBitsFromConsensus {
		dir := t.TempDir()
		j, err := openTestJournal(dir, prefix)
		if err != nil {
			t.Fatal(err)
		}
		tip := int64(len(prefix) - 1)
		v := &batchView{j: j, tip0: tip}
		exp, err := getNextWorkRequired(v, tip, st.Time, LookupConsensus(p.Net, tip))
		if err != nil {
			t.Fatal(err)
		}
		binary.LittleEndian.PutUint32(h[72:76], exp)
	}
	return h
}

func openTestJournal(dir string, headers [][]byte) (*store.HeaderJournal, error) {
	if len(headers) == 0 {
		return nil, fmt.Errorf("no headers")
	}
	path := filepath.Join(dir, "h.bin")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	j, err := store.OpenHeaderJournal(path, headers[0])
	if err != nil {
		return nil, err
	}
	if len(headers) > 1 {
		if err := j.AppendHeaders(headers[1:]); err != nil {
			return nil, err
		}
	}
	return j, nil
}

func openTestJournalInDir(t *testing.T, dir string, headers [][]byte) *store.HeaderJournal {
	t.Helper()
	j, err := openTestJournal(dir, headers)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func applyHeaderMutations(h []byte, muts []headerMutation) {
	for _, m := range muts {
		if m.Offset < 0 || m.Offset >= len(h) {
			continue
		}
		h[m.Offset] ^= byte(m.Xor)
	}
}

type headerMutation struct {
	Offset int  `json:"offset"`
	Xor    int  `json:"xor"`
	Index  *int `json:"index,omitempty"` // validate_batch: nil = all batch headers; set = one index
}

func assertAcceptReject(t *testing.T, err error, wantAccept bool, wantSubstr string) {
	t.Helper()
	if wantAccept {
		if err != nil {
			t.Fatalf("expected accept, got: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatal("expected reject, got nil error")
	}
	if wantSubstr != "" && !strings.Contains(err.Error(), wantSubstr) {
		t.Fatalf("error %q does not contain %q", err.Error(), wantSubstr)
	}
}

// standardRebootTestnetJournalSteps builds genesis plus (count-1) linked headers for differential fixtures.
func standardRebootTestnetJournalSteps(count int) []headerBuildStep {
	if count < 1 {
		return nil
	}
	steps := make([]headerBuildStep, count)
	steps[0] = headerBuildStep{LinkPrev: false}
	for i := 1; i < count; i++ {
		steps[i] = headerBuildStep{
			LinkPrev:             true,
			Time:                 1747000000 + uint32(i)*1000,
			NonceXor:             i,
			SetBitsFromConsensus: true,
		}
	}
	return steps
}

// standardRebootTestnetBatchSteps builds headers chained from an existing journal tip.
// Batch times must be strictly after the journal tip (MTP window); journalLen is the stored chain length.
func standardRebootTestnetBatchSteps(count, journalLen int) []headerBuildStep {
	if journalLen < 1 {
		journalLen = 1
	}
	base := uint32(1747000000 + journalLen*1000) // journal tip at (journalLen-1)*1000 + genesis base
	steps := make([]headerBuildStep, count)
	for i := 0; i < count; i++ {
		steps[i] = headerBuildStep{
			LinkPrev:             true,
			Time:                 base + uint32(i)*1000,
			NonceXor:             i + 1,
			SetBitsFromConsensus: true,
		}
	}
	return steps
}

func standardRebootTestnetJournalNowUnix(count int) int64 {
	if count < 1 {
		return 1747020000
	}
	return int64(1747000000+(count-1)*1000) + 6000
}

func standardRebootTestnetBatchNowUnix(count, journalLen int) int64 {
	if count < 1 {
		return 1747020000
	}
	if journalLen < 1 {
		journalLen = 1
	}
	lastBatch := int64(1747000000 + journalLen*1000 + (count-1)*1000)
	return lastBatch + 6000
}
