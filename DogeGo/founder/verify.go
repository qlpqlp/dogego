// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package founder

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/config"
)

// Check is one reboot-testnet founder preflight row.
type Check struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // ok | warn | issue
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// VerifyResult reports whether dogecoinconf.json looks ready to run a reboot testnet founder node.
type VerifyResult struct {
	OK       bool     `json:"ok"`
	Network  string   `json:"network"`
	DataDir  string   `json:"datadir,omitempty"`
	ChainDir string   `json:"chain_dir,omitempty"`
	P2PPort  int      `json:"p2p_port"`
	Checks   []Check  `json:"checks"`
	Issues   []string `json:"issues,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
	Notes    []string `json:"notes,omitempty"`
	Doc      string   `json:"doc,omitempty"`
}

// Verify evaluates founder readiness from a saved config file (no running node required).
func Verify(f config.File) VerifyResult {
	network := strings.TrimSpace(f.Network)
	if network == "" {
		network = "testnet"
	}
	p2pPort := chain.Port
	if p, err := chain.ParamsFor(chain.RebootTestnet); err == nil {
		p2pPort = p.Port
	}

	r := VerifyResult{
		Network: network,
		P2PPort: p2pPort,
		Doc:     "docs/OPERATOR.md § Reboot testnet founder checklist",
	}

	datadir := strings.TrimSpace(f.DataDir)
	if datadir == "" {
		if d, err := config.PreferredSaveDir(); err == nil {
			datadir = d
			r.Notes = append(r.Notes, "datadir_default_applied")
		}
	} else if abs, err := config.ResolveDataDir(datadir); err == nil {
		datadir = abs
	}
	r.DataDir = datadir
	r.ChainDir = filepath.Join(datadir, "testnet")

	appendCheck := func(c Check) {
		r.Checks = append(r.Checks, c)
		switch c.Status {
		case "issue":
			r.Issues = append(r.Issues, c.ID+": "+c.Message)
		case "warn":
			r.Warnings = append(r.Warnings, c.ID+": "+c.Message)
		}
	}

	if !config.IsRebootTestnetNetwork(network) {
		appendCheck(Check{
			ID: "network", Status: "issue",
			Message: "network=" + network + " is not reboot testnet",
			Fix:     `Set "network": "testnet" in dogecoinconf.json (reboot testnet, not legacy testnet3 or mainnet).`,
		})
	} else {
		appendCheck(Check{
			ID: "network", Status: "ok",
			Message: "network=testnet (reboot testnet)",
		})
	}

	mode := strings.ToLower(strings.TrimSpace(f.NodeMode))
	if mode == "spv" {
		appendCheck(Check{
			ID: "node_mode", Status: "issue",
			Message: "node_mode=spv cannot solo-mine or serve full blocks to joiners",
			Fix:     `Use "node_mode": "full" (wizard default) for a founder node.`,
		})
	} else {
		appendCheck(Check{
			ID: "node_mode", Status: "ok",
			Message: "full node mode (or default full)",
		})
	}

	m := config.FromFile(f)
	mineRuns := config.EffectiveMine(m, false, false) || strings.TrimSpace(f.MiningAddress) != ""
	if !mineRuns {
		appendCheck(Check{
			ID: "mine", Status: "issue",
			Message: "background mining will not run (nowallet without miningaddress, or spv)",
			Fix:     `Enable the wallet, set "mine": true, or set "miningaddress" for coinbase payout.`,
		})
	} else {
		msg := "solo founder mining will run (reboot testnet default)"
		if strings.TrimSpace(f.MiningAddress) != "" {
			msg = "miningaddress configured for coinbase"
		}
		appendCheck(Check{ID: "mine", Status: "ok", Message: msg})
	}

	p2p := strings.ToLower(strings.TrimSpace(f.P2PConnectivity))
	if p2p == "" {
		p2p = "both"
	}
	switch p2p {
	case "cgnat":
		appendCheck(Check{
			ID: "p2p_inbound", Status: "warn",
			Message: "p2p_connectivity=cgnat is outbound-only - joiners cannot connect inbound to you",
			Fix:     `Founders sharing addnode should use "p2p_connectivity": "both" or "classic" and forward TCP ` + strconv.Itoa(p2pPort) + `.`,
		})
	case "classic", "both":
		appendCheck(Check{
			ID: "p2p_inbound", Status: "ok",
			Message: "p2p_connectivity=" + p2p + " (listen + outbound)",
		})
	default:
		appendCheck(Check{
			ID: "p2p_inbound", Status: "warn",
			Message: "unknown p2p_connectivity=" + p2p,
			Fix:     `Use "both" (default), "classic", or "cgnat".`,
		})
	}

	if ln, err := net.Listen("tcp", ":"+strconv.Itoa(p2pPort)); err != nil {
		appendCheck(Check{
			ID: "p2p_port", Status: "warn",
			Message: "P2P port " + strconv.Itoa(p2pPort) + " is not free: " + err.Error(),
			Fix:     "Stop Dogecoin Core or another node on this port before starting the founder.",
		})
	} else {
		_ = ln.Close()
		appendCheck(Check{
			ID: "p2p_port", Status: "ok",
			Message: "P2P port " + strconv.Itoa(p2pPort) + " is free",
		})
	}

	if datadir != "" {
		if st, err := os.Stat(filepath.Join(datadir, "blocks")); err == nil && st.IsDir() {
			appendCheck(Check{
				ID: "legacy_core_layout", Status: "warn",
				Message: "datadir contains Core-style blocks/ - use a fresh folder for reboot testnet",
				Fix:     "Point datadir at a new directory; do not reuse testnet3 blocks/chainstate.",
			})
		}
		if st, err := os.Stat(filepath.Join(datadir, "chainstate")); err == nil && st.IsDir() {
			appendCheck(Check{
				ID: "legacy_core_layout", Status: "warn",
				Message: "datadir contains Core-style chainstate/",
				Fix:     "Use a fresh datadir for DogeGo reboot testnet (see OPERATOR.md founder checklist).",
			})
		}
		headers := filepath.Join(r.ChainDir, "headers.bin")
		if st, err := os.Stat(headers); err == nil && !st.IsDir() && st.Size() > 80 {
			appendCheck(Check{
				ID: "datadir_existing", Status: "warn",
				Message: "testnet/headers.bin already exists - continuing an existing chain, not a fresh genesis founder",
				Fix:     "Delete testnet/ for a clean founder start, or keep data if you are resuming this reboot testnet.",
			})
		} else if _, err := os.Stat(r.ChainDir); err == nil {
			appendCheck(Check{
				ID: "datadir_fresh", Status: "ok",
				Message: "no header chain yet (or empty) - suitable for new founder",
			})
		} else {
			appendCheck(Check{
				ID: "datadir_fresh", Status: "ok",
				Message: "testnet/ chain folder will be created on first start",
			})
		}
	}

	r.OK = len(r.Issues) == 0
	return r
}
