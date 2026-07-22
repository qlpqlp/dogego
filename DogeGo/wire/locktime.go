// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package wire

// LocktimeThreshold separates height- vs time-based nLockTime (Core LOCKTIME_THRESHOLD).
const LocktimeThreshold = 500_000_000

// SequenceFinal is the disabled nSequence value (Core CTxIn::SEQUENCE_FINAL).
const SequenceFinal uint32 = 0xffffffff
