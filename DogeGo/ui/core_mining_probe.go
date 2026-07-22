// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"dogego/chain"
	"dogego/config"
	"dogego/consensus"
)

// CoreMiningCheck is one mining workflow row (Milestone E).
type CoreMiningCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok, warning, issue, skipped
	Value  any    `json:"value,omitempty"`
	Note   string `json:"note,omitempty"`
}

// CoreMiningProbeResult is returned by GET /api/core-mining-probe.
type CoreMiningProbeResult struct {
	CheckedAt        string            `json:"checked_at"`
	OK               bool              `json:"ok"`
	Network          string            `json:"network,omitempty"`
	Blocks           int64             `json:"blocks,omitempty"`
	Headers          int64             `json:"headers,omitempty"`
	IBD              bool              `json:"ibd,omitempty"`
	AuxEra           bool              `json:"aux_era,omitempty"`
	GBTFieldsOK      bool              `json:"gbt_fields_ok,omitempty"`
	CreateAuxOK      bool              `json:"createaux_ok,omitempty"`
	CreateAuxSkipped bool              `json:"createaux_skipped,omitempty"`
	MiningInfoOK     bool              `json:"mininginfo_ok,omitempty"`
	AuxPowParityOK   bool              `json:"auxpow_parity_ok,omitempty"`
	CoreConfigured   bool              `json:"core_configured,omitempty"`
	CoreAvailable    bool              `json:"core_available,omitempty"`
	CoreAligned      bool              `json:"core_aligned,omitempty"`
	CoreRPCAddr      string            `json:"core_rpc_addr,omitempty"`
	Checks           []CoreMiningCheck `json:"checks,omitempty"`
	Issues           []string          `json:"issues,omitempty"`
	Warnings         []string          `json:"warnings,omitempty"`
	Notes            []string          `json:"notes,omitempty"`
	Hint             string            `json:"hint,omitempty"`
}

var coreMiningRequiredRPC = []string{
	"getmininginfo", "getblocktemplate", "createauxblock", "submitauxblock", "submitblock", "generatetoaddress",
}

var coreMiningGBTRequiredFields = []string{
	"capabilities", "version", "rules", "vbrequired", "coinbaseaux", "previousblockhash",
	"bits", "target", "height", "curtime", "mintime", "sigoplimit", "sizelimit", "weightlimit",
	"coinbasevalue", "transactions", "mutable", "noncerange", "longpollid",
}

// ProbeCoreMining mirrors scripts/core_mining_workflow.ps1 (GBT + aux template + optional Core side-by-side).
func ProbeCoreMining(network string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreMiningProbeResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreMiningProbeResult{
		Network:        strings.TrimSpace(network),
		CoreConfigured: CoreCompareEnabled(network, conf),
		CoreRPCAddr:    ep.Addr,
		CheckedAt:      time.Now().UTC().Format(time.RFC3339),
		Hint:           "Milestone E mining cert - getmininginfo, getblocktemplate (Digishield bits + BIP22 longpoll), createauxblock in aux era, optional Core GBT side-by-side when core_rpc_addr is set and tips align. Mirrors scripts/core_mining_workflow.ps1 and dogego cert mining offline gates.",
	}
	if invoke == nil {
		out.Issues = append(out.Issues, "dogego_rpc_not_ready")
		return out
	}

	rpcInfo, rpcErr := invokeDogeGoRPC(invoke, "getrpcinfo", nil)
	if rpcErr != nil {
		out.Issues = append(out.Issues, "getrpcinfo_failed")
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "getrpcinfo", Status: "issue", Note: rpcErr.Error()})
	} else {
		methods, _ := rpcInfo["method"].(map[string]interface{})
		missing := false
		for _, m := range coreMiningRequiredRPC {
			st := "ok"
			if methods == nil || methods[m] == nil {
				out.Issues = append(out.Issues, "rpc_method_missing_"+m)
				st = "issue"
				missing = true
			}
			out.Checks = append(out.Checks, CoreMiningCheck{Name: "rpc." + m, Status: st})
		}
		if missing {
			out.OK = false
			return out
		}
	}

	info, infoErr := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	if infoErr != nil {
		out.Issues = append(out.Issues, "getblockchaininfo_failed")
		out.OK = false
		return out
	}
	if blk, ok := intFromAny(info["blocks"]); ok {
		out.Blocks = blk
	}
	if hdr, ok := intFromAny(info["headers"]); ok {
		out.Headers = hdr
	}
	if ibd, ok := info["initialblockdownload"].(bool); ok {
		out.IBD = ibd
	}
	if v, ok := info["dogego_auxpow_parent_chain_id_core_parity"].(bool); ok {
		out.AuxPowParityOK = v
		if !v {
			out.Issues = append(out.Issues, "auxpow_parent_chain_id_parity")
		}
	} else {
		out.AuxPowParityOK = true
	}

	net := parseProbeNetwork(network)
	nextHeight := out.Blocks + 1
	out.AuxEra = nextHeight >= consensus.AuxpowActivationHeight(net)

	gbt, gbtErr := invokeDogeGoRPC(invoke, "getblocktemplate", []any{map[string]interface{}{}})
	if gbtErr != nil {
		out.Issues = append(out.Issues, "getblocktemplate_failed")
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "getblocktemplate", Status: "issue", Note: gbtErr.Error()})
	} else {
		out.GBTFieldsOK = validateMiningGBTFields(gbt, &out)
		st := "ok"
		if !out.GBTFieldsOK {
			st = "issue"
		}
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "getblocktemplate_fields", Status: st})
		if h, ok := intFromAny(gbt["height"]); ok && h > 0 {
			out.Notes = append(out.Notes, fmt.Sprintf("gbt_height=%d", h))
		}
	}

	mi, miErr := invokeDogeGoRPC(invoke, "getmininginfo", nil)
	if miErr != nil {
		out.Issues = append(out.Issues, "getmininginfo_failed")
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "getmininginfo", Status: "issue", Note: miErr.Error()})
	} else {
		out.MiningInfoOK = miningInfoShapeOK(mi)
		st := "ok"
		if !out.MiningInfoOK {
			st = "issue"
			out.Issues = append(out.Issues, "getmininginfo_shape")
		}
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "getmininginfo", Status: st})
	}

	if out.AuxEra {
		addr, addrErr := miningProbePayoutAddress(network)
		if addrErr != nil {
			out.Issues = append(out.Issues, "createaux_payout_address")
			out.Checks = append(out.Checks, CoreMiningCheck{Name: "createauxblock", Status: "issue", Note: addrErr.Error()})
		} else {
			caux, cErr := invokeDogeGoRPC(invoke, "createauxblock", []any{addr})
			if cErr != nil {
				out.Issues = append(out.Issues, "createauxblock_failed")
				out.Checks = append(out.Checks, CoreMiningCheck{Name: "createauxblock", Status: "issue", Note: cErr.Error()})
			} else {
				out.CreateAuxOK = validateCreateAuxBlock(caux, &out)
				st := "ok"
				if !out.CreateAuxOK {
					st = "issue"
					out.Issues = append(out.Issues, "createauxblock_shape")
				}
				out.Checks = append(out.Checks, CoreMiningCheck{Name: "createauxblock", Status: st})
			}
		}
	} else {
		out.CreateAuxSkipped = true
		out.Notes = append(out.Notes, fmt.Sprintf("createaux_skipped_pre_aux_era (next=%d activation=%d)", nextHeight, consensus.AuxpowActivationHeight(net)))
		out.Checks = append(out.Checks, CoreMiningCheck{Name: "createauxblock", Status: "skipped", Note: "pre-auxpow era"})
	}

	if out.CoreConfigured && gbt != nil && gbtErr == nil {
		compareMiningGBTWithCore(ep, gbt, out.Blocks, &out)
	}

	out.OK = len(out.Issues) == 0 && out.GBTFieldsOK && out.MiningInfoOK && (out.CreateAuxOK || out.CreateAuxSkipped)
	return out
}

func parseProbeNetwork(network string) chain.Network {
	n := strings.ToLower(strings.TrimSpace(network))
	switch n {
	case "testnet", "reboottestnet":
		return chain.RebootTestnet
	default:
		return chain.MainnetDogecoin
	}
}

func miningProbePayoutAddress(network string) (string, error) {
	p, err := chain.ParamsFor(parseProbeNetwork(network))
	if err != nil {
		return "", err
	}
	return chain.RandomP2PKHAddress(p)
}

func validateMiningGBTFields(gbt map[string]any, out *CoreMiningProbeResult) bool {
	ok := true
	for _, f := range coreMiningGBTRequiredFields {
		if _, has := gbt[f]; !has {
			out.Issues = append(out.Issues, "gbt_missing_"+f)
			ok = false
		}
	}
	caps := stringSliceFromAny(gbt["capabilities"])
	if len(caps) == 0 || !containsStr(caps, "proposal") || !containsStr(caps, "longpoll") {
		out.Issues = append(out.Issues, "gbt_capabilities")
		ok = false
	}
	if lp, _ := gbt["longpollid"].(string); strings.TrimSpace(lp) == "" {
		out.Issues = append(out.Issues, "gbt_longpollid_empty")
		ok = false
	}
	if bits, _ := gbt["bits"].(string); strings.TrimSpace(bits) == "" {
		out.Issues = append(out.Issues, "gbt_bits_empty")
		ok = false
	}
	if tgt, _ := gbt["target"].(string); strings.TrimSpace(tgt) == "" {
		out.Issues = append(out.Issues, "gbt_target_empty")
		ok = false
	}
	return ok
}

func miningInfoShapeOK(mi map[string]any) bool {
	for _, k := range []string{"blocks", "difficulty", "networkhashps", "pooledtx", "chain"} {
		if _, ok := mi[k]; !ok {
			return false
		}
	}
	return true
}

func validateCreateAuxBlock(caux map[string]any, out *CoreMiningProbeResult) bool {
	ok := true
	for _, k := range []string{"hash", "chainid", "target", "coinbasevalue"} {
		if _, has := caux[k]; !has {
			out.Issues = append(out.Issues, "createaux_missing_"+k)
			ok = false
		}
	}
	if id, ok := caux["chainid"].(int32); ok && id != 0x62 {
		out.Issues = append(out.Issues, "createaux_chainid")
		ok = false
	}
	if id, ok := caux["chainid"].(float64); ok && int32(id) != 0x62 {
		out.Issues = append(out.Issues, "createaux_chainid")
		ok = false
	}
	return ok
}

func compareMiningGBTWithCore(ep CoreParityEndpoints, dgGBT map[string]any, dgBlocks int64, out *CoreMiningProbeResult) {
	coreInfo, err := invokeExternalRPCAny(ep.Addr, ep.User, ep.Pass, "getblockchaininfo", nil)
	if err != nil {
		out.Notes = append(out.Notes, "core_unreachable_for_mining_compare")
		return
	}
	out.CoreAvailable = true
	ci, ok := coreInfo.(map[string]interface{})
	if !ok {
		out.Warnings = append(out.Warnings, "core_blockchaininfo_shape")
		return
	}
	coreBlocks, _ := intFromAny(ci["blocks"])
	if coreBlocks != dgBlocks {
		out.Notes = append(out.Notes, fmt.Sprintf("core_tip_mismatch dogego=%d core=%d", dgBlocks, coreBlocks))
		return
	}
	coreGBT, err := invokeExternalRPCAny(ep.Addr, ep.User, ep.Pass, "getblocktemplate", []any{map[string]interface{}{}})
	if err != nil {
		out.Warnings = append(out.Warnings, "core_getblocktemplate_failed")
		return
	}
	cg, ok := coreGBT.(map[string]interface{})
	if !ok {
		out.Warnings = append(out.Warnings, "core_gbt_shape")
		return
	}
	aligned := true
	for _, k := range []string{"previousblockhash", "bits", "target", "height"} {
		dv := strFromAny(dgGBT[k])
		cv := strFromAny(cg[k])
		if dv == "" || cv == "" {
			continue
		}
		if dv != cv {
			aligned = false
			out.Issues = append(out.Issues, "core_gbt_drift_"+k)
		}
	}
	out.CoreAligned = aligned
	if aligned {
		out.Notes = append(out.Notes, "core_gbt_aligned")
	} else {
		out.Warnings = append(out.Warnings, "core_gbt_drift")
	}
}

func stringSliceFromAny(v any) []string {
	switch s := v.(type) {
	case []string:
		return s
	case []interface{}:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				out = append(out, str)
			}
		}
		return out
	default:
		return nil
	}
}

func containsStr(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
