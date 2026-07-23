# Building Raccoon-G-44 (libdogecoin in-tree port)

DogeGo vendors the Dogecoin Foundation Raccoon-G-44 C port under
[`pqcrypto/raccoon_g/native/`](../pqcrypto/raccoon_g/native/) — the same tree as
[libdogecoin `src/raccoon_g` @ 0.1.5-dev](https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g)
and [Core green PR #8](https://github.com/dogecoinfoundation/dogecoin/pull/8).
DogeGo does **not** link the full libdogecoin library; it compiles this port
via CGO.

**Author (Foundation in-tree port):** [Ed Tubbs](https://github.com/edtubbs)
([@EdTubbs](https://x.com/EdTubbs)), Dogecoin Foundation. Acknowledgements:
[CREDITS.md](CREDITS.md).

There is **no placeholder crypto**. Without the CGO build, Raccoon-G keygen /
sign / verify return an error (`backend=unavailable`). Falcon and Dilithium
remain available in pure-Go builds. OP_RETURN tag `RCG4` format checks still
work without CGO.

## Why GitHub does **not** cross-compile Raccoon

Raccoon-G needs **CGO** (a real C compiler) plus **libgmp** and **libmpfr**.
Go can cross-compile *pure* Go with `GOOS`/`GOARCH` from one machine, but
**CGO + foreign C libraries does not cross-compile reliably**:

- The host would need a full cross-toolchain (e.g. MinGW targeting Windows
  from Linux) **and** matching cross-built GMP/MPFR for that target.
- Linking paths, ABI, and Windows/macOS system libraries differ per OS.
- GitHub’s simple `GOOS=windows go build` with `CGO_ENABLED=0` cannot produce
  a working `raccoon_g` binary.

So release CI uses **native runners** (build Linux on Linux, Windows on
Windows, macOS on macOS), installs GMP/MPFR on that OS, then:

```text
CGO_ENABLED=1 go build -tags raccoon_g ./cmd/dogego
```

That is intentional: not “cross-compile,” but **same-OS compile** so every
GitHub Release asset ships real Raccoon-G with no end-user install steps.

Workflows:

- [`.github/workflows/release.yml`](../../.github/workflows/release.yml) — tag `v*` releases
- [`.github/workflows/raccoon_g.yml`](../../.github/workflows/raccoon_g.yml) — PR/push compile gate for `pqcrypto/`

## Official releases (automatic)

Push a version tag (`v*`). Release CI builds each platform on its own runner:

| Asset | Runner | Deps installed by CI |
|-------|--------|----------------------|
| `dogego-linux-amd64` | `ubuntu-latest` | `libgmp-dev` `libmpfr-dev` `libzstd-dev` |
| `dogego-windows-amd64.exe` | `windows-latest` + MSYS2 MinGW | `mingw-w64-*-gmp/mpfr/zstd` |
| `dogego-darwin-amd64` | `macos-13` | Homebrew `gmp` `mpfr` `zstd` |
| `dogego-darwin-arm64` | `macos-14` | Homebrew `gmp` `mpfr` `zstd` |

Users who download those release binaries get real Raccoon-G with **no manual
commands**. Local `build.ps1` on Windows may still use `CGO_ENABLED=0` for a
fast pure-Go binary (Falcon/Dilithium only); use a release build or
`./build_raccoon.sh` when you need Raccoon locally.

## Local enable

```bash
# Debian/Ubuntu
sudo apt-get install -y libgmp-dev libmpfr-dev libzstd-dev build-essential
CGO_ENABLED=1 go build -tags raccoon_g -o dogego ./cmd/dogego

# macOS
brew install gmp mpfr zstd pkg-config
CGO_ENABLED=1 go build -tags raccoon_g -o dogego ./cmd/dogego

# or from DogeGo/: ./build_raccoon.sh
```

Windows local: MSYS2 MinGW + the same packages as CI, then
`CGO_ENABLED=1 go build -tags raccoon_g`.

`GET /api/core-pq-probe` reports `libdogecoin-raccoon_g` /
`libdogecoin_compatible=true` when `raccoong_is_ready()` succeeds.

## Layout

| Path | Role |
|------|------|
| `native/` | Vendored libdogecoin `src/raccoon_g` |
| `shims/` | Minimal dogecoin.h / sha2 / mem / ctaes / random for the port |
| `../raccoon_cgo.go` | CGO wrapper (`-tags raccoon_g`) |

Pinned Python reference: `p-11/lattice-hd-wallets@461a5ed9` (see `native/README.md`).
