/*
 * Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
 * SPDX-License-Identifier: MIT
 */
#include <dogecoin/random.h>

#include <string.h>

#if defined(_WIN32)
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <bcrypt.h>
#pragma comment(lib, "bcrypt.lib")
#else
#include <fcntl.h>
#include <unistd.h>
#endif

void dogecoin_random_init(void) {}

dogecoin_bool dogecoin_random_bytes(uint8_t* buf, uint32_t len, const uint8_t update_seed) {
    (void)update_seed;
    if (!buf || len == 0) return true;
#if defined(_WIN32)
    return BCryptGenRandom(NULL, buf, len, BCRYPT_USE_SYSTEM_PREFERRED_RNG) == 0;
#else
    int fd = open("/dev/urandom", O_RDONLY);
    if (fd < 0) return false;
    uint32_t got = 0;
    while (got < len) {
        ssize_t n = read(fd, buf + got, len - got);
        if (n <= 0) {
            close(fd);
            return false;
        }
        got += (uint32_t)n;
    }
    close(fd);
    return true;
#endif
}
