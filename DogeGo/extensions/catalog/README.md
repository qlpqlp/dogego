# DogeGo extensions catalog

This folder is the **remote catalog** (`catalog.json`) plus **reference packages** for each extension. DogeGo runs on **Windows, Linux, and macOS**.

**Packaging guide:** [BUILDING.md](BUILDING.md) (wasm universal, per-platform zips, universal subprocess zip, source compile).

## Layout (every package)

| File / folder | Purpose |
|---------------|---------|
| `dogego.extension.json` | Manifest (required for catalog + zip install) |
| `README.md` | How to enable, build, and install on your OS |
| `build.sh` / `build.ps1` | Build release zip on Linux/macOS or Windows |
| `dist/*.zip` | **Optional local output** (gitignored for subprocess; never commit OS-specific binaries) |

## Packages in this catalog

| ID | Type | Folder | Install |
|----|------|--------|---------|
| `dogego.zkl2` | **subprocess** (universal zip) | [zkl2/](zkl2/) | Catalog **Install** (zip) then **Enable** |
| `dogego.bbpow` | **subprocess** (research, testnet) | [bbpow/](bbpow/) | Build zip locally (`build.ps1` / `build.sh`) then Install → Enable on **testnet only** |
| `example.go` | **subprocess** | [example-go/](example-go/) | Catalog **Install** (source from GitHub + compile) or per-platform / universal zip see [BUILDING.md](BUILDING.md) |
| `example.wasm` | **wasm** (portable) | [example-wasm/](example-wasm/) | Catalog **Install** (one zip for all OSes) |

## Quick reference

| Type | End user needs Go? | Catalog fields |
|------|-------------------|----------------|
| `builtin` | No | `"builtin": true` |
| `wasm` | No | `download_url` + `sha256` |
| `subprocess` (prebuilt) | No | `downloads` per platform, or `downloads.universal` + `entry.binaries` in manifest |
| `subprocess` (source) | Yes (on install) | `repository` (GitHub tree) or source zip upload |

## Shared docs

- [BUILDING.md](BUILDING.md) **how to package** (platform keys, zip layouts, catalog JSON)
- [AUTHORING.md](AUTHORING.md) third-party extensions
- [HELLO_WORLD.md](HELLO_WORLD.md) `example.go` walkthrough
- [SUBPROCESS_PROTOCOL.md](SUBPROCESS_PROTOCOL.md) / [WASM_PROTOCOL.md](WASM_PROTOCOL.md)

## Publishing catalog changes

1. Edit `catalog.json` (ids, versions, `docs_path`, `icon.png` in each package).
2. Follow [BUILDING.md](BUILDING.md) for `download_url`, `downloads`, or `repository`.
3. Pin `sha256` when publishing HTTPS zips.
