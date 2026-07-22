// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package runner

import (
	"encoding/json"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"dogego/wallet/corewallet"
	"dogego/walletmigration"
)

// ChainSnap is a minimal getblockchaininfo snapshot.
type ChainSnap struct {
	Blocks int64  `json:"blocks,omitempty"`
	Chain  string `json:"chain,omitempty"`
}

// PreflightOptions configures RunPreflight.
type PreflightOptions struct {
	OfflineOnly         bool
	RequireCore         bool
	RequireWalletDat    bool
	DogeGoPort          int
	CorePort            int
	Host                string
	PortTimeout         time.Duration
	RPCTimeout          time.Duration
	MaxBlockDelta       int64
	WalletDatPath       string
	WalletDatPassphrase string
	WalletDatNetwork    string
	WalletDatImport     bool
}

// PreflightResult reports dogego-live runner readiness (mirrors ci_runner_preflight.ps1).
type PreflightResult struct {
	OK              bool                              `json:"ok"`
	OfflineOnly     bool                              `json:"offline_only,omitempty"`
	Issues          []string                          `json:"issues,omitempty"`
	Warnings        []string                          `json:"warnings,omitempty"`
	Notes           []string                          `json:"notes,omitempty"`
	DogeGo          *ChainSnap                        `json:"dogego,omitempty"`
	Core            *ChainSnap                        `json:"core,omitempty"`
	WalletMigration *walletmigration.LiveProbeResult  `json:"wallet_migration,omitempty"`
	WalletDatImport *walletmigration.LiveImportResult `json:"wallet_dat_import,omitempty"`
	Doc             string                            `json:"doc,omitempty"`
}

// RunPreflight evaluates CI runner tool and live RPC readiness.
func RunPreflight(opts PreflightOptions) PreflightResult {
	if opts.DogeGoPort == 0 {
		opts.DogeGoPort = 44556
	}
	if opts.CorePort == 0 {
		opts.CorePort = 44555
	}
	if opts.Host == "" {
		opts.Host = "127.0.0.1"
	}
	if opts.PortTimeout <= 0 {
		opts.PortTimeout = 500 * time.Millisecond
	}
	if opts.RPCTimeout <= 0 {
		opts.RPCTimeout = 8 * time.Second
	}
	if opts.MaxBlockDelta <= 0 {
		opts.MaxBlockDelta = 3
	}
	requireCore := opts.RequireCore || strings.TrimSpace(os.Getenv("DOGEGO_CORE_COMPARE_REQUIRED")) == "1"
	requireWalletDat := opts.RequireWalletDat || strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT_REQUIRED")) == "1"

	r := PreflightResult{
		OfflineOnly: opts.OfflineOnly,
		Doc:         DogegoLiveWorkflow10Doc,
	}

	if !hasGo() {
		r.Issues = append(r.Issues, "go_missing")
	} else if out, err := exec.Command("go", "version").CombinedOutput(); err == nil {
		r.Notes = append(r.Notes, strings.TrimSpace(string(out)))
	}

	if opts.OfflineOnly {
		addWalletMigrationPreflight(&r, opts, requireWalletDat, false)
		r.OK = len(r.Issues) == 0
		return r
	}

	if !portOpen(opts.Host, opts.DogeGoPort, opts.PortTimeout) {
		r.Issues = append(r.Issues, "dogego_rpc_port_not_listening")
	} else {
		r.Notes = append(r.Notes, "dogego_port_listening="+strconv.Itoa(opts.DogeGoPort))
	}

	coreCLI := resolveCoreCLI()
	if requireCore {
		if coreCLI == "" {
			r.Issues = append(r.Issues, "dogecoin_cli_missing")
		} else if !portOpen(opts.Host, opts.CorePort, opts.PortTimeout) {
			r.Issues = append(r.Issues, "core_rpc_port_not_listening")
		} else {
			r.Notes = append(r.Notes, "core_port_listening="+strconv.Itoa(opts.CorePort))
		}
	}

	var dgInfo map[string]any
	if portOpen(opts.Host, opts.DogeGoPort, opts.PortTimeout) {
		info, err := invokeJSONRPC(opts.Host, opts.DogeGoPort, "", "", "getblockchaininfo", nil, opts.RPCTimeout)
		if err != nil {
			r.Issues = append(r.Issues, "dogego_rpc_unreachable")
		} else {
			dgInfo = info
			blocks, chain := chainSnap(info)
			r.DogeGo = &ChainSnap{Blocks: blocks, Chain: chain}
			r.Notes = append(r.Notes, "dogego_chain="+chain+" blocks="+strconv.FormatInt(blocks, 10))
			if chain != "" && !strings.Contains(strings.ToLower(chain), "test") {
				r.Warnings = append(r.Warnings, "dogego_not_testnet_chain")
			}
		}
		if _, err := invokeJSONRPC(opts.Host, opts.DogeGoPort, "", "", "getwalletinfo", nil, opts.RPCTimeout); err != nil {
			r.Issues = append(r.Issues, "dogego_wallet_unreachable")
		}
	}

	var coreInfo map[string]any
	if coreCLI != "" && portOpen(opts.Host, opts.CorePort, opts.PortTimeout) {
		user := strings.TrimSpace(os.Getenv("DOGEGO_CORE_RPC_USER"))
		pass := os.Getenv("DOGEGO_CORE_RPC_PASS")
		info, err := invokeCoreCLI(coreCLI, opts.CorePort, user, pass, "getblockchaininfo")
		if err != nil {
			if requireCore {
				r.Issues = append(r.Issues, "core_rpc_unreachable")
			} else {
				r.Warnings = append(r.Warnings, "core_rpc_unreachable")
			}
		} else {
			coreInfo = info
			blocks, chain := chainSnap(info)
			r.Core = &ChainSnap{Blocks: blocks, Chain: chain}
			r.Notes = append(r.Notes, "core_chain="+chain+" blocks="+strconv.FormatInt(blocks, 10))
		}
	}

	if dgInfo != nil && coreInfo != nil {
		dgBlocks, _ := chainSnap(dgInfo)
		coreBlocks, _ := chainSnap(coreInfo)
		delta := dgBlocks - coreBlocks
		if delta < 0 {
			delta = -delta
		}
		r.Notes = append(r.Notes, "block_delta="+strconv.FormatInt(delta, 10))
		if delta > opts.MaxBlockDelta {
			if requireCore {
				r.Issues = append(r.Issues, "block_height_delta_too_large")
			} else {
				r.Warnings = append(r.Warnings, "block_height_delta_too_large")
			}
		}
	}

	addWalletMigrationPreflight(&r, opts, requireWalletDat, true)

	if n := strings.TrimSpace(os.Getenv("RUNNER_NAME")); n != "" {
		r.Notes = append(r.Notes, "runner="+n)
	}
	if labels := strings.TrimSpace(os.Getenv("RUNNER_LABELS")); labels != "" {
		r.Notes = append(r.Notes, "labels="+labels)
	} else if labels := strings.TrimSpace(os.Getenv("GITHUB_RUNNER_LABELS")); labels != "" {
		r.Notes = append(r.Notes, "labels="+labels)
	}

	r.OK = len(r.Issues) == 0
	return r
}

func addWalletMigrationPreflight(r *PreflightResult, opts PreflightOptions, requireWalletDat, liveRPC bool) {
	walletDat, configured := ResolveWalletDatPathConfigured(opts.WalletDatPath)
	if walletDat == "" && !requireWalletDat {
		if auto := ResolveWalletDatPath(""); auto != "" {
			walletDat = auto
			configured = false
		}
	}
	walletPass := opts.WalletDatPassphrase
	if walletPass == "" {
		walletPass = os.Getenv("DOGEGO_WALLET_DAT_PASSPHRASE")
	}
	walletNet := strings.TrimSpace(opts.WalletDatNetwork)
	if walletNet == "" {
		walletNet = strings.TrimSpace(os.Getenv("DOGEGO_WALLET_DAT_NETWORK"))
	}
	if walletNet == "" {
		walletNet = "reboottestnet"
	}
	if walletDat == "" {
		if requireWalletDat {
			r.Issues = append(r.Issues, "wallet_dat_required_missing")
		}
		return
	}
	if liveRPC {
		client := walletmigration.RPCClientForHostPort(opts.Host, opts.DogeGoPort)
		var live *walletmigration.LiveImportResult
		var err error
		if opts.WalletDatImport {
			live, err = walletmigration.LiveImportViaRPC(client, walletDat, walletPass)
		} else {
			live, err = walletmigration.LiveProbeViaRPC(client, walletDat)
		}
		if err != nil {
			live = &walletmigration.LiveImportResult{Path: walletDat, Status: "import_failed", Error: err.Error()}
		}
		r.WalletDatImport = live
		if live.Probe != nil {
			r.WalletMigration = &walletmigration.LiveProbeResult{
				Path:    walletDat,
				Network: walletNet,
				Probe:   live.Probe,
			}
			if live.Status == "passed" || live.Status == "passed_encrypted" {
				r.WalletMigration.ExtractOK = true
				r.WalletMigration.ExtractedKeys = live.KeysImported
			}
		}
		extractOK := false
		if walletPass != "" {
			if fileLive, fileErr := walletmigration.ProbeFile(walletDat, walletPass, walletNet); fileErr == nil {
				extractOK = fileLive.ExtractOK
				if fileLive.Probe != nil && r.WalletMigration != nil {
					r.WalletMigration.ExtractOK = fileLive.ExtractOK
					r.WalletMigration.ExtractedKeys = fileLive.ExtractedKeys
				}
			}
		}
		if opts.WalletDatImport {
			applyLiveImportPreflight(r, live, requireWalletDat)
		} else {
			applyLiveProbePreflight(r, live, requireWalletDat, extractOK)
		}
		return
	}
	live, err := walletmigration.ProbeFile(walletDat, walletPass, walletNet)
	if err != nil {
		if walletmigration.WalletDatProbeOptional(configured, requireWalletDat) {
			return
		}
		if requireWalletDat {
			r.Issues = append(r.Issues, "wallet_dat_probe_failed")
		} else {
			r.Warnings = append(r.Warnings, "wallet_dat_probe_failed")
		}
		r.Notes = append(r.Notes, "wallet_dat_error="+err.Error())
		return
	}
	if live.Probe == nil || !live.Probe.IsBDB {
		if walletmigration.WalletDatProbeOptional(configured, requireWalletDat) {
			return
		}
	}
	r.WalletMigration = live
	if live.Probe == nil || !live.Probe.IsBDB {
		if requireWalletDat {
			r.Issues = append(r.Issues, "wallet_dat_not_bdb")
		} else {
			r.Warnings = append(r.Warnings, "wallet_dat_not_bdb")
		}
	} else {
		r.Notes = append(r.Notes, walletDatProbeNote(live.Probe))
	}
	if walletPass != "" && !live.ExtractOK {
		if requireWalletDat {
			r.Issues = append(r.Issues, "wallet_dat_extract_failed")
		} else {
			r.Warnings = append(r.Warnings, "wallet_dat_extract_failed")
		}
		if live.ExtractError != "" {
			r.Notes = append(r.Notes, "wallet_dat_extract_error="+live.ExtractError)
		}
	} else if live.Probe != nil && live.Probe.CanImport && !live.Probe.NeedsPassphrase && !live.ExtractOK {
		if requireWalletDat {
			r.Issues = append(r.Issues, "wallet_dat_extract_failed")
		} else {
			r.Warnings = append(r.Warnings, "wallet_dat_extract_failed")
		}
		if live.ExtractError != "" {
			r.Notes = append(r.Notes, "wallet_dat_extract_error="+live.ExtractError)
		}
	}
}

func applyLiveProbePreflight(r *PreflightResult, live *walletmigration.LiveImportResult, requireWalletDat, extractOK bool) {
	if walletmigration.LiveProbeOK(live, requireWalletDat, extractOK) {
		if live != nil && live.Status == "probe_passed" {
			r.Notes = append(r.Notes, "wallet_dat_probe_ok")
		} else if live != nil && live.Status == "probe_needs_passphrase" && extractOK {
			r.Notes = append(r.Notes, "wallet_dat_probe_ok encrypted_extract_ok")
		}
		return
	}
	issue := "wallet_dat_probe_failed"
	if live != nil {
		switch live.Status {
		case "not_bdb":
			issue = "wallet_dat_not_bdb"
		case "probe_needs_passphrase":
			issue = "wallet_dat_extract_failed"
		case "probe_blocked":
			issue = "wallet_dat_probe_failed"
		}
		if live.Probe != nil {
			r.Notes = append(r.Notes, walletDatProbeNote(live.Probe))
		}
		if live.Error != "" {
			r.Notes = append(r.Notes, "wallet_dat_probe_error="+live.Error)
		}
	}
	if requireWalletDat {
		r.Issues = append(r.Issues, issue)
	} else {
		r.Warnings = append(r.Warnings, issue)
	}
}

func walletDatProbeNote(p *corewallet.ProbeResult) string {
	note := "wallet_dat_probe keys=" + strconv.Itoa(p.KeyCount) + " encrypted_keys=" + strconv.Itoa(p.EncryptedKeys)
	if p.PoolCount > 0 {
		note += " pool=" + strconv.Itoa(p.PoolCount)
		if p.PoolPubkeys > 0 {
			note += " pool_pubkeys=" + strconv.Itoa(p.PoolPubkeys)
		}
		if p.PoolKeysMatched > 0 {
			note += " pool_keys_matched=" + strconv.Itoa(p.PoolKeysMatched)
		}
		if p.PoolKeysUnmatched > 0 {
			note += " pool_keys_unmatched=" + strconv.Itoa(p.PoolKeysUnmatched)
			if hint := corewallet.PoolUnmatchedHint(p.PoolKeysUnmatched); hint != "" {
				note += " pool_unmatched_hint=" + hint
			}
		}
		if idx := corewallet.PoolIndexRangeNote(p); idx != "" {
			note += " " + idx
		}
		note += " keypool_note=keypoolrefill"
	}
	return note
}

func applyLiveImportPreflight(r *PreflightResult, live *walletmigration.LiveImportResult, requireWalletDat bool) {
	if walletmigration.LiveImportOK(live, requireWalletDat) {
		if live != nil && (live.Status == "passed" || live.Status == "passed_encrypted") {
			r.Notes = append(r.Notes, "wallet_dat_import_ok keys_imported="+strconv.Itoa(live.KeysImported))
			if live.KeypoolHint != "" {
				r.Notes = append(r.Notes, "wallet_dat_keypool_hint="+live.KeypoolHint)
			} else if live.Probe != nil && live.Probe.PoolCount > 0 {
				r.Notes = append(r.Notes, "wallet_dat_keypool_hint="+corewallet.PoolKeypoolHint())
			}
			if live.PoolUnmatchedHint != "" {
				r.Notes = append(r.Notes, "wallet_dat_pool_unmatched_hint="+live.PoolUnmatchedHint)
			} else if live.Probe != nil && live.Probe.PoolKeysUnmatched > 0 {
				if hint := corewallet.PoolUnmatchedHint(live.Probe.PoolKeysUnmatched); hint != "" {
					r.Notes = append(r.Notes, "wallet_dat_pool_unmatched_hint="+hint)
				}
			}
			if live.KeypoolRefillSize != nil && *live.KeypoolRefillSize > 0 {
				r.Notes = append(r.Notes, "wallet_dat_keypool_refill_size="+strconv.Itoa(*live.KeypoolRefillSize))
			}
			if live.PoolIndicesReplayed != nil {
				r.Notes = append(r.Notes, "wallet_dat_pool_indices_replayed="+strconv.FormatBool(*live.PoolIndicesReplayed))
			}
		}
		return
	}
	issue := "wallet_dat_import_failed"
	if live != nil {
		switch live.Status {
		case "required_missing":
			issue = "wallet_dat_required_missing"
		case "not_bdb":
			issue = "wallet_dat_not_bdb"
		case "probe_failed":
			issue = "wallet_dat_probe_failed"
		case "skipped_needs_passphrase", "skipped_encrypted_or_blocked":
			issue = "wallet_dat_extract_failed"
		}
		if live.Probe != nil {
			r.Notes = append(r.Notes, walletDatProbeNote(live.Probe))
		}
		if live.Error != "" {
			r.Notes = append(r.Notes, "wallet_dat_import_error="+live.Error)
		}
	}
	if requireWalletDat {
		r.Issues = append(r.Issues, issue)
	} else {
		r.Warnings = append(r.Warnings, issue)
	}
}

func resolveCoreCLI() string {
	if p := strings.TrimSpace(os.Getenv("DOGEGO_CORE_CLI")); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	if p, err := exec.LookPath("dogecoin-cli"); err == nil {
		return p
	}
	return ""
}

func invokeCoreCLI(cli string, port int, user, pass, method string) (map[string]any, error) {
	args := []string{"-rpcport=" + strconv.Itoa(port), method}
	if user != "" {
		args = append([]string{"-rpcuser=" + user, "-rpcpassword=" + pass}, args...)
	}
	out, err := exec.Command(cli, args...).CombinedOutput()
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, err
	}
	return m, nil
}
