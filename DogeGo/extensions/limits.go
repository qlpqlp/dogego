// Copyright (c) 2026 Paulo Vidal (https://x.com/inevitable360, https://github.com/qlpqlp)
// Copyright (c) 2026 Dogecoin Foundation
//
// SPDX-License-Identifier: MIT

package extensions

// MaxExtensionZipBytes limits compressed extension zip upload/download size.
const MaxExtensionZipBytes = 96 << 20 // 96 MiB

// MaxExtensionZipExtractBytes limits total unpacked bytes when installing a zip.
// Universal subprocess zips include all platform binaries (~86 MiB for dogego.zkl2).
const MaxExtensionZipExtractBytes = 128 << 20 // 128 MiB
