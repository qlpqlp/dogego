# Building extension packages

DogeGo extensions ship as **`dogego.extension.json`** plus assets. How you package assets depends on the **entry type** and whether end users have **Go** installed.

Platform keys use Go’s `GOOS-GOARCH` form (lowercase), for example:

| Key | Host |
|-----|------|
| `windows-amd64` | Windows 64-bit |
| `windows-arm64` | Windows ARM |
| `linux-amd64` | Linux 64-bit |
| `linux-arm64` | Linux ARM |
| `darwin-amd64` | macOS Intel |
| `darwin-arm64` | macOS Apple Silicon |
| `universal` | Fat package (see below) |

DogeGo picks the key matching the machine it runs on.

---

## 1. Builtin (`entry.type: builtin`)

- Go module compiled **into the DogeGo binary** (required for P2P overlay and block hooks).
- Catalog lists `repository` and/or zip; operators **Install** then **Enable**.
- Example: `dogego.zkl2` sources in [zkl2/](zkl2/); install copies the package to `<datadir>/extensions/dogego.zkl2/`.

---

## 2. Wasm one zip for all OSes (`entry.type: wasm`)

Wasm bytecode is **portable**. Ship one zip for Windows, Linux, and macOS.

**Manifest:**

```json
{
  "entry": {
    "type": "wasm",
    "module": "example.wasm",
    "wasm": "ping.wasm"
  }
}
```

**Catalog:**

```json
{
  "id": "example.wasm",
  "download_url": "https://…/ping.zip",
  "sha256": "…"
}
```

Zip layout:

```
dogego.extension.json
ping.wasm
```

See [example-wasm/](example-wasm/).

---

## 3. Subprocess per-platform zips (catalog `downloads`)

Native binaries are **OS-specific**. Publish **one zip per platform** and list them in the catalog. DogeGo installs only the zip for the host.

**Recommended zip names:**

- `myext-windows-amd64.zip`
- `myext-linux-amd64.zip`
- `myext-darwin-arm64.zip`

Each zip contains the binary at the **root** (or use `entry.binaries`; see §4):

```
dogego.extension.json
hello-ext          # or hello-ext.exe on Windows
```

**Manifest** (same in every per-platform zip):

```json
{
  "entry": {
    "type": "subprocess",
    "module": "example.go",
    "binary": "hello-ext"
  }
}
```

**Catalog** (`downloads` map):

```json
{
  "id": "example.go",
  "downloads": {
    "windows-amd64": {
      "download_url": "https://…/hello-windows-amd64.zip",
      "sha256": "…"
    },
    "linux-amd64": {
      "download_url": "https://…/hello-linux-amd64.zip",
      "sha256": "…"
    },
    "darwin-arm64": {
      "download_url": "https://…/hello-darwin-arm64.zip",
      "sha256": "…"
    }
  }
}
```

On install, DogeGo calls `dogego_instextension example.go` → downloads **only** the URL for `windows-amd64` (etc.). **End users do not need Go.**

Legacy: a single `download_url` still works when you ship one platform-only zip.

---

## 4. Subprocess universal zip (`entry.binaries` + optional catalog `universal`)

Ship **all platform binaries in one zip**. DogeGo copies the correct file to `entry.binary` at install time.

**Manifest:**

```json
{
  "entry": {
    "type": "subprocess",
    "module": "example.go",
    "binary": "hello-ext",
    "binaries": {
      "windows-amd64": "bin/windows-amd64/hello-ext.exe",
      "linux-amd64": "bin/linux-amd64/hello-ext",
      "darwin-amd64": "bin/darwin-amd64/hello-ext",
      "darwin-arm64": "bin/darwin-arm64/hello-ext"
    }
  }
}
```

**Zip layout:**

```
dogego.extension.json
bin/
  windows-amd64/hello-ext.exe
  linux-amd64/hello-ext
  darwin-arm64/hello-ext
  darwin-amd64/hello-ext
```

**Catalog** (one fat zip):

```json
{
  "downloads": {
    "universal": {
      "download_url": "https://…/hello-universal.zip",
      "sha256": "…"
    }
  }
}
```

Or upload the universal zip manually via **Settings → Extensions → Install zip**.

---

## 5. Subprocess source-only (compile at install)

For developers or when you do not publish binaries. Zip contains **Go source**, no executable.

```
dogego.extension.json
hello/
  main.go
```

**Manifest:** same as §3 (`entry.binary` names the output executable).

**Catalog:** use `repository` (GitHub tree URL) instead of `download_url`:

```json
{
  "repository": "https://github.com/owner/repo/tree/main/path/to/extension"
}
```

On install, DogeGo fetches the folder and runs `go build` **if Go is on PATH**. Operators without Go should use §3 or §4 instead.

---

## 6. Choosing a strategy

| Goal | Package type | Catalog field | End user needs Go? |
|------|--------------|---------------|-------------------|
| Shipped with DogeGo | `builtin` | `builtin: true` | No |
| Portable plugin | `wasm` | `download_url` | No |
| Published natives | `subprocess` | `downloads` per platform | No |
| One zip, all OSes | `subprocess` | `downloads.universal` + `entry.binaries` | No |
| Hackable / dev | `subprocess` source | `repository` | Yes |
| Local dev | any | Install zip you built | Build machine may need Go |

---

## 7. Build scripts (subprocess)

**Single platform** (current [example-go/build.ps1](example-go/build.ps1) / [build.sh](example-go/build.sh)):

```bash
go build -ldflags="-s -w" -trimpath -o hello-ext ./hello
zip dist/hello-$(go env GOOS)-$(go env GOARCH).zip dogego.extension.json hello-ext
```

**Universal zip** (CI builds each `GOOS/GOARCH`, then packs):

```bash
# after cross-compiling into bin/<goos>-<goarch>/hello-ext[.exe]
zip -r dist/hello-universal.zip dogego.extension.json bin/
```

Pin `sha256` in `catalog.json` after each release.

---

## See also

- [AUTHORING.md](AUTHORING.md) manifest fields and permissions
- [HELLO_WORLD.md](HELLO_WORLD.md) `example.go` walkthrough
- [SUBPROCESS_PROTOCOL.md](SUBPROCESS_PROTOCOL.md) RPC protocol
- [WASM_PROTOCOL.md](WASM_PROTOCOL.md) wasm RPC protocol
- [README.md](README.md) catalog layout
