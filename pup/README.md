# DogeGo on DogeBox

Installs **[DogeGo](https://github.com/qlpqlp/dogego)** (Go Dogecoin full node + web dashboard) on DogeBox via this silly-pups source.

## Why TLS was still enabling

Older pins (`9d88c34` and earlier) **do not** implement `-notls`. The wizard always defaulted to `webui_tls_local` + OS CA install. Nix `postPatch` workarounds were fragile, and the old setup UI treated missing `webui_tls_local` in JSON (`omitempty`) as “TLS on”, so saving the wizard could turn HTTPS back on.

This pup pins a DogeGo git **rev** in `pup.nix` (native `-notls` / `DOGEGO_NO_TLS`) and starts with:

```bash
DOGEGO_NO_TLS=1 DOGEGO_NOTLS=1 dogego node -webui $DBX_PUP_IP:2013 -nobrowser -notls
```

## Doginals / wallet API (after pin refresh)

Once `pup.nix` `src.rev` includes DogeGo with **dogego.doginals v0.4.0**, wallets can call Doginals wallet reads at `/api/ext/dogego.doginals/v1/*` when the extension is installed and enabled (generic `/api/ext/{id}` gateway; routes owned by the extension). That is independent of Dogecoin consensus.

## Setup

Source is built from `https://github.com/qlpqlp/dogego` with `modRoot = "DogeGo"`.

There is **no DogeBox pup config UI**. The pup starts the DogeGo **setup wizard** over **plain HTTP**. Use datadir `./dogedata` (under `/storage/dogego`) and finish setup in the web UI.

### Refreshing the PUP after a DogeGo release

`manifest.json` → `container.build.nixFileSha256` is the **LF-normalized SHA-256 of `pup.nix` only** (not `dogego.exe`). To ship new DogeGo code on DogeBox:

1. Push the commit to `main` (or a release tag).
2. Update `pup.nix` `src.rev` to that commit and set `src.hash` / `goModules.outputHash` from the Nix `got: sha256-…` errors (or `nix-prefetch-git`).
3. Recompute `nixFileSha256`:

```bash
# Linux / macOS (LF)
sha256sum pup/pup.nix | awk '{print $1}'
# or: openssl dgst -sha256 pup/pup.nix
```

Paste the hex into `pup/manifest.json` → `container.build.nixFileSha256`.

### Manual equivalent

```bash
cd /storage/dogego && DOGEGO_NO_TLS=1 dogego node -webui $DBX_PUP_IP:2013 -nobrowser -notls
```

Wizard default data dir is `./dogedata` → `/storage/dogego/dogedata`.

If an old install already wrote `webui_tls_local: true` into `dogecoinconf.json`, this entrypoint clears those flags on start (or delete the conf and re-run the wizard).
