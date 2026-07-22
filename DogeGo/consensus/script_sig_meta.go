// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// ScriptSigPushes decodes all pushes in a push-only script (typical legacy scriptSig).
func ScriptSigPushes(script []byte) ([][]byte, error) {
	var pushes [][]byte
	i := 0
	for i < len(script) {
		data, next, err := ReadScriptPush(script, i)
		if err != nil {
			return nil, err
		}
		pushes = append(pushes, data)
		i = next
	}
	return pushes, nil
}

// ScriptSigRedeemMetas returns RedeemScriptMeta for each push that matches a known template.
func ScriptSigRedeemMetas(script []byte) []map[string]interface{} {
	if part, err := ParsePQCarrierPartScriptSig(script); err == nil {
		if meta := PQCarrierFields(part); meta != nil {
			return []map[string]interface{}{meta}
		}
	}
	pushes, err := ScriptSigPushes(script)
	if err != nil {
		return nil
	}
	var out []map[string]interface{}
	for i, p := range pushes {
		meta := RedeemScriptMeta(p)
		if meta == nil {
			continue
		}
		entry := make(map[string]interface{}, len(meta)+1)
		for k, v := range meta {
			entry[k] = v
		}
		entry["dogego_push_index"] = i
		out = append(out, entry)
	}
	return out
}
