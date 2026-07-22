# dogego.zkl2 (subprocess extension)

ZK L2 extension package: manifest, docs, icon, and `zkl2-ext` subprocess binary (universal zip).

## Install

**Settings → Extensions → Install** on `dogego.zkl2`, or:

```bash
dogego-cli dogego_instextension dogego.zkl2
# or local / published zip:
dogego-cli dogego_instextensionzip zkl2-universal.zip
dogego-cli dogego_enableextension dogego.zkl2
```

Install copies the package under `<datadir>/<network>/extensions/dogego.zkl2/` (catalog zip or upload). **Enable** starts the `zkl2-ext` subprocess; DogeGo proxies chain, wallet, and P2P overlay host calls.

## Package layout

```
zkl2/
  dogego.extension.json
  icon.png
  cmd/zkl2-ext/         # subprocess entrypoint
  *.go                  # extension implementation (runs in zkl2-ext)
  docs/USER_GUIDE.md
  docs/PROTOCOL.md
  build-universal.ps1   # universal zip (all platforms)
  dist/zkl2-universal.zip
  dist/zkl2.zip         # alias for catalog download_url
```

## Build universal zip (publishers)

```powershell
cd extensions\catalog\zkl2
.\build-universal.ps1
```

```bash
cd extensions/catalog/zkl2 && ./build-universal.sh
```

## Optional verifying key

`<datadir>/<network>/extensions/dogego.zkl2/data/vk/default.vk`

See **USER_GUIDE.md**.

Docs:

- `docs/USER_GUIDE.md`
- `docs/PROTOCOL.md`
