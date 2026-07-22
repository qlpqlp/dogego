// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package consensus

import _ "embed"

// Embedded copies ship with the binary so live UI probes work outside a git checkout.
//go:embed testdata/mempool_parity_rpc.json
var embeddedMempoolParityRPCJSON []byte

//go:embed testdata/core_mempool_vectors.json
var embeddedCoreMempoolVectorsJSON []byte
