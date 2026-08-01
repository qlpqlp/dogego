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
	"sort"
	"strings"
	"testing"

	"dogego/secp256k1"

	"dogego/chain"
	"dogego/mempool"
	"dogego/pow"
	"dogego/store"
	"dogego/wire"
)

func testdataDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "testdata"
	}
	return filepath.Join(filepath.Dir(file), "testdata")
}

func writeJSONFixture(name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	dir := testdataDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0o644)
}

// TestUpdateCoreTestdata regenerates consensus/testdata/*.json (UPDATE_CORE_TESTDATA=1).
func TestCatalogMainnetFieldHeaderVectorsProbe(t *testing.T) {
	vecs := catalogMainnetFieldHeaderVectors()
	t.Logf("datadir=%s field_headers=%d", mainnetFieldDataDir(), len(vecs))
	for _, v := range vecs {
		t.Log(v.Name, v.Height)
	}
	if len(vecs) == 0 {
		t.Skip("no local mainnet headers for field export (set DOGEGO_FIELD_DATADIR)")
	}
}

func TestCatalogMainnetFieldBlocksProbe(t *testing.T) {
	gen, _ := chain.MainnetGenesisBlockRaw()
	genHex := strings.ToUpper(hex.EncodeToString(gen))
	chainDir := mainnetFieldDataDir()
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		t.Skip(err)
	}
	tip, _ := j.TipHeight()
	t.Logf("datadir=%s header_tip=%d", chainDir, tip)
	if diskTip, err := store.OpenRawBlockStore(chainDir); err == nil {
		if dt, derr := diskTip.ProbeBundledContiguousTip(); derr == nil {
			t.Logf("bundled_disk_contiguous_tip=%d", dt)
		}
	}
	for _, h := range []int64{1, 2, 3, 100, 200, 272} {
		hdr, err := j.ReadHeaderAt(h)
		if err != nil {
			t.Logf("height %d header: %v", h, err)
			continue
		}
		rs, err := store.OpenRawBlockStore(chainDir)
		if err != nil {
			t.Fatal(err)
		}
		hash := pow.BlockHashLE(hdr)
		raw, err := rs.Get(hash)
		t.Logf("height %d get err=%v len=%d", h, err, len(raw))
		if cr, cerr := rs.GetByContiguousHeight(h); cerr == nil && len(cr) >= 80 {
			jh := pow.BlockHashLE(hdr)
			bh := pow.BlockHashLE(cr[:80])
			t.Logf("height %d contiguous len=%d journal_hash=%x body_hash=%x match=%v", h, len(cr), jh[:4], bh[:4], jh == bh)
		} else if cerr != nil {
			t.Logf("height %d contiguous err=%v", h, cerr)
		}
	}
	blocks, err := catalogMainnetFieldBlocks()
	if err != nil {
		t.Fatal(err)
	}
	var real int
	for _, e := range blocks {
		if e.Height > 0 && strings.ToUpper(e.Hex) != genHex {
			real++
		}
	}
	t.Logf("catalog entries=%d real=%d", len(blocks), real)
	if real == 0 {
		t.Skip("no raw blocks on disk")
	}
}

func TestUpdateCoreTestdata(t *testing.T) {
	if os.Getenv("UPDATE_CORE_TESTDATA") != "1" {
		t.Skip("set UPDATE_CORE_TESTDATA=1 to regenerate consensus/testdata")
	}
	if err := exportAllCoreTestdata(); err != nil {
		t.Fatal(err)
	}
}

func exportAllCoreTestdata() error {
	if err := writeJSONFixture("core_mempool_vectors.json", catalogCoreMempoolVectors()); err != nil {
		return err
	}
	if err := WriteMempoolParityRPCFixture(); err != nil {
		return err
	}
	if err := writeJSONFixture("core_script_vectors.json", catalogCoreScriptVectors()); err != nil {
		return err
	}
	if err := writeJSONFixture("core_difficulty_vectors.json", catalogCoreDifficultyVectors()); err != nil {
		return err
	}
	if err := writeJSONFixture("core_header_vectors.json", catalogCoreHeaderVectors()); err != nil {
		return err
	}
	blocks, err := catalogCoreBlockVectors()
	if err != nil {
		return err
	}
	if err := writeJSONFixture("core_block_vectors.json", blocks); err != nil {
		return err
	}
	filters, err := catalogCoreBlockFilterVectors()
	if err != nil {
		return err
	}
	if err := writeJSONFixture("blockfilters_core.json", filters); err != nil {
		return err
	}
	field, err := catalogMainnetFieldBlocks()
	if err != nil {
		return err
	}
	if err := writeJSONFixture("mainnet_field_blocks.json", field); err != nil {
		return err
	}
	if aux := catalogMainnetFieldAuxpowEntries(); len(aux) > 0 {
		if err := writeJSONFixture("mainnet_field_auxpow.json", aux); err != nil {
			return err
		}
	}
	return nil
}

func tryReadMainnetFieldAuxHex(height int64) (string, bool) {
	chainDir := mainnetFieldDataDir()
	auxPath := filepath.Join(chainDir, "headers_aux.bin")
	if _, err := os.Stat(auxPath); err != nil {
		return "", false
	}
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return "", false
	}
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		return "", false
	}
	tip, err := j.TipHeight()
	if err != nil || height > tip {
		return "", false
	}
	aux, err := store.OpenHeaderAuxJournal(auxPath, tip+1)
	if err != nil {
		return "", false
	}
	b, err := aux.ReadAt(height)
	if err == nil && len(b) > 0 {
		return strings.ToUpper(hex.EncodeToString(b)), true
	}
	// headers_aux.bin can lag raw blocks (sparse empty slots); extract CAuxPow from stored block.
	blockRaw, ok := tryReadMainnetFieldBlockRaw(height)
	if !ok {
		return "", false
	}
	blob, hasAux, err := wire.ExtractAuxPowBytesFromBlock(blockRaw)
	if err != nil || !hasAux || len(blob) == 0 {
		return "", false
	}
	return strings.ToUpper(hex.EncodeToString(blob)), true
}

func catalogMainnetFieldAuxpowEntries() []MainnetFieldAuxpowEntry {
	byHeight := map[int64]MainnetFieldAuxpowEntry{}
	if committed, err := LoadMainnetFieldAuxpowEntries(); err == nil {
		for _, e := range committed {
			byHeight[e.Height] = e
		}
	}
	for _, h := range []int64{371337, 371338, 371339} {
		hx, ok := tryReadMainnetFieldHeaderHex(h)
		if !ok {
			if hx, ok = loadCommittedHeaderHexAt(h); !ok {
				continue
			}
		}
		auxHex, ok := tryReadMainnetFieldAuxHex(h)
		if !ok {
			continue
		}
		byHeight[h] = MainnetFieldAuxpowEntry{
			Height:    h,
			HeaderHex: strings.ToUpper(hx),
			AuxHex:    auxHex,
		}
	}
	if len(byHeight) == 0 {
		return nil
	}
	heights := make([]int64, 0, len(byHeight))
	for h := range byHeight {
		heights = append(heights, h)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	out := make([]MainnetFieldAuxpowEntry, 0, len(heights))
	for _, h := range heights {
		out = append(out, byHeight[h])
	}
	return out
}

func catalogCoreMempoolVectors() []MempoolDifferentialVector {
	templates := []string{
		"coinbase", "duplicate_vin", "missing_prevout", "min_relay_fee", "p2pkh_roundtrip",
		"p2sh_nested_p2pkh", "p2sh_multisig", "bare_multisig", "p2sh_cltv_p2pk", "p2sh_csv_p2pk",
		"p2pk_non_standard_input", "dust_output_reject", "witness_reject", "bare_multisig_output_disabled",
		"op_return_nonzero_reject", "package_ancestor_limit", "package_descendant_limit",
		"package_ancestor_size", "package_descendant_size", "mempool_double_spend",
		"rbf_insufficient_fee", "rbf_sufficient_fee", "rbf_not_replaceable", "rbf_fullrbf",
		"coinbase_immature", "vout_empty", "vout_negative", "vin_empty", "vout_toolarge",
		"prevout_null", "vout_empty_scriptpubkey", "txouttotal_toolarge", "tx_oversize",
		"unspendable_output", "op_return_zero", "pq_commitment_op_return", "pq_commitment_nonzero_reject", "pq_carrier_p2sh_accept", "absurd_fee", "multi_op_return",
		"tx_version_nonstandard", "scriptsig_not_pushonly", "non_final", "tx_size_small_reject",
		"scriptsig_size_reject", "discourage_nop_reject", "op_return_oversize_reject",
		"p2sh_sigops_reject", "non_standard_output_reject", "datacarrier_disabled_reject",
		"p2sh_redeem_missing_reject", "discourage_nop1_reject", "rbf_too_many_descendants",
		"rbf_too_many_conflicts", "rbf_new_unconfirmed_input",
		"tx_version_zero_reject", "discourage_nop6_reject", "non_bip68_final",
	}
	out := make([]MempoolDifferentialVector, 0, len(templates))
	for _, tmpl := range templates {
		accept, reason := evalMempoolDifferentialTemplate(tmpl)
		out = append(out, MempoolDifferentialVector{
			Name:             tmpl,
			Template:         tmpl,
			WantAccept:       accept,
			WantRejectReason: reason,
		})
	}
	return out
}

func evalMempoolDifferentialTemplate(tmpl string) (wantAccept bool, wantReason string) {
	switch tmpl {
	case "min_relay_fee", "rbf_insufficient_fee", "rbf_not_replaceable", "coinbase_immature",
		"rbf_too_many_descendants", "rbf_too_many_conflicts", "rbf_new_unconfirmed_input", "non_bip68_final":
		err := EvaluateMempoolDifferentialCheck(tmpl)
		return false, MempoolRejectReason(err)
	case "rbf_sufficient_fee", "rbf_fullrbf":
		err := EvaluateMempoolDifferentialCheck(tmpl)
		if err != nil {
			return false, MempoolRejectReason(err)
		}
		return true, ""
	case "package_ancestor_limit", "package_descendant_limit", "package_ancestor_size", "package_descendant_size":
		err := evalPackageMempoolTemplate(tmpl)
		if err == nil {
			return true, ""
		}
		return false, MempoolRejectReason(err)
	case "mempool_double_spend":
		err := evalMempoolDoubleSpendTemplate()
		if err == nil {
			return true, ""
		}
		return false, MempoolRejectReason(err)
	}
	tx, adm, err := buildMempoolAdmissionCase(tmpl)
	if err != nil {
		return false, err.Error()
	}
	err = AcceptMempoolTxAdmission(tx, adm)
	if err == nil {
		return true, ""
	}
	return false, MempoolRejectReason(err)
}

func evalPackageMempoolTemplate(tmpl string) error {
	switch tmpl {
	case "package_ancestor_limit":
		pool := mempool.New(100)
		var prev [32]byte
		prev[0] = 0xaa
		parentHash := prev
		for i := 0; i < 26; i++ {
			parent := &wire.Tx{
				Version: 1,
				Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
				Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
			}
			if err := pool.Add(parent.SerializeForHash()); err != nil {
				return err
			}
			parentHash = parent.TxHash()
		}
		child := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parentHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: p2pkhScript()}},
		}
		sizes, err := pool.BuildMempoolSizes()
		if err != nil {
			return err
		}
		return CheckMempoolPackageLimits(child, pool, sizes, 25, 25, 101)
	case "package_descendant_limit":
		pool := mempool.New(100)
		var prevHash [32]byte
		prevHash[0] = 0xaa
		for i := 0; i < 25; i++ {
			tx := &wire.Tx{
				Version: 1,
				Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
				Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
			}
			if err := pool.Add(tx.SerializeForHash()); err != nil {
				return err
			}
			prevHash = tx.TxHash()
		}
		sizes, err := pool.BuildMempoolSizes()
		if err != nil {
			return err
		}
		extra := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: prevHash, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: fixtureChildOutValue(), PkScript: p2pkhScript()}},
		}
		return CheckMempoolPackageLimits(extra, pool, sizes, 25, 25, 101)
	case "package_ancestor_size":
		pool := mempool.New(100)
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(parent.SerializeForHash()); err != nil {
			return err
		}
		child := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: p2pkhScript()}},
		}
		ph := parent.TxHash()
		var k [36]byte
		copy(k[:32], ph[:])
		view := mempoolStubPrevOutView{}
		view[k] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
		return CheckMempoolPackageSizeLimits(child, pool, view, 1, 101)
	case "package_descendant_size":
		pool := mempool.New(100)
		parent := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: [32]byte{9}, PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 50_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(parent.SerializeForHash()); err != nil {
			return err
		}
		child1 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: parent.TxHash(), PrevIdx: 0, Sequence: 0xffffffff, Script: make([]byte, 900)}},
			Vout:    []wire.TxOut{{Value: 49_000_000, PkScript: p2pkhScript()}},
		}
		if err := pool.Add(child1.SerializeForHash()); err != nil {
			return err
		}
		child2 := &wire.Tx{
			Version: 1,
			Vin:     []wire.TxIn{{PrevHash: child1.TxHash(), PrevIdx: 0, Sequence: 0xffffffff}},
			Vout:    []wire.TxOut{{Value: 48_000_000, PkScript: p2pkhScript()}},
		}
		ph := parent.TxHash()
		c1h := child1.TxHash()
		view := mempoolStubPrevOutView{}
		view[outpointKey(ph, 0)] = PrevOut{Value: parent.Vout[0].Value, PkScript: parent.Vout[0].PkScript}
		view[outpointKey(c1h, 0)] = PrevOut{Value: child1.Vout[0].Value, PkScript: child1.Vout[0].PkScript}
		return CheckMempoolPackageSizeLimits(child2, pool, view, 101, 1)
	default:
		return fmt.Errorf("unknown package template %q", tmpl)
	}
}

func evalMempoolDoubleSpendTemplate() error {
	sec := make([]byte, 32)
	sec[0] = 0x66
	priv, pub := secp256k1.PrivKeyFromBytes(sec)
	pubC := pub.SerializeCompressed()
	h160 := hash160(pubC)
	pkScript := append([]byte{0x76, 0xa9, 0x14}, h160[:]...)
	pkScript = append(pkScript, 0x88, 0xac)
	funding := &wire.Tx{
		Version: 1,
		Vin:     []wire.TxIn{{PrevHash: [32]byte{3}, PrevIdx: 0, Sequence: 0xffffffff}},
		Vout:    []wire.TxOut{{Value: 1_000_000_000, PkScript: pkScript}},
	}
	fundRaw, _ := funding.Serialize()
	pool := mempool.New(10)
	_ = pool.Add(fundRaw)
	spend1, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 900_000_000)
	if err != nil {
		return err
	}
	_ = pool.Add(spend1.SerializeForHash())
	spend2, err := buildSignedSpendTx(funding, pkScript, priv, pubC, 800_000_000)
	if err != nil {
		return err
	}
	adm := MempoolAdmission{
		View:             AdmissionPrevOutView(pool, nil, nil),
		Pool:             pool,
		MinRelayFeePerKB: DefaultMinRelayTxFeePerKB,
	}
	return adm.CheckSpendConflicts(spend2)
}

func catalogCoreScriptVectors() []coreScriptVector {
	templates := allScriptDifferentialTemplateNames()
	out := make([]coreScriptVector, 0, len(templates))
	for _, tmpl := range templates {
		err := evalScriptDifferentialTemplate(tmpl)
		out = append(out, coreScriptVector{
			Name:       tmpl,
			Template:   tmpl,
			WantAccept: err == nil,
		})
	}
	return out
}

func allScriptDifferentialTemplateNames() []string {
	return []string{
		"p2pkh_roundtrip", "p2pk_roundtrip", "p2sh_nested_p2pkh", "p2sh_multisig", "bare_multisig",
		"p2sh_cltv_p2pk", "p2sh_csv_p2pk", "op_if_else_true", "op_if_else_false", "op_notif_true_branch",
		"op_notif_false_branch", "op_verify_true", "op_verify_false", "op_toaltstack_roundtrip",
		"op_fromaltstack_empty", "op_pick", "op_depth", "op_drop_empty", "op_roll", "op_rot",
		"op_nested_if", "op_nested_if_false", "op_disabled_cat", "op_over", "op_2dup", "op_ifdup",
		"op_unbalanced_if", "op_swap", "op_tuck", "op_nip", "op_3dup", "op_equal", "op_equal_false",
		"op_size", "op_2over", "op_booland", "op_boolor", "op_numequal", "op_equalverify", "op_2swap",
		"op_not", "op_1add", "op_2rot", "op_numequal_false", "op_numnotequal", "op_add", "op_negate",
		"op_booland_false", "op_return", "op_sub", "op_1sub", "op_lessthan", "op_greaterthan_false",
		"op_lessthanorequal", "op_greaterthanorequal", "op_greaterthanorequal_false", "op_within",
		"op_disabled_mul", "op_min", "op_max", "op_disabled_div", "op_within_false",
		"op_lessthanorequal_false", "op_greaterthan", "op_numnotequal_false", "op_disabled_mod",
		"op_codeseparator", "op_numequalverify", "op_equalverify_false", "op_disabled_lshift",
		"op_disabled_rshift", "op_numequalverify_false", "op_disabled_2mul", "op_disabled_and",
		"op_boolor_false", "op_disabled_or", "op_disabled_xor", "op_disabled_2div", "op_drop", "op_dup",
		"op_disabled_left", "op_disabled_right", "op_disabled_invert", "op_disabled_substr", "op_2drop",
		"op_abs", "op_0notequal", "op_0notequal_false", "op_reserved", "op_nop", "op_1negate", "op_ver",
		"op_reserved1", "op_reserved2", "op_else_unbalanced", "op_endif_unbalanced", "op_16", "op_2",
		"op_15", "op_if_empty_stack", "op_verify_empty_stack", "op_pick_underflow", "op_3", "op_10",
		"op_notif_empty_stack", "op_roll_underflow", "op_depth_empty", "op_4", "op_5", "op_6", "op_7",
		"op_8", "op_equalverify_empty", "op_numequalverify_empty", "op_9", "op_11", "op_12", "op_13",
		"op_14", "op_over_underflow", "op_tuck_underflow", "op_rot_underflow", "op_2drop_underflow",
		"op_2swap_underflow", "op_dup_empty", "op_swap_underflow", "op_nip_underflow", "op_ifdup_empty",
		"op_2over_underflow", "op_3dup_underflow", "op_2rot_underflow", "op_2dup_underflow",
		"op_toaltstack_empty", "op_2drop_empty",
	}
}

func catalogCoreDifficultyVectors() []coreDifficultyVector {
	return []coreDifficultyVector{
		{
			Name: "mainnet_get_next_work_30480", Network: "mainnet", Mode: "get_next_work",
			PrevHeight: 30479, Tip0: 30238, LastRetargetTime: 1388149872, PrevTime: 1388163922,
			PrevBits: "0x1c00974f", CandidateTime: 1388163982, WantBits: "0x1c0093a1",
		},
		{
			Name: "mainnet_calculate_next_work_240", Network: "mainnet", Mode: "calculate_next_work",
			PrevHeight: 239, Tip0: 238, LastRetargetTime: 1386474927, PrevTime: 1386475638,
			PrevBits: "0x1e0ffff0", CandidateTime: 1386475698, WantBits: "0x1e00ffff",
		},
		{
			Name: "reboot_testnet_calculate_next_work_1", Network: "reboottestnet", Mode: "calculate_next_work",
			PrevHeight: 1, Tip0: 0, LastRetargetTime: 1747000060, PrevTime: 1747000060,
			PrevBits: "0x1e0ffff0", CandidateTime: 1747000120, WantBits: "0x1e0e2214",
		},
	}
}

func catalogCoreHeaderVectors() []coreHeaderVector {
	var out []coreHeaderVector
	for _, cp := range chain.RebootTestnetHeaderCheckpoints {
		out = append(out, coreHeaderVector{
			Name: "reboot_checkpoint_" + fmt.Sprint(cp.Height), Kind: "checkpoint",
			Network: "reboottestnet", Height: cp.Height, WantAccept: true,
		})
	}
	out = append(out, coreHeaderVector{
		Name: "reboot_checkpoint_bad_hash", Kind: "checkpoint", Network: "reboottestnet",
		Height: 0, Mutations: []headerMutation{{Offset: 4, Xor: 1}},
		WantAccept: false, WantErrorSubstr: "checkpoint hash mismatch",
	})
	for _, cp := range chain.MainnetHeaderCheckpoints {
		vec := coreHeaderVector{
			Name: "mainnet_checkpoint_" + fmt.Sprint(cp.Height), Kind: "checkpoint",
			Network: "mainnet", Height: cp.Height,
		}
		if cp.Height == 0 {
			vec.WantAccept = true
		} else if hx, ok := tryReadMainnetCheckpointHeaderHex(cp.Height); ok {
			vec.HeaderHex = strings.ToUpper(hx)
			vec.WantAccept = true
		} else if hx, ok := CommittedAuxpowHeaderHex(cp.Height); ok {
			vec.HeaderHex = hx
			vec.WantAccept = true
		} else {
			vec.WantAccept = false
			vec.WantErrorSubstr = "checkpoint hash mismatch"
		}
		out = append(out, vec)
	}
	lengths := []int{10, 48, 100, 200, 600}
	for _, n := range lengths {
		out = append(out, coreHeaderVector{
			Name: fmt.Sprintf("reboot_stored_journal_%d", n), Kind: "stored_headers",
			Network: "reboottestnet", JournalLength: n, Start: 0, WantAccept: true,
		})
		out = append(out, coreHeaderVector{
			Name: fmt.Sprintf("reboot_segment_stored_%d", n), Kind: "segment_stored_headers",
			Network: "reboottestnet", JournalLength: n, Start: 0, WantAccept: true,
		})
	}
	batchSizes := []int{1, 6, 12, 24, 48}
	for _, n := range batchSizes {
		out = append(out, coreHeaderVector{
			Name: fmt.Sprintf("reboot_batch_accept_%d", n), Kind: "validate_batch",
			Network: "reboottestnet", JournalLength: 10, BatchLength: n, WantAccept: true,
		})
	}
	idx24 := 24
	out = append(out, coreHeaderVector{
		Name: "reboot_batch_bad_nbits_24", Kind: "validate_batch", Network: "reboottestnet",
		JournalLength: 10, BatchLength: 48,
		BatchMutations: []headerMutation{{Offset: 72, Xor: 0xff, Index: &idx24}},
		WantAccept: false, WantErrorSubstr: "bad nBits",
	})
	idx6 := 6
	out = append(out, coreHeaderVector{
		Name: "reboot_batch_bad_prev_6", Kind: "validate_batch", Network: "reboottestnet",
		JournalLength: 10, BatchLength: 48,
		BatchMutations: []headerMutation{{Offset: 4, Xor: 0xff, Index: &idx6}},
		WantAccept: false, WantErrorSubstr: "bad prev",
	})
	for i := 0; i < 20; i++ {
		out = append(out, coreHeaderVector{
			Name: fmt.Sprintf("reboot_stored_range_%d", i), Kind: "stored_headers",
			Network: "reboottestnet", JournalLength: 50 + i*5, Start: int64(i), WantAccept: true,
		})
	}
	for i := 0; i < 18; i++ {
		out = append(out, coreHeaderVector{
			Name: fmt.Sprintf("reboot_stored_extra_%d", i), Kind: "stored_headers",
			Network: "reboottestnet", JournalLength: 30 + i*10, Start: 0, WantAccept: true,
		})
	}
	out = append(out, catalogMainnetFieldHeaderVectors()...)
	return out
}

// tryReadMainnetFieldHeaderHex returns header80 hex from local mainnet headers.bin when present.
func tryReadMainnetFieldHeaderHex(height int64) (string, bool) {
	if height < 0 {
		return "", false
	}
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return "", false
	}
	j, err := store.OpenHeaderChain(mainnetFieldDataDir(), gen[:80])
	if err != nil {
		return "", false
	}
	tip, err := j.TipHeight()
	if err != nil || tip < height {
		return "", false
	}
	h80, err := j.ReadHeaderAt(height)
	if err != nil || len(h80) != 80 {
		return "", false
	}
	return hex.EncodeToString(h80), true
}

// tryReadMainnetCheckpointHeaderHex returns header80 for Core mapCheckpoints heights:
// local headers.bin, then committed field_header / checkpoint rows in core_header_vectors.json.
func tryReadMainnetCheckpointHeaderHex(height int64) (string, bool) {
	if hx, ok := tryReadMainnetFieldHeaderHex(height); ok {
		return hx, true
	}
	if hx, ok := loadCommittedHeaderHexAt(height); ok {
		return hx, true
	}
	if hx, ok := tryReadMainnetCheckpointHeaderFromCoreCLI(height); ok {
		return hx, true
	}
	return "", false
}

func loadCommittedHeaderHexAt(height int64) (string, bool) {
	path := filepath.Join(testdataDir(), "core_header_vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var all []coreHeaderVector
	if err := json.Unmarshal(raw, &all); err != nil {
		return "", false
	}
	for _, v := range all {
		if v.Height != height {
			continue
		}
		if v.Kind != "field_header" && v.Kind != "checkpoint" {
			continue
		}
		hx := strings.TrimSpace(v.HeaderHex)
		if hx == "" {
			continue
		}
		return strings.ToUpper(hx), true
	}
	return "", false
}

func catalogMainnetFieldHeaderVectors() []coreHeaderVector {
	byHeight := map[int64]coreHeaderVector{}
	for _, v := range loadCommittedFieldHeaderVectors() {
		byHeight[v.Height] = v
	}
	for _, h := range []int64{1, 2, 3, 100, 200, 272, 10006, 10000, 100000, 371337, 371338, 371339, 371340} {
		hx, ok := tryReadMainnetFieldHeaderHex(h)
		if !ok {
			if hx, ok = CommittedAuxpowHeaderHex(h); !ok {
				continue
			}
		}
		byHeight[h] = coreHeaderVector{
			Name:       fmt.Sprintf("mainnet_field_header_%d", h),
			Kind:       "field_header",
			Network:    "mainnet",
			Height:     h,
			HeaderHex:  strings.ToUpper(hx),
			WantAccept: true,
		}
	}
	for _, spec := range mainnetCanonicalBlockSpecs {
		raw, err := buildMainnetCanonicalBlockRaw(spec)
		if err != nil {
			continue
		}
		byHeight[spec.Height] = coreHeaderVector{
			Name:       fmt.Sprintf("mainnet_field_header_%d", spec.Height),
			Kind:       "field_header",
			Network:    "mainnet",
			Height:     spec.Height,
			HeaderHex:  strings.ToUpper(hex.EncodeToString(raw[:80])),
			WantAccept: true,
		}
	}
	heights := make([]int64, 0, len(byHeight))
	for h := range byHeight {
		heights = append(heights, h)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })
	out := make([]coreHeaderVector, 0, len(heights))
	for _, h := range heights {
		out = append(out, byHeight[h])
	}
	return out
}

func loadCommittedFieldHeaderVectors() []coreHeaderVector {
	path := filepath.Join(testdataDir(), "core_header_vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var all []coreHeaderVector
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil
	}
	var out []coreHeaderVector
	for _, v := range all {
		if v.Kind == "field_header" && strings.TrimSpace(v.HeaderHex) != "" {
			out = append(out, v)
		}
	}
	return out
}

func catalogCoreBlockVectors() ([]coreBlockVector, error) {
	raw0, _ := minimalBlockRaw()
	_, hash0 := minimalBlockRaw()
	raw1, _, err := minimalChainedBlockRaw(hash0, 1747000060, 166042)
	if err != nil {
		return nil, err
	}
	mainnetGen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return nil, err
	}
	vecs := []coreBlockVector{
		{
			Name: "core_hex_genesis_payload_accept", Kind: "check_block_payload",
			Network: "reboottestnet", Height: 0, Source: "hex",
			Hex: strings.ToUpper(hex.EncodeToString(raw0)), WantAccept: true,
		},
		{
			Name: "mainnet_hex_genesis_payload_accept", Kind: "check_block_payload",
			Network: "mainnet", Height: 0, Source: "hex",
			Hex: strings.ToUpper(hex.EncodeToString(mainnetGen)), WantAccept: true,
		},
		{
			Name: "core_hex_block_one_payload_accept", Kind: "check_block_payload",
			Network: "reboottestnet", Height: 1, Source: "hex",
			Hex: strings.ToUpper(hex.EncodeToString(raw1)), WantAccept: true,
		},
		{
			Name: "mainnet_genesis_payload_accept", Kind: "check_block_payload",
			Network: "mainnet", Height: 0, Source: "chain_genesis", WantAccept: true,
		},
		{
			Name: "stored_genesis_connect", Kind: "stored_block_bodies",
			Network: "reboottestnet", Height: 0, Source: "minimal", ChainTipHeight: 0, WantAccept: true,
		},
		{
			Name: "stored_chain_512_connect", Kind: "stored_block_bodies",
			Network: "reboottestnet", Height: 0, Source: "minimal", ChainTipHeight: 512, WantAccept: true,
		},
	}
	mutations := []struct {
		name, mut, substr string
		accept            bool
	}{
		{"bad_merkle", "bad_merkle", "merkle", false},
		{"duplicate_txid", "duplicate_txid", "duplicate", false},
		{"bad_cb_multiple", "bad_cb_multiple", "bad-cb-multiple", false},
		{"bad_cb_missing", "bad_cb_missing", "bad-cb-missing", false},
		{"header_hash_mismatch", "header_hash_mismatch", "hash", false},
		{"duplicate_spend", "duplicate_spend", "bad-txns-spent", false},
		{"bad_vout_negative", "bad_vout_negative", "negative", false},
		{"bad_vout_empty", "bad_vout_empty", "empty", false},
		{"bad_prevout_null", "bad_prevout_null", "prevout", false},
		{"bad_cb_length", "bad_cb_length", "bad-cb-length", false},
		{"bad_vout_empty_scriptpubkey", "bad_vout_empty_scriptpubkey", "scriptpubkey", false},
		{"bad_vin_empty", "bad_vin_empty", "vin", false},
		{"bad_vout_toolarge", "bad_vout_toolarge", "toolarge", false},
		{"bad_txouttotal_toolarge", "bad_txouttotal_toolarge", "toolarge", false},
		{"bad_witness", "bad_witness", "witness", false},
		{"unspendable_output_with_value", "unspendable_output_with_value", "", true},
		{"bad_vout_script_toolarge", "bad_vout_script_toolarge", "bad-blk-oversize", false},
		{"bad_tx_oversize", "bad_tx_oversize", "oversize", false},
		{"oversize_coinbase", "oversize_coinbase", "bad-blk-oversize", false},
	}
	for _, m := range mutations {
		vecs = append(vecs, coreBlockVector{
			Name: "check_" + m.name, Kind: "check_block_payload", Network: "reboottestnet",
			Height: 1, Source: "minimal", Mutation: m.mut, WantAccept: m.accept,
			WantErrorSubstr: m.substr,
		})
	}
	for h := int64(1); h <= 20; h++ {
		vecs = append(vecs, coreBlockVector{
			Name: fmt.Sprintf("stored_chain_tip_%d", h), Kind: "stored_block_bodies",
			Network: "reboottestnet", Height: 0, Source: "minimal", ChainTipHeight: h, WantAccept: true,
		})
	}
	field, err := catalogMainnetFieldBlockPayloadVectors()
	if err != nil {
		return nil, err
	}
	vecs = append(vecs, field...)
	return vecs, nil
}

func catalogCoreBlockFilterVectors() ([]coreBlockFilterVector, error) {
	raw, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return nil, err
	}
	hash := pow.BlockHashHex(raw[:80])
	spk := "4104678afdb0fe5548271967f1a67130b7105cd6a828e03909a67962e0ea1f61deb649f6bc3f4cef38c4f35504e51ec112de5c384df7ba0b8d578a4c702b6bf11d5fac"
	filter, err := buildBasicFilterFromCoreVector(coreBlockFilterVector{
		Block:       strings.ToUpper(hex.EncodeToString(raw)),
		PrevScripts: []string{spk},
	})
	if err != nil {
		return nil, err
	}
	var zero [32]byte
	hdr := BlockFilterHeader(BlockFilterHash(filter), zero)
	return []coreBlockFilterVector{{
		Height:          0,
		BlockHash:       hash,
		Block:           strings.ToUpper(hex.EncodeToString(raw)),
		PrevScripts:     []string{spk},
		BasicFilter:     strings.ToUpper(hex.EncodeToString(filter)),
		BasicHeader:     pow.LEUint256DisplayHex(hdr[:]),
		PrevBasicHeader: pow.LEUint256DisplayHex(zero[:]),
		Note:            "mainnet genesis coinbase prevout script (Core blockfilters corpus seed)",
	}}, nil
}

func catalogMainnetFieldBlocks() ([]mainnetFieldBlockEntry, error) {
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return nil, err
	}
	out := []mainnetFieldBlockEntry{{
		Height: 0,
		Hex:    strings.ToUpper(hex.EncodeToString(gen)),
	}}
	heights := []int64{1, 2, 3, 100, 200, 272, 10006, mainnetFieldMultiTxBlockHeight}
	seen := map[int64]bool{0: true}
	for _, h := range heights {
		if seen[h] {
			continue
		}
		if h == mainnetFieldMultiTxBlockHeight {
			if e, err := mainnetFieldMultiTxBlock15504Entry(); err == nil {
				out = append(out, e)
				seen[h] = true
			}
			continue
		}
		raw, ok := tryReadMainnetFieldBlockRaw(h)
		if !ok {
			continue
		}
		out = append(out, mainnetFieldBlockEntry{
			Height: h,
			Hex:    strings.ToUpper(hex.EncodeToString(raw)),
		})
		seen[h] = true
	}
	// Also export contiguous tip from rawblocks_sync.json when present.
	if tip, ok := tryReadMainnetFieldContiguousTip(); ok && !seen[tip] && tip > 0 {
		if raw, ok := tryReadMainnetFieldBlockRaw(tip); ok {
			out = append(out, mainnetFieldBlockEntry{
				Height: tip,
				Hex:    strings.ToUpper(hex.EncodeToString(raw)),
			})
			seen[tip] = true
		}
	}
	canon, err := mainnetCanonicalFieldBlocks()
	if err != nil {
		return nil, err
	}
	for _, e := range canon {
		// Committed canonical blocks override disk export (operator datadir bodies may disagree).
		hexU := strings.ToUpper(strings.TrimSpace(e.Hex))
		replaced := false
		for i := range out {
			if out[i].Height == e.Height {
				out[i].Hex = hexU
				replaced = true
				break
			}
		}
		if !replaced {
			out = append(out, mainnetFieldBlockEntry{
				Height: e.Height,
				Hex:    hexU,
			})
		}
		seen[e.Height] = true
	}
	return out, nil
}

func tryReadMainnetFieldBlockRaw(height int64) ([]byte, bool) {
	if height < 0 {
		return nil, false
	}
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		return nil, false
	}
	chainDir := mainnetFieldDataDir()
	j, err := store.OpenHeaderChain(chainDir, gen[:80])
	if err != nil {
		return nil, false
	}
	tip, err := j.TipHeight()
	if err != nil || tip < height {
		return nil, false
	}
	hdr, err := j.ReadHeaderAt(height)
	if err != nil {
		return nil, false
	}
	rs, err := store.OpenRawBlockStore(chainDir)
	if err != nil {
		return nil, false
	}
	raw, err := rs.Get(pow.BlockHashLE(hdr))
	if err != nil {
		raw, err = rs.GetByContiguousHeight(height)
		if err != nil || len(raw) < 80 {
			return nil, false
		}
	}
	if len(raw) < 80 {
		return nil, false
	}
	if pow.BlockHashLE(raw[:80]) != pow.BlockHashLE(hdr) {
		return nil, false
	}
	return raw, true
}

func tryReadMainnetFieldContiguousTip() (int64, bool) {
	chainDir := mainnetFieldDataDir()
	raw, err := os.ReadFile(filepath.Join(chainDir, "rawblocks_sync.json"))
	if err != nil {
		return 0, false
	}
	var cp struct {
		Contiguous int64 `json:"contiguous_raw_height"`
	}
	if err := json.Unmarshal(raw, &cp); err != nil || cp.Contiguous < 1 {
		return 0, false
	}
	return cp.Contiguous, true
}
