/*
 * Minimal random.h for DogeGo raccoon_g CGO (subset of libdogecoin).
 */
#ifndef DOGEGO_RACCOON_G_SHIM_RANDOM_H
#define DOGEGO_RACCOON_G_SHIM_RANDOM_H

#include <dogecoin/dogecoin.h>

LIBDOGECOIN_BEGIN_DECL

LIBDOGECOIN_API void dogecoin_random_init(void);
LIBDOGECOIN_API dogecoin_bool dogecoin_random_bytes(uint8_t* buf, uint32_t len, const uint8_t update_seed);

LIBDOGECOIN_END_DECL

#endif /* DOGEGO_RACCOON_G_SHIM_RANDOM_H */
