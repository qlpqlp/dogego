// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package ui

// cloneStringAnyMap returns a shallow copy of m, recursively cloning nested map[string]any
// values so summary builders can patch sync_activity without racing a shared P2P snapshot.
func cloneStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		if nested, ok := v.(map[string]any); ok {
			out[k] = cloneStringAnyMap(nested)
			continue
		}
		out[k] = v
	}
	return out
}
