// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"dogego/chain"
	"dogego/pow"
	"dogego/wire"
)

type coreBlockFilterVector struct {
	Height          int      `json:"height"`
	BlockHash       string   `json:"block_hash"`
	Block           string   `json:"block"`
	PrevScripts     []string `json:"prev_scripts"`
	BasicFilter     string   `json:"basic_filter"`
	BasicHeader     string   `json:"basic_header"`
	PrevBasicHeader string   `json:"prev_basic_header"`
	Note            string   `json:"note"`
}

func loadCoreBlockFilterVectors(t *testing.T) []coreBlockFilterVector {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "blockfilters_core.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var vecs []coreBlockFilterVector
	if err := json.Unmarshal(raw, &vecs); err != nil {
		t.Fatal(err)
	}
	return vecs
}

func decodeDisplayHash32(display string) ([32]byte, error) {
	return chain.Hash256FromDisplayHex(strings.TrimSpace(display))
}

func buildBasicFilterFromCoreVector(v coreBlockFilterVector) ([]byte, error) {
	blockRaw, err := hex.DecodeString(v.Block)
	if err != nil {
		return nil, err
	}
	pb, err := wire.ParseBlock(blockRaw)
	if err != nil {
		return nil, err
	}
	var ins [][]byte
	for _, s := range v.PrevScripts {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		b, err := hex.DecodeString(s)
		if err != nil {
			return nil, err
		}
		ins = append(ins, b)
	}
	if len(blockRaw) < 80 {
		return nil, fmt.Errorf("short block")
	}
	hashLE := pow.BlockHashLE(blockRaw[:80])
	outs := CollectBasicFilterOutputScripts(pb)
	return BuildBasicGCSFilter(hashLE, outs, ins), nil
}

// TestCoreBasicFilterVectors matches encoded filters against Bitcoin Core blockfilters.json subset.
func TestCoreBasicFilterVectors(t *testing.T) {
	for _, v := range loadCoreBlockFilterVectors(t) {
		t.Run(v.BlockHash, func(t *testing.T) {
			got, err := buildBasicFilterFromCoreVector(v)
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(v.BasicFilter)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != string(want) {
				t.Fatalf("height %d filter mismatch\n got %x\nwant %x\nnote: %s", v.Height, got, want, v.Note)
			}
		})
	}
}

// TestCoreBasicFilterHeaderVectors matches filter header chain against Core vectors.
func TestCoreBasicFilterHeaderVectors(t *testing.T) {
	for _, v := range loadCoreBlockFilterVectors(t) {
		t.Run(v.BlockHash+"_header", func(t *testing.T) {
			enc, err := buildBasicFilterFromCoreVector(v)
			if err != nil {
				t.Fatal(err)
			}
			prev, err := decodeDisplayHash32(v.PrevBasicHeader)
			if err != nil {
				t.Fatal(err)
			}
			hdr := BlockFilterHeader(BlockFilterHash(enc), prev)
			want, err := decodeDisplayHash32(v.BasicHeader)
			if err != nil {
				t.Fatal(err)
			}
			if hdr != want {
				t.Fatalf("height %d header mismatch\n got %x\nwant %x", v.Height, hdr, want)
			}
		})
	}
}
