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
)

// MempoolParityRPCRow is a stateless testmempoolaccept fixture safe for live Core+DogeGo RPC comparison.
type MempoolParityRPCRow struct {
	Name             string `json:"name"`
	Template         string `json:"template"`
	Hex              string `json:"hex"`
	WantAccept       bool   `json:"want_accept"`
	WantRejectReason string `json:"want_reject_reason,omitempty"`
}

// rpcStatelessMempoolTemplates are fixtures that do not require a pre-seeded mempool on the node.
var rpcStatelessMempoolTemplates = map[string]bool{
	"coinbase":                true,
	"duplicate_vin":         true,
	"missing_prevout":         true,
	"vin_empty":               true,
	"vout_empty":              true,
	"vout_negative":           true,
	"vout_toolarge":           true,
	"prevout_null":            true,
	"vout_empty_scriptpubkey": true,
	"txouttotal_toolarge":     true,
	"tx_oversize":             true,
	"tx_version_zero_reject":  true,
	"witness_reject":          true,
	"unspendable_output":      true,
	"tx_size_small_reject":    true,
	"scriptsig_size_reject":   true,
	"op_return_oversize_reject": true,
	"bare_multisig_output_disabled": true,
	"op_return_nonzero_reject":   true,
	"multi_op_return":            true,
	"tx_version_nonstandard":     true,
	"scriptsig_not_pushonly":     true,
	"discourage_nop_reject":      true,
	"discourage_nop1_reject":     true,
	"discourage_nop6_reject":     true,
	"non_standard_output_reject": true,
	"p2sh_redeem_missing_reject": true,
	"p2sh_sigops_reject":         true,
	"op_return_zero":             true,
	"pq_commitment_nonzero_reject": true,
	"datacarrier_disabled_reject": true,
	"non_bip68_final":            true,
}

// BuildMempoolParityRPCRows builds hex rows for side-by-side RPC probes (stateless corpus only).
func BuildMempoolParityRPCRows() ([]MempoolParityRPCRow, error) {
	vecs, err := LoadMempoolDifferentialVectors()
	if err != nil {
		return nil, err
	}
	var out []MempoolParityRPCRow
	for _, v := range vecs {
		if !rpcStatelessMempoolTemplates[v.Template] {
			continue
		}
		fix, err := BuildMempoolDifferentialFixture(v.Template)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", v.Name, err)
		}
		out = append(out, MempoolParityRPCRow{
			Name:             v.Name,
			Template:         v.Template,
			Hex:              hex.EncodeToString(fix.Raw),
			WantAccept:       v.WantAccept,
			WantRejectReason: v.WantRejectReason,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no stateless mempool parity rows")
	}
	return out, nil
}

// LoadMempoolParityRPCRows reads consensus/testdata/mempool_parity_rpc.json (disk or embedded).
func LoadMempoolParityRPCRows() ([]MempoolParityRPCRow, error) {
	raw, err := readConsensusTestdata("mempool_parity_rpc.json", embeddedMempoolParityRPCJSON)
	if err != nil {
		return BuildMempoolParityRPCRows()
	}
	var rows []MempoolParityRPCRow
	if err := json.Unmarshal(raw, &rows); err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return BuildMempoolParityRPCRows()
	}
	return rows, nil
}

// WriteMempoolParityRPCFixture updates consensus/testdata/mempool_parity_rpc.json from live builders.
func WriteMempoolParityRPCFixture() error {
	rows, err := BuildMempoolParityRPCRows()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		return err
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return fmt.Errorf("runtime.Caller")
	}
	path := filepath.Join(filepath.Dir(file), "testdata", "mempool_parity_rpc.json")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
