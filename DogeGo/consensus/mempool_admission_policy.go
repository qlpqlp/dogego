// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import "dogego/wire"

// AcceptMempoolTxPolicy runs mempool admission through sigop checks but skips script verification.
// Used for unsigned package-limit differential fixtures where script is not the policy under test.
func AcceptMempoolTxPolicy(tx *wire.Tx, adm MempoolAdmission) error {
	return acceptMempoolTx(tx, adm, false)
}
