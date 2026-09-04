# DogeOS examples

Snippets for the **Chikyū Testnet** defaults shipped with `dogego.dogeos`. If you override RPC in Settings, substitute your URL and chain ID from `helpers`.

Official reference: [Developer Quickstart](https://docs.dogeos.com/en/developers/developer-quickstart).

## MetaMask / wallet_addEthereumChain

```json
{
  "chainId": "0x5fdaf3",
  "chainName": "DogeOS Chikyū Testnet",
  "nativeCurrency": { "name": "Dogecoin", "symbol": "DOGE", "decimals": 18 },
  "rpcUrls": ["https://rpc.testnet.dogeos.com/"],
  "blockExplorerUrls": ["https://blockscout.testnet.dogeos.com"]
}
```

Chain ID decimal: `6281971` (`0x5fdaf3`).

## Hardhat

```ts
const config: HardhatUserConfig = {
  networks: {
    dogeosTestnet: {
      url: "https://rpc.testnet.dogeos.com/",
      chainId: 6281971,
      accounts: process.env.PRIVATE_KEY ? [process.env.PRIVATE_KEY] : [],
    },
  },
};
```

```bash
npx hardhat run scripts/deploy.ts --network dogeosTestnet
```

## Foundry

```toml
# foundry.toml
[rpc_endpoints]
dogeos = "https://rpc.testnet.dogeos.com/"
```

```bash
forge create src/Counter.sol:Counter --rpc-url https://rpc.testnet.dogeos.com/ --private-key $PRIVATE_KEY
cast block-number --rpc-url https://rpc.testnet.dogeos.com/
```

## ethers v6

```js
import { ethers } from "ethers";

const provider = new ethers.JsonRpcProvider("https://rpc.testnet.dogeos.com/", 6281971);
const tip = await provider.getBlockNumber();
console.log("DogeOS tip", tip);
```

## viem

```ts
import { createPublicClient, http, defineChain } from "viem";

const dogeosChikyu = defineChain({
  id: 6281971,
  name: "DogeOS Chikyū Testnet",
  nativeCurrency: { name: "Dogecoin", symbol: "DOGE", decimals: 18 },
  rpcUrls: { default: { http: ["https://rpc.testnet.dogeos.com/"] } },
  blockExplorers: {
    default: { name: "Blockscout", url: "https://blockscout.testnet.dogeos.com" },
  },
});

const client = createPublicClient({ chain: dogeosChikyu, transport: http() });
console.log(await client.getBlockNumber());
```

## curl via public RPC

```bash
curl -s -X POST https://rpc.testnet.dogeos.com/ \
  -H "content-type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]}'
```

## Via DogeGo extension HTTP

With DogeGo HTTP API enabled and `dogego.dogeos` loaded:

```bash
curl -s http://127.0.0.1:22555/api/ext/dogego.dogeos/v1/status
curl -s http://127.0.0.1:22555/api/ext/dogego.dogeos/v1/probe
curl -s http://127.0.0.1:22555/api/ext/dogego.dogeos/v1/helpers
```

(Port may differ; use your node’s RPC/HTTP bind address.)

## Via dogego-cli

```bash
dogego-cli dogego_ext_dogego_dogeos_probe
dogego-cli dogego_ext_dogego_dogeos_rpccall '{"method":"eth_blockNumber","params_json":"[]"}'
```

## Faucet and bridge

- Faucet: https://faucet.testnet.dogeos.com/
- Bridge guide: https://docs.dogeos.com/en/getting-started/user-guide/bridge
- Wallet setup: https://docs.dogeos.com/en/getting-started/user-guide/setup
