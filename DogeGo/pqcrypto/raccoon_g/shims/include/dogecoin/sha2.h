/*
 * Minimal sha2.h for DogeGo raccoon_g CGO (subset of libdogecoin).
 * Implements the HMAC/SHA entry points used by keygen_kdf.c and thrc.c.
 */
#ifndef DOGEGO_RACCOON_G_SHIM_SHA2_H
#define DOGEGO_RACCOON_G_SHIM_SHA2_H

#include <stddef.h>
#include <stdint.h>

#include <dogecoin/dogecoin.h>

LIBDOGECOIN_BEGIN_DECL

#define SHA256_BLOCK_LENGTH 64
#define SHA256_DIGEST_LENGTH 32
#define SHA512_BLOCK_LENGTH 128
#define SHA512_DIGEST_LENGTH 64

typedef struct _sha256_context {
    uint32_t state[8];
    uint64_t bitcount;
    uint8_t buffer[SHA256_BLOCK_LENGTH];
} sha256_context;

typedef struct _sha512_context {
    uint64_t state[8];
    uint64_t bitcount[2];
    uint8_t buffer[SHA512_BLOCK_LENGTH];
} sha512_context;

typedef struct _hmac_sha256_context {
    uint8_t o_key_pad[SHA256_BLOCK_LENGTH];
    sha256_context ctx;
} hmac_sha256_context;

typedef struct _hmac_sha512_context {
    uint8_t o_key_pad[SHA512_BLOCK_LENGTH];
    sha512_context ctx;
} hmac_sha512_context;

LIBDOGECOIN_API void sha256_raw(const uint8_t* data, size_t len, uint8_t digest[SHA256_DIGEST_LENGTH]);

LIBDOGECOIN_API void hmac_sha256_init(hmac_sha256_context* hctx, const uint8_t* key, const uint32_t keylen);
LIBDOGECOIN_API void hmac_sha256_write(hmac_sha256_context* hctx, const uint8_t* msg, const uint32_t msglen);
LIBDOGECOIN_API void hmac_sha256_finalize(hmac_sha256_context* hctx, uint8_t* hmac);
LIBDOGECOIN_API void hmac_sha256(const uint8_t* key, const size_t keylen, const uint8_t* msg, const size_t msglen, uint8_t* hmac);

LIBDOGECOIN_API void hmac_sha512(const uint8_t* key, const size_t keylen, const uint8_t* msg, const size_t msglen, uint8_t* hmac);

LIBDOGECOIN_END_DECL

#endif /* DOGEGO_RACCOON_G_SHIM_SHA2_H */
