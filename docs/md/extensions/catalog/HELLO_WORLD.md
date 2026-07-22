# Hello World extension (`example.go`)

Minimal **third-party** subprocess extension (Go). Packaging options are documented in [BUILDING.md](BUILDING.md).

| Surface | What it demonstrates |
|---------|----------------------|
| **RPC / CLI** | `dogego_ext_example_go_*` methods |
| **Web UI** | Settings → Extensions panel (`ui_panel`) |
| **chain_read** | `chain_tip` via host (no wallet access) |

Source: [example-go/hello/](example-go/hello/)

## Install options

| Method | Needs Go on user machine? |
|--------|---------------------------|
| **Settings → Install** (catalog `repository`) | Yes DogeGo fetches source and runs `go build` |
| **Per-platform zip** (`downloads` in catalog) | No |
| **Universal zip** (`entry.binaries` in manifest) | No |
| **Local zip** you built with `build.ps1` / `build.sh` | No (if zip includes your OS binary) |

### Catalog install (source)

**Settings → Extensions → Install** on `example.go`, or:

```bash
dogego-cli dogego_instextension example.go
dogego-cli dogego_enableextension example.go
```

Requires **Go on PATH** where DogeGo runs.

### Prebuilt zip (no Go for end users)

Build on each target OS (or CI), name zips by platform see [BUILDING.md](BUILDING.md):

```bash
cd extensions/catalog/example-go && ./build.sh    # → dist/hello-<goos>-<goarch>.zip
```

Upload via **Settings → Extensions → Install zip**, or publish URLs in catalog `downloads`.

## Try RPC (CLI)

```bash
dogego-cli dogego_ext_example_go_ping
dogego-cli dogego_ext_example_go_chain_tip
dogego-cli dogego_ext_example_go_info
```

## Web UI panel

1. Enable `example.go`.
2. Open **Documentation & status** on the extension card.
3. Panel copy comes from the extension `info` RPC, not DogeGo locales.

## Universal manifest example

```json
{
  "entry": {
    "type": "subprocess",
    "module": "example.go",
    "binary": "hello-ext",
    "binaries": {
      "windows-amd64": "bin/windows-amd64/hello-ext.exe",
      "linux-amd64": "bin/linux-amd64/hello-ext",
      "darwin-arm64": "bin/darwin-arm64/hello-ext"
    }
  }
}
```

See [AUTHORING.md](AUTHORING.md) and [SUBPROCESS_PROTOCOL.md](SUBPROCESS_PROTOCOL.md).
