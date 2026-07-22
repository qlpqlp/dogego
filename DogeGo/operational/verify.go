// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package operational

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"dogego/chain"
	"dogego/config"
	"dogego/founder"
)

// Check is one operational readiness row (config/disk/live).
type Check struct {
	ID      string `json:"id"`
	Status  string `json:"status"` // ok | warn | issue
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

// VerifyResult reports whether a single-network config looks ready for daily operation.
type VerifyResult struct {
	OK         bool     `json:"ok"`
	Network    string   `json:"network"`
	Role       string   `json:"role,omitempty"`
	DataDir    string   `json:"datadir,omitempty"`
	ChainDir   string   `json:"chain_dir,omitempty"`
	P2PPort    int      `json:"p2p_port,omitempty"`
	Checks     []Check  `json:"checks"`
	Issues     []string `json:"issues,omitempty"`
	Warnings   []string `json:"warnings,omitempty"`
	NextSteps  []string `json:"next_steps,omitempty"`
	Notes      []string `json:"notes,omitempty"`
	Doc        string   `json:"doc,omitempty"`
	Founder    *founder.VerifyResult `json:"founder,omitempty"`
}

// Verify evaluates operational readiness from saved config (no running node required).
func Verify(f config.File) VerifyResult {
	network := strings.TrimSpace(f.Network)
	if network == "" {
		network = "mainnet"
	}
	if config.IsRebootTestnetNetwork(network) {
		return verifyRebootTestnet(f)
	}
	return verifyMainnet(f)
}

func verifyRebootTestnet(f config.File) VerifyResult {
	fr := founder.Verify(f)
	r := VerifyResult{
		OK:       fr.OK,
		Network:  fr.Network,
		Role:     "reboot_testnet_founder",
		DataDir:  fr.DataDir,
		ChainDir: fr.ChainDir,
		P2PPort:  fr.P2PPort,
		Doc:      "docs/MAINNET_TESTNET_OPERATIONAL.md § Reboot testnet",
		Founder:  &fr,
	}
	for _, c := range fr.Checks {
		r.Checks = append(r.Checks, Check{ID: c.ID, Status: c.Status, Message: c.Message, Fix: c.Fix})
	}
	r.Issues = append(r.Issues, fr.Issues...)
	r.Warnings = append(r.Warnings, fr.Warnings...)
	r.Notes = append(r.Notes, fr.Notes...)
	if fr.OK {
		r.NextSteps = []string{
			"Start node: dogego node -conf dogecoinconf.testnet.json (or wizard Save & start)",
			"Share addnode FOUNDER_HOST:44556 with joiners",
			"CI parity: dogego cert setup-parity -mine-bootstrap then dogego cert weekly-live",
			"Features → Founder probe or GET /api/core-founder-probe when running",
		}
	} else {
		r.NextSteps = []string{"Fix founder checks, then dogego cert founder -json"}
	}
	return r
}

func verifyMainnet(f config.File) VerifyResult {
	p2pPort := chain.Port
	if p, err := chain.ParamsFor(chain.MainnetDogecoin); err == nil {
		p2pPort = p.Port
	}
	r := VerifyResult{
		Network: networkLabel(f),
		Role:    "mainnet_full_node",
		P2PPort: p2pPort,
		Doc:     "docs/MAINNET_TESTNET_OPERATIONAL.md § Mainnet",
	}
	datadir := resolveDataDir(f)
	r.DataDir = datadir
	r.ChainDir = filepath.Join(datadir, "mainnet")

	appendCheck := func(c Check) {
		r.Checks = append(r.Checks, c)
		switch c.Status {
		case "issue":
			r.Issues = append(r.Issues, c.ID+": "+c.Message)
		case "warn":
			r.Warnings = append(r.Warnings, c.ID+": "+c.Message)
		}
	}

	net := strings.TrimSpace(f.Network)
	if net == "" {
		net = "mainnet"
	}
	if config.IsRebootTestnetNetwork(net) {
		appendCheck(Check{
			ID: "network", Status: "issue",
			Message: "network=testnet in mainnet verify path",
			Fix:     `Use dogego cert operational with network=mainnet or -dual for both.`,
		})
	} else {
		appendCheck(Check{ID: "network", Status: "ok", Message: "network=mainnet"})
	}

	mode := strings.ToLower(strings.TrimSpace(f.NodeMode))
	if mode == "spv" {
		appendCheck(Check{
			ID: "node_mode", Status: "issue",
			Message: "node_mode=spv cannot serve full blocks or explorer tx index",
			Fix:     `Use "node_mode": "full" for a mainnet full node.`,
		})
	} else {
		appendCheck(Check{ID: "node_mode", Status: "ok", Message: "full node mode (or default full)"})
	}

	if f.NoTxIndex {
		appendCheck(Check{
			ID: "tx_index", Status: "warn",
			Message: "no_tx_index disables explorer address/tx search",
			Fix:     "Remove no_tx_index unless you accept limited explorer.",
		})
	} else {
		appendCheck(Check{ID: "tx_index", Status: "ok", Message: "transaction index enabled (default)"})
	}

	rpc := strings.TrimSpace(f.RPCAddr)
	if rpc == "" && strings.TrimSpace(f.WebUI) == "" {
		appendCheck(Check{
			ID: "operator_api", Status: "warn",
			Message: "no rpc listen and no webui - hard to operate without HTTP dashboard or RPC",
			Fix:     `Set "webui": "127.0.0.1:2013" and/or "rpc": "127.0.0.1:22557".`,
		})
	} else {
		appendCheck(Check{ID: "operator_api", Status: "ok", Message: "RPC and/or web dashboard configured"})
	}

	checkLegacyCoreLayout(datadir, appendCheck)
	checkP2PPortFree(p2pPort, appendCheck)

	if datadir != "" {
		if st, err := os.Stat(r.ChainDir); err == nil && st.IsDir() {
			appendCheck(Check{ID: "chain_dir", Status: "ok", Message: "mainnet/ chain data present"})
		} else {
			appendCheck(Check{
				ID: "chain_dir", Status: "ok",
				Message: "mainnet/ will be created on first start (fresh IBD)",
			})
		}
	}

	if f.NoWallet {
		appendCheck(Check{
			ID: "wallet", Status: "warn",
			Message: "nowallet set - no built-in wallet on mainnet",
			Fix:     "Remove nowallet for operator wallet tab; use -nowallet only on relay-only nodes.",
		})
	} else {
		appendCheck(Check{ID: "wallet", Status: "ok", Message: "built-in wallet enabled (default)"})
	}

	r.OK = len(r.Issues) == 0
	if r.OK {
		r.NextSteps = []string{
			"Start node and open dashboard http://127.0.0.1:2013/",
			"Long IBD: scripts/watch_sync.ps1 or dogego cert ibd-convergence -interval-sec 120",
			"Post-aux build check: scripts/upgrade_post_aux_verify.ps1",
			"Offline cert bundle: dogego cert milestones-bde",
			"Live sign-off: dogego cert weekly-live on dogego-live (Core 24/24)",
		}
	} else {
		r.NextSteps = []string{"Fix issues above, then re-run dogego cert operational"}
	}
	return r
}

func networkLabel(f config.File) string {
	n := strings.TrimSpace(f.Network)
	if n == "" {
		return "mainnet"
	}
	return n
}

func resolveDataDir(f config.File) string {
	datadir := strings.TrimSpace(f.DataDir)
	if datadir == "" {
		if d, err := config.PreferredSaveDir(); err == nil {
			datadir = d
		}
	} else if abs, err := config.ResolveDataDir(datadir); err == nil {
		datadir = abs
	}
	return datadir
}

func checkLegacyCoreLayout(datadir string, appendCheck func(Check)) {
	if datadir == "" {
		return
	}
	if st, err := os.Stat(filepath.Join(datadir, "blocks")); err == nil && st.IsDir() {
		appendCheck(Check{
			ID: "legacy_core_layout", Status: "issue",
			Message: "datadir contains Core blocks/ - not DogeGo native layout",
			Fix:     "Use a fresh datadir; sync via P2P into headers.bin + rawblocks/.",
		})
	}
	if st, err := os.Stat(filepath.Join(datadir, "chainstate")); err == nil && st.IsDir() {
		appendCheck(Check{
			ID: "legacy_core_layout", Status: "issue",
			Message: "datadir contains Core chainstate/",
			Fix:     "Point datadir at a dedicated DogeGo folder (see INTENTIONAL_DIFFERENCES.md).",
		})
	}
}

func checkP2PPortFree(port int, appendCheck func(Check)) {
	if ln, err := net.Listen("tcp", ":"+strconv.Itoa(port)); err != nil {
		appendCheck(Check{
			ID: "p2p_port", Status: "warn",
			Message: "P2P port " + strconv.Itoa(port) + " busy: " + err.Error(),
			Fix:     "Stop other nodes on this port or change listen in config.",
		})
	} else {
		_ = ln.Close()
		appendCheck(Check{
			ID: "p2p_port", Status: "ok",
			Message: "P2P port " + strconv.Itoa(port) + " is free",
		})
	}
}
