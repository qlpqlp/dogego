# example.wasm (wasm extension)

Portable **wasm** ping demo. The same `ping.wasm` runs on Windows, Linux, and macOS.

## Install

**From catalog** (portable zip) via Settings → Extensions → **Install**, or build locally:

### Linux / macOS

```bash
cd extensions/catalog/example-wasm
# Compile ping.wat → ping.wasm if needed (wabt wat2wasm), then:
./build.sh
```

### Windows

```powershell
cd extensions\catalog\example-wasm
.\build.ps1
```

Upload `ping.zip` or use catalog install when `download_url` is published.

## Package layout

```
example-wasm/
  dogego.extension.json
  icon.png
  docs/README.md
  ping.wat              # wasm source
  ping.wasm             # module (portable)
  build.sh / build.ps1
  ping.zip              # release artifact (manifest + icon + wasm)
```

## CLI

```bash
dogego-cli dogego_enableextension example.wasm
dogego-cli dogego_ext_example_wasm_ping
```

Protocol: [WASM_PROTOCOL.md](../WASM_PROTOCOL.md)
