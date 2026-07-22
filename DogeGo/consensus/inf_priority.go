// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// InfPriority matches Core policy/fees.h INF_PRIORITY (1e9 * MAX_MONEY).
// estimatesmartpriority returns this when the mempool enforces a positive minimum fee.
const InfPriority = 1e9 * float64(MaxMoney)
