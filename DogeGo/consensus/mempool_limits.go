// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

// DefaultMaxMempoolDescendants matches Core DEFAULT_DESCENDANT_LIMIT (BIP125 package sizing).
const DefaultMaxMempoolDescendants = 25

// DefaultMaxMempoolAncestors matches Core DEFAULT_ANCESTOR_LIMIT for admission policy.
const DefaultMaxMempoolAncestors = 25

// DefaultMaxMempoolAncestorSizeKB matches Core DEFAULT_ANCESTOR_SIZE_LIMIT (-limitancestorsize).
const DefaultMaxMempoolAncestorSizeKB = 101

// DefaultMaxMempoolDescendantSizeKB matches Core DEFAULT_DESCENDANT_SIZE_LIMIT (-limitdescendantsize).
const DefaultMaxMempoolDescendantSizeKB = 101
