// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"dogego/chain"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func loadMainnetFieldBlockEntries(t *testing.T) []mainnetFieldBlockEntry {
	t.Helper()
	path := filepath.Join("testdata", "mainnet_field_blocks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = stripUTF8BOM(raw)
	var entries []mainnetFieldBlockEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	if len(entries) < 1 {
		t.Fatalf("mainnet_field_blocks.json need >=1 entry, got %d", len(entries))
	}
	return entries
}

func openMainnetFieldHeaderChain(t *testing.T) (*store.HeaderJournal, *store.HeaderAuxJournal, chain.Params) {
	t.Helper()
	chainDir := MainnetFieldDataDir()
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		t.Skipf("no local mainnet header chain: %v", err)
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	var aux *store.HeaderAuxJournal
	auxPath := filepath.Join(chainDir, "headers_aux.bin")
	if _, err := os.Stat(auxPath); err == nil {
		aux, err = store.OpenHeaderAuxJournal(auxPath, tip+1)
		if err != nil {
			t.Fatalf("open headers_aux: %v", err)
		}
	}
	t.Logf("mainnet field datadir=%s tip=%d layout=%s", chainDir, tip, j.HeaderLayout())
	return j, aux, p
}

// TestCoreMainnetFieldHeaderPoW validates real mainnet headers from operator dogedata (field evidence).
func TestCoreMainnetFieldHeaderPoW(t *testing.T) {
	j, aux, p := openMainnetFieldHeaderChain(t)
	tip, err := j.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	windows := []struct {
		name  string
		start int64
		end   int64
	}{
		{"early_legacy_1_32", 1, 32},
		{"legacy_100_132", 100, 132},
		{"legacy_10000_10032", 10000, 10032},
	}
	if tip >= 371337 {
		windows = append(windows, struct {
			name  string
			start int64
			end   int64
		}{"auxpow_activation_371337_371340", 371337, 371340})
	}
	if tip > 64 {
		windows = append(windows, struct {
			name  string
			start int64
			end   int64
		}{"recent_tip_window", tip - 32, tip})
	}
	for _, w := range windows {
		w := w
		if w.end > tip {
			w.end = tip
		}
		if w.start > w.end {
			continue
		}
		t.Run(w.name, func(t *testing.T) {
			if err := ValidateStoredHeaders(j, aux, p, w.start, w.end, now); err != nil {
				t.Fatalf("ValidateStoredHeaders %d..%d: %v", w.start, w.end, err)
			}
		})
	}
}

// TestCoreMainnetFieldBlockHexVectors runs CheckBlockPayload on committed mainnet_field_blocks.json hex.
func TestCoreMainnetFieldBlockHexVectors(t *testing.T) {
	genHex := strings.ToUpper(hex.EncodeToString(mustMainnetGenesis(t)))
	entries := loadMainnetFieldBlockEntries(t)
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		e := e
		hexU := strings.ToUpper(strings.TrimSpace(e.Hex))
		if e.Height > 0 && hexU == genHex {
			t.Run(fmt.Sprintf("height_%d", e.Height), func(t *testing.T) {
				t.Skip("placeholder genesis hex - run scripts/export_mainnet_field_blocks.ps1")
			})
			continue
		}
		t.Run(fmt.Sprintf("height_%d", e.Height), func(t *testing.T) {
			t.Parallel()
			decoded, err := hex.DecodeString(e.Hex)
			if err != nil {
				t.Fatal(err)
			}
			id := pow.BlockHashLE(decoded[:80])
			if err := CheckBlockPayload(decoded, id, e.Height, p.Net); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestCoreMainnetFieldBlockCheckBlock runs CheckBlock on parsed committed field blocks (full parse path).
func TestCoreMainnetFieldBlockCheckBlock(t *testing.T) {
	genHex := strings.ToUpper(hex.EncodeToString(mustMainnetGenesis(t)))
	entries := loadMainnetFieldBlockEntries(t)
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		e := e
		if e.Height <= 0 {
			continue
		}
		hexU := strings.ToUpper(strings.TrimSpace(e.Hex))
		if hexU == genHex {
			continue
		}
		t.Run(fmt.Sprintf("height_%d", e.Height), func(t *testing.T) {
			t.Parallel()
			decoded, err := hex.DecodeString(e.Hex)
			if err != nil {
				t.Fatal(err)
			}
			pb, err := wire.ParseBlock(decoded)
			if err != nil {
				t.Fatal(err)
			}
			if err := CheckBlock(pb, e.Height, p.Net); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestCoreMainnetFieldMultiTxBlock15504 validates the committed multi-tx mainnet field block (Milestone A corpus).
func TestCoreMainnetFieldMultiTxBlock15504(t *testing.T) {
	const wantHeight = mainnetFieldMultiTxBlockHeight
	entry, err := mainnetFieldMultiTxBlock15504Entry()
	if err != nil {
		t.Skipf("mainnet field block height %d not committed - %v", wantHeight, err)
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(entry.Hex))
	if err != nil {
		t.Fatal(err)
	}
	pb, err := wire.ParseBlock(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(pb.Txs) < 2 {
		t.Fatalf("height %d tx count=%d want multi-tx block", wantHeight, len(pb.Txs))
	}
	p, err := chain.ParamsFor(chain.MainnetDogecoin)
	if err != nil {
		t.Fatal(err)
	}
	if err := CheckBlock(pb, wantHeight, p.Net); err != nil {
		t.Fatal(err)
	}
}

// TestCoreMainnetFieldCanonicalHeaderPoW verifies scrypt PoW on canonical mainnet header80 rows.
func TestCoreMainnetFieldCanonicalHeaderPoW(t *testing.T) {
	for _, spec := range mainnetCanonicalBlockSpecs {
		spec := spec
		t.Run(fmt.Sprintf("height_%d", spec.Height), func(t *testing.T) {
			raw, err := buildMainnetCanonicalBlockRaw(spec)
			if err != nil {
				t.Fatal(err)
			}
			if err := verifyFieldHeaderPoW(chain.MainnetDogecoin, spec.Height, raw[:80]); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func mustMainnetGenesis(t *testing.T) []byte {
	t.Helper()
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		t.Fatal(err)
	}
	return gen
}

func mainnetFieldBlockPayloads(t *testing.T, minHeight, maxHeight int64) map[int64][]byte {
	t.Helper()
	gen := mustMainnetGenesis(t)
	genHex := strings.ToUpper(hex.EncodeToString(gen))
	entries := loadMainnetFieldBlockEntries(t)
	out := map[int64][]byte{}
	for _, e := range entries {
		if e.Height < minHeight || e.Height > maxHeight {
			continue
		}
		hexU := strings.ToUpper(strings.TrimSpace(e.Hex))
		if hexU == genHex {
			continue
		}
		decoded, err := hex.DecodeString(strings.TrimSpace(e.Hex))
		if err != nil {
			t.Fatalf("height %d: %v", e.Height, err)
		}
		out[e.Height] = decoded
	}
	return out
}

func runMainnetFieldStoredConnect(t *testing.T, maxHeight int64) {
	t.Helper()
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, maxHeight)
	if len(byHeight) == 0 {
		t.Skip("no real mainnet field blocks in testdata - run scripts/export_mainnet_field_blocks.ps1 -CanonicalOnly")
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:80])
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
	genID := pow.BlockHashLE(gen[:80])
	if err := rs.Put(genID, gen); err != nil {
		t.Fatal(err)
	}
	var tip int64
	for h := int64(1); h <= maxHeight; h++ {
		raw, ok := byHeight[h]
		if !ok {
			break
		}
		if err := j.AppendHeaders([][]byte{raw[:80]}); err != nil {
			t.Fatal(err)
		}
		id := pow.BlockHashLE(raw[:80])
		if err := rs.Put(id, raw); err != nil {
			t.Fatal(err)
		}
		tip = h
	}
	if tip == 0 {
		t.Skip("no contiguous field blocks from height 1")
	}
	if err := ValidateStoredBlockBodies(j, rs, ix, nil, chain.MainnetDogecoin, 0, tip); err != nil {
		t.Fatal(err)
	}
}

func runMainnetFieldBundledStoredConnect(t *testing.T, maxHeight int64) {
	t.Helper()
	gen := mustMainnetGenesis(t)
	byHeight := mainnetFieldBlockPayloads(t, 1, maxHeight)
	if len(byHeight) == 0 {
		t.Skip("no real mainnet field blocks in testdata")
	}
	dir := t.TempDir()
	j, err := store.OpenHeaderJournal(filepath.Join(dir, "headers.bin"), gen[:80])
	if err != nil {
		t.Fatal(err)
	}
	rs, err := store.OpenRawBlockStoreWithOpts(dir, store.BlockStorageOpts{Layout: store.BlockLayoutBundled, Zstd: false})
	if err != nil {
		t.Fatal(err)
	}
	ix, err := store.OpenTxIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	rs.EnableTxIndexing(ix, true)
	if err := rs.Put(pow.BlockHashLE(gen[:80]), gen); err != nil {
		t.Fatal(err)
	}
	var tip int64
	for h := int64(1); h <= maxHeight; h++ {
		raw, ok := byHeight[h]
		if !ok {
			break
		}
		if err := j.AppendHeaders([][]byte{raw[:80]}); err != nil {
			t.Fatal(err)
		}
		if err := rs.Put(pow.BlockHashLE(raw[:80]), raw); err != nil {
			t.Fatal(err)
		}
		tip = h
	}
	if tip == 0 {
		t.Skip("no contiguous field blocks from height 1")
	}
	if err := ValidateStoredBlockBodies(j, rs, ix, nil, chain.MainnetDogecoin, 0, tip); err != nil {
		t.Fatal(err)
	}
	cont, err := rs.ProbeBundledContiguousTip()
	if err != nil || cont != tip {
		t.Fatalf("bundled contiguous tip=%d err=%v want %d", cont, err, tip)
	}
}

// TestCoreMainnetFieldStoredBlockConnect connects real mainnet genesis + field blocks 1-3 (CheckBlock + ConnectBlockRaw).
func TestCoreMainnetFieldStoredBlockConnect(t *testing.T) {
	runMainnetFieldStoredConnect(t, 3)
}

// TestCoreMainnetFieldConnectCorpus is the Milestone A exit gate: contiguous stored connect (0-3) plus sparse coinbase connect at later heights.
func TestCoreMainnetFieldConnectCorpus(t *testing.T) {
	t.Run("contiguous_stored_0_3", func(t *testing.T) {
		runMainnetFieldStoredConnect(t, 3)
	})
	t.Run("contiguous_bundled_stored_0_3", func(t *testing.T) {
		runMainnetFieldBundledStoredConnect(t, 3)
	})
	t.Run("sparse_coinbase_high_heights", func(t *testing.T) {
		byHeight := mainnetFieldBlockPayloads(t, 100, 10006)
		for _, h := range []int64{100, 200, 272, 10006} {
			h := h
			raw, ok := byHeight[h]
			if !ok {
				t.Fatalf("missing field block height %d", h)
			}
			t.Run(fmt.Sprintf("height_%d", h), func(t *testing.T) {
				if err := CheckBlockCoinbaseSubsidyPayload(raw, h, chain.MainnetDogecoin, nil); err != nil {
					t.Fatal(err)
				}
				if err := ConnectSparseCoinbaseBlockRaw(raw, h, chain.MainnetDogecoin); err != nil {
					t.Fatal(err)
				}
			})
		}
	})
}

// TestCoreMainnetFieldDiskBundledConnect validates stored bodies on operator dogedata when bundled chain is present.
func TestCoreMainnetFieldDiskBundledConnect(t *testing.T) {
	dc, err := OpenMainnetFieldDiskChain()
	if err != nil {
		t.Skip(err)
	}
	tip, err := dc.Journal.TipHeight()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("datadir=%s header_tip=%d bundled_contiguous=%d", dc.ChainDir, tip, dc.BundledContiguous)
	cases := MainnetFieldDiskConnectCases(dc.BundledContiguous)
	if len(cases) == 0 {
		t.Skipf("bundled contiguous tip=%d want >=3", dc.BundledContiguous)
	}
	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			if testing.Short() && c.End > 272 {
				t.Skip("short mode: skip large disk connect tier (set -short=false or DOGEGO_FIELD_DISK_CONNECT_MAX)")
			}
			if err := ValidateStoredBlockBodies(dc.Journal, dc.Raw, dc.TxIndex, nil, chain.MainnetDogecoin, 0, c.End); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("measure_contiguous_matches_probe", func(t *testing.T) {
		probe, err := dc.Raw.ProbeBundledContiguousTip()
		if err != nil {
			t.Skip(err) // per-height rawblocks layout has no bundled blk*.dat scan
		}
		measured := store.MeasureContiguousBodiesOnDisk(dc.Journal, dc.Raw, chain.MainnetDogecoin, 0, 0)
		reconciled := store.ReconcileBundledContiguousTip(dc.Journal, dc.Raw, chain.MainnetDogecoin)
		want := probe
		if measured >= 0 && (probe < 0 || measured < probe) {
			want = measured
		}
		if reconciled != want {
			t.Fatalf("reconciled=%d want min(probe=%d, measured=%d)", reconciled, probe, measured)
		}
		if probe != measured {
			t.Logf("bundled tip drift: ProbeBundledContiguousTip=%d MeasureContiguousBodiesOnDisk=%d (reconciled=%d)", probe, measured, reconciled)
		}
		if dc.BundledContiguous != reconciled {
			t.Logf("open bundled_contiguous=%d differs from end-of-test reconciled=%d (datadir may have grown during connect)", dc.BundledContiguous, reconciled)
		}
	})
}

// TestMainnetFieldHeadersMatchCanonical ensures field_header rows match canonical header80 bytes.
func TestMainnetFieldHeadersMatchCanonical(t *testing.T) {
	byHeight := map[int64]string{}
	for _, v := range loadCoreHeaderVectors(t) {
		if v.Kind != "field_header" {
			continue
		}
		byHeight[v.Height] = strings.ToUpper(strings.TrimSpace(v.HeaderHex))
	}
	for _, spec := range mainnetCanonicalBlockSpecs {
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			t.Fatalf("height %d: %v", spec.Height, err)
		}
		want := strings.ToUpper(hex.EncodeToString(raw[:80]))
		got, ok := byHeight[spec.Height]
		if !ok {
			t.Fatalf("core_header_vectors missing field_header height %d (run UPDATE_CORE_TESTDATA=1)", spec.Height)
		}
		if got != want {
			t.Fatalf("height %d header_hex mismatch with canonical", spec.Height)
		}
	}
}

// TestMainnetFieldBlocksMatchCanonical ensures committed field blocks match verified canonical specs.
func TestMainnetFieldBlocksMatchCanonical(t *testing.T) {
	canon, err := mainnetCanonicalFieldBlocks()
	if err != nil {
		t.Fatal(err)
	}
	byHeight := map[int64]string{}
	for _, e := range loadMainnetFieldBlockEntries(t) {
		byHeight[e.Height] = strings.ToUpper(strings.TrimSpace(e.Hex))
	}
	for _, e := range canon {
		got, ok := byHeight[e.Height]
		if !ok {
			t.Fatalf("mainnet_field_blocks.json missing canonical height %d", e.Height)
		}
		want := strings.ToUpper(strings.TrimSpace(e.Hex))
		if got != want {
			t.Fatalf("height %d hex mismatch with canonical (run UPDATE_CORE_TESTDATA=1)", e.Height)
		}
	}
}

// TestCoreMainnetCheckpointHeaderAccept verifies committed checkpoint rows accept Core mapCheckpoints hashes.
func TestCoreMainnetCheckpointHeaderAccept(t *testing.T) {
	for _, v := range loadCoreHeaderVectors(t) {
		if v.Kind != "checkpoint" || v.Network != "mainnet" || !v.WantAccept {
			continue
		}
		v := v
		t.Run(v.Name, func(t *testing.T) {
			var h80 []byte
			if hx := strings.TrimSpace(v.HeaderHex); hx != "" {
				b, err := hex.DecodeString(hx)
				if err != nil {
					t.Fatal(err)
				}
				if len(b) != 80 {
					t.Fatalf("header_hex len=%d", len(b))
				}
				h80 = b
			} else if v.Height == 0 {
				h80 = genesisHeader80(t, chain.MainnetDogecoin)
			} else {
				t.Skip("no header_hex for non-genesis checkpoint")
			}
			if err := checkHeaderCheckpoint(chain.MainnetDogecoin, v.Height, h80); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// TestCoreMainnetFieldHeaderVectorsLegacyPoW verifies scrypt PoW on committed field_header rows (pre-auxpow).
func TestCoreMainnetFieldHeaderVectorsLegacyPoW(t *testing.T) {
	for _, v := range loadCoreHeaderVectors(t) {
		if v.Kind != "field_header" || v.Network != "mainnet" || !v.WantAccept {
			continue
		}
		if strings.TrimSpace(v.HeaderHex) == "" {
			t.Fatalf("%s missing header_hex", v.Name)
		}
		h := decodeHeaderHexFixture(t, v)
		if isAuxpowVersionU(nVersionLE(h)) {
			continue
		}
		v := v
		t.Run(v.Name, func(t *testing.T) {
			if err := verifyFieldHeaderPoW(chain.MainnetDogecoin, v.Height, h); err != nil {
				t.Fatal(err)
			}
			if _, ok := chain.CheckpointHashAt(chain.MainnetDogecoin, v.Height); ok {
				if err := checkHeaderCheckpoint(chain.MainnetDogecoin, v.Height, h); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

// TestCoreMainnetCheckpointHeaderRejectCorpus verifies genesis-shaped headers reject at non-genesis checkpoint heights.
func TestCoreMainnetCheckpointHeaderRejectCorpus(t *testing.T) {
	var n int
	for _, v := range loadCoreHeaderVectors(t) {
		if v.Kind != "checkpoint" || v.Network != "mainnet" || v.WantAccept || v.Height == 0 {
			continue
		}
		if strings.TrimSpace(v.WantErrorSubstr) == "" {
			continue
		}
		v := v
		t.Run(v.Name, func(t *testing.T) {
			h := genesisHeader80(t, chain.MainnetDogecoin)
			applyHeaderMutations(h, v.Mutations)
			err := checkHeaderCheckpoint(chain.MainnetDogecoin, v.Height, h)
			if err == nil {
				t.Fatal("expected checkpoint reject")
			}
			if !strings.Contains(err.Error(), v.WantErrorSubstr) {
				t.Fatalf("err=%q want substr %q", err.Error(), v.WantErrorSubstr)
			}
		})
		n++
	}
	if n < 10 {
		t.Fatalf("checkpoint reject rows=%d want >=10", n)
	}
}

// TestCoreMainnetCheckpointHeaderLegacyPoW verifies scrypt PoW on committed pre-auxpow checkpoint header_hex rows.
func TestCoreMainnetCheckpointHeaderLegacyPoW(t *testing.T) {
	var n int
	for _, v := range loadCoreHeaderVectors(t) {
		if v.Kind != "checkpoint" || v.Network != "mainnet" || !v.WantAccept {
			continue
		}
		if strings.TrimSpace(v.HeaderHex) == "" || v.Height == 0 {
			continue
		}
		h := decodeHeaderHexFixture(t, v)
		if isAuxpowVersionU(nVersionLE(h)) {
			continue
		}
		v := v
		t.Run(v.Name, func(t *testing.T) {
			if err := verifyFieldHeaderPoW(chain.MainnetDogecoin, v.Height, h); err != nil {
				t.Fatal(err)
			}
		})
		n++
	}
	if n < 2 {
		t.Fatalf("legacy checkpoint PoW rows=%d want >=2 (104679, 145000)", n)
	}
}

// TestCoreMainnetFieldAuxpowHeaderCheckpoint verifies auxpow-era field_header rows match checkpoints or committed auxpow proofs.
func TestCoreMainnetFieldAuxpowHeaderCheckpoint(t *testing.T) {
	var heights []int64
	for _, h := range []int64{371337, 371338, 371339} {
		var found bool
		for _, v := range loadCoreHeaderVectors(t) {
			if v.Kind != "field_header" || v.Network != "mainnet" || v.Height != h {
				continue
			}
			found = true
			hdr := decodeHeaderHexFixture(t, v)
			if !isAuxpowVersionU(nVersionLE(hdr)) {
				t.Fatalf("height %d field_header must be auxpow version", h)
			}
			if err := verifyCommittedFieldHeader(chain.MainnetDogecoin, h, hdr); err != nil {
				t.Fatalf("height %d: %v", h, err)
			}
		}
		if !found {
			t.Fatalf("core_header_vectors missing mainnet_field_header_%d", h)
		}
		heights = append(heights, h)
	}
	if len(heights) != 3 {
		t.Fatalf("auxpow field_header heights=%v", heights)
	}
}

// TestCoreMainnetFieldAuxpowOfflineValidate checks committed auxpow proofs (activation window) without operator datadir.
func TestCoreMainnetFieldAuxpowOfflineValidate(t *testing.T) {
	entries, err := LoadMainnetFieldAuxpowEntries()
	if err != nil {
		t.Fatalf("mainnet_field_auxpow.json: %v (export with DOGEGO_FIELD_DATADIR + UPDATE_CORE_TESTDATA=1)", err)
	}
	var at371337 bool
	for _, e := range entries {
		e := e
		t.Run(fmt.Sprintf("height_%d", e.Height), func(t *testing.T) {
			h80, err := hex.DecodeString(strings.TrimSpace(e.HeaderHex))
			if err != nil || len(h80) != 80 {
				t.Fatalf("header_hex: %v len=%d", err, len(h80))
			}
			auxB, err := hex.DecodeString(strings.TrimSpace(e.AuxHex))
			if err != nil || len(auxB) == 0 {
				t.Fatalf("aux_hex: %v len=%d", err, len(auxB))
			}
			if !isAuxpowVersionU(nVersionLE(h80)) {
				t.Fatal("expected auxpow version bit")
			}
			ap, err := wire.ReadAuxPow(bytes.NewReader(auxB))
			if err != nil {
				t.Fatal(err)
			}
			dc := LookupConsensus(chain.MainnetDogecoin, e.Height)
			if err := checkAuxPow(h80, ap, dc); err != nil {
				t.Fatal(err)
			}
			if err := checkHeaderCheckpoint(chain.MainnetDogecoin, e.Height, h80); err != nil {
				t.Fatal(err)
			}
		})
		if e.Height == 371337 {
			at371337 = true
		}
	}
	if !at371337 {
		t.Fatal("mainnet_field_auxpow.json missing height 371337")
	}
}

// TestMainnetFieldBlocksMatchBlockVectors keeps core_block_vectors.json mainnet hex rows aligned with mainnet_field_blocks.json.
func TestMainnetFieldBlocksMatchBlockVectors(t *testing.T) {
	path := filepath.Join("testdata", "mainnet_field_blocks.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = stripUTF8BOM(raw)
	var entries []mainnetFieldBlockEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	byHeight := map[int64]string{}
	for _, e := range entries {
		byHeight[e.Height] = strings.ToUpper(strings.TrimSpace(e.Hex))
	}
	vecs := loadCoreBlockVectors(t)
	for _, v := range vecs {
		if v.Network != "mainnet" || v.Source != "hex" || v.Height <= 0 {
			continue
		}
		want, ok := byHeight[v.Height]
		if !ok {
			continue
		}
		got := strings.ToUpper(strings.TrimSpace(v.Hex))
		if got != want {
			t.Fatalf("vector %q height %d hex mismatch with mainnet_field_blocks.json", v.Name, v.Height)
		}
	}
}
