/*
 * Amalgamation entry for DogeGo CGO. Included from raccoon_cgo.go so the
 * vendored libdogecoin raccoon_g tree + shims compile into the pqcrypto package.
 */
#include "shim_mem.c"
#include "shim_sha2.c"
#include "shim_random.c"
#include "ctaes.c"

#include "../native/shake256.c"
#include "../native/polyr.c"
#include "../native/ntt.c"
#include "../native/gaussian.c"
#include "../native/keygen_kdf.c"
#include "../native/thrc.c"
#include "../native/raccoong.c"
