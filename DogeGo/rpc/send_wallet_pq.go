// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package rpc

import (
	"encoding/json"
	"maps"
)

// peelPQCommitFromSendOptions removes pqcommit from fund options when the wallet flag is enabled.
func peelPQCommitFromSendOptions(opts map[string]interface{}, paths *DataPaths, errPrefix string) (map[string]interface{}, int, string) {
	if opts == nil {
		return nil, 0, ""
	}
	raw, ok := opts["pqcommit"]
	if !ok {
		return nil, 0, ""
	}
	if !rpcWalletPqCommitmentsEnabled(paths) {
		return nil, -8, errPrefix + ": pqcommit requires setwalletflag pq_commitments true"
	}
	delete(opts, "pqcommit")
	spec, ok := raw.(map[string]interface{})
	if !ok {
		return nil, -8, errPrefix + ": pqcommit must be object with tag and commitment"
	}
	return spec, 0, ""
}

func marshalWalletOutputs(outputs map[string]float64, pqSpec map[string]interface{}) ([]byte, error) {
	keys := sortedOutputAddresses(outputs)
	m := make(map[string]interface{}, len(keys)+1)
	for _, k := range keys {
		m[k] = outputs[k]
	}
	if pqSpec != nil {
		m["pqcommit"] = pqSpec
	}
	return json.Marshal(m)
}

// cloneSendFundOptions returns a copy of optional send fund options (may be nil).
func cloneSendFundOptions(opts map[string]interface{}) map[string]interface{} {
	if opts == nil {
		return nil
	}
	return maps.Clone(opts)
}
