# DogeOS extension for DogeGo

Connect DogeGo to the [DogeOS](https://docs.dogeos.com/en/developers) EVM application layer: live RPC health, metrics, developer helpers, and a small public HTTP API.

DogeOS is **not** Dogecoin L1 and **not** the Doginals L2 overlay. It is an EVM-compatible app network that uses DOGE for gas (~3s blocks on testnet).

## Install

```bash
# From this folder
./build-universal.ps1   # or build-universal.sh on Unix

dogego-cli dogego_instextensionzip path/to/dogeos-universal.zip
dogego-cli dogego_enableextension dogego.dogeos
```

## Default network: Chikyū Testnet

| Field | Value |
|-------|--------|
| Network | DogeOS Chikyū Testnet |
| RPC | https://rpc.testnet.dogeos.com/ |
| Chain ID | 6281971 |
| Symbol | DOGE |
| Explorer | https://blockscout.testnet.dogeos.com |
| Faucet | https://faucet.testnet.dogeos.com/ |

Mainnet is listed in Settings as a placeholder until DogeOS publishes RPC + chain ID. You can always set a **custom RPC URL** for private or future endpoints.

Official quickstart: [Developer Quickstart](https://docs.dogeos.com/en/developers/developer-quickstart).

## UI workspace

After enable, open the DogeOS panel:

- **Home** — tip block, chain ID, gas, RPC URL, network table, docs/faucet/explorer links
- **Metrics** — background probes, 15m uptime %, latency, chain-id match, history table
- **Helpers** — MetaMask `wallet_addEthereumChain`, Hardhat, Foundry, ethers, viem, curl, cast
- **Tools** — balance, contract check, receipt, block, raw RPC
- **Settings** — `chikyu-testnet` / `mainnet`, custom RPC, poll interval, metrics on/off

## RPC methods

Prefix with `dogego_ext_dogego_dogeos_` when calling via dogego-cli.

| Method | Purpose |
|--------|---------|
| `info` | Snapshot + UI |
| `probe` | Immediate health probe |
| `metrics` | History / uptime |
| `helpers` | Snippets JSON |
| `networks` | Profiles |
| `getbalance` / `getcode` / `getreceipt` / `getblock` | Read helpers |
| `rpccall` | Raw JSON-RPC |
| `getconfig` / `setconfig` | Persist settings |

Examples:

```bash
dogego-cli dogego_ext_dogego_dogeos_probe
dogego-cli dogego_ext_dogego_dogeos_helpers
dogego-cli dogego_ext_dogego_dogeos_getbalance '{"address":"0x…"}'
dogego-cli dogego_ext_dogego_dogeos_setconfig '{"network_id":"chikyu-testnet","poll_seconds":15}'
```

## HTTP API

Host proxies `/api/ext/dogego.dogeos/*` to `httphandle`:

| Path | Description |
|------|-------------|
| `GET /api/ext/dogego.dogeos/v1` | Manifest |
| `GET …/v1/status` | Network + last probe |
| `GET …/v1/networks` | Profiles |
| `GET …/v1/helpers` | Snippets |
| `GET …/v1/metrics` | Probe summary |
| `GET …/v1/probe` | Live probe |
| `GET …/v1/balance/{address}` | Balance |
| `GET …/v1/tx/{hash}` | Receipt + explorer |
| `GET …/v1/block?number=latest` | Block |

## Get testnet DOGE

1. Add Chikyū to MetaMask using Helpers → MetaMask params (or the table above).
2. Use the [faucet](https://faucet.testnet.dogeos.com/).
3. Deploy with Hardhat/Foundry pointed at the same RPC (see [EXAMPLES.md](EXAMPLES.md)).

## See also

- [EXAMPLES.md](EXAMPLES.md) — copy-paste tooling
- [Building on DogeOS](https://docs.dogeos.com/en/developers)
- [Contract deployment tutorial](https://docs.dogeos.com/en/developers/guides/contract-deployment-tutorial)
