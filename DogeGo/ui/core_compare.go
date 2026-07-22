// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"dogego/config"
)

// CoreCompareField is one compared getblockchaininfo field.
type CoreCompareField struct {
	Name     string `json:"name"`
	DogeGo   any    `json:"dogego"`
	Core     any    `json:"core,omitempty"`
	Match    bool   `json:"match"`
	Note     string `json:"note,omitempty"`
}

// CoreCompareResult is returned by GET /api/core-compare.
type CoreCompareResult struct {
	Available       bool               `json:"core_available"`
	CoreConfigured  bool               `json:"core_configured,omitempty"`
	CoreRPCAddr     string             `json:"core_rpc_addr"`
	DogeGoRPCAddr string             `json:"dogego_rpc_addr"`
	Network       string             `json:"network"`
	ComparedAt    string             `json:"compared_at"`
	ChainOK            bool               `json:"chain_ok"`
	VerifyOK           bool               `json:"verify_ok"`
	ConnectLagOK       bool               `json:"connect_lag_ok,omitempty"`
	ProtocolLockOK     bool               `json:"protocol_lock_ok,omitempty"`
	DeploymentChecked  bool               `json:"deployment_checked,omitempty"`
	Fields             []CoreCompareField `json:"fields"`
	Errors        []string           `json:"errors,omitempty"`
	Hint          string             `json:"hint,omitempty"`
}

// DefaultCoreRPCAddr returns the loopback Core JSON-RPC address for side-by-side probes.
func DefaultCoreRPCAddr(network string) string {
	return ResolveCoreParityEndpoints(network, config.File{}).Addr
}

func coreRPCAuth() (user, pass string) {
	return coreRPCAuthFromEnv()
}

func invokeExternalRPC(addr, user, pass, method string, params []any) (map[string]any, error) {
	v, err := invokeExternalRPCAny(addr, user, pass, method, params)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object result for %s", method)
	}
	return m, nil
}

func invokeExternalRPCAny(addr, user, pass, method string, params []any) (any, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, fmt.Errorf("empty rpc addr")
	}
	if !strings.HasPrefix(addr, "http://") && !strings.HasPrefix(addr, "https://") {
		addr = "http://" + addr
	}
	if !strings.HasSuffix(addr, "/") {
		addr += "/"
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "1.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if user != "" {
		req.SetBasicAuth(user, pass)
	}
	client := &http.Client{Timeout: 12 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return nil, err
	}
	if wrap.Error != nil {
		return nil, fmt.Errorf("%s", wrap.Error.Message)
	}
	if len(wrap.Result) == 0 {
		return nil, fmt.Errorf("no result")
	}
	var result any
	if err := json.Unmarshal(wrap.Result, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func invokeDogeGoRPCAny(invoke func(string, []json.RawMessage) map[string]interface{}, method string, params []any) (any, error) {
	if invoke == nil {
		return nil, fmt.Errorf("dogego rpc not ready")
	}
	var rawParams []json.RawMessage
	if len(params) > 0 {
		b, err := json.Marshal(params)
		if err != nil {
			return nil, err
		}
		rawParams = []json.RawMessage{b}
	}
	out := invoke(method, rawParams)
	if errObj, ok := out["error"].(map[string]interface{}); ok && errObj != nil {
		msg, _ := errObj["message"].(string)
		if msg == "" {
			msg = "rpc error"
		}
		return nil, fmt.Errorf("%s", msg)
	}
	if res, ok := out["result"]; ok {
		return res, nil
	}
	return nil, fmt.Errorf("unexpected dogego result")
}

func invokeDogeGoRPC(invoke func(string, []json.RawMessage) map[string]interface{}, method string, params []any) (map[string]any, error) {
	v, err := invokeDogeGoRPCAny(invoke, method, params)
	if err != nil {
		return nil, err
	}
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected object result")
	}
	converted := make(map[string]any, len(m))
	for k, v := range m {
		converted[k] = v
	}
	return converted, nil
}

func boolFromAny(v any) bool {
	switch b := v.(type) {
	case bool:
		return b
	case string:
		return strings.EqualFold(b, "true")
	default:
		return false
	}
}

func intFromAny(v any) (int64, bool) {
	switch n := v.(type) {
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	default:
		return 0, false
	}
}

func strFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprint(v)
	}
}

func parityMaxDelta(envKey string, def int64) int64 {
	if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n >= 0 {
			return n
		}
	}
	return def
}

// ProbeCoreCompare compares getblockchaininfo between DogeGo and Dogecoin Core when Core is reachable.
func ProbeCoreCompare(network, dogeRPCAddr string, conf config.File, invoke func(string, []json.RawMessage) map[string]interface{}) CoreCompareResult {
	ep := ResolveCoreParityEndpoints(network, conf)
	out := CoreCompareResult{
		Available:      false,
		CoreConfigured: CoreRPCExplicitlyConfigured(network, conf),
		CoreRPCAddr:    ep.Addr,
		DogeGoRPCAddr: strings.TrimSpace(dogeRPCAddr),
		Network:       strings.TrimSpace(network),
		ComparedAt:    time.Now().UTC().Format(time.RFC3339),
		Hint:          "Run Dogecoin Core on loopback beside DogeGo, or set core_rpc_addr in Settings → Advanced. Mainnet: Core :22555, DogeGo :22557. Compare includes chain tips, chainwork, mediantime, verificationprogress when caught up, verifychain, UTXO set, mempool size, network version, and protocol-lock deployments when both nodes are caught up.",
	}
	user, pass := ep.User, ep.Pass
	dg, err := invokeDogeGoRPC(invoke, "getblockchaininfo", nil)
	if err != nil {
		out.Errors = append(out.Errors, "dogego: "+err.Error())
		return out
	}
	if !out.CoreConfigured {
		out.Hint = "Optional side-by-side cert: set core_rpc_addr in Settings → Advanced (or DOGEGO_CORE_RPC_ADDR). Solo protocol-lock sanity runs from getdeploymentinfo when available."
		compareDeploymentSoloSanity(&out, invoke, out.Network)
		deployOK := out.ProtocolLockOK
		hadDeploy := out.DeploymentChecked
		sfMatch := compareSoftforkSoloSanity(&out, dg, out.Network)
		switch {
		case hadDeploy && out.DeploymentChecked:
			out.ProtocolLockOK = deployOK && sfMatch
		case out.DeploymentChecked:
			out.ProtocolLockOK = sfMatch
		}
		updateProtocolLockField(&out, "")
		return out
	}
	core, err := invokeExternalRPC(out.CoreRPCAddr, user, pass, "getblockchaininfo", nil)
	if err != nil {
		out.Errors = append(out.Errors, "core: "+err.Error())
		compareDeploymentSoloSanity(&out, invoke, out.Network)
		deployOK := out.ProtocolLockOK
		hadDeploy := out.DeploymentChecked
		sfMatch := compareSoftforkSoloSanity(&out, dg, out.Network)
		switch {
		case hadDeploy && out.DeploymentChecked:
			out.ProtocolLockOK = deployOK && sfMatch
		case out.DeploymentChecked:
			out.ProtocolLockOK = sfMatch
		}
		updateProtocolLockField(&out, "")
		return out
	}
	out.Available = true

	maxHeader := parityMaxDelta("DOGEGO_PARITY_MAX_HEADER_DELTA", 100)
	maxBlock := parityMaxDelta("DOGEGO_PARITY_MAX_BLOCK_DELTA", 500)

	dgChain := strFromAny(dg["chain"])
	coreChain := strFromAny(core["chain"])
	chainMatch := dgChain != "" && dgChain == coreChain
	out.Fields = append(out.Fields, CoreCompareField{Name: "chain", DogeGo: dgChain, Core: coreChain, Match: chainMatch})

	dgBest := strFromAny(dg["bestblockhash"])
	coreBest := strFromAny(core["bestblockhash"])
	out.Fields = append(out.Fields, CoreCompareField{Name: "bestblockhash", DogeGo: dgBest, Core: coreBest, Match: dgBest != "" && dgBest == coreBest})

	dgHdr, dgHdrOK := intFromAny(dg["headers"])
	coreHdr, coreHdrOK := intFromAny(core["headers"])
	hdrMatch := dgHdrOK && coreHdrOK && abs64(dgHdr-coreHdr) <= maxHeader
	note := ""
	if dgHdrOK && coreHdrOK && !hdrMatch {
		note = fmt.Sprintf("delta %d (max %d)", dgHdr-coreHdr, maxHeader)
	}
	out.Fields = append(out.Fields, CoreCompareField{Name: "headers", DogeGo: dgHdr, Core: coreHdr, Match: hdrMatch, Note: note})

	dgBlk, dgBlkOK := intFromAny(dg["blocks"])
	coreBlk, coreBlkOK := intFromAny(core["blocks"])
	blkMatch := dgBlkOK && coreBlkOK && abs64(dgBlk-coreBlk) <= maxBlock
	note = ""
	if dgBlkOK && coreBlkOK && !blkMatch {
		note = fmt.Sprintf("delta %d (max %d)", dgBlk-coreBlk, maxBlock)
	}
	out.Fields = append(out.Fields, CoreCompareField{Name: "blocks", DogeGo: dgBlk, Core: coreBlk, Match: blkMatch, Note: note})

	if dgDiff, dgDiffOK := floatFromAny(dg["difficulty"]); dgDiffOK {
		if coreDiff, coreDiffOK := floatFromAny(core["difficulty"]); coreDiffOK {
			diffMatch := floatNearlyEqual(dgDiff, coreDiff)
			diffNote := ""
			if !diffMatch {
				diffNote = "tips may differ during catch-up"
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "difficulty", DogeGo: dgDiff, Core: coreDiff, Match: diffMatch, Note: diffNote,
			})
		}
	}

	dgIBD, _ := dg["initialblockdownload"].(bool)
	coreIBD, _ := core["initialblockdownload"].(bool)
	out.Fields = append(out.Fields, CoreCompareField{Name: "initialblockdownload", DogeGo: dgIBD, Core: coreIBD, Match: dgIBD == coreIBD})

	consensusOK := compareChainInfoConsensusFields(&out, dg, core)

	if v, ok := dg["dogego_contiguous_raw_height"]; ok {
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "dogego_contiguous_raw_height", DogeGo: v, Core: "-",
			Match: true, Note: "DogeGo-only (stored bodies vs headers)",
		})
	}

	dgVerify, dgVErr := invokeDogeGoRPCAny(invoke, "verifychain", []any{3, 0})
	coreVerify, coreVErr := invokeExternalRPCAny(out.CoreRPCAddr, user, pass, "verifychain", []any{3, 0})
	if dgVErr == nil && coreVErr == nil {
		dgVB := boolFromAny(dgVerify)
		coreVB := boolFromAny(coreVerify)
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "verifychain(3,0)", DogeGo: dgVB, Core: coreVB, Match: dgVB && coreVB,
		})
		out.VerifyOK = dgVB && coreVB
	} else if dgVErr != nil {
		out.Errors = append(out.Errors, "dogego verifychain: "+dgVErr.Error())
	}

	out.ChainOK = chainMatch && hdrMatch && blkMatch && consensusOK

	if tips, tipsErr := invokeDogeGoRPCAny(invoke, "getchaintips", nil); tipsErr == nil {
		active := 0
		if arr, ok := tips.([]interface{}); ok {
			for _, item := range arr {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				if strings.EqualFold(strFromAny(m["status"]), "active") {
					active++
				}
			}
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "getchaintips(active)", DogeGo: active, Core: "-",
			Match: active == 1, Note: "DogeGo-only; expect exactly one active tip",
		})
	}
	if lag, ok := intFromAny(dg["dogego_filter_index_lag"]); ok && lag > 0 {
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "dogego_filter_index_lag", DogeGo: lag, Core: "-",
			Match: true, Note: "DogeGo-only",
		})
	}
	maxConnectLag := parityMaxDelta("DOGEGO_PARITY_MAX_CONNECT_LAG", 256)
	if lag, ok := intFromAny(dg["dogego_connect_lag"]); ok {
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "dogego_connect_lag", DogeGo: lag, Core: "-",
			Match: lag <= maxConnectLag,
			Note:  "DogeGo-only; stored bodies ahead of chainActive",
		})
	}

	if !dgIBD && !coreIBD && out.Available {
		dgUtxo, dgUErr := invokeDogeGoRPC(invoke, "gettxoutsetinfo", nil)
		coreUtxo, coreUErr := invokeExternalRPC(out.CoreRPCAddr, user, pass, "gettxoutsetinfo", nil)
		if dgUErr == nil && coreUErr == nil {
			dgH, _ := intFromAny(dgUtxo["height"])
			coreH, _ := intFromAny(coreUtxo["height"])
			heightMatch := dgH == coreH
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "gettxoutsetinfo.height", DogeGo: dgH, Core: coreH,
				Match: heightMatch, Note: "when both nodes caught up",
			})
			dgHash := strFromAny(dgUtxo["hash_serialized"])
			coreHash := strFromAny(coreUtxo["hash_serialized"])
			if dgHash != "" && coreHash != "" {
				out.Fields = append(out.Fields, CoreCompareField{
					Name: "gettxoutsetinfo.hash_serialized", DogeGo: dgHash, Core: coreHash,
					Match: dgHash == coreHash,
					Note:  "may differ during catch-up",
				})
			}
		}
		dgMem, dgMErr := invokeDogeGoRPC(invoke, "getmempoolinfo", nil)
		coreMem, coreMErr := invokeExternalRPC(out.CoreRPCAddr, user, pass, "getmempoolinfo", nil)
		if dgMErr == nil && coreMErr == nil {
			dgSize, _ := intFromAny(dgMem["size"])
			coreSize, _ := intFromAny(coreMem["size"])
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "getmempoolinfo.size", DogeGo: dgSize, Core: coreSize,
				Match: true, Note: "informational; may differ on solo nodes",
			})
			compareMempoolRelayPolicyFields(&out, dgMem, coreMem)
		}
		dgNet, dgNErr := invokeDogeGoRPC(invoke, "getnetworkinfo", nil)
		coreNet, coreNErr := invokeExternalRPC(out.CoreRPCAddr, user, pass, "getnetworkinfo", nil)
		if dgNErr == nil && coreNErr == nil {
			dgVN, _ := intFromAny(dgNet["version"])
			coreVN, _ := intFromAny(coreNet["version"])
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "getnetworkinfo.version", DogeGo: dgVN, Core: coreVN, Match: dgVN == coreVN,
			})
		}
	}

	compareDeploymentParity(&out, invoke, user, pass)
	deployOK := out.ProtocolLockOK
	hadDeploy := out.DeploymentChecked
	if out.Available {
		sfMatch := compareSoftforkParity(&out, dg, core)
		switch {
		case hadDeploy && out.DeploymentChecked:
			out.ProtocolLockOK = deployOK && sfMatch
		case out.DeploymentChecked:
			out.ProtocolLockOK = sfMatch
		}
	}
	if !out.DeploymentChecked {
		compareDeploymentSoloSanity(&out, invoke, out.Network)
		deployOK = out.ProtocolLockOK
		hadDeploy = out.DeploymentChecked
		sfMatch := compareSoftforkSoloSanity(&out, dg, out.Network)
		switch {
		case hadDeploy && out.DeploymentChecked:
			out.ProtocolLockOK = deployOK && sfMatch
		case out.DeploymentChecked:
			out.ProtocolLockOK = sfMatch
		}
	}
	updateProtocolLockField(&out, "")

	out.ConnectLagOK = true
	for _, f := range out.Fields {
		if f.Name == "dogego_connect_lag" && !f.Match {
			out.ConnectLagOK = false
			break
		}
	}

	return out
}

func compareMempoolRelayPolicyFields(out *CoreCompareResult, dgMem, coreMem map[string]any) {
	type relayField struct {
		name string
		note string
	}
	for _, rf := range []relayField{
		{"getmempoolinfo.fullrbf", "mempoolfullrbf policy"},
		{"getmempoolinfo.minrelaytxfee", "configured min relay DOGE/kB"},
		{"getmempoolinfo.incrementalrelayfee", "BIP125 incremental relay DOGE/kB"},
	} {
		key := strings.TrimPrefix(rf.name, "getmempoolinfo.")
		dgVal, dgOK := dgMem[key]
		coreVal, coreOK := coreMem[key]
		if !dgOK || !coreOK {
			continue
		}
		match := false
		switch dgVal.(type) {
		case bool:
			match = boolFromAny(dgVal) == boolFromAny(coreVal)
		default:
			dgF, dgFok := floatFromAny(dgVal)
			coreF, coreFok := floatFromAny(coreVal)
			match = dgFok && coreFok && floatNearlyEqual(dgF, coreF)
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name: rf.name, DogeGo: dgVal, Core: coreVal, Match: match, Note: rf.note,
		})
	}
	if dgPkg, dgOK := dgMem["dogego_package_policy"].(map[string]any); dgOK && len(dgPkg) > 0 {
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "getmempoolinfo.dogego_package_policy", DogeGo: dgPkg,
			Match: true, Note: "DogeGo-only; Core exposes package limits via CLI flags only",
		})
	}
}

func abs64(n int64) int64 {
	if n < 0 {
		return -n
	}
	return n
}

func floatFromAny(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func floatNearlyEqual(a, b float64) bool {
	if a == b {
		return true
	}
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	if diff < 1e-12 {
		return true
	}
	denom := a
	if denom < 0 {
		denom = -denom
	}
	if denom == 0 {
		denom = b
		if denom < 0 {
			denom = -denom
		}
	}
	if denom == 0 {
		return true
	}
	return diff/denom < 1e-6
}
