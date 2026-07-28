# Chain parameters & networks

Where Dogecoin **mainnet**, **reboot testnet**, P2P peers, DNS seeders, genesis, checkpoints, and consensus height rules live in DogeGo - and how that compares to Dogecoin Core.

**Short answer:** parameters are **not** in a single file today. They are **mostly under `chain/`**, with consensus eras, assume-valid defaults, and operator overrides in other packages. The main lookup for wire/P2P/genesis is **`chain.ParamsFor()`**.

Core’s canonical spec is **`../src/chainparams.cpp`** (parent repo). When you change chain identity, keep DogeGo and Core in sync or document an intentional fork.

---

## Supported networks

| Config / CLI `network` | `chain.Network` constant | Core equivalent | Datadir subfolder |
|------------------------|--------------------------|---------------|-------------------|
| `mainnet` | `MainnetDogecoin` | `CMainParams` | `mainnet/` |
| `testnet`, `reboottestnet` | `RebootTestnet` | `CTestNetParams` (rebooted) | `testnet/` |

Parsing: `chain/network.go` (`ParseNetwork`).

**Not implemented:** legacy testnet3, regtest (`CRegTestParams`).

Default when unset: **`testnet`** (`config/merge.go`, `config/validate.go`, CLI).

---

## Parameter map (what lives where)

### Wire, P2P, genesis, address prefixes - `chain/`

| File | Contents |
|------|----------|
| **`chain/params.go`** | `Params` struct, **`ParamsFor(net)`** - main entry point for magic, port, genesis fields, DNS/fixed peers, base58 versions, `RelaxedPoW` |
| **`chain/testnet.go`** | Reboot testnet globals (magic `fd d4 dc e1`, port **44556**, genesis time/nonce/bits/hash) |
| **`chain/mainnet_seeds.go`** | Fixed mainnet peers (`MainnetFixedSeedAddrs`) - generated from Core |
| **`chain/testnet_seeds.go`** | Fixed reboot-testnet peers (`TestnetFixedSeedAddrs`) |
| **`chain/_gen_seeds.py`** | Regenerates seed `.go` files from `src/chainparamsseeds.h` |
| **`chain/checkpoints.go`** | Checkpoint height → block hash maps (mainnet + testnet) |
| **`chain/minimum_work.go`** | Mainnet `nMinimumChainWork` |
| **`chain/maxtipage.go`** | Default `-maxtipage` (86400 s) |
| **`chain/genesis_block.go`** | Serialized genesis block + shared coinbase |
| **`chain/params_dns.go`** | `WithDNSSeeds` / `WithoutDNSSeeds` helpers |
| **`chain/network.go`** | Network string parsing |
| **`chain/datadir.go`** | Subdir names per network |
| **`chain/address.go`**, **`chain/wif.go`** | Base58 encoding using `Params` version bytes |

### Quick reference values

| | Mainnet | Reboot testnet |
|---|---------|----------------|
| P2P magic | `c0 c0 c0 c0` | `fd d4 dc e1` |
| P2P port | **22556** | **44556** |
| DNS seeds | `seed.multidoge.org`, `seed2.multidoge.org` | none (Core also empty) |
| Fixed seeds | `mainnet_seeds.go` (~250+) | `testnet_seeds.go` (33) |
| Genesis block hash | `1a91e3dace36…` | `d5d619f8be02…` |
| P2PKH prefix | `D` (0x1e) | `T` (0x41) |
| `RelaxedPoW` | `false` | `false` (real scrypt PoW; same as Core reboot testnet) |

### Height-dependent consensus - `consensus/`

| File | Contents |
|------|----------|
| **`consensus/dogeconsensus.go`** | `LookupConsensus(height)` - Digishield, AuxPoW, BIP34/65/66, CSV, coinbase maturity |
| **`consensus/versionbits.go`** | BIP9 deployments (CSV, segwit disabled) |
| **`consensus/subsidy_params.go`** | Halving interval, post-145k rewards |
| **`consensus/assumevalid.go`** | Default assume-valid block hash per network |
| **`consensus/header_checkpoints.go`** | Checkpoint enforcement toggle |
| **`pow/powlimit.go`** | Shared proof-of-work limit |
| **`pow/genesis.go`** | 80-byte genesis header builder |

### Operator overrides - `config/` + `dogecoinconf.json`

These **select** or **tune** a hardcoded network; they do **not** redefine magic/genesis:

| Field | Effect |
|-------|--------|
| `"network"` | `mainnet` / `testnet` / `reboottestnet` → `ParamsFor` |
| `"dnsseed"` | Extra DNS hostnames merged into `Params.DNSSeeds` |
| `"dnsseed_lookup": false` | Skip built-in DNS discovery |
| `"assumevalid"` | Override default assume-valid hash (`"0"` = verify all) |
| `"checkpoints": false` | Disable checkpoint enforcement |
| `"maxtipage"` | Stale tip threshold (seconds) |
| `"peer"`, `"addnode"` | Manual peers (not chain params) |

Schema: `config/conf.go`. Merge: `config/merge.go`. Wired in `node/run.go`.

### DogeGo-only ports (not P2P)

| | Mainnet | Testnet | Reboot testnet |
|---|---------|---------|----------------|
| JSON-RPC default | `127.0.0.1:22557` | `127.0.0.1:44555` | `127.0.0.1:44556` |
| Web UI default | `127.0.0.1:2013` | same | same |

`config/rpc_defaults.go` - chosen so DogeGo can run beside Core on `:22555` / `:22556`.

### Discovery & dial - `p2p/` + `node/`

| File | Role |
|------|------|
| `p2p/discover.go` | DNS lookup then fixed seeds → candidate `host:port` list |
| `p2p/addrorder.go` | IPv4-first shuffle |
| `node/run.go` | Applies config DNS merge, starts discovery, handshake uses `Params.Magic` / `ProtocolVersion` |

---

## Changing testnet (or mainnet) parameters

### What you can change in one place

For a **new reboot testnet** experiment, start here:

1. **`chain/testnet.go`** - magic, port, genesis time/nonce/bits/hash hex
2. **`chain/params.go`** - `ParamsFor(RebootTestnet)` assembly (address prefixes, `RelaxedPoW`)
3. **`consensus/dogeconsensus.go`** - era heights (Digishield, AuxPoW, BIP heights)
4. **`chain/checkpoints.go`** - at least height 0 checkpoint hash
5. **`consensus/assumevalid.go`** - usually empty for testnet
6. Regenerate or edit **`chain/testnet_seeds.go`** (or run `chain/_gen_seeds.py` after updating Core seeds)

Then run parity tests under `chain/*_test.go` and `consensus/*_test.go`.

### What stays in Core sync

If this repo’s **`src/chainparams.cpp`** is still the spec, update **both** Core and DogeGo, or generate Go from Core (see roadmap below).

### Operator JSON cannot replace genesis

Do not put magic, genesis, or checkpoint tables in `dogecoinconf.json`. Use `"network"` to pick a built-in chain.

---

## Call graph (runtime)

```text
dogecoinconf.json / CLI -network
        │
        ▼
config.Merge ──► chain.ParseNetwork
        │
        ▼
chain.ParamsFor(net) ──► p2p.DiscoverPeers / node handshake
        │
        ├── consensus.LookupConsensus(height)  (validation)
        ├── chain.CheckpointAt(height)         (if checkpoints enabled)
        └── consensus.AssumeValid policy       (IBD fast path)
```

---

## Roadmap: single source of truth

Today only **fixed seeds** are codegen’d (`chain/_gen_seeds.py` ← `chainparamsseeds.h`). Everything else is hand-maintained against `chainparams.cpp`.

**Target for contributors:**

1. One struct per network (e.g. `chain/networks/mainnet.go`, `chain/networks/reboottestnet.go`) holding wire + genesis + checkpoints + min work + assumevalid + consensus era table.
2. CI check that generated Go matches `src/chainparams.cpp` (or a checked-in JSON export).
3. `ParamsFor` returns the full bundle; `consensus.LookupConsensus` becomes a thin wrapper.

Until that lands, use this document as the checklist when editing chain identity.

---

## Related docs

- [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) - full repo map for contributors
- [OPERATOR.md](OPERATOR.md) - `dogecoinconf.json` operator reference
- [ARCHITECTURE.md](ARCHITECTURE.md) - package roles
- Parent: `src/chainparams.cpp`, `src/chainparamsseeds.h`
