// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"dogego/consensus"
)

// compareDeploymentParity checks consensus deployment (buried + BIP9) active-state
// against Dogecoin Core when Core is reachable.
func compareDeploymentParity(out *CoreCompareResult, invoke func(string, []json.RawMessage) map[string]interface{}, user, pass string) {
	dgDep, dgErr := invokeDogeGoRPC(invoke, "getdeploymentinfo", nil)
	if dgErr != nil {
		return
	}
	coreDep, coreErr := invokeExternalRPC(out.CoreRPCAddr, user, pass, "getdeploymentinfo", nil)
	if coreErr != nil {
		return
	}
	dgMap := deploymentActiveMap(dgDep["deployments"])
	coreMap := deploymentActiveMap(coreDep["deployments"])
	if len(dgMap) == 0 || len(coreMap) == 0 {
		return
	}
	out.DeploymentChecked = true
	allMatch := appendDeploymentActiveFields(out, dgMap, coreMap, "all consensus deployments share Core active-state (no protocol fork)")
	if !compareDeploymentDetail(out, dgDep["deployments"], coreDep["deployments"]) {
		allMatch = false
	}
	out.ProtocolLockOK = allMatch
}

// deploymentDetail holds Core-comparable BIP9 / buried fields for one deployment.
type deploymentDetail struct {
	typ    string
	active bool
	height int64
	hasH   bool
	bit    int64
	hasBit bool
	status string
	since  int64
	hasS   bool
}

// compareDeploymentDetail compares BIP9 bit/status/since and buried activation height
// per deployment. Returns false on any mismatch. Purely additive compare fields.
func compareDeploymentDetail(out *CoreCompareResult, dgDeps, coreDeps any) bool {
	dg := deploymentDetailMap(dgDeps)
	core := deploymentDetailMap(coreDeps)
	if len(dg) == 0 || len(core) == 0 {
		return true
	}
	names := make([]string, 0, len(dg))
	for name := range dg {
		if _, ok := core[name]; ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	allMatch := true
	for _, name := range names {
		d := dg[name]
		c := core[name]
		if d.hasBit && c.hasBit {
			match := d.bit == c.bit
			if !match {
				allMatch = false
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "deployment." + name + ".bit", DogeGo: d.bit, Core: c.bit, Match: match,
			})
		}
		if d.status != "" && c.status != "" {
			match := d.status == c.status
			if !match {
				allMatch = false
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "deployment." + name + ".status", DogeGo: d.status, Core: c.status, Match: match,
				Note: statusMismatchNote(match),
			})
		}
		if d.hasS && c.hasS && (d.status == "active" || c.status == "active") {
			match := d.since == c.since
			if !match {
				allMatch = false
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "deployment." + name + ".since", DogeGo: d.since, Core: c.since, Match: match,
			})
		}
		if d.hasH && c.hasH {
			match := d.height == c.height
			if !match {
				allMatch = false
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "deployment." + name + ".height", DogeGo: d.height, Core: c.height, Match: match,
				Note: "activation height",
			})
		}
	}
	return allMatch
}

func statusMismatchNote(match bool) string {
	if match {
		return ""
	}
	return "BIP9 status mismatch (protocol lock check)"
}

func deploymentDetailMap(v any) map[string]deploymentDetail {
	m := asStringMap(v)
	if m == nil {
		return nil
	}
	out := make(map[string]deploymentDetail, len(m))
	for name, dep := range m {
		depMap := asStringMap(dep)
		if depMap == nil {
			continue
		}
		d := deploymentDetail{
			typ:    strFromAny(depMap["type"]),
			active: boolFromAny(depMap["active"]),
		}
		if h, ok := intFromAny(depMap["height"]); ok {
			d.height = h
			d.hasH = true
		}
		if bip9 := asStringMap(depMap["bip9"]); bip9 != nil {
			if b, ok := intFromAny(bip9["bit"]); ok {
				d.bit = b
				d.hasBit = true
			}
			d.status = strFromAny(bip9["status"])
			if s, ok := intFromAny(bip9["since"]); ok {
				d.since = s
				d.hasS = true
			}
		}
		out[name] = d
	}
	return out
}

func asStringMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return nil
}

// compareDeploymentSoloSanity checks DogeGo getdeploymentinfo against consensus params at tip.
// Runs when Core is not configured or unreachable so solo operators still get a protocol-lock signal.
func compareDeploymentSoloSanity(out *CoreCompareResult, invoke func(string, []json.RawMessage) map[string]interface{}, network string) {
	dgDep, dgErr := invokeDogeGoRPC(invoke, "getdeploymentinfo", nil)
	if dgErr != nil {
		return
	}
	height, ok := intFromAny(dgDep["height"])
	if !ok {
		return
	}
	actual := deploymentActiveMap(dgDep["deployments"])
	expected, err := expectedDeploymentsActive(network, height)
	if err != nil || len(expected) == 0 || len(actual) == 0 {
		return
	}
	out.DeploymentChecked = true
	allMatch := appendDeploymentActiveFields(out, actual, expected, fmt.Sprintf("solo sanity at height %d (no Core configured)", height))
	out.ProtocolLockOK = allMatch
}

func expectedDeploymentsActive(network string, height int64) (map[string]bool, error) {
	net, err := networkFromUISlug(network)
	if err != nil {
		return nil, err
	}
	dc := consensus.LookupConsensus(net, height)
	out := map[string]bool{
		"bip34": height >= int64(dc.BIP34Height),
		"bip66": height >= int64(dc.BIP66Height),
		"bip65": height >= int64(dc.BIP65Height),
		"csv":   height >= int64(dc.CSVHeight),
	}
	return out, nil
}

func appendDeploymentActiveFields(out *CoreCompareResult, dogeMap, refMap map[string]bool, lockNote string) bool {
	names := make([]string, 0, len(dogeMap)+len(refMap))
	seen := make(map[string]struct{})
	for name := range dogeMap {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range refMap {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	allMatch := true
	for _, name := range names {
		dgVal, dgOK := dogeMap[name]
		refVal, refOK := refMap[name]
		match := dgOK && refOK && dgVal == refVal
		note := ""
		if !match {
			allMatch = false
			switch {
			case !refOK:
				note = "reference does not report this deployment"
			case !dgOK:
				note = "DogeGo does not report this deployment"
			default:
				note = "active-state mismatch (protocol lock check)"
			}
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name:   "deployment." + name + ".active",
			DogeGo: dgVal,
			Core:   refVal,
			Match:  match,
			Note:   note,
		})
	}
	out.Fields = append(out.Fields, CoreCompareField{
		Name:   "deployment.protocol_lock",
		DogeGo: allMatch,
		Core:   true,
		Match:  allMatch,
		Note:   lockNote,
	})
	return allMatch
}

// updateProtocolLockField refreshes deployment.protocol_lock after deployment + softfork checks.
func updateProtocolLockField(out *CoreCompareResult, note string) {
	if !out.DeploymentChecked {
		return
	}
	for i := range out.Fields {
		if out.Fields[i].Name == "deployment.protocol_lock" {
			out.Fields[i].Match = out.ProtocolLockOK
			out.Fields[i].DogeGo = out.ProtocolLockOK
			if note != "" {
				out.Fields[i].Note = note
			}
			return
		}
	}
	lockNote := note
	if lockNote == "" {
		lockNote = "protocol lock (deployments + softforks)"
	}
	out.Fields = append(out.Fields, CoreCompareField{
		Name: "deployment.protocol_lock", DogeGo: out.ProtocolLockOK, Core: true,
		Match: out.ProtocolLockOK, Note: lockNote,
	})
}

// compareSoftforkParity checks getblockchaininfo softforks + bip9_softforks vs Core.
func compareSoftforkParity(out *CoreCompareResult, dgInfo, coreInfo map[string]any) bool {
	allMatch := true
	dgReject := softforkRejectMap(dgInfo["softforks"])
	coreReject := softforkRejectMap(coreInfo["softforks"])
	if len(dgReject) > 0 && len(coreReject) > 0 {
		out.DeploymentChecked = true
		if !appendSoftforkRejectFields(out, dgReject, coreReject) {
			allMatch = false
		}
	}
	dgBip9 := bip9SoftforkStatusMap(dgInfo["bip9_softforks"])
	coreBip9 := bip9SoftforkStatusMap(coreInfo["bip9_softforks"])
	if len(dgBip9) > 0 && len(coreBip9) > 0 {
		out.DeploymentChecked = true
		if !appendBip9SoftforkActiveFields(out, dgBip9, coreBip9) {
			allMatch = false
		}
	}
	return allMatch
}

// compareSoftforkSoloSanity checks getblockchaininfo softforks against consensus params at tip.
func compareSoftforkSoloSanity(out *CoreCompareResult, dgInfo map[string]any, network string) bool {
	blocks, ok := intFromAny(dgInfo["blocks"])
	if !ok {
		return true
	}
	expected, err := expectedDeploymentsActive(network, blocks)
	if err != nil || len(expected) == 0 {
		return true
	}
	allMatch := true
	dgReject := softforkRejectMap(dgInfo["softforks"])
	if len(dgReject) > 0 {
		out.DeploymentChecked = true
		ref := map[string]bool{
			"bip34": expected["bip34"],
			"bip66": expected["bip66"],
			"bip65": expected["bip65"],
		}
		if !appendSoftforkRejectFields(out, dgReject, ref) {
			allMatch = false
		}
	}
	dgBip9 := bip9SoftforkStatusMap(dgInfo["bip9_softforks"])
	if len(dgBip9) > 0 {
		out.DeploymentChecked = true
		refBip9 := make(map[string]string, len(expected))
		for name, active := range expected {
			if name == "bip34" || name == "bip66" || name == "bip65" {
				continue
			}
			if active {
				refBip9[name] = "active"
			} else {
				refBip9[name] = "not_active"
			}
		}
		if !appendBip9SoftforkActiveFields(out, dgBip9, refBip9) {
			allMatch = false
		}
	}
	return allMatch
}

func appendSoftforkRejectFields(out *CoreCompareResult, dogeMap, refMap map[string]bool) bool {
	names := make([]string, 0, len(dogeMap)+len(refMap))
	seen := make(map[string]struct{})
	for name := range dogeMap {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range refMap {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	allMatch := true
	for _, name := range names {
		dgVal, dgOK := dogeMap[name]
		refVal, refOK := refMap[name]
		match := dgOK && refOK && dgVal == refVal
		note := ""
		if !match {
			allMatch = false
			switch {
			case !refOK:
				note = "reference does not report this softfork"
			case !dgOK:
				note = "DogeGo does not report this softfork"
			default:
				note = "reject.status mismatch (protocol lock check)"
			}
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name: "softfork." + name + ".reject", DogeGo: dgVal, Core: refVal, Match: match, Note: note,
		})
	}
	return allMatch
}

func appendBip9SoftforkActiveFields(out *CoreCompareResult, dogeStatus, refStatus map[string]string) bool {
	names := make([]string, 0, len(dogeStatus)+len(refStatus))
	seen := make(map[string]struct{})
	for name := range dogeStatus {
		names = append(names, name)
		seen[name] = struct{}{}
	}
	for name := range refStatus {
		if _, ok := seen[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	allMatch := true
	for _, name := range names {
		dgStatus, dgOK := dogeStatus[name]
		refStatusVal, refOK := refStatus[name]
		dgActive := bip9StatusActive(dgStatus)
		refActive := bip9StatusActive(refStatusVal)
		if refStatusVal == "not_active" {
			refActive = false
		}
		match := dgOK && refOK && dgActive == refActive
		note := ""
		if !match {
			allMatch = false
			note = "BIP9 softfork active-state mismatch (protocol lock check)"
		}
		out.Fields = append(out.Fields, CoreCompareField{
			Name:   "bip9_softfork." + name + ".active",
			DogeGo: dgActive,
			Core:   refActive,
			Match:  match,
			Note:   note,
		})
		if dgOK && refOK && dgStatus != "" && refStatusVal != "" && refStatusVal != "not_active" {
			statusMatch := dgStatus == refStatusVal
			if !statusMatch {
				allMatch = false
			}
			out.Fields = append(out.Fields, CoreCompareField{
				Name: "bip9_softfork." + name + ".status", DogeGo: dgStatus, Core: refStatusVal, Match: statusMatch,
			})
		}
	}
	return allMatch
}

func bip9StatusActive(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "active")
}

func softforkRejectMap(v any) map[string]bool {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make(map[string]bool, len(arr))
	for _, item := range arr {
		m := asStringMap(item)
		if m == nil {
			continue
		}
		id := strFromAny(m["id"])
		if id == "" {
			continue
		}
		reject := asStringMap(m["reject"])
		if reject == nil {
			continue
		}
		out[id] = boolFromAny(reject["status"])
	}
	return out
}

func bip9SoftforkStatusMap(v any) map[string]string {
	m := asStringMap(v)
	if m == nil {
		return nil
	}
	out := make(map[string]string, len(m))
	for name, dep := range m {
		depMap := asStringMap(dep)
		if depMap == nil {
			continue
		}
		out[name] = strFromAny(depMap["status"])
	}
	return out
}

// deploymentActiveMap flattens getdeploymentinfo.deployments into name -> active bool.
func deploymentActiveMap(v any) map[string]bool {
	m := asStringMap(v)
	if m == nil {
		return nil
	}
	out := make(map[string]bool, len(m))
	for name, dep := range m {
		depMap := asStringMap(dep)
		if depMap == nil {
			continue
		}
		out[name] = boolFromAny(depMap["active"])
	}
	return out
}
