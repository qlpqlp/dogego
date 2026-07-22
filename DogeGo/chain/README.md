# `chain/` - network identity

P2P wire params, genesis, DNS/fixed seeds, checkpoints, and base58 address prefixes for **mainnet** and **reboot testnet**.

## Entry points

- **`ParamsFor(net Network) (Params, error)`** in `params.go` - use this from `node/`, `p2p/`, `rpc/`, `ui/`.
- **`ParseNetwork(s string) (Network, error)`** in `network.go` - maps `mainnet` / `testnet` / `reboottestnet`.

## Files

| File | Role |
|------|------|
| `params.go` | `Params` struct + `ParamsFor` |
| `testnet.go` | Reboot testnet constants (magic, port, genesis) |
| `mainnet_seeds.go`, `testnet_seeds.go` | Fixed peers (generated) |
| `_gen_seeds.py` | Regenerate seeds from `src/chainparamsseeds.h` |
| `checkpoints.go` | Height → hash checkpoint maps |
| `minimum_work.go`, `maxtipage.go` | Mainnet chain-work floor, stale tip age |
| `genesis_block.go` | Full genesis block bytes |

Consensus **height rules** (Digishield, AuxPoW, BIP heights) are in **`consensus/dogeconsensus.go`**, not here.

## Full documentation

See **[docs/CHAIN_PARAMETERS.md](../docs/CHAIN_PARAMETERS.md)** and **[docs/DEVELOPER_GUIDE.md](../docs/DEVELOPER_GUIDE.md)**.
