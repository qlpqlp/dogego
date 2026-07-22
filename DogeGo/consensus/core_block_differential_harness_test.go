// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"path/filepath"
	"testing"

	"dogego/chain"
	"dogego/store"
)

func loadCoreBlockVectors(t *testing.T) []coreBlockVector {
	t.Helper()
	var vecs []coreBlockVector
	loadJSONFixture(t, "core_block_vectors.json", &vecs)
	if len(vecs) == 0 {
		t.Fatal("no block differential vectors loaded")
	}
	return vecs
}

// TestCoreBlockDifferentialVectors replays Core-shaped block accept/reject cases against
// CheckBlockPayload and stored-body connect validation.
func TestCoreBlockDifferentialVectors(t *testing.T) {
	for _, v := range loadCoreBlockVectors(t) {
		v := v
		t.Run(v.Name, func(t *testing.T) {
			net, err := networkFromFixture(v.Network)
			if err != nil {
				t.Fatal(err)
			}
			switch v.Kind {
			case "check_block_payload":
				runCheckBlockPayloadVector(t, net, v)
			case "stored_block_bodies":
				runStoredBlockBodiesVector(t, net, v)
			default:
				t.Fatalf("unsupported block vector kind %q", v.Kind)
			}
		})
	}
}

func runCheckBlockPayloadVector(t *testing.T, net chain.Network, v coreBlockVector) {
	t.Helper()
	raw, id, err := blockRawForVector(v)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckBlockPayload(raw, id, v.Height, net)
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}

func runStoredBlockBodiesVector(t *testing.T, net chain.Network, v coreBlockVector) {
	t.Helper()
	raw, hash, err := blockRawForVector(v)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), raw[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableTxIndexing(ix, true)
	if err := rs.Put(hash, raw); err != nil {
		t.Fatal(err)
	}
	endHeight := v.resolvedChainTipHeight()
	if endHeight > 0 {
		prevHash := hash
		for h := int64(1); h <= endHeight; h++ {
			nextRaw, nextHash, err := minimalChainedBlockRaw(prevHash, 1747000000+uint32(h)*60, 2139303+uint32(h))
			if err != nil {
				t.Fatal(err)
			}
			if err := j.AppendHeaders([][]byte{nextRaw[:80]}); err != nil {
				t.Fatal(err)
			}
			if err := rs.Put(nextHash, nextRaw); err != nil {
				t.Fatal(err)
			}
			prevHash = nextHash
		}
	}
	err = ValidateStoredBlockBodies(j, rs, ix, nil, net, 0, endHeight)
	assertAcceptReject(t, err, v.WantAccept, v.WantErrorSubstr)
}
