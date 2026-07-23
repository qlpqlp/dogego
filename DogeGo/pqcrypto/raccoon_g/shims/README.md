# DogeGo raccoon_g shims

Minimal replacements for libdogecoin headers/sources that the vendored
`native/` tree includes (`dogecoin.h`, `sha2.h`, `mem.h`, `ctaes.h`, `random.h`).

- `ctaes.c` is the upstream libdogecoin constant-time AES.
- `shim_sha2.c` / `shim_mem.c` / `shim_random.c` implement only the APIs used by
  `raccoon_g` (HMAC-SHA256/512, SHA-256, mem zero/alloc, OS RNG).
- `amalgamation.c` is included from `pqcrypto/raccoon_cgo.go` when building with
  `CGO_ENABLED=1 -tags raccoon_g`.

See [`../BUILD.md`](../BUILD.md).
