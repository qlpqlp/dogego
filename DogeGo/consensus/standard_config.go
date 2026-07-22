// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// StandardPolicyFromNodeConfig builds relay policy from dogecoinconf.json / CLI merged values.
func StandardPolicyFromNodeConfig(hardDust int64, acceptDataCarrier, permitBareMultisig bool, datacarrierSize int) StandardPolicy {
	pol := DefaultStandardPolicy()
	if hardDust > 0 {
		pol.HardDustLimitKoinu = hardDust
	}
	pol.AcceptDataCarrier = acceptDataCarrier
	pol.AllowBareMultisig = permitBareMultisig
	if datacarrierSize > 0 {
		pol.MaxDatacarrierBytes = datacarrierSize
	}
	return pol
}
