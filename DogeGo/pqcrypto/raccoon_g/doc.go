// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT
// See LICENSE for copyright attribution to upstream Bitcoin/Dogecoin Core.

/*
Package raccoon_g vendors the Dogecoin Foundation libdogecoin in-tree Raccoon-G-44
port (src/raccoon_g on branch 0.1.5-dev). DogeGo compiles this tree via CGO; it
does not link the full libdogecoin library.

Primary author of the Foundation in-tree Raccoon-G-44 port:
  Ed Tubbs (Dogecoin Foundation) â€” https://github.com/edtubbs Â· https://x.com/EdTubbs

Upstream:
  https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g
Core (green) integration:
  https://github.com/dogecoinfoundation/dogecoin/pull/8
Pinned Python reference: p-11/lattice-hd-wallets@461a5ed9

Enable:

  CGO_ENABLED=1 go build -tags raccoon_g ./...

Requires libgmp and libmpfr (same as libdogecoin --enable-raccoon-g).
See BUILD.md. Without this tag, Raccoon crypto returns an error (no placeholder).

Acknowledgements: docs/CREDITS.md
*/
package raccoon_g

// UpstreamRef documents the libdogecoin tree this package tracks.
const UpstreamRef = "dogecoinfoundation/libdogecoin@0.1.5-dev/src/raccoon_g"

// UpstreamAuthor credits the Foundation developer of the in-tree Raccoon-G-44 port.
const UpstreamAuthor = "Ed Tubbs (Dogecoin Foundation)"

// UpstreamAuthorGitHub is https://github.com/edtubbs
const UpstreamAuthorGitHub = "https://github.com/edtubbs"

// UpstreamAuthorX is https://x.com/EdTubbs
const UpstreamAuthorX = "https://x.com/EdTubbs"

// CorePR documents the Core green integration of the same port.
const CorePR = "https://github.com/dogecoinfoundation/dogecoin/pull/8"

// LatticeHDCommit is the pinned lattice-hd-wallets commit from native/README.md.
const LatticeHDCommit = "461a5ed9b6d57e3bf8c381be3bb79325ab21d906"

// Wire sizes (byte-exact with libdogecoin thrc.h / raccoong_*_len).
const (
	PKBytes  = 16144
	SKBytes  = 32272
	SigBytes = 20768
)
