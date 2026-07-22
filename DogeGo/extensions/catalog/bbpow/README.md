# BBPoW research extension (`dogego.bbpow`)

Experimental DogeGo extension exploring **Bitcoin-Backed Proof-of-Work** (BBPoW / CAuxPoW): letting Dogecoin *research* recognition of Bitcoin SHA-256 work via coinbase commitments.

**This is not AuxPoW.** Classic AuxPoW needs the same PoW algorithm on both chains. Bitcoin is SHA-256; Dogecoin merge-mining today is Scrypt via Litecoin.

**This does not change Dogecoin L1.** Enabling the extension verifies proofs off-chain. Accepting BBPoW blocks on mainnet would be a **Dogecoin hard fork**.

## Networks

Manifest hard-gates **`testnet` only**.

## Install (testnet node)

```powershell
cd DogeGo/extensions/catalog/bbpow
./build.ps1
# then: Settings → Extensions → Install zip → dist/bbpow.zip → Enable
```

```bash
dogego-cli dogego_instextensionzip path/to/bbpow.zip
dogego-cli dogego_enableextension dogego.bbpow
dogego-cli dogego_ext_dogego_bbpow_compare
```

## Docs

- [USER_GUIDE.md](docs/USER_GUIDE.md)
- [PROTOCOL.md](docs/PROTOCOL.md)
