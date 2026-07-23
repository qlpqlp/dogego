/*
 * Copyright (c) 2026 Paulo Vidal / Dogecoin Foundation
 * SPDX-License-Identifier: MIT
 */
#include <dogecoin/mem.h>

#include <stdlib.h>
#include <string.h>

void* dogecoin_malloc(size_t size) { return malloc(size); }
void* dogecoin_calloc(size_t count, size_t size) { return calloc(count, size); }
void* dogecoin_realloc(void* ptr, size_t size) { return realloc(ptr, size); }
void dogecoin_free(void* ptr) { free(ptr); }

volatile void* dogecoin_mem_zero(volatile void* dst, size_t len) {
    if (!dst || len == 0) return dst;
    volatile unsigned char* p = (volatile unsigned char*)dst;
    while (len--) {
        *p++ = 0;
    }
    return dst;
}
