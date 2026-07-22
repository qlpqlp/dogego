// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import (
	"os"
	"path/filepath"

	"dogego/chain"
	"dogego/store"
)

// MainnetFieldDiskStatus summarizes operator dogedata/mainnet readiness for live field evidence tests.
type MainnetFieldDiskStatus struct {
	ChainDir             string `json:"chain_dir"`
	HeadersPresent       bool   `json:"headers_present"`
	TipHeight            int64  `json:"tip_height,omitempty"`
	HeaderLayout         string `json:"header_layout,omitempty"`
	HasAuxJournal        bool   `json:"has_aux_journal"`
	HasRawBlocks         bool   `json:"has_rawblocks"`
	ContiguousRaw        int64  `json:"contiguous_raw,omitempty"`
	LiveHeaderPoWReady   bool   `json:"live_header_pow_ready"`
	LiveDiskConnectReady bool   `json:"live_disk_connect_ready"`
	Error                string `json:"error,omitempty"`
}

// ProbeMainnetFieldDiskStatus inspects DOGEGO_FIELD_DATADIR or default dogedata/mainnet for Milestone A live tests.
func ProbeMainnetFieldDiskStatus() MainnetFieldDiskStatus {
	out := MainnetFieldDiskStatus{ChainDir: MainnetFieldDataDir()}
	seg := filepath.Join(out.ChainDir, "headers")
	mono := filepath.Join(out.ChainDir, "headers.bin")
	if _, err := os.Stat(seg); err != nil {
		if _, err2 := os.Stat(mono); err2 != nil {
			out.Error = "headers missing"
			return out
		}
	}
	out.HeadersPresent = true
	if _, err := os.Stat(filepath.Join(out.ChainDir, "headers_aux.bin")); err == nil {
		out.HasAuxJournal = true
	}
	if _, err := os.Stat(filepath.Join(out.ChainDir, "rawblocks")); err == nil {
		out.HasRawBlocks = true
	}
	gen, err := chain.MainnetGenesisBlockRaw()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	j, err := store.OpenHeaderChain(out.ChainDir, gen[:80])
	if err != nil {
		out.Error = err.Error()
		return out
	}
	tip, err := j.TipHeight()
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.TipHeight = tip
	out.HeaderLayout = j.HeaderLayout()
	out.LiveHeaderPoWReady = tip >= 0
	if disk, err := OpenMainnetFieldDiskChain(); err == nil {
		out.ContiguousRaw = disk.BundledContiguous
		out.LiveDiskConnectReady = true
	}
	return out
}
