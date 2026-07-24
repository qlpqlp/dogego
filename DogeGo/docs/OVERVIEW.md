# DogeGo overview

DogeGo is a Beta **Dogecoin node and tooling stack written in Go**. It aims for **RPC and UX shapes compatible with Dogecoin Core** where practical, while storing chain data in a **Go-native layout** (`headers.bin`, `rawblocks/*.bin`, optional flat `indexes/tx/`) rather than Core's LevelDB `blocks/` + `chainstate/`.

**Dogecoin Core remains the consensus reference** for production validation. When behavior differs, DogeGo documents it (see `ROADMAP.md`, RPC `dogego_*` notes, and this file).

**Protocol lock:** DogeGo does **not** introduce mainnet protocol forks or new consensus activations. Mainnet block, header, script, subsidy, and auxpow rules follow Core; implementation gaps are documented in [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md). Offline and live audit gates: `dogego cert offline`, differential harness, optional Core side-by-side compare on `dogego-live`.

---

## What works today (high level)

- **P2P**: single outbound peer, headers-first sync, optional raw block fetch windows, **BIP37 bloom** (full-node server + SPV wallet filtered-block client; BIP157 preferred when advertised), JSON-RPC subset (`rpc/dispatch.go` - see `SupportedMethods`; includes **`gettxout`**, **`gettxoutproof`** / **`verifytxoutproof`**, **`decodescript`**, **`createrawtransaction`**, with raw-block spend scan for `gettxout` - not Core UTXO DB).
- **Web UI**: Qt-inspired dashboard (loopback), first-run **setup wizard**, chain overview, wallet Send/Receive (incl. optional PQ OP_RETURN + carrier sends), explorer, mempool, analytics charts, **Docs**, **Features** (17 live operator-cert gates), console log, **Settings** (incl. mempool package limits & RPC auth). See **[docs/WEB_UI.md](WEB_UI.md)** and **[docs/DOCUMENTATION.md](DOCUMENTATION.md)**. Browser display prefs in `localStorage` only. Optional Pebble **analytics** sidecar (`analytics/`, `dogego indexer`).
- **Wallet (mainnet + testnet)**: BIP44 HD `wallet.json`, Send/Receive/History, PSBT subset, optional HWI external signer (`signer_cmd`), and Core **`wallet.dat` migration** (`dogego_probewalletdat` / `dogego_importwalletdat` - native BDB, pool probe metadata, **`pool_indices_replayed`** on HD import via `wallet/pool_replay.go`, **`pool_keys_unmatched`** for pool-only rows, **`keypool_hint`**, encrypted via passphrase, or Core RPC fallback). Not Core `wallet.dat` on-disk format.

---

## Analytics

The **Pebble** store (`dogego_analytics.db`) is an **auxiliary catalog**: indexer checkpoints, metric samples, and **reorg event history** (depth, heights, AuxPoW, displaced coinbase miners, UTC hour). It is **not** Core chainstate and **not** a substitute for full-node UTXO validation.

---

## Post-quantum (PQC) - design alignment, not full implementation

DogeGo tracks the **draft BIP-style specification** for *post-quantum signature commitments* used in libdogecoin’s carrier branch (Phase 1: canonical **tagged OP_RETURN** commitments for Falcon / Dilithium2 / Raccoon-G; Phase 2: future opcode path). Source spec (upstream):

`https://github.com/edtubbs/libdogecoin/blob/0.1.5-dev-pqc-carrier/doc/spec/bip-post-quantum-signature-commitments.mediawiki`

**Raccoon-G-44:** vendored Foundation in-tree C port under `pqcrypto/raccoon_g/native` ([libdogecoin `src/raccoon_g`](https://github.com/dogecoinfoundation/libdogecoin/tree/0.1.5-dev/src/raccoon_g), [Core green PR #8](https://github.com/dogecoinfoundation/dogecoin/pull/8)), authored by Foundation engineer [Ed Tubbs](https://github.com/edtubbs) ([@EdTubbs](https://x.com/EdTubbs)) — see [CREDITS.md](CREDITS.md). No placeholder. **GitHub Releases do not cross-compile CGO** — each OS builds on a native runner with GMP/MPFR installed (`CGO_ENABLED=1 -tags raccoon_g`). Why and how: [RACCOON_G_BUILD.md](RACCOON_G_BUILD.md) (same text as `pqcrypto/raccoon_g/BUILD.md`).

**Status in this repository:** PQ OP_RETURN FLC1/DIL2/RCG4, TX_C/TX_R carriers, wallet flags, web Send, `GET /api/core-pq-probe`, and `dogego cert pq` ship today. More soak testing is welcome. This is not a consensus softfork and is not a production PQ-hardening claim yet.

**Plain HTTP / DogeBox:** use **`dogego node -notls`** or **`DOGEGO_NO_TLS=1`** to skip local HTTPS and CA install (wizard + dashboard). See [WEB_UI.md](WEB_UI.md#local-https-and--notls) and [SECURITY.md](SECURITY.md#local-https-optional).

---

## Security & encryption (roadmap)

Planned directions (see `ROADMAP.md`):

- **Transport**: optional TLS/reverse-proxy in front of RPC and UI in production deployments.
- **At rest**: optional encryption for wallet keys / analytics DB is **not** implemented in-tree today; treat disk as sensitive.

---

## Faster than Core?

DogeGo may be faster for **some tasks** (single-process Go, smaller surface) but **does not** replicate Core’s full validation pipeline. Any performance claim should be **benchmarked** on a specific workload.

---

## Offline certification

Cross-platform cert commands (no node or Core required):

| Command | Scope |
|---------|--------|
| `dogego cert offline` | CI push/PR gate (`offlinegate/`; `scripts/ci_offline_gate.{ps1,sh}`) |
| `dogego cert wallet-import` | BIP39/BIP38 + signer + wallet.dat (`scripts/wallet_import_cert.{ps1,sh}`) |
| `dogego cert operator` | Milestone E deep cert (core + field-evidence + wallet-import; `scripts/operator_workflow_cert.{ps1,sh}`) |
| `dogego cert field-evidence` | Milestone A mainnet field corpus (`scripts/field_evidence_cert.{ps1,sh}`) |
| `dogego cert wallet-migration` | wallet.dat probe/import (`scripts/wallet_migration_cert.{ps1,sh}`; `-offline-only`) |
| `dogego cert pq` | PQ OP_RETURN + TX_C/TX_R carrier format cert (no production PQ safety claim; `scripts/pq_cert.{ps1,sh}`) |

Before `dogego-live` scheduled CI: `cert offline` + `cert wallet-import` (see [ROADMAP.md](../ROADMAP.md) certification exit checklist). Bundle: `scripts/cert_offline_prerequisites.{ps1,sh}` (`-IncludePQ` / `-IncludeOperator` on PS1; `INCLUDE_PQ=1` / `INCLUDE_OPERATOR=1` on shell).

---

## Related files

| Topic | Location |
|--------|-----------|
| Documentation index | [docs/DOCUMENTATION.md](DOCUMENTATION.md) |
| External apps / JSON-RPC | [docs/INTEGRATION.md](INTEGRATION.md) |
| RPC workflows | [docs/RPC.md](RPC.md) |
| Wallet workflows | [docs/WALLET.md](WALLET.md) |
| Roadmap / phases | `ROADMAP.md` (Phase 12 = full documentation) |
| RPC methods (live) | Dashboard **Features** tab or `rpc/dispatch.go` |
| Web static UI | `ui/static/index.html`, `GET /api/docs` |
| Analytics | `analytics/`, `indexer/` |
