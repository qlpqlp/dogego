# DogeGo on DogeBox

Installs **[DogeGo](https://github.com/qlpqlp/dogego)** (Go Dogecoin full node + web dashboard) on DogeBox via this silly-pups source.

## Why TLS was still enabling

Older pins (`9d88c34` and earlier) **do not** implement `-notls`. The wizard always defaulted to `webui_tls_local` + OS CA install. Nix `postPatch` workarounds were fragile, and the old setup UI treated missing `webui_tls_local` in JSON (`omitempty`) as “TLS on”, so saving the wizard could turn HTTPS back on.

This pup pins DogeGo **`2eb7e69`** (native `-notls` / `DOGEGO_NO_TLS`) and starts with:

```bash
DOGEGO_NO_TLS=1 DOGEGO_NOTLS=1 dogego node -webui $DBX_PUP_IP:2013 -nobrowser -notls
```

## Setup

Source is built from `https://github.com/qlpqlp/dogego` with `modRoot = "DogeGo"`.

There is **no DogeBox pup config UI**. The pup starts the DogeGo **setup wizard** over **plain HTTP**. Use datadir `./dogedata` (under `/storage/dogego`) and finish setup in the web UI.

After the first Nix build, paste the `got: sha256-…` values into `pup.nix` (`src.hash` and `goModules` `outputHash`), then set `manifest.json` → `container.build.nixFileSha256` to the **LF-normalized** SHA-256 of `pup.nix`.

### Manual equivalent

```bash
cd /storage/dogego && DOGEGO_NO_TLS=1 dogego node -webui $DBX_PUP_IP:2013 -nobrowser -notls
```

Wizard default data dir is `./dogedata` → `/storage/dogego/dogedata`.

If an old install already wrote `webui_tls_local: true` into `dogecoinconf.json`, this entrypoint clears those flags on start (or delete the conf and re-run the wizard).
