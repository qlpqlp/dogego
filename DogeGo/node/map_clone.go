// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package node

// cloneStringAnyMap returns a shallow copy of m, recursively cloning nested map[string]any
// values so dashboard consumers can mutate without racing a shared cache entry.
func cloneStringAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch nested := v.(type) {
		case map[string]any:
			out[k] = cloneStringAnyMap(nested)
		default:
			out[k] = v
		}
	}
	return out
}
