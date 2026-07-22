// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/store"
	"dogego/wire"
)

type coreHeaderVector struct {
	Name             string            `json:"name"`
	Kind             string            `json:"kind"`
	Network          string            `json:"network"`
	Height           int64             `json:"height"`
	HeaderHex        string            `json:"header_hex,omitempty"`
	Mutations        []headerMutation  `json:"mutations"`
	Journal          []headerBuildStep `json:"journal"`
	JournalLength    int               `json:"journal_length"`
	Batch            []headerBuildStep `json:"batch"`
	BatchLength      int               `json:"batch_length"`
	BatchMutations   []headerMutation  `json:"batch_mutations"`
	JournalMutations []headerMutation  `json:"journal_mutations"`
	Start            int64             `json:"start"`
	End              int64             `json:"end"`
	NowUnix          int64             `json:"now_unix"`
	WantAccept       bool              `json:"want_accept"`
	WantErrorSubstr  string            `json:"want_error_substr"`
}

func (v coreHeaderVector) resolvedJournal() []headerBuildStep {
	if len(v.Journal) > 0 {
		return v.Journal
	}
	if v.JournalLength > 0 {
		return standardRebootTestnetJournalSteps(v.JournalLength)
	}
	return []headerBuildStep{{LinkPrev: false}}
}

func (v coreHeaderVector) resolvedJournalLen() int {
	if v.JournalLength > 0 {
		return v.JournalLength
	}
	if len(v.Journal) > 0 {
		return len(v.Journal)
	}
	return 1
}

func (v coreHeaderVector) resolvedBatch() []headerBuildStep {
	if len(v.Batch) > 0 {
		return v.Batch
	}
	if v.BatchLength > 0 {
		return standardRebootTestnetBatchSteps(v.BatchLength, v.resolvedJournalLen())
	}
	return nil
}

func (v coreHeaderVector) resolvedNowUnix() int64 {
	if v.NowUnix != 0 {
		return v.NowUnix
	}
	if v.BatchLength > 0 {
		return standardRebootTestnetBatchNowUnix(v.BatchLength, v.resolvedJournalLen())
	}
	if len(v.Batch) > 0 {
		return standardRebootTestnetBatchNowUnix(len(v.Batch), v.resolvedJournalLen())
	}
	if v.JournalLength > 0 {
		return standardRebootTestnetJournalNowUnix(v.JournalLength)
	}
	if len(v.Journal) > 0 {
		return standardRebootTestnetJournalNowUnix(len(v.Journal))
	}
	return 1747020000
}

func (v coreHeaderVector) resolvedEnd() int64 {
	if v.End != 0 {
		return v.End
	}
	if v.JournalLength > 0 {
		return int64(v.JournalLength - 1)
	}
	return v.End
}

func loadCoreHeaderVectors(t *testing.T) []coreHeaderVector {
	t.Helper()
	var vecs []coreHeaderVector
	loadJSONFixture(t, "core_header_vectors.json", &vecs)
	if len(vecs) == 0 {
		t.Fatal("no header differential vectors loaded")
	}
	return vecs
}

// TestCoreHeaderDifferentialVectors replays Core-shaped header accept/reject cases:
// checkpoints, stored journal validation, and incoming batch validation.
func TestCoreHeaderDifferentialVectors(t *testing.T) {
	prev := HeaderCheckpointsEnabled()
	SetHeaderCheckpointsEnabled(true)
	t.Cleanup(func() { SetHeaderCheckpointsEnabled(prev) })

	for _, v := range loadCoreHeaderVectors(t) {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			net, err := networkFromFixture(v.Network)
			if err != nil {
				t.Fatal(err)
			}
			switch v.Kind {
			case "checkpoint":
				runHeaderCheckpointVector(t, net, v)
			case "stored_headers":
				runStoredHeadersVector(t, net, v)
			case "segment_stored_headers":
				runSegmentStoredHeadersVector(t, net, v)
			case "validate_batch":
				runValidateBatchVector(t, net, v)
			case "field_header":
				runFieldHeaderVector(t, net, v)
			default:
				t.Fatalf("unsupported vector kind %q", v.Kind)
			}
		})
	}
}

func runHeaderCheckpointVector(t *testing.T, net chain.Network, v coreHeaderVector) {
	t.Helper()
	var h []byte
	if strings.TrimSpace(v.HeaderHex) != "" {
		h = decodeHeaderHexFixture(t, v)
	} else {
		h = genesisHeader80(t, net)
	}
	applyHeaderMutations(h, v.Mutations)
	err := checkHeaderCheckpoint(net, v.Height, h)
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func runFieldHeaderVector(t *testing.T, net chain.Network, v coreHeaderVector) {
	t.Helper()
	h := decodeHeaderHexFixture(t, v)
	err := verifyCommittedFieldHeader(net, v.Height, h)
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func decodeHeaderHexFixture(t *testing.T, v coreHeaderVector) []byte {
	t.Helper()
	hx := strings.TrimSpace(v.HeaderHex)
	if hx == "" {
		t.Fatal("missing header_hex")
	}
	b, err := hex.DecodeString(hx)
	if err != nil {
		t.Fatalf("header_hex decode: %v", err)
	}
	if len(b) != 80 {
		t.Fatalf("header_hex len=%d want 80", len(b))
	}
	return append([]byte(nil), b...)
}

func runStoredHeadersVector(t *testing.T, net chain.Network, v coreHeaderVector) {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	// Differential vectors are about Core-shaped acceptance rules and error strings.
	// Reboot testnet uses real PoW in production, but these fixtures do not mine headers.
	// RelaxedPoW is required so the suite can reach nBits/prev/MTP validation paths.
	if p.IsRebootTestnet() {
		p.RelaxedPoW = true
	}
	headers := buildHeaderChain(t, net, v.resolvedJournal())
	applyJournalMutations(headers, v.JournalMutations)
	dir := t.TempDir()
	j := openTestJournalInDir(t, dir, headers)
	err = ValidateStoredHeaders(j, nil, p, v.Start, v.resolvedEnd(), v.resolvedNowUnix())
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func runSegmentStoredHeadersVector(t *testing.T, net chain.Network, v coreHeaderVector) {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsRebootTestnet() {
		p.RelaxedPoW = true
	}
	headers := buildHeaderChain(t, net, v.resolvedJournal())
	applyJournalMutations(headers, v.JournalMutations)
	dir := t.TempDir()
	j, err := store.OpenHeaderChain(dir, headers[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) > 1 {
		if err := j.AppendHeaders(headers[1:]); err != nil {
			t.Fatal(err)
		}
	}
	if j.HeaderLayout() != "segments" {
		t.Fatalf("layout=%q want segments", j.HeaderLayout())
	}
	err = ValidateStoredHeaders(j, nil, p, v.Start, v.resolvedEnd(), v.resolvedNowUnix())
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func runValidateBatchVector(t *testing.T, net chain.Network, v coreHeaderVector) {
	t.Helper()
	p, err := chain.ParamsFor(net)
	if err != nil {
		t.Fatal(err)
	}
	if p.IsRebootTestnet() {
		p.RelaxedPoW = true
	}
	journal := buildHeaderChain(t, net, v.resolvedJournal())
	j := openTestJournalInDir(t, t.TempDir(), journal)
	batchHeaders := buildHeadersFromJournalTip(t, net, j, v.resolvedBatch())
	for i := range batchHeaders {
		var muts []headerMutation
		for _, m := range v.BatchMutations {
			if m.Index == nil || *m.Index == i {
				muts = append(muts, headerMutation{Offset: m.Offset, Xor: m.Xor})
			}
		}
		applyHeaderMutations(batchHeaders[i], muts)
	}
	decoded := make([]wire.DecodedHeader, len(batchHeaders))
	for i, h := range batchHeaders {
		decoded[i] = wire.DecodedHeader{Header80: h}
	}
	err = ValidateHeaders(j, p, decoded, v.resolvedNowUnix())
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func applyJournalMutations(headers [][]byte, muts []headerMutation) {
	if len(muts) == 0 || len(headers) == 0 {
		return
	}
	for _, m := range muts {
		idx := len(headers) - 1
		if m.Index != nil {
			idx = *m.Index
		}
		if idx < 0 || idx >= len(headers) {
			continue
		}
		applyHeaderMutations(headers[idx], []headerMutation{{Offset: m.Offset, Xor: m.Xor}})
	}
}

// TestCoreHeaderDifferentialSuiteIncludesMTP ensures the differential track also runs the
// existing MTP regression (see TestValidateHeadersRejectsTimeRegression).
func TestCoreHeaderDifferentialSuiteIncludesMTP(t *testing.T) {
	t.Run("mtp_regression", func(t *testing.T) {
		TestValidateHeadersRejectsTimeRegression(t)
	})
}
