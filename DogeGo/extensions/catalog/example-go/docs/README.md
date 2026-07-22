# example.go (subprocess extension)

Subprocess demo: forwarded `ping` RPC plus host `chain_tip` via `chain_read` (no wallet access).

Full packaging guide: [BUILDING.md](../BUILDING.md) (per-platform zips, universal zip, source install).

## Install paths

| Path | Who |
|------|-----|
| Catalog **Install** | Fetches GitHub source; DogeGo runs `go build` (Go required) |
| `dist/hello-<goos>-<goarch>.zip` | Prebuilt for one OS no Go for end user |
| `dist/hello-universal.zip` | All platforms in one zip see `entry.binaries` in [BUILDING.md](../BUILDING.md) |

## Build single-platform zip

Same OS as your DogeGo node:

```bash
cd extensions/catalog/example-go && ./build.sh
```

```powershell
cd extensions\catalog\example-go; .\build.ps1
```

Output: `dist/hello.zip` with `dogego.extension.json`, `icon.png`, and native `hello-ext[.exe]`.

Rename for catalog publishing, e.g. `hello-windows-amd64.zip`, and list under `downloads` in `catalog.json`.

## Package layout

```
example-go/
  dogego.extension.json
  icon.png
  hello/main.go
  build.sh / build.ps1
  dist/                    # gitignored
  docs/README.md
```

Optional universal layout:

```
dogego.extension.json
icon.png
bin/windows-amd64/hello-ext.exe
bin/linux-amd64/hello-ext
bin/darwin-arm64/hello-ext
```

## CLI

```bash
dogego-cli dogego_instextensionzip /path/to/dist/hello.zip
dogego-cli dogego_enableextension example.go
dogego-cli dogego_ext_example_go_ping
```

Walkthrough: [HELLO_WORLD.md](../HELLO_WORLD.md)
