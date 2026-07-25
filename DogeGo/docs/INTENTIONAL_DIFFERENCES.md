# Intentional differences from Dogecoin Core

When in doubt, **Core is the specification**. DogeGo targets compatibility for common full-node workflows but does not claim binary parity.

## Consensus (locked on mainnet)

DogeGo **must not** change Dogecoin mainnet consensus rules: block acceptance, script verification flags, subsidy schedule, auxpow, difficulty, checkpoints, and BIP9 deployment heights match Core unless documented as a **bug fix** with differential tests.

| Topic | DogeGo |
|-------|--------|
| **Protocol fork** | **None.** No new soft forks, no changed activations on mainnet. |
| **Chain reorg ("fork")** | Header journal truncate + UTXO replay when a competing tip has more work (Core-style election). |
| **Reboot testnet** | Separate network parameters (`network=testnet`); not mainnet. DNS seed **`seed.dogego.org` first** (DogeGo helper on a DogeBox full node for quick discovery; Core ships no DNS seeds for this chain). |
| **PQ commitments** | Optional OP_RETURN tags + off-chain carrier RPCs; **no consensus fork** (see ROADMAP Phase 10). |
| **Witness** | Code present (wire decode, weight/vsize); **disabled** at consensus/mempool like Core (`IsWitnessEnabled` false). See [SEGWIT_STATUS.md](SEGWIT_STATUS.md). |

Implementation gaps below (storage, addrman, RPC subset, …) are **not** protocol changes.

## Storage

| Core | DogeGo |
|------|--------|
| `blocks/blk*.dat` + `chainstate/` LevelDB | `headers.bin`, `rawblocks/` (per-file default or **bundled** `blk*.dat`), `indexes/tx/` (**one file per txid**), in-memory UTXO cache + optional `utxo.cache` |
| Txindex during IBD | Core indexes cheaply in LevelDB on connect. DogeGo **defers** per-txid writes on Put while bodies lag headers (~512+), then indexes on **ConnectBlock** / repair so download is not disk-starved. |
| Same datadir | **Not compatible** - use a dedicated DogeGo datadir; sync via P2P (headers + block bodies). This is **Dogecoin Core's own `blocks/` + `chainstate/` layout**, not Litecoin storage; AuxPoW does not require sharing datadirs with Core or Litecoin. |

Legacy `storage_mode` / `core_datadir` / `auto_import_core` settings are rejected at startup.

## P2P

| Topic | DogeGo behavior |
|-------|-----------------|
| **BIP152 compact blocks** | **Partial** BIP152 v1: `sendcmpct` negotiation (up to 3 high-bandwidth peers), `cmpctblock` announce, inbound reconstruct + `getblocktxn`/`blocktxn` serve, `getdata` `MSG_CMPCT_BLOCK` (falls back to full `block` when cmpct cannot encode, e.g. AuxPoW). AuxPoW relay uses `inv`/`block` including to HB peers. BIP152 v2 (witness) not implemented. |
| **Addrman** | Partial **256 tried + 1024 new hash buckets** with per-bucket slot caps; flat capacity matches Core slot totals (**16384** tried / **65536** new). Explicit `vvTried`/`vvNew` slot indices and multi-ref new buckets still in progress. `learned_addrs.json` v3 + block peer scores. Inbound **`addr`** uses Core-style **token-bucket** rate limits (`addr_rate_limited`); persistent **addnode** peers are whitelisted and seeded **tried**. Successful **inbound handshakes** on routable host:ports are also learned into the **new** table (not tried). Outbound dials **prefer IPv4 before IPv6** (within score order); after an IPv6 dial fails with **network unreachable**, further IPv6 dials are skipped for the process (common on DogeBox/containers without a v6 route). **`getnodeaddresses`** returns addrbook rows. Block delivery refreshes **LastSeen**; all outbound handshakes (relay maintainer, feeler, configured `-peer`, primary redial, block-assist, header probe/recovery, **`addnode onetry`**, fork-election probes) update try/success/failure (persisted to `learned_addrs.json`); hard block/header fetch failures increment addrbook **attempts** (cooldown) as well as block peer scorer penalties. |
| **OS firewall** | Optional **`firewall=auto`** adds inbound P2P + outbound binary rules when missing (Windows / Linux ufw+firewalld / macOS). Core has no equivalent; operators can set **`never`**. |
| **UPnP / NAT-PMP** | **`upnp=auto`** (default when listening): maps the P2P TCP port via IGD2/IGD1 or NAT-PMP; **`getnetworkinfo.localaddresses`** includes the mapped public endpoint (score 8). Set **`upnp=disable`** to skip. Refreshes every 20 minutes (Core-like). |
| **ZMQ** | **`zmqpubhashblock`**, **`zmqpubhashtx`**, **`zmqpubrawblock`**, **`zmqpubrawtx`** in config (Core field names). Pure-Go PUB implementation; same multipart message shape. Wallet/local `sendrawtransaction` paths do not publish until relayed via P2P admission. |
| **Block download stall** | Near tip / non-body-IBD: Core **`BLOCK_STALLING_TIMEOUT` (2s)** on the frontier in-flight height → disconnect + hard peer cooldown. **Deep body IBD** (headers far ahead): **~15s** soft release clears the in-flight claim and brief scorer cooldown **without** disconnecting the peer (avoids churn that collapses blk/min on ancient getdata). **`BLOCK_DOWNLOAD_TIMEOUT_*`** per-lane in-flight window still disconnects when a batch is held too long. |
| **`-dbcache` / IBD focus** | **`dbcache`** (MB; **0 = auto** from free RAM) budgets UTXO RAM before `utxo.cache` flush. **`ibd_optimize`** (default on) raises assist parallelism, lengthens flush intervals, defers analytics until bodies catch up, and prefers download over aggressive post-batch inline connect during deep IBD. |
| **Witness inv/tx** | Rejected at admission; no segwit relay. |
| **Tx relay** | Outbound relay uses **`inv`(MSG_TX)** + peer **`getdata`** (not unsolicited full `tx`); feefilter per peer (BIP133); does not **`inv`** the gossip source peer (wallet/local accept still fans out to other relays). |
| **Block relay** | Inbound relay **`block`** messages are not sent back to the delivering peer (same as tx inv exclusion). |
| **BIP157 compact filters** | Full serve path when tx index is on; advertises **`NODE_COMPACT_FILTERS`** on `version`/`addr`. Encoded filters match Bitcoin Core `blockfilters.json` test vectors (subset); use **`reindexblockfilters`** after encoder changes. Dogecoin Core in this repo has no block-filter RPC. |
| **`getpeerinfo`** | Core-shaped **`bytesrecv_per_msg`** / **`bytessent_per_msg`**, **`addr_processed`** / **`addr_rate_limited`**, **`whitelisted`** for persistent **addnode** peers, **`lastsend`**/**`lastrecv`** from wire activity, **`banscore`** from misbehavior tracker, **`last_block`**/**`last_transaction`** when that peer delivered blocks/txs; block-assist workers remain separate rows without full session stats. Minimal P2P (outbound-only, no listen) uses the same field set on the lone primary row. |
| **`getnetworkinfo`** | **`localaddresses`** lists P2P listen bind + per-connection local IPs (not Core `mapLocalHost` discovery). **`networks[]`** includes onion as unreachable (no Tor integration); **`ipv6.reachable`** flips to false after auto IPv6 dial skip. |
| **`getpeerinfo` sync fields** | **`synced_headers`** / **`synced_blocks`** track per-peer header tip and last stored block height from that peer (approximation of Core `nSyncHeight` / `nCommonHeight`). |
| **`setban`** | Disconnects **relay** sessions on add; **primary** sync peer is not dropped via `disconnectnode` / ban disconnect. |
| **`getconnectioncount`** | Counts **P2P sessions** only (not block-assist IBD TCP workers); dashboard `connections_total` may still include assist for operators. |
| **`ping` RPC** | Queues immediate outbound P2P pings (Core `fPingQueued`); does not measure RPC queue delay separately. |
| **`getnetworkinfo.connections`** | Core node count only; DogeGo exposes assist workers separately via **`dogego_block_assist_connections`**. When **`networkactive`** is false, connection counts and assist fields are zero (relays disconnected; primary may still be up). |
| **`setmaxconnections`** | DogeGo caps outbound sessions at **32** (config `maxoutbound`); Core allows much higher **`nMaxConnections`**. RPC persists **`maxoutbound`** to `dogecoinconf.json`. |
| **P2P RPC errors** | Unwired P2P hooks return Core message with JSON-RPC code **-31** (`RPC_CLIENT_P2P_DISABLED`). Ban/disconnect/addnode filter codes match Core **-23** / **-24** / **-29** / **-30** where applicable. |
| **`disconnectnode`** | Cannot drop the **primary** sync peer (operator must redial or restart); Core allows disconnecting any connected node. |
| **Configured `-peer` / `peer` in conf** | If the peer is **unreachable**, handshake fails, or header sync fails (transport, bad headers), DogeGo **falls back to DNS/fixed seeds**. Primary **redial** tries the configured peer first, then other learned peers. After repeated primary loss during IBD, the node **stays up** (background header recovery + block-assist) instead of exiting. |
| **`getnettotals.uploadtarget`** | DogeGo does not implement Core outbound bandwidth throttling; fields are present with **target** 0 and **serve_historical_blocks** true. |
| **`setnetworkactive`** | Disabling network drops **relay** sessions; **primary** sync TCP stays up until operator restarts or redials. |
| **Permanent `setban`** | `banned_until` **0** in `listbanned` means no expiry (manual remove / `clearbanned`). |

## Consensus / mempool

| Topic | DogeGo behavior |
|-------|-----------------|
| **Legacy subsidy (height &lt; 145000)** | Matches Core `GetDogecoinBlockSubsidy` including Boost `uniform_int` (not plain `%` on MT output). |
| **AuxPoW merged mining (Litecoin parent)** | Each aux-era Dogecoin block carries a **CAuxPow** blob: parent **80-byte header**, parent coinbase tx, and merkle branches. Core validates PoW and merge-mining layout from that embedded data (`CAuxPow::check`, `CheckAuxPowProofOfWork`). **Neither Core nor DogeGo syncs or stores a separate Litecoin chain.** DogeGo: `checkAuxPow` in `consensus/headers_validate.go`, `headers_aux.bin`, `createauxblock` / `submitauxblock`. |
| **AuxPoW `hashBlock` on wire** | Not required to equal parent header hash during `checkAuxPow` (Core `CAuxPow::check` omits that check). |
| **AuxPoW parent chain ID** | Matches Core: reject only when the parent header encodes Dogecoin chain id `0x62`; other parent chain IDs (e.g. Litecoin merge-mining) are allowed. Pre-2026 DogeGo builds that required parent chain id `0` could stall mainnet header sync near height **510000** (~8% UI progress). |
| **SegWit** | Implemented but **disabled** (`IsWitnessEnabled` false; BIP9 `Timeout: 0`), matching Dogecoin Core. Softfork upgrade notes: [SEGWIT_STATUS.md](SEGWIT_STATUS.md). |
| **Script** | P2PKH, P2PK, P2SH (incl. multisig/CLTV/CSV redeems). Mempool standard flags include **NULLDUMMY** (multisig) and **DISCOURAGE_UPGRADABLE_NOPS** (reserved NOPs; CLTV/CSV when not active). **Witness v0/v1+ `scriptPubKey` templates are non-standard** (segwit disabled, like Core). Buried blocks use height flags only. Legacy `script_tests.json` **1059/1059** rows pass; witness rows skipped (segwit disabled). |
| **Deployments** | Buried heights + BIP9 version-bits reporting via `getdeploymentinfo` / `getblockchaininfo`; not every Core deployment statistic. |
| **Header `nTime` checks** | Uses network-adjusted time (median peer offset when multi-peer P2P is up, else primary handshake `nTime`) for “too new” / MTP rules during `headers` sync - aligned with Core **GetTime**, not raw wall clock alone. |
| **PQ commitments (relay)** | Canonical OP_RETURN tags (FLC1/DIL2/RCG4) and Phase-1 PQ carrier P2SH outputs pass **`IsStandardTx`** / mempool admission when size/fee rules hold; verifier-side format/carrier verify via RPC. Wallet **`pq_commitments`** / **`pq_carrier`** flags gate RPC sends; **`dogego cert pq`** covers offline format/carrier tests only. |
| **Mempool** | In-memory pool with RBF, package limits, feefilter aggregate, orphan cap - partial vs full Core policy. |
| **`-assumevalid`** | **Supported** on mainnet (same default hash/height as Core). Skips script verification on buried blocks; last ~20k blocks below tip still checked. `verifychain` always verifies scripts. Not Core’s LevelDB chainstate layout. |
| **`verificationprogress`** | DogeGo RPC/UI: contiguous **block bodies ÷ header tip**. Core: tx-count / time model during IBD. Use dashboard **sync status** + `dogego_sync_eta` for operator ETA. **`dogego_tx_verification_progress`** mirrors Core’s tx curve on mainnet. |
| **`initialblockdownload`** | Uses bodies lag, **`nMinimumChainWork`**, **`-maxtipage`**, then **latches off** (Core `latchToFalse`; will not flip back to true until restart). |
| **Fee priority era** | Core no longer tracks coin-age priority confirmations; `estimatepriority` is always -1 and `estimatesmartpriority` returns `INF_PRIORITY` when min relay applies (DogeGo matches). |

## RPC / wallet

| Topic | DogeGo behavior |
|-------|-----------------|
| **Built-in wallet** | BIP44 HD `wallet.json` on mainnet/testnet; scrypt encryption + Core-style wallet RPC subset; HD keypool auto top-up in JSON (not Core `wallet.dat` keypool file - **`dogego_probewalletdat`** reports Core BDB **`pool_count`**, **`pool_pubkeys`**, **`pool_entries`** index+pubkey with `spends_key_matched`, **`pool_keys_matched`** / **`pool_keys_unmatched`**, **`pool_unmatched_entries`**, **`pool_unmatched_hint`**, **`pool_index_min`/`pool_index_max`**, **`pool_indices_replayed`**; **`wallet/pool_replay.go`** reserves matched HD receive pubkeys on import - stores Core indices in **`hd_keypool_core_index`** (`pool_core_indices_stored`; exposed on **`getwalletinfo`**, **`dogego_probewalletdat`**, **`getaddressinfo`**, **`validateaddress`**, and **`dogego_listwalletaddresses`** when wallet wired); unmatched legacy pool rows still need **`keypoolrefill`** and native import may return **`keypool_refill_size`**); PSBT subset with BIP32 deriv paths + optional **`signer_cmd`** external signer; **`dogego_importwalletdat`** / **`dogego_probewalletdat`** migrate or inspect Core BDB `wallet.dat` (native read incl. **encrypted** wallets via `options.passphrase` - Core `CCrypter` SHA512 master-key derivation + AES-256-CBC - or Core `dumpwallet` fallback). Descriptor import subset (`pkh`, `sh(pkh)`, `sh(multi)`, timelock `sh(cltv/csv …)`, bare `multi` with flag); **`deriveaddresses`** / **`extractdescriptor`** (non-range); **`dumpwallet`** / **`importwallet`** emit/parse **`descriptor=1`** lines; no `wsh()` or deep nested trees. |
| **Mining** | `getblocktemplate` Digishield bits + BIP22 longpoll; `createauxblock` / `submitauxblock` for merge-mining; `generatetoaddress` for local mining. |
| **gettxoutproof** | 80-byte header Merkle proofs; auxpow-sized headers not supported in proofs. |

See [ROADMAP.md](../ROADMAP.md) for the full parity checklist.
