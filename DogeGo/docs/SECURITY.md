# DogeGo security notes

DogeGo is **beta** software. Please test it and report issues. Storage and crash recovery differ from Dogecoin Core’s LevelDB layout; use the Features self-cert probes if you want long-haul soak evidence.

## Protocol fidelity

DogeGo follows **Dogecoin Core mainnet consensus rules**. It does not introduce protocol forks, new activations, or alternate block acceptance on mainnet. Gaps vs Core are implementation and operator-surface differences (see [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md)), verified by offline differential tests and optional side-by-side Core compare on `dogego-live`.

## Threat model (summary)

| Asset | Risk | Mitigation |
|-------|------|------------|
| RPC / web UI | Unauthorized control of node or wallet | Bind to loopback; **`rpc_cookie`** or strong `rpc_user`/`rpc_password`; DogeGo **auto-enables `rpc_cookie`** when `rpcallowip` or non-loopback RPC bind would expose JSON-RPC without credentials; optional `rpcwhitelist`, `rpclimit`, `rpcauthmaxfail` |
| P2P | Eclipse, DoS, invalid data | Header/body validation; misbehavior scoring; `setban` + persistent `banlist.json` |
| Wallet (`wallet.json`) | Key theft | **scrypt + AES-GCM encryption** (`encryptwallet` / UI encrypt); **mainnet spend/export blocked until encrypted**; file mode **0600**; PQ carrier keys stored inside the encrypted secrets blob |
| Web UI remote reads | Balance/history leak over LAN | Wallet read APIs (`/api/wallet`, txs, utxos, addresses) are **loopback-only**; optional 6-digit PIN + WebAuthn for other dashboard reads when `webui_remote_auth` is on |
| Datadir | Tampering | OS file permissions; separate user account for the node process |

## Operator checklist

- [ ] Run RPC and web UI on `127.0.0.1` unless you use a TLS reverse proxy with auth ([OPERATOR.md](OPERATOR.md)).
- [ ] Enable HTTP Basic auth (`rpc_cookie` recommended, or `rpc_user` / `rpc_password`).
- [ ] If RPC listens beyond loopback, set `rpcallowip` to explicit subnets (loopback is always allowed). DogeGo refuses unauthenticated remote RPC and may auto-enable `rpc_cookie`.
- [ ] On **mainnet**, run **`encryptwallet`** (or Settings → Wallet encrypt) **before** depositing funds. Plaintext new wallets are for testnet convenience only.
- [ ] Do not use `allow_unverified_mempool` on any network with real value.
- [ ] Keep `dogego` updated via GitHub Releases; set `DOGEGO_NO_UPDATE_CHECK=1` to disable polling. Review [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) vs Core before relying on behavior.
- [ ] Back up encrypted `wallet.json` on encrypted media; store the passphrase separately.
- [ ] Use `webui_tls_cert/key` or a TLS reverse proxy when the dashboard is not loopback-only; session cookies set **`Secure`** when TLS is enabled.
- [ ] Optional local HTTPS: `webui_tls_local` / `rpc_tls_local` auto-generate certs under `datadir/tls/`; `local_tls_trust_ca` or `dogego tls trust-ca` installs the local CA into your OS user trust store (best-effort; Firefox may need manual import).

## Local HTTPS (optional)

DogeGo defaults to **HTTPS on loopback** for new wizard installs (`webui_tls_local` + `local_tls_trust_ca`). Plain HTTP remains available when those flags are off, or when you start with **`-notls`** / **`DOGEGO_NO_TLS=1`** (skips cert generation and OS CA install — use this on DogeBox and other hosts without TLS).

1. Set `webui_tls_local=true` and/or `rpc_tls_local=true` in `dogecoinconf.json` (or Settings → Interface → Local HTTPS).
2. Restart the node. PEM files are created under `{datadir}/tls/` (`local-ca.crt`, `webui.crt`, `rpc.crt`).
3. Open the dashboard with `https://127.0.0.1:2013` (or your configured bind).
4. To avoid browser warnings, set `local_tls_trust_ca=true` or run `dogego tls trust-ca` (loopback-only API: POST `/api/tls/trust-ca` from Settings).

**DogeBox / plain HTTP:** `dogego node -notls` (or `DOGEGO_NO_TLS=1`). The setup wizard then serves `http://` and does not install a local CA.

| OS | Trust install |
|----|----------------|
| Windows | `certutil -addstore -user Root` |
| macOS | `security add-trusted-cert` (login keychain) |
| Linux | NSS `~/.pki/nssdb` via `certutil` (install `libnss3-tools` for Chrome/Chromium) |

Firefox does not use the OS/NSS store on all platforms; import `datadir/tls/local-ca.crt` in Firefox certificate settings if needed.

Explicit operator PEM paths (`webui_tls_cert`/`webui_tls_key`, `rpc_tls_cert`/`rpc_tls_key`) override auto-generation when both cert and key are set.

## Compared to Dogecoin Core

| Topic | Core | DogeGo |
|-------|------|--------|
| Wallet encryption | Supported | **Supported** (scrypt N=32768, AES-GCM); mainnet spend blocked until encrypted |
| Addrman / eclipse resistance | Partial | Bucketed addrman + inbound eviction (addnode/HB protect, /16 preference); offline eclipse-pressure soak. Live eclipse soak + Core minping/novel-tx protect still open. |
| Chainstate integrity | LevelDB + assumevalid | Native `rawblocks/` + in-memory UTXO; **`-assumevalid`** skips scripts on buried blocks (mainnet default); **`verifychain`** level 4 forces full script checks |
| RPC surface | Full + hardened defaults | Subset; see `help`; remote RPC requires auth |

## Reporting issues

Security-sensitive bugs in DogeGo should be reported through your project's usual channel (private disclosure preferred). This document is not a formal audit.
