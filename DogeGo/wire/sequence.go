// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// BIP68 relative lock-time sequence flags (Core CTxIn).
const (
	SequenceLocktimeDisableFlag = 1 << 31
	SequenceLocktimeTypeFlag    = 1 << 22
	SequenceLocktimeMask        = 0x0000ffff
	SequenceLocktimeGranularity = 9
)
