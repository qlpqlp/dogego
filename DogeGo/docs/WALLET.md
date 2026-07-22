# DogeGo built-in wallet guide

DogeGo ships a **built-in HD wallet** (`wallet.json` per network). It is **not** Core `wallet.dat` format, but **`dogego_importwalletdat`** can migrate keys from a Core `wallet.dat` - natively (unencrypted, or encrypted with `options.passphrase`) or via Core `dumpwallet` when `core_rpc_addr` is set.

**External signers:** configure `signer_cmd` (HWI-compatible stdin/stdout JSON) for `enumeratesigners`, `signerdisplayaddress`, and automatic PSBT signing in `walletprocesspsbt`. `getwalletinfo` and `GET /api/wallet` expose **`signer_cmd_configured`** when set (not the command string).

**Web UI:** Send, Receive, History tabs when the wallet is enabled.

**RPC catalog:** Features tab or `help` - wallet methods are listed in [RPC.md](RPC.md).

## Enable / disable

- Default: wallet on for mainnet and testnet unless `-nowallet` or config disables it.
- File: `<datadir>/<network>/wallet.json` (BIP44 coin type **3**).
- Encryption: `encryptwallet`, `walletpassphrase`, `walletlock` - format is DogeGo-specific ([OPERATOR.md](OPERATOR.md)).

## Common workflows

| Goal | RPC / UI | Notes |
|------|----------|--------|
| New receive address | `getnewaddress`, Receive tab, `POST /api/wallet/address/new` | HD keypool; web address book **Generate** button |
| Address labels | `setlabel`, `getaddressesbylabel`, Receive → Address book | Core-shaped `getaddressesbylabel` object keyed by address (`purpose`: `receive` or `send` for change); web inline edit via `POST /api/wallet/address/label` |
| List tracked addresses | `dogego_listwalletaddresses`, `GET /api/wallet/addresses` | Sorted receive → change → watch/cosigner/import |
| Balance | `getbalance`, `getbalances`, Overview | UTXO cache at chainActive |
| Send | `sendtoaddress`, `sendfrom`, `sendmany`, Send tab | Optional fee `options` object; web Send shows **fee hints** on insufficient-fee errors |
| Coin control | Send tab → Advanced, `fundrawtransaction` `inputs` | Web `/api/wallet/utxos` lists all wallet **SpendScripts** (HD receive/change) |
| PQ commitment output | `setwalletflag pq_commitments true` then `sendtoaddress` options `pqcommit` | Phase-1 OP_RETURN tag (`FLC1` / `DIL2` / `RCG4`) + 32-byte commitment; or `createrawtransaction` without wallet. Offline cert: **`dogego cert pq`** (format/carrier only; no production PQ safety claim) |
| PQ carrier send (TX_C/TX_R) | `setwalletflag pq_carrier true` then `dogego_sendpqcarrier` or Send tab carrier mode | Builds/signs/broadcasts linked TX_C + TX_R P2SH pair; requires **`pq_carrier_enabled`**. Web: Settings → Wallet toggle; `POST /api/wallet/send` with **`pq_mode: "carrier"`**. Live probe: **`GET /api/core-pq-probe`** |
| List coins | `listunspent` | `include_unsafe`, `avoid_reuse`, watch-only flags |
| Import WIF | `importprivkey` | Optional rescan |
| Watch address | `importaddress`, `importpubkey` | P2PKH / P2SH with redeem |
| Descriptors | `importdescriptors`, `listdescriptors`, `getdescriptorinfo` | Subset: pkh, sh(pkh/multi), cltv/csv sh |
| Fund raw tx | `fundrawtransaction` | Smart fee, change, lock unspents |
| Sign | `signrawtransactionwithwallet`, `signrawtransaction` | Unlock if encrypted |
| Bump fee | `bumpfee`, `psbtbumpfee` | BIP125; auto from change or PSBT |
| PSBT | `walletcreatefundedpsbt`, `walletprocesspsbt` | BIP32 deriv paths; optional external signer via `signer_cmd` |
| Core migration | `dogego_importwalletdat` | Native BDB (unencrypted or encrypted via `passphrase`); returns pool metadata + **`keypool_hint`** + **`pool_unmatched_hint`** + **`keypool_refill_size`** when pool-only rows remain + **`pool_indices_replayed`** when HD replay succeeds; Core `dumpwallet` fallback; text dump import |
| Inspect before import | `dogego_probewalletdat` | Dry-run: `encrypted`, `encrypted_keys`, `pool_count`, `pool_pubkeys`, `pool_entries` (index+pubkey + `spends_key_matched`), `pool_keys_matched`, `pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `pool_index_min`/`pool_index_max`, `pool_indices_replayed` (false on probe; true after import when matched HD pubkeys replay), `needs_passphrase`, key/watch counts, `can_import` |
| External signer | `enumeratesigners`, `signerdisplayaddress` | Requires `signer_cmd` in config (e.g. HWI `--stdin`) |
| Rescan | `rescan` | SyncUtxo + block scan for scripts |
| Backup | `backupwallet`, `dumpwallet` | Text dump with WIF / descriptors |
| Simulate | `simulaterawtransaction` | Balance delta for raw hex txs |

## UTXO model

Balances and coin selection use the node’s **in-memory UTXO cache** synced to **chainActive**, not a separate wallet chain index. After long IBD or `invalidateblock`, run **`rescan`** if imports lag.

**Solo mining / many coinbases:** the web dashboard reads balance, history, and **Send coin control** from the UTXO cache (`/api/wallet`, `/api/wallet/txs`, `/api/wallet/utxos`) so it stays fast with hundreds of immature coinbases. Coin control matches all HD **SpendScripts**, not only the default address. The background solo miner (`mine=true`) includes mempool txs in each block so sends confirm without manual `generatetoaddress`. JSON-RPC **`getwalletinfo`**, **`getbalance`**, **`listunspent`**, **`listtransactions`**, and **`gettransaction`** use the UTXO-cache fast path; when **`wallet.db`** is indexed through the chain tip (**`dogego_wallet_scan_index_ok`** on **`getwalletinfo`**), **`listtransactions`** reads history from the scan index instead of walking every UTXO. When receive rows exist but the index lags tip, **`dogego_wallet_history_fast_path`** is set (history stays fast; rescan backfills fee/hex). Fresh wallets without scan rows expose **`dogego_wallet_listtransactions_utxo_walk`** until the first rescan builds **`wallet.db`** receive history (slow with many solo-miner coinbases); **`dogego_wallet_listtransactions_scan_pending`** is set while rescan runs before the first receive rows, and the History tab defers heavy **`listtransactions`** until the index is ready (>64 UTXOs). **`dogego_wallet_history_deferred`** and **`dogego_wallet_history_defer_reason`** on **`getwalletinfo`** mirror **`GET /api/wallet/txs`** defer (`ibd_active`, `connect_lag`, `scan_building`). Confirmed send **`fee`** and **`hex`** are stored in **`wallet.db`** at broadcast and block scan (compact tx index). Run **`rescan`** (JSON-RPC) or **`POST /api/wallet/rescan`** (Settings → Wallet) to backfill older history; **`getwalletinfo`** reports **`wallet_index_height`**, **`needs_rescan`**, and **`scanning`** when scan metadata is wired. See [OPERATOR.md](OPERATOR.md) § Solo mining and wallet UI performance. The Features tab wallet probe surfaces `spendable_utxo_count`, index lag, and rescan hints from `getwalletinfo` when RPC is wired.

## PSBT

- Legacy transactions only (no witness in mempool admission).
- HD keys include **BIP32 derivation paths** in PSBT fields (`walletcreatefundedpsbt` / `walletprocesspsbt`).
- When `signer_cmd` is set, `walletprocesspsbt` with `sign=true` also forwards the PSBT to the external signer after local signing (signer failures return RPC errors; HWI subprocess respects a 120s timeout). The wallet probe may complete the PSBT with local keys and set **`hardware_psbt_hint`** noting that HWI `signpsbt` was not exercised.
- **Receive keypool:** `keypoolrefill` fills receive+change **up to** `newsize`. `PeekReceiveAddress` / `getaccountaddress` does not drain the pool; a confirmed **receive** to a still-pooled address removes that index (Core-style) and may top up via the half-target watermark.
- `descriptorprocesspsbt` is an alias for `walletprocesspsbt`.

## External signer config

```json
{
  "signer_cmd": "python hwi.py --chain dogecoin --stdin",
  "core_rpc_addr": "127.0.0.1:22555",
  "core_rpc_user": "rpcuser",
  "core_rpc_password": "rpcpass"
}
```

Unlock the Core wallet before `dogego_importwalletdat` with `via_core_rpc`.

## Core wallet.dat migration

1. **Probe** (no import): `dogego_probewalletdat /path/to/wallet.dat` - returns `is_bdb`, `encrypted`, `key_count`, `encrypted_keys`, `watch_count`, `pool_count`, `pool_pubkeys`, `pool_entries` (with `spends_key_matched` when applicable), `pool_keys_matched`, `pool_keys_unmatched`, `pool_unmatched_entries`, `pool_unmatched_hint`, `pool_index_min`/`pool_index_max`, `pool_indices_replayed`, `needs_passphrase`, `can_import`, `hint`. When a built-in HD wallet is wired, also returns **`hd_keypool_core_index`** and **`pool_core_indices_stored`** from `wallet.json`.
2. **Import** (auto): `dogego_importwalletdat /path/to/wallet.dat` - tries native BDB read first, then Core `dumpwallet` when `core_rpc_addr` is set. Native import may return **`pool_count`**, **`pool_entries`**, **`pool_keys_matched`/`pool_keys_unmatched`**, **`pool_unmatched_entries`**, **`pool_unmatched_hint`**, **`pool_indices_replayed`** (true when `wallet/pool_replay.go` reserves matched HD receive pubkeys, scanning up to 2000 BIP44 indices), **`pool_core_indices_stored`**, **`keypool_refill_size`**, **`keypool_hint`**, and **`pool_index_min`/`pool_index_max`** when Core keypool entries are present; spend keys import via `ckey`/`key` and HD wallets run **`keypoolrefill`** (Core pool indices persist in **`hd_keypool_core_index`**).
3. **Force native:** `dogego_importwalletdat wallet.dat '{"native_bdb":true}'`
4. **Encrypted native:** `dogego_importwalletdat wallet.dat '{"passphrase":"…"}'` - decrypts **`ckey`** and **`walletdescriptorckey`** records with the wallet passphrase (Core `CCrypter` scheme: SHA512 master-key derivation + AES-256-CBC). No Core process required.
5. **Force Core:** `dogego_importwalletdat wallet.dat '{"via_core_rpc":true}'`
6. **Web UI:** Receive tab → **Core wallet.dat** card - path, optional **wallet passphrase** (encrypted files), probe + import (`POST /api/wallet/probe-walletdat`, `POST /api/wallet/import` with `type: "walletdat"`, optional `passphrase`).

Offline certification: `dogego cert wallet-migration` (wallet.dat only) or **`dogego cert wallet-import`** (BIP39/BIP38 + signer + wallet.dat; mirrors `scripts/wallet_import_cert.ps1`). `scripts/wallet_migration_cert.ps1` is included in the import cert bundle. Optional live file probe/import: `-wallet-dat PATH` / `DOGEGO_WALLET_DAT`, `-passphrase` / `DOGEGO_WALLET_DAT_PASSPHRASE`, and `-live-import` against a running DogeGo JSON-RPC node. Weekly dogego-live readiness can require this fixture with `dogego cert weekly -require-wallet-dat` or `DOGEGO_WALLET_DAT_REQUIRED=1`; the weekly gate runs the RPC import path. Offline go tests also build synthetic Core-shaped `wallet.dat` fixtures (`wallet/bdb/fixture.go`, `wallet/corewallet/fixture.go`) for probe → extract → import E2E without Core installed.

### Pool-only rows (`pool_keys_unmatched`)

Core may keep pubkeys in the BDB **pool** before they are issued as spend keys. DogeGo’s native reader imports spend keys from `ckey`/`key` records only. When a pool pubkey has **no** matching spend key in wallet.dat, it appears as **`pool_keys_unmatched`** with **`pool_unmatched_entries`** and **`pool_unmatched_hint`** on probe/import - DogeGo cannot derive the private key from the pool row alone (unlike Core’s full keypool file semantics). Native import may return **`keypool_refill_size`** when pool-only rows remain (tops up HD receive keys via **`keypoolrefill`**). Matched pool rows whose pubkeys map to your HD receive chain may set **`pool_indices_replayed: true`** on import via `wallet/pool_replay.go`. After migration, run **`keypoolrefill`** on an HD wallet to top up fresh receive keys. **`getaddressinfo`** and **`validateaddress`** report **`iskeypool`** for unused HD receive keypool entries and **`hd_keypool_core_index`** when a Core pool index was stored for that address; **`dogego_listwalletaddresses`** and the Receive address book expose the same fields.

## Post-quantum wallet flags

Two optional flags on the built-in wallet (persisted in `wallet.json`, exposed on **`getwalletinfo`** and **`GET /api/wallet`**):

| Flag | RPC | Web UI | Purpose |
|------|-----|--------|---------|
| **`pq_commitments`** | `setwalletflag pq_commitments true/false` | Settings → Wallet; **`POST /api/wallet/flags`** `{ "pq_commitments_enabled": true }` | Attach FLC1/DIL2/RCG4 OP_RETURN commitment on sends (`sendtoaddress` `pqcommit` option) |
| **`pq_carrier`** | `setwalletflag pq_carrier true/false` | Settings → Wallet; **`POST /api/wallet/flags`** `{ "pq_carrier_enabled": true }` | Enable TX_C/TX_R carrier RPCs (`dogego_createpqcarrier`, `dogego_sendpqcarrier`) and Send tab **carrier** mode |

Verifier-side only: **`dogego_verifypqcommitment`**, **`dogego_verifypqcarrier`**, offline **`dogego cert pq`**, live **`GET /api/core-pq-probe`**. History and BlockStep classify OP_RETURN and carrier sends (Quantum filter / PQ chips). **Not consensus-enforced**; no production PQ safety claim.

## Roadmap

Full workflow pages with examples: **ROADMAP.md → Phase 12**. Encrypted `wallet.dat` (legacy **`ckey`** and descriptor-only **`walletdescriptorckey`**) imports natively when you pass `options.passphrase`; `via_core_rpc` remains available as a fallback when Core is running side-by-side.
