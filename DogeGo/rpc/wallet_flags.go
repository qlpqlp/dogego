// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"strings"
)

// execSetWalletFlag sets supported wallet flags (avoid_reuse).
func execSetWalletFlag(paths *DataPaths, params []json.RawMessage) (interface{}, int, string) {
	if len(params) != 2 {
		return nil, -32602, "Wrong number of arguments"
	}
	var flag string
	if err := json.Unmarshal(params[0], &flag); err != nil {
		return nil, -8, "setwalletflag: flag must be a string"
	}
	flag = strings.ToLower(strings.TrimSpace(flag))
	value, code, msg := parseRPCBoolOpt(params[1], false, "setwalletflag", "value")
	if code != 0 {
		return nil, code, msg
	}
	if paths == nil || paths.WalletSetAvoidReuse == nil || rpcWalletAddress(paths) == "" {
		return nil, -1, "setwalletflag: wallet is not implemented in DogeGo"
	}
	switch flag {
	case "avoid_reuse":
		if err := paths.WalletSetAvoidReuse(value); err != nil {
			return nil, -1, "setwalletflag: "+err.Error()
		}
		return true, 0, ""
	case "pq_commitments":
		if paths.WalletSetPqCommitmentsEnabled == nil {
			return nil, -4, "setwalletflag: unknown flag "+flag
		}
		if err := paths.WalletSetPqCommitmentsEnabled(value); err != nil {
			return nil, -1, "setwalletflag: "+err.Error()
		}
		return true, 0, ""
	case "pq_carrier":
		if paths.WalletSetPqCarrierEnabled == nil {
			return nil, -4, "setwalletflag: unknown flag "+flag
		}
		if err := paths.WalletSetPqCarrierEnabled(value); err != nil {
			return nil, -1, "setwalletflag: "+err.Error()
		}
		return true, 0, ""
	default:
		return nil, -4, "setwalletflag: unknown flag "+flag
	}
}

func rpcWalletAvoidReuse(paths *DataPaths) bool {
	if paths != nil && paths.WalletAvoidReuse != nil {
		return paths.WalletAvoidReuse()
	}
	return false
}

func rpcWalletPqCommitmentsEnabled(paths *DataPaths) bool {
	if paths != nil && paths.WalletPqCommitmentsEnabled != nil {
		return paths.WalletPqCommitmentsEnabled()
	}
	return false
}
