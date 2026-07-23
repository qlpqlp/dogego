/*
 * Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
 * SPDX-License-Identifier: MIT
 *
 * Minimal libdogecoin type shims so the vendored raccoon_g C sources can build
 * under DogeGo CGO without linking the full libdogecoin tree.
 *
 * The Foundation in-tree Raccoon-G-44 port (src/raccoon_g) was developed by
 * Ed Tubbs — https://github.com/edtubbs · https://x.com/EdTubbs
 * See DogeGo docs/CREDITS.md.
 */
#ifndef DOGEGO_RACCOON_G_SHIM_DOGECOIN_H
#define DOGEGO_RACCOON_G_SHIM_DOGECOIN_H

#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#ifdef __cplusplus
#define LIBDOGECOIN_BEGIN_DECL extern "C" {
#define LIBDOGECOIN_END_DECL }
#else
#define LIBDOGECOIN_BEGIN_DECL
#define LIBDOGECOIN_END_DECL
#endif

#ifndef LIBDOGECOIN_API
#define LIBDOGECOIN_API
#endif

typedef int dogecoin_bool;
#ifndef true
#define true 1
#endif
#ifndef false
#define false 0
#endif

typedef uint8_t uint256_t[32];

#endif /* DOGEGO_RACCOON_G_SHIM_DOGECOIN_H */
