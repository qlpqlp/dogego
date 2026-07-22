# DogeGo PUP (DogeBox)

Packaging so DogeBox can install **DogeGo** from this GitHub repo, compile it with Nix, and run the node with the web dashboard.

Pattern matches [silly-pups](https://github.com/qlpqlp/silly-pups/) (`dogebox.json` + per-pup `manifest.json` / `pup.nix` / `logo.png`).

There is **no DogeBox pup config UI**. After start, open the DogeGo web dashboard and configure network, wallet, RPC, etc. there (or via `dogecoinconf.json` under the datadir).

If this DogeBox runs reboot testnet with public P2P, point DNS **`seed.dogego.org`** at it so new DogeGo nodes discover the network quickly (built-in first DNS seed; see root README and `DogeGo/docs/CHAIN_PARAMETERS.md`).

## Add this repo as a DogeBox pup source

1. Push this repository (including `DogeGo/` and `pup/`) to `https://github.com/qlpqlp/dogego`.
2. On DogeBox, add a custom pup source pointing at that URL (same flow as silly-pups).
3. Install the **DogeGo** pup. The first build will fail on bootstrap Nix hashes; copy the `got: sha256-...` values into `pup/pup.nix` (`fetchgit.hash` and `vendorHash`), recompute `nixFileSha256`, push, and reinstall/upgrade.

## Layout

| File | Role |
|------|------|
| `../dogebox.json` | Source catalog (`location: "pup"`) |
| `manifest.json` | Pup metadata, ports, service name |
| `pup.nix` | `fetchgit` + `buildGoModule` (`modRoot = "DogeGo"`) + `run.sh` |
| `logo.png` | Pup icon in DogeBox UI |

## Runtime

`run.sh` only sets what DogeBox needs:

```text
dogego node -datadir /storage/dogego -webui $DBX_PUP_IP:2013 -nobrowser
```

Everything else (network, mode, peers, RPC, mining, …) is configured in the DogeGo web UI.

## Hash checklist (after any `pup.nix` edit)

Normalize to LF, then:

**PowerShell (LF-normalized):**

```powershell
$p = 'pup\pup.nix'
$raw = [System.IO.File]::ReadAllText($p)
$lf = $raw -replace "`r`n", "`n"
$b = [System.Text.Encoding]::UTF8.GetBytes($lf)
$h = [System.Security.Cryptography.SHA256]::Create().ComputeHash($b)
($h | ForEach-Object ToString x2) -join ''
```

Put that hex string in `manifest.json` → `container.build.nixFileSha256`.

Prefer pinning `pup.nix` `rev` to a release tag (`refs/tags/v0.1.0`) once you cut a GitHub release, instead of floating `refs/heads/main`.
