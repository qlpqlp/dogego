/*
 * Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
 * SPDX-License-Identifier: MIT
 *
 * Compact SHA-256 / SHA-512 + HMAC for raccoon_g CGO (byte-compatible with
 * libdogecoin hmac_sha256 / hmac_sha512 / sha256_raw used by raccoon_g).
 */
#include <dogecoin/sha2.h>

#include <string.h>

static uint32_t rotr32(uint32_t x, uint32_t n) { return (x >> n) | (x << (32 - n)); }
static uint64_t rotr64(uint64_t x, uint32_t n) { return (x >> n) | (x << (64 - n)); }

static void sha256_transform(sha256_context* ctx, const uint8_t data[64]) {
    static const uint32_t K[64] = {
        0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5, 0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
        0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3, 0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
        0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc, 0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
        0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7, 0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
        0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13, 0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
        0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3, 0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
        0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5, 0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
        0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208, 0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2};
    uint32_t m[64], a, b, c, d, e, f, g, h, t1, t2;
    for (int i = 0; i < 16; i++) {
        m[i] = ((uint32_t)data[i * 4] << 24) | ((uint32_t)data[i * 4 + 1] << 16) |
               ((uint32_t)data[i * 4 + 2] << 8) | ((uint32_t)data[i * 4 + 3]);
    }
    for (int i = 16; i < 64; i++) {
        uint32_t s0 = rotr32(m[i - 15], 7) ^ rotr32(m[i - 15], 18) ^ (m[i - 15] >> 3);
        uint32_t s1 = rotr32(m[i - 2], 17) ^ rotr32(m[i - 2], 19) ^ (m[i - 2] >> 10);
        m[i] = m[i - 16] + s0 + m[i - 7] + s1;
    }
    a = ctx->state[0];
    b = ctx->state[1];
    c = ctx->state[2];
    d = ctx->state[3];
    e = ctx->state[4];
    f = ctx->state[5];
    g = ctx->state[6];
    h = ctx->state[7];
    for (int i = 0; i < 64; i++) {
        uint32_t S1 = rotr32(e, 6) ^ rotr32(e, 11) ^ rotr32(e, 25);
        uint32_t ch = (e & f) ^ ((~e) & g);
        t1 = h + S1 + ch + K[i] + m[i];
        uint32_t S0 = rotr32(a, 2) ^ rotr32(a, 13) ^ rotr32(a, 22);
        uint32_t maj = (a & b) ^ (a & c) ^ (b & c);
        t2 = S0 + maj;
        h = g;
        g = f;
        f = e;
        e = d + t1;
        d = c;
        c = b;
        b = a;
        a = t1 + t2;
    }
    ctx->state[0] += a;
    ctx->state[1] += b;
    ctx->state[2] += c;
    ctx->state[3] += d;
    ctx->state[4] += e;
    ctx->state[5] += f;
    ctx->state[6] += g;
    ctx->state[7] += h;
}

static void sha256_init(sha256_context* ctx) {
    ctx->bitcount = 0;
    ctx->state[0] = 0x6a09e667;
    ctx->state[1] = 0xbb67ae85;
    ctx->state[2] = 0x3c6ef372;
    ctx->state[3] = 0xa54ff53a;
    ctx->state[4] = 0x510e527f;
    ctx->state[5] = 0x9b05688c;
    ctx->state[6] = 0x1f83d9ab;
    ctx->state[7] = 0x5be0cd19;
    memset(ctx->buffer, 0, sizeof(ctx->buffer));
}

static void sha256_write(sha256_context* ctx, const uint8_t* data, size_t len) {
    size_t fill = (size_t)((ctx->bitcount / 8) % 64);
    ctx->bitcount += (uint64_t)len * 8;
    if (fill) {
        size_t left = 64 - fill;
        if (len < left) {
            memcpy(ctx->buffer + fill, data, len);
            return;
        }
        memcpy(ctx->buffer + fill, data, left);
        sha256_transform(ctx, ctx->buffer);
        data += left;
        len -= left;
    }
    while (len >= 64) {
        sha256_transform(ctx, data);
        data += 64;
        len -= 64;
    }
    if (len) memcpy(ctx->buffer, data, len);
}

static void sha256_finalize(sha256_context* ctx, uint8_t digest[32]) {
    uint8_t pad[64];
    size_t used = (size_t)((ctx->bitcount / 8) % 64);
    pad[0] = 0x80;
    size_t pad_len = (used < 56) ? (56 - used) : (120 - used);
    memset(pad + 1, 0, pad_len - 1);
    uint64_t bits = ctx->bitcount;
    uint8_t lenbe[8];
    for (int i = 0; i < 8; i++) lenbe[7 - i] = (uint8_t)(bits >> (i * 8));
    sha256_write(ctx, pad, pad_len);
    sha256_write(ctx, lenbe, 8);
    for (int i = 0; i < 8; i++) {
        digest[i * 4] = (uint8_t)(ctx->state[i] >> 24);
        digest[i * 4 + 1] = (uint8_t)(ctx->state[i] >> 16);
        digest[i * 4 + 2] = (uint8_t)(ctx->state[i] >> 8);
        digest[i * 4 + 3] = (uint8_t)(ctx->state[i]);
    }
}

void sha256_raw(const uint8_t* data, size_t len, uint8_t digest[SHA256_DIGEST_LENGTH]) {
    sha256_context ctx;
    sha256_init(&ctx);
    sha256_write(&ctx, data, len);
    sha256_finalize(&ctx, digest);
}

static void sha512_transform(sha512_context* ctx, const uint8_t data[128]) {
    static const uint64_t K[80] = {
        0x428a2f98d728ae22ULL, 0x7137449123ef65cdULL, 0xb5c0fbcfec4d3b2fULL, 0xe9b5dba58189dbbcULL,
        0x3956c25bf348b538ULL, 0x59f111f1b605d019ULL, 0x923f82a4af194f9bULL, 0xab1c5ed5da6d8118ULL,
        0xd807aa98a3030242ULL, 0x12835b0145706fbeULL, 0x243185be4ee4b28cULL, 0x550c7dc3d5ffb4e2ULL,
        0x72be5d74f27b896fULL, 0x80deb1fe3b1696b1ULL, 0x9bdc06a725c71235ULL, 0xc19bf174cf692694ULL,
        0xe49b69c19ef14ad2ULL, 0xefbe4786384f25e3ULL, 0x0fc19dc68b8cd5b5ULL, 0x240ca1cc77ac9c65ULL,
        0x2de92c6f592b0275ULL, 0x4a7484aa6ea6e483ULL, 0x5cb0a9dcbd41fbd4ULL, 0x76f988da831153b5ULL,
        0x983e5152ee66dfabULL, 0xa831c66d2db43210ULL, 0xb00327c898fb213fULL, 0xbf597fc7beef0ee4ULL,
        0xc6e00bf33da88fc2ULL, 0xd5a79147930aa725ULL, 0x06ca6351e003826fULL, 0x142929670a0e6e70ULL,
        0x27b70a8546d22ffcULL, 0x2e1b21385c26c926ULL, 0x4d2c6dfc5ac42aedULL, 0x53380d139d95b3dfULL,
        0x650a73548baf63deULL, 0x766a0abb3c77b2a8ULL, 0x81c2c92e47edaee6ULL, 0x92722c851482353bULL,
        0xa2bfe8a14cf10364ULL, 0xa81a664bbc423001ULL, 0xc24b8b70d0f89791ULL, 0xc76c51a30654be30ULL,
        0xd192e819d6ef5218ULL, 0xd69906245565a910ULL, 0xf40e35855771202aULL, 0x106aa07032bbd1b8ULL,
        0x19a4c116b8d2d0c8ULL, 0x1e376c085141ab53ULL, 0x2748774cdf8eeb99ULL, 0x34b0bcb5e19b48a8ULL,
        0x391c0cb3c5c95a63ULL, 0x4ed8aa4ae3418acbULL, 0x5b9cca4f7763e373ULL, 0x682e6ff3d6b2b8a3ULL,
        0x748f82ee5defb2fcULL, 0x78a5636f43172f60ULL, 0x84c87814a1f0ab72ULL, 0x8cc702081a6439ecULL,
        0x90befffa23631e28ULL, 0xa4506cebde82bde9ULL, 0xbef9a3f7b2c67915ULL, 0xc67178f2e372532bULL,
        0xca273eceea26619cULL, 0xd186b8c721c0c207ULL, 0xeada7dd6cde0eb1eULL, 0xf57d4f7fee6ed178ULL,
        0x06f067aa72176fbaULL, 0x0a637dc5a2c898a6ULL, 0x113f9804bef90daeULL, 0x1b710b35131c471bULL,
        0x28db77f523047d84ULL, 0x32caab7b40c72493ULL, 0x3c9ebe0a15c9bebcULL, 0x431d67c49c100d4cULL,
        0x4cc5d4becb3e42b6ULL, 0x597f299cfc657e2aULL, 0x5fcb6fab3ad6faecULL, 0x6c44198c4a475817ULL};
    uint64_t m[80], a, b, c, d, e, f, g, h, t1, t2;
    for (int i = 0; i < 16; i++) {
        m[i] = ((uint64_t)data[i * 8] << 56) | ((uint64_t)data[i * 8 + 1] << 48) |
               ((uint64_t)data[i * 8 + 2] << 40) | ((uint64_t)data[i * 8 + 3] << 32) |
               ((uint64_t)data[i * 8 + 4] << 24) | ((uint64_t)data[i * 8 + 5] << 16) |
               ((uint64_t)data[i * 8 + 6] << 8) | ((uint64_t)data[i * 8 + 7]);
    }
    for (int i = 16; i < 80; i++) {
        uint64_t s0 = rotr64(m[i - 15], 1) ^ rotr64(m[i - 15], 8) ^ (m[i - 15] >> 7);
        uint64_t s1 = rotr64(m[i - 2], 19) ^ rotr64(m[i - 2], 61) ^ (m[i - 2] >> 6);
        m[i] = m[i - 16] + s0 + m[i - 7] + s1;
    }
    a = ctx->state[0];
    b = ctx->state[1];
    c = ctx->state[2];
    d = ctx->state[3];
    e = ctx->state[4];
    f = ctx->state[5];
    g = ctx->state[6];
    h = ctx->state[7];
    for (int i = 0; i < 80; i++) {
        uint64_t S1 = rotr64(e, 14) ^ rotr64(e, 18) ^ rotr64(e, 41);
        uint64_t ch = (e & f) ^ ((~e) & g);
        t1 = h + S1 + ch + K[i] + m[i];
        uint64_t S0 = rotr64(a, 28) ^ rotr64(a, 34) ^ rotr64(a, 39);
        uint64_t maj = (a & b) ^ (a & c) ^ (b & c);
        t2 = S0 + maj;
        h = g;
        g = f;
        f = e;
        e = d + t1;
        d = c;
        c = b;
        b = a;
        a = t1 + t2;
    }
    ctx->state[0] += a;
    ctx->state[1] += b;
    ctx->state[2] += c;
    ctx->state[3] += d;
    ctx->state[4] += e;
    ctx->state[5] += f;
    ctx->state[6] += g;
    ctx->state[7] += h;
}

static void sha512_init(sha512_context* ctx) {
    ctx->bitcount[0] = 0;
    ctx->bitcount[1] = 0;
    ctx->state[0] = 0x6a09e667f3bcc908ULL;
    ctx->state[1] = 0xbb67ae8584caa73bULL;
    ctx->state[2] = 0x3c6ef372fe94f82bULL;
    ctx->state[3] = 0xa54ff53a5f1d36f1ULL;
    ctx->state[4] = 0x510e527fade682d1ULL;
    ctx->state[5] = 0x9b05688c2b3e6c1fULL;
    ctx->state[6] = 0x1f83d9abfb41bd6bULL;
    ctx->state[7] = 0x5be0cd19137e2179ULL;
    memset(ctx->buffer, 0, sizeof(ctx->buffer));
}

static void sha512_write(sha512_context* ctx, const uint8_t* data, size_t len) {
    size_t fill = (size_t)((ctx->bitcount[1] >> 3) & 0x7f);
    uint64_t bits = ((uint64_t)len) << 3;
    ctx->bitcount[1] += bits;
    if (ctx->bitcount[1] < bits) ctx->bitcount[0]++;
    if (fill) {
        size_t left = 128 - fill;
        if (len < left) {
            memcpy(ctx->buffer + fill, data, len);
            return;
        }
        memcpy(ctx->buffer + fill, data, left);
        sha512_transform(ctx, ctx->buffer);
        data += left;
        len -= left;
    }
    while (len >= 128) {
        sha512_transform(ctx, data);
        data += 128;
        len -= 128;
    }
    if (len) memcpy(ctx->buffer, data, len);
}

static void sha512_finalize(sha512_context* ctx, uint8_t digest[64]) {
    uint8_t pad[128];
    size_t used = (size_t)((ctx->bitcount[1] >> 3) & 0x7f);
    pad[0] = 0x80;
    size_t pad_len = (used < 112) ? (112 - used) : (240 - used);
    memset(pad + 1, 0, pad_len - 1);
    uint8_t lenbe[16];
    memset(lenbe, 0, 16);
    for (int i = 0; i < 8; i++) {
        lenbe[15 - i] = (uint8_t)(ctx->bitcount[1] >> (i * 8));
        lenbe[7 - i] = (uint8_t)(ctx->bitcount[0] >> (i * 8));
    }
    sha512_write(ctx, pad, pad_len);
    sha512_write(ctx, lenbe, 16);
    for (int i = 0; i < 8; i++) {
        for (int j = 0; j < 8; j++) {
            digest[i * 8 + j] = (uint8_t)(ctx->state[i] >> (56 - j * 8));
        }
    }
}

void hmac_sha256_init(hmac_sha256_context* hctx, const uint8_t* key, const uint32_t keylen) {
    uint8_t key_block[SHA256_BLOCK_LENGTH];
    memset(key_block, 0, sizeof(key_block));
    if (keylen > SHA256_BLOCK_LENGTH) {
        sha256_raw(key, keylen, key_block);
    } else {
        memcpy(key_block, key, keylen);
    }
    uint8_t ipad[SHA256_BLOCK_LENGTH];
    for (int i = 0; i < SHA256_BLOCK_LENGTH; i++) {
        ipad[i] = key_block[i] ^ 0x36;
        hctx->o_key_pad[i] = key_block[i] ^ 0x5c;
    }
    sha256_init(&hctx->ctx);
    sha256_write(&hctx->ctx, ipad, sizeof(ipad));
}

void hmac_sha256_write(hmac_sha256_context* hctx, const uint8_t* msg, const uint32_t msglen) {
    sha256_write(&hctx->ctx, msg, msglen);
}

void hmac_sha256_finalize(hmac_sha256_context* hctx, uint8_t* hmac) {
    uint8_t inner[SHA256_DIGEST_LENGTH];
    sha256_finalize(&hctx->ctx, inner);
    sha256_context octx;
    sha256_init(&octx);
    sha256_write(&octx, hctx->o_key_pad, SHA256_BLOCK_LENGTH);
    sha256_write(&octx, inner, SHA256_DIGEST_LENGTH);
    sha256_finalize(&octx, hmac);
}

void hmac_sha256(const uint8_t* key, const size_t keylen, const uint8_t* msg, const size_t msglen, uint8_t* hmac) {
    hmac_sha256_context hctx;
    hmac_sha256_init(&hctx, key, (uint32_t)keylen);
    hmac_sha256_write(&hctx, msg, (uint32_t)msglen);
    hmac_sha256_finalize(&hctx, hmac);
}

void hmac_sha512(const uint8_t* key, const size_t keylen, const uint8_t* msg, const size_t msglen, uint8_t* hmac) {
    uint8_t key_block[SHA512_BLOCK_LENGTH];
    memset(key_block, 0, sizeof(key_block));
    if (keylen > SHA512_BLOCK_LENGTH) {
        sha512_context t;
        sha512_init(&t);
        sha512_write(&t, key, keylen);
        sha512_finalize(&t, key_block);
    } else {
        memcpy(key_block, key, keylen);
    }
    uint8_t ipad[SHA512_BLOCK_LENGTH], opad[SHA512_BLOCK_LENGTH];
    for (int i = 0; i < SHA512_BLOCK_LENGTH; i++) {
        ipad[i] = key_block[i] ^ 0x36;
        opad[i] = key_block[i] ^ 0x5c;
    }
    sha512_context ictx, octx;
    uint8_t inner[SHA512_DIGEST_LENGTH];
    sha512_init(&ictx);
    sha512_write(&ictx, ipad, sizeof(ipad));
    sha512_write(&ictx, msg, msglen);
    sha512_finalize(&ictx, inner);
    sha512_init(&octx);
    sha512_write(&octx, opad, sizeof(opad));
    sha512_write(&octx, inner, SHA512_DIGEST_LENGTH);
    sha512_finalize(&octx, hmac);
}
