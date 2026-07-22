// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import "fmt"

// ParseDispatchResponse returns result or an error from a Dispatch response map.
func ParseDispatchResponse(resp map[string]interface{}) (interface{}, error) {
	if resp == nil {
		return nil, fmt.Errorf("rpc dispatch: no response")
	}
	if errObj, ok := resp["error"]; ok && errObj != nil {
		msg := "rpc error"
		if em, ok := errObj.(map[string]interface{}); ok {
			if s, ok := em["message"].(string); ok && s != "" {
				msg = s
			}
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return resp["result"], nil
}
