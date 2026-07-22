// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

package chain

import "dogego/version"

// ClientVersion is the DogeGo release version exposed to RPC and the web UI.
const ClientVersion = version.ClientVersion

// CoreBaseVersion is the Dogecoin Core release targeted for consensus/P2P parity.
const CoreBaseVersion = version.CoreBaseVersion

// BuildSubVersion returns the P2P sub-version string for this node.
func BuildSubVersion(uaComment string) string {
	return version.BuildSubVersion(uaComment)
}
