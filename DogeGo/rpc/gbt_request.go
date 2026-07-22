// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"math"
)

func gbtClientRules(req map[string]interface{}) map[string]struct{} {
	out := map[string]struct{}{}
	if req == nil {
		return out
	}
	rules, ok := req["rules"].([]interface{})
	if !ok {
		return out
	}
	for _, r := range rules {
		if s, ok := r.(string); ok && s != "" {
			out[s] = struct{}{}
		}
	}
	return out
}

// gbtLegacyMaxVersion implements pre-versionbits clients (Core: maxversion when rules omitted).
func gbtLegacyMaxVersion(req map[string]interface{}, clientRules map[string]struct{}) (maxVer int64, ok bool) {
	if len(clientRules) > 0 || req == nil {
		return 0, false
	}
	v, ok := jsonNumberAsInt64(req["maxversion"])
	if !ok {
		return 0, false
	}
	return v, true
}

func gbtLegacyVersionForce(req map[string]interface{}, clientRules map[string]struct{}) bool {
	v, ok := gbtLegacyMaxVersion(req, clientRules)
	return ok && v >= 2
}

func jsonNumberAsInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case float64:
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return 0, false
		}
		return int64(n), true
	case json.Number:
		i, err := n.Int64()
		return i, err == nil
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}
