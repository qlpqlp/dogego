/*
 * Minimal mem.h for DogeGo raccoon_g CGO (subset of libdogecoin).
 */
#ifndef DOGEGO_RACCOON_G_SHIM_MEM_H
#define DOGEGO_RACCOON_G_SHIM_MEM_H

#include <dogecoin/dogecoin.h>

LIBDOGECOIN_BEGIN_DECL

LIBDOGECOIN_API void* dogecoin_malloc(size_t size);
LIBDOGECOIN_API void* dogecoin_calloc(size_t count, size_t size);
LIBDOGECOIN_API void* dogecoin_realloc(void* ptr, size_t size);
LIBDOGECOIN_API void dogecoin_free(void* ptr);
LIBDOGECOIN_API volatile void* dogecoin_mem_zero(volatile void* dst, size_t len);

LIBDOGECOIN_END_DECL

#endif /* DOGEGO_RACCOON_G_SHIM_MEM_H */
