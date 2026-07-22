// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

import "encoding/json"

// ParseListRPCResult decodes dogego_listextensions JSON-RPC envelopes into installed rows.
func ParseListRPCResult(listRes interface{}) []InstalledRow {
	raw, err := json.Marshal(listRes)
	if err != nil {
		return nil
	}
	var nested struct {
		Result struct {
			Extensions []InstalledRow `json:"extensions"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil && len(nested.Result.Extensions) > 0 {
		return nested.Result.Extensions
	}
	var envelope struct {
		Result []InstalledRow `json:"result"`
	}
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Result) > 0 {
		return envelope.Result
	}
	var wrapped struct {
		Extensions []InstalledRow `json:"extensions"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Extensions) > 0 {
		return wrapped.Extensions
	}
	var direct []InstalledRow
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct
	}
	return nil
}
