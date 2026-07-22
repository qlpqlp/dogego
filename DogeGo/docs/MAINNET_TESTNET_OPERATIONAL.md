# Mainnet + reboot testnet operational guide

Daily-operator checklist for running DogeGo as a **full node** on **mainnet** (public Dogecoin chain) and/or **reboot testnet** (modern test network with solo founder mining).

**Not Dogecoin Core:** use a **dedicated datadir** (`headers.bin` / `headers/`, `rawblocks/`, `indexes/`). Do not point at Core `blocks/` + `chainstate/`.

Commands below work on **Windows, Linux, and macOS** unless marked otherwise. Prefer `dogego cert …` and the dashboard; see [scripts/README.md](../scripts/README.md) for `.sh` / `.ps1` twins.

---

## Quick preflight (CLI)

Single network (from saved config):

```bash
dogego cert operational
dogego cert operational -conf dogecoinconf.mainnet.json -json
dogego cert operational -conf dogecoinconf.testnet.json
```

Dual-run (mainnet + testnet side by side):

```bash
dogego cert operational -dual -datadir /path/to/dogego-data
```

Loopback API (node running):

```text
GET /api/core-operational-probe
```

---

## Choose your layout

| Mode | Config | Dashboard | Use case |
|------|--------|-----------|----------|
| **Mainnet only** | `dogecoinconf.json` with `"network": "mainnet"` | http://localhost:2013/ | Public chain IBD, wallet, explorer |
| **Reboot testnet founder** | `"network": "testnet"`, `"mine": true`, `"node_mode": "full"` | http://localhost:2013/ (or :2014 in dual) | Solo mine, share `addnode` with joiners |
| **Dual mainnet + testnet** | Setup wizard profile **Dual mainnet + reboot testnet** | Mainnet :2013, testnet :2014 | Dev + mainnet watch concurrently |

Dual-run writes:

- `<datadir>/dogecoinconf.mainnet.json`
- `<datadir>/dogecoinconf.testnet.json`
- `<datadir>/instances.json`

Chain data: `<datadir>/mainnet/` and `<datadir>/testnet/`.

---

## Mainnet

### Before first start

1. Fresh datadir (no Core `blocks/` or `chainstate/`).
2. `"network": "mainnet"`, `"node_mode": "full"`.
3. `"webui": "localhost:2013"` and/or `"rpc": "127.0.0.1:22557"`.
4. Wallet on unless relay-only (`nowallet` disables Send/Receive).
5. `dogego cert operational -conf dogecoinconf.mainnet.json`

### After start

| Step | Command / UI |
|------|----------------|
| Watch sync | Overview → Sync; Console `getblockchaininfo`; optional Windows `scripts/watch_sync.ps1` |
| Post-aux / mining sanity | Features probes or `curl http://localhost:2013/api/core-mining-probe`; optional Windows `scripts/upgrade_post_aux_verify.ps1` |
| IBD progress window | `dogego cert ibd-convergence -interval-sec 120` |
| Long soak log | Overview sync dock; optional Windows `scripts/log_ibd_progress.ps1 -DiskOnly` |
| Operator cert (offline) | `dogego cert milestones-bde` |
| Live cert (CI) | `dogego cert weekly-live` on `dogego-live` |

**Normal IBD:** header % can sit near ~8% while bodies lag - headers pass **510000** quickly on current builds. AuxPoW parent headers are validated **inside each block** (no separate Litecoin sync).

See [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) for stall recovery (`bad nBits`, pruned peers, body replay).

---

## Reboot testnet

Modern params: Digishield from height **1**, **10k DOGE** tail subsidy, genesis subsidy **88**, P2P port **44556**, real scrypt PoW (`RelaxedPoW=false`, same as Core). Digishield min-difficulty helps solo mining when hashrate is low.

**Peer discovery:** DogeGo queries DNS seed **`seed.dogego.org` first** (a DogeBox running a public DogeGo reboot-testnet full node, so new installs find peers quickly), then Core fixed seeds from `chainparamsseeds.h`. Extra `"dnsseed"` entries in config are appended after that. See [CHAIN_PARAMETERS.md](CHAIN_PARAMETERS.md).

### Founder checklist

1. Setup wizard: **Reboot testnet founder** or `"network": "testnet"`, `"mine": true`.
2. `"p2p_connectivity": "both"` (or `classic`) if joiners connect inbound.
3. Forward TCP **44556** on the public internet when sharing `addnode`.
4. `dogego cert founder` (or Features → **Founder probe**).
5. Save & start; confirm `getblockchaininfo` `blocks` increases.

### Joiner checklist

1. `"network": "testnet"`, `"node_mode": "full"`.
2. Leave peer discovery automatic (hits **`seed.dogego.org`** then fixed seeds), or set `"addnode": ["FOUNDER_HOST:44556"]` for a private founder.
3. No solo `mine=true` required.

### CI / Core parity (testnet)

```bash
dogego cert setup-parity -mine-bootstrap
dogego cert weekly-live -mine-bootstrap
```

Stateful mempool **24/24** compares with Core on reboot testnet when `core_rpc_addr` is set.

---

## Dual-run operation

1. Setup wizard → profile **Dual mainnet + reboot testnet** → Save & start (starts both processes when configured).
2. `dogego cert operational -dual`
3. Mainnet: sync from public peers (long IBD).
4. Testnet: founder mines locally; optional `addnode` for lab peers.
5. Tray: switch dashboards between :2013 and :2014.

---

## Certification path to “fully operational” (release gate)

Code-ready today (offline):

```bash
go run ./cmd/dogego cert offline
go run ./cmd/dogego cert milestones-bde
go run ./cmd/dogego cert operational -dual
```

Operator-owned live sign-off (see [ROADMAP.md](../ROADMAP.md)):

| Milestone | Close with |
|-----------|------------|
| **B** | `dogego cert live-soak` (multi-hour) |
| **D** | `dogego cert weekly-live` (Core 24/24 on testnet) |
| **E** | `dogego cert workflow10` + seventeen web gates green |

Until those run green on `dogego-live`, [STANDALONE_FULLNODE_ACCEPTANCE.md](STANDALONE_FULLNODE_ACCEPTANCE.md) rows stay **partial** even when daily operation works.

---

## Related docs

| Doc | Topic |
|-----|--------|
| [STANDALONE_NODE_QUICKSTART.md](STANDALONE_NODE_QUICKSTART.md) | Build + first run |
| [CORE_OPERATOR_RUNBOOK.md](CORE_OPERATOR_RUNBOOK.md) | IBD stalls, auxpow, maintenance |
| [OPERATOR.md](OPERATOR.md) | Config reference, founder § |
| [WEB_UI.md](WEB_UI.md) | Dashboard tabs, probes |
| [INTENTIONAL_DIFFERENCES.md](INTENTIONAL_DIFFERENCES.md) | Storage vs Core |
